package geocore

import (
	"fmt"
	"sort"

	"github.com/flywave/go-segy"
)

// ImportSEGY 读取 SEG-Y 文件，转换为结构化网格（振幅体）与线集几何。
//
// 网格轴的标签范围（inline/crossline 编号）来自道头，**不假设从 0 或 1 开始**：
// 数据体按 label→index 映射落位，编号从 1000 开始的文件同样能完整读入。
// Grid.Dims = {inlineCount, crosslineCount, sampleCount}，Data["amplitude"] 的
// 扁平顺序与 geology.SeismicCube 一致：((inlineIdx*crosslineCount)+crosslineIdx)*sampleCount+sample。
func ImportSEGY(path string) (*Grid, *Geometry, error) {
	s, err := segy.Open(path)
	if err != nil {
		return nil, nil, err
	}

	nz := int(s.BinaryHeader.SamplesPerTrace)
	// SEG-Y 道头采样间隔单位是微秒；go-geology 的 SeismicCube.SampleInterval 用毫秒
	sampleIntervalMS := float64(s.BinaryHeader.SampleInterval) / 1000.0
	if sampleIntervalMS <= 0 {
		sampleIntervalMS = 1.0
	}

	// 收集真实的 inline/crossline 标签（升序），建立 label→index 映射
	ilLabels, xlLabels, ilIndex, xlIndex := collectAxisLabels(s.Traces)
	nx, ny := len(ilLabels), len(xlLabels)

	// 二维测线（道头没有 inline/crossline）时退化为「每道一条 inline」的窄数据体，
	// 避免所有道落进同一个 (0,0) 单元互相覆盖、只留下最后一道。
	if nx <= 1 && ny <= 1 && len(s.Traces) > 1 {
		nx = len(s.Traces)
		ny = 1
		ilLabels = make([]int32, nx)
		xlLabels = []int32{0}
		ilIndex = make(map[int32]int, nx)
		xlIndex = map[int32]int{0: 0}
		for i := range ilLabels {
			ilLabels[i] = int32(i)
			ilIndex[int32(i)] = i
		}
	}

	var grid *Grid
	var geom *Geometry

	if nx > 0 && ny > 0 && nz > 0 {
		origin, spacing := surveyGeometry(s.Traces, ilLabels, xlLabels, ilIndex, xlIndex, sampleIntervalMS)

		grid = &Grid{
			Origin:  origin,
			Spacing: spacing,
			Dims:    [3]int{nx, ny, nz},
			Data:    make(map[string][]float64),
			Meta:    make(map[string]string),
		}
		grid.InlineMin = ilLabels[0]
		grid.InlineMax = ilLabels[len(ilLabels)-1]
		grid.CrosslineMin = xlLabels[0]
		grid.CrosslineMax = xlLabels[len(xlLabels)-1]
		grid.TimeMin = 0
		grid.TimeMax = float64(nz-1) * sampleIntervalMS

		grid.Meta["name"] = s.FileName()
		grid.Meta["sample_format"] = fmt.Sprintf("%d", s.BinaryHeader.SampleFormat)
		grid.Meta["sample_interval_ms"] = fmt.Sprintf("%g", sampleIntervalMS)

		amp := make([]float64, nx*ny*nz)
		dim := 0 // 单道实际采样点数，可能小于道头声明
		for ti, tr := range s.Traces {
			ilIdx, xlIdx := 0, 0
			if nx == len(s.Traces) && ny == 1 {
				ilIdx = ti
			} else {
				ilIdx = ilIndex[tr.Header.Inline]
				xlIdx = xlIndex[tr.Header.Crossline]
			}
			for k, v := range tr.Samples {
				if k >= nz {
					break
				}
				amp[(ilIdx*ny+xlIdx)*nz+k] = float64(v)
				if k+1 > dim {
					dim = k + 1
				}
			}
		}
		grid.Meta["samples_per_trace"] = fmt.Sprintf("%d", dim)
		grid.Data["amplitude"] = amp
	}

	// Add traces as LineSet geometry
	if len(s.Traces) > 0 {
		verts := make([][3]float64, len(s.Traces)*nz)
		cells := make([][]uint32, len(s.Traces)*(nz-1))
		ci := 0
		for ti, tr := range s.Traces {
			base := ti * nz
			for k := 0; k < nz; k++ {
				verts[base+k] = [3]float64{
					float64(ti), float64(k) * sampleIntervalMS, float64(tr.Samples[k]),
				}
			}
			for k := 0; k < nz-1; k++ {
				cells[ci] = []uint32{uint32(base + k), uint32(base + k + 1)}
				ci++
			}
		}
		geom = &Geometry{
			Vertices: verts,
			Cells:    cells,
			Attrs:    make(map[string][]float64),
			Meta:     make(map[string]string),
		}
		geom.Meta["name"] = s.FileName()
		geom.Meta["traces"] = fmt.Sprintf("%d", len(s.Traces))
	}

	return grid, geom, nil
}

