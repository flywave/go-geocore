package geocore

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/flywave/go-segy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportSEGY_AxisLabels 校验数据体按真实 inline/crossline 标签落位，
// 且标签范围被记录（test.segy 是 10×10、编号 0..9 的真实测线）。
func TestImportSEGY_AxisLabels(t *testing.T) {
	if _, err := os.Stat("../go-segy/testdata/test.segy"); os.IsNotExist(err) {
		t.Skip("SEGY test data not found")
	}
	grid, _, err := ImportSEGY("../go-segy/testdata/test.segy")
	require.NoError(t, err)
	require.NotNil(t, grid)

	assert.Equal(t, 10, grid.Dims[0])
	assert.Equal(t, 10, grid.Dims[1])
	assert.Equal(t, int32(0), grid.InlineMin)
	assert.Equal(t, int32(9), grid.InlineMax)
	assert.Equal(t, int32(0), grid.CrosslineMin)
	assert.Equal(t, int32(9), grid.CrosslineMax)
	assert.Greater(t, grid.Spacing[2], 0.0, "采样间隔应被设置")
	assert.Greater(t, grid.TimeMax, 0.0)

	amp := grid.Data["amplitude"]
	require.Len(t, amp, 10*10*grid.Dims[2])

	nonzero := 0
	for _, v := range amp {
		if math.Abs(v) > 1e-9 {
			nonzero++
		}
	}
	assert.Greater(t, nonzero, 0, "振幅体不应全为零")
}

// TestImportSEGY_NonZeroBaseLabels 用编号从 1000/2000 开始的合成 SEG-Y 校验
// label→index 映射：旧实现把原始标签当零基下标，编号不从 0 开始时所有道都会越界丢弃。
func TestImportSEGY_NonZeroBaseLabels(t *testing.T) {
	const (
		nIL, nXL, nS = 3, 3, 4
		ilBase       = 1000
		xlBase       = 2000
	)

	dir := t.TempDir()
	path := filepath.Join(dir, "nonzero.sgy")

	segyFile := &segy.SEGYFile{
		BinaryHeader: segy.BinaryHeader{
			SamplesPerTrace: nS,
			SampleInterval:  2000, // µs → 2 ms
			SampleFormat:    segy.FormatIEEE,
			ByteOrder:       "big",
		},
		ByteOrder: binary.BigEndian,
	}
	for i := 0; i < nIL; i++ {
		for j := 0; j < nXL; j++ {
			var th [240]byte
			bo := binary.BigEndian
			bo.PutUint32(th[188:], uint32(ilBase+i))
			bo.PutUint32(th[192:], uint32(xlBase+j))
			bo.PutUint16(th[114:], uint16(nS))
			bo.PutUint16(th[116:], 2000)
			// CDP 坐标：inline 沿 x、crossline 沿 y（与 SeismicCube.InlineCrosslineToXY 的约定一致），间距 25m
			bo.PutUint32(th[180:], uint32(100000+i*25))
			bo.PutUint32(th[184:], uint32(200000+j*25))

			samples := make([]float32, nS)
			for k := range samples {
				samples[k] = float32(i*nXL*nS + j*nS + k + 1) // 非零、可区分
			}
			segyFile.Traces = append(segyFile.Traces, segy.Trace{
				Header:  segy.TraceHeader{Inline: int32(ilBase + i), Crossline: int32(xlBase + j), Raw: th},
				Samples: samples,
			})
		}
	}

	var buf bytes.Buffer
	require.NoError(t, segy.NewWriter(&buf, segyFile).Write())
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0644))

	grid, _, err := ImportSEGY(path)
	require.NoError(t, err)
	require.NotNil(t, grid)

	assert.Equal(t, [3]int{nIL, nXL, nS}, grid.Dims)
	assert.Equal(t, int32(ilBase), grid.InlineMin)
	assert.Equal(t, int32(ilBase+nIL-1), grid.InlineMax)
	assert.Equal(t, int32(xlBase), grid.CrosslineMin)
	assert.Equal(t, int32(xlBase+nXL-1), grid.CrosslineMax)

	// 采样间隔 µs → ms
	assert.InDelta(t, 2.0, grid.Spacing[2], 1e-9)
	assert.InDelta(t, 6.0, grid.TimeMax, 1e-9)

	// 原点/间距来自 CDP 坐标
	assert.InDelta(t, 100000, grid.Origin[0], 1e-6)
	assert.InDelta(t, 200000, grid.Origin[1], 1e-6)
	assert.InDelta(t, 25, grid.Spacing[0], 1e-6)
	assert.InDelta(t, 25, grid.Spacing[1], 1e-6)

	amp := grid.Data["amplitude"]
	require.Len(t, amp, nIL*nXL*nS)
	for i := 0; i < nIL; i++ {
		for j := 0; j < nXL; j++ {
			for k := 0; k < nS; k++ {
				want := float64(i*nXL*nS + j*nS + k + 1)
				got := amp[(i*nXL+j)*nS+k]
				assert.InDelta(t, want, got, 1e-6, "cell (%d,%d,%d)", i, j, k)
			}
		}
	}
}

// TestImportSEGY_2DLineNoLabels E5 是二维测线，道头没有 inline/crossline；
// 应退化为「每道一条 inline」而不是把 810 道压进一个单元。
func TestImportSEGY_2DLineNoLabels(t *testing.T) {
	if _, err := os.Stat("../go-segy/testdata/E5_MIG_DMO_FINAL.sgy"); os.IsNotExist(err) {
		t.Skip("SEGY test data not found")
	}
	grid, _, err := ImportSEGY("../go-segy/testdata/E5_MIG_DMO_FINAL.sgy")
	require.NoError(t, err)
	require.NotNil(t, grid)
	assert.Equal(t, 810, grid.Dims[0])
	assert.Equal(t, 1, grid.Dims[1])
	assert.Equal(t, 2001, grid.Dims[2])
}
