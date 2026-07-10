package geocore

import (
	"fmt"

	"github.com/flywave/go-segy"
)

func ImportSEGY(path string) (*Grid, *Geometry, error) {
	s, err := segy.Open(path)
	if err != nil {
		return nil, nil, err
	}

	nx := s.InlineCount()
	ny := s.CrosslineCount()
	nz := int(s.BinaryHeader.SamplesPerTrace)

	var grid *Grid
	var geom *Geometry

	if nx > 0 && ny > 0 && nz > 0 {
		grid = &Grid{
			Dims: [3]int{nx, ny, nz},
			Data: make(map[string][]float64),
			Meta: make(map[string]string),
		}
		grid.Meta["name"] = s.FileName()
		grid.Meta["sample_format"] = fmt.Sprintf("%d", s.BinaryHeader.SampleFormat)
		grid.Meta["sample_interval_ms"] = fmt.Sprintf("%d", int(s.BinaryHeader.SampleInterval))

		amp := make([]float64, nx*ny*nz)
		for _, tr := range s.Traces {
			il := int(tr.Header.Inline)
			xl := int(tr.Header.Crossline)
			for k, v := range tr.Samples {
				idx := il*ny*nz + xl*nz + k
				if idx < len(amp) {
					amp[idx] = float64(v)
				}
			}
		}
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
					float64(ti), float64(k * int(s.BinaryHeader.SampleInterval)), float64(tr.Samples[k]),
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
