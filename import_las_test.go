package geocore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportLASProject_CollarAndStrata 真实 LAS 文件 → Project：
// 井口坐标/高程来自头部 X/Y/ELEV，地层由 LITH 编码曲线分段得到。
func TestImportLASProject_CollarAndStrata(t *testing.T) {
	p, err := ImportLASProject("testdata/borehole.las")
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Len(t, p.Wells, 1)

	w := p.Wells[0]
	// 井号取 UWI（比带空格的 WELL 名更适合做 ID）
	assert.Equal(t, "42-999-00001-00-00", w.ID)
	assert.InDelta(t, 551234.567, w.X, 0.001)
	assert.InDelta(t, 6234567.890, w.Y, 0.001)
	assert.InDelta(t, 125.0, w.Elevation, 0.001)
	assert.Equal(t, "header", w.Meta["collar_source"])
	assert.Equal(t, "LITH", w.Meta["lithology_curve"])

	// 曲线
	require.Contains(t, w.Logs, "GR")
	assert.Len(t, w.Logs["GR"].Points, 31)
	assert.InDelta(t, 0, w.Logs["GR"].Points[0].Depth, 0.001)

	// 地层：LITH 编码 1/2/3/2 的四段，顶底界面来自真实采样深度
	require.Len(t, w.Strata, 4)
	want := []struct {
		lith      string
		top, base float64
		thickness float64
		topElev   float64
	}{
		{"1", 0, 9, 9, 125},
		{"2", 9, 18, 9, 116},
		{"3", 18, 26, 8, 107},
		{"2", 26, 31, 5, 99},
	}
	for i, exp := range want {
		s := w.Strata[i]
		assert.Equal(t, i, s.Index, "地层按深度排序，Index 从浅到深")
		assert.Equal(t, exp.lith, s.Lithology)
		assert.InDelta(t, exp.top, s.TopMD, 0.001)
		assert.InDelta(t, exp.base, s.BaseMD, 0.001)
		assert.InDelta(t, exp.thickness, s.Thickness, 0.001)
		assert.InDelta(t, exp.topElev, s.TopElev, 0.001)
	}
	// 井深 = 最深地层底界
	assert.InDelta(t, 31, w.Depth, 0.001)
}

// TestImportLASProject_NoCollarFailsLoudly 头部没有井口坐标时必须报错，不臆造坐标
func TestImportLASProject_NoCollarFailsLoudly(t *testing.T) {
	_, err := ImportLASProject("testdata/no_collar.las")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "缺少井口坐标")
	assert.Contains(t, err.Error(), "HasCollar")
}

// TestImportLASProject_PartialCollarFailsLoudly 只有 X 没有 Y 也算坐标缺失
func TestImportLASProject_PartialCollarFailsLoudly(t *testing.T) {
	src, err := os.ReadFile("testdata/borehole.las")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "x_only.las")
	var kept []string
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "Y.") {
			continue
		}
		kept = append(kept, line)
	}
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0644))

	_, err = ImportLASProject(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "缺少井口坐标")
}

// TestImportLASProject_CollarOverride 调用方显式给坐标时可用（LAS 头部无坐标的常见情形）
func TestImportLASProject_CollarOverride(t *testing.T) {
	p, err := ImportLASProjectWithConfig("testdata/no_collar.las", &LASProjectConfig{
		HasCollar: true, X: 1000, Y: 2000, Elevation: 50,
		WellID: "BH-X",
	})
	require.NoError(t, err)
	require.Len(t, p.Wells, 1)

	w := p.Wells[0]
	assert.Equal(t, "BH-X", w.ID)
	assert.InDelta(t, 1000, w.X, 0.001)
	assert.InDelta(t, 2000, w.Y, 0.001)
	assert.InDelta(t, 50, w.Elevation, 0.001)
	assert.Equal(t, "caller", w.Meta["collar_source"])
	// 岩性曲线仍在，地层按覆盖后的井口高程换算
	require.Len(t, w.Strata, 4)
	assert.InDelta(t, 50, w.Strata[0].TopElev, 0.001)
}