// collectAxisLabels 汇总所有道的 inline/crossline 标签并建立 label→index 映射
func collectAxisLabels(traces []segy.Trace) ([]int32, []int32, map[int32]int, map[int32]int) {
	ilSet := make(map[int32]bool)
	xlSet := make(map[int32]bool)
	for _, tr := range traces {
		ilSet[tr.Header.Inline] = true
		xlSet[tr.Header.Crossline] = true
	}
	ilLabels := sortedInt32Keys(ilSet)
	xlLabels := sortedInt32Keys(xlSet)
	return ilLabels, xlLabels, indexMap(ilLabels), indexMap(xlLabels)
}

func sortedInt32Keys(set map[int32]bool) []int32 {
	out := make([]int32, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func indexMap(labels []int32) map[int32]int {
	m := make(map[int32]int, len(labels))
	for i, v := range labels {
		m[v] = i
	}
	return m
}

// traceXY 取道的平面坐标：优先 CDP X/Y，其次源点坐标，再次检波点坐标。
// 全部为零时返回 ok=false（例如二维测线没有写入坐标）。
func traceXY(tr segy.Trace) (float64, float64, bool) {
	if tr.Header.CDPX != 0 || tr.Header.CDPY != 0 {
		return float64(tr.Header.CDPX), float64(tr.Header.CDPY), true
	}
	if tr.Header.SourceX != 0 || tr.Header.SourceY != 0 {
		return tr.Header.SourceX, tr.Header.SourceY, true
	}
	if tr.Header.GroupX != 0 || tr.Header.GroupY != 0 {
		return tr.Header.GroupX, tr.Header.GroupY, true
	}
	return 0, 0, false
}

// surveyGeometry 推导网格原点与间距。
// 有真实道坐标时用坐标范围/道数；没有坐标时退回标签步长（此时几何是标签空间，
// 不能直接与钻孔做空间相关，调用方应通过 SRS 或角点显式纠正）。
func surveyGeometry(traces []segy.Trace, ilLabels, xlLabels []int32,
	ilIndex, xlIndex map[int32]int, sampleIntervalMS float64) ([3]float64, [3]float64) {
	nx, ny := len(ilLabels), len(xlLabels)

	minX, minY := 0.0, 0.0
	maxX, maxY := 0.0, 0.0
	haveXY := false
	for _, tr := range traces {
		x, y, ok := traceXY(tr)
		if !ok {
			continue
		}
		if !haveXY {
			minX, maxX, minY, maxY = x, x, y, y
			haveXY = true
			continue
		}
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	var sx, sy float64
	if haveXY {
		sx = spanStep(maxX-minX, nx)
		sy = spanStep(maxY-minY, ny)
	} else {
		sx = labelStep(ilLabels)
		sy = labelStep(xlLabels)
	}

	return [3]float64{minX, minY, 0}, [3]float64{sx, sy, sampleIntervalMS}
}

func spanStep(span float64, count int) float64 {
	if count > 1 && span > 0 {
		return span / float64(count-1)
	}
	return 1
}

func labelStep(labels []int32) float64 {
	if len(labels) < 2 {
		return 1
	}
	total := 0
	for i := 1; i < len(labels); i++ {
		total += int(labels[i] - labels[i-1])
	}
	step := float64(total) / float64(len(labels)-1)
	if step <= 0 {
		return 1
	}
	return step
}