// TestImportLASProject_NoLithologyCurve 没有岩性曲线时只产出井口与曲线，
// 不静默造层（地层留给调用方另行提供）
func TestImportLASProject_NoLithologyCurve(t *testing.T) {
	const sample = "../go-las/testdata/sample_well.las"
	if _, err := os.Stat(sample); os.IsNotExist(err) {
		t.Skip("LAS test data not found")
	}

	p, err := ImportLASProject(sample)
	require.NoError(t, err)
	require.Len(t, p.Wells, 1)

	w := p.Wells[0]
	assert.InDelta(t, 551234.567, w.X, 0.001)
	assert.InDelta(t, 6234567.890, w.Y, 0.001)
	assert.InDelta(t, 125.0, w.Elevation, 0.001)
	assert.Empty(t, w.Strata)
	assert.NotEmpty(t, w.Logs)
	assert.Equal(t, "42-123-12345-00-00", w.ID)
}

// TestImportLAS_CollarNotStartDepth 回归：井口高程不再取自 STRT（起始深度），
// 否则井口会被埋到地下
func TestImportLAS_CollarNotStartDepth(t *testing.T) {
	const sample = "../go-las/testdata/sample_well.las"
	if _, err := os.Stat(sample); os.IsNotExist(err) {
		t.Skip("LAS test data not found")
	}

	w, err := ImportLAS(sample)
	require.NoError(t, err)
	assert.InDelta(t, 125.0, w.Elevation, 0.001)
	assert.NotEqual(t, 1670.0, w.Elevation)
}

// TestStrataFromLithologyCurve_DescendingDepth 深度递减的曲线（LAS 里很常见，
// STEP 为负）也要产出从浅到深、厚度为正的地层
func TestStrataFromLithologyCurve_DescendingDepth(t *testing.T) {
	curve := &LogCurve{
		Mnemonic: "LITH",
		Points: []LogSample{
			{Depth: 30, Value: 2},
			{Depth: 29, Value: 2},
			{Depth: 28, Value: 3},
			{Depth: 27, Value: 3},
			{Depth: 26, Value: 3},
			{Depth: 25, Value: 1},
		},
	}
	strata := strataFromLithologyCurve(curve, 100, map[float64]string{1: "clay", 2: "sandstone", 3: "shale"})
	require.Len(t, strata, 3)

	assert.Equal(t, "clay", strata[0].Lithology)
	assert.InDelta(t, 25, strata[0].TopMD, 0.001)
	assert.InDelta(t, 26, strata[0].BaseMD, 0.001)

	assert.Equal(t, "shale", strata[1].Lithology)
	assert.InDelta(t, 26, strata[1].TopMD, 0.001)
	assert.InDelta(t, 29, strata[1].BaseMD, 0.001)

	assert.Equal(t, "sandstone", strata[2].Lithology)
	assert.InDelta(t, 29, strata[2].TopMD, 0.001)
	assert.InDelta(t, 31, strata[2].BaseMD, 0.001)

	for i, s := range strata {
		assert.Equal(t, i, s.Index)
		assert.Greater(t, s.Thickness, 0.0)
		assert.Greater(t, s.TopElev, s.BaseElev)
	}
}

// TestLASHeaderFields 头部扫描：井信息段与参数段的助记符都要拿到
func TestLASHeaderFields(t *testing.T) {
	fields, err := lasHeaderFields("testdata/borehole.las")
	require.NoError(t, err)
	assert.Equal(t, "551234.567", fields["X"])
	assert.Equal(t, "6234567.890", fields["Y"])
	assert.Contains(t, fields["ELEV"], "125.000")
	assert.Equal(t, "42-999-00001-00-00", fields["UWI"])

	// 数据段不能被当成头部字段
	assert.NotContains(t, fields, "0.0000")
}

// TestImportLASProject_WellIDFallback 没有 UWI/API 时用 WELL 名，再退到文件名
func TestImportLASProject_WellIDFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fallback_well.las")
	src, err := os.ReadFile("testdata/borehole.las")
	require.NoError(t, err)
	content := strings.Replace(string(src), "UWI .     42-999-00001-00-00:           UNIQUE WELL ID\n", "", 1)
	require.NotEqual(t, string(src), content, "UWI 行必须被移除，否则用例没有意义")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	p, err := ImportLASProject(path)
	require.NoError(t, err)
	assert.Equal(t, "TEST BOREHOLE 01", p.Wells[0].ID)
}
