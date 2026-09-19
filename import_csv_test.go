package geocore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportBoreholeCSV_Elevations 校验岩性 CSV 的 MD 区间被换算为绝对高程，
// 且井口高程来自 collars.z 而不是被 TopElev（旧实现恒为 0）覆盖。
func TestImportBoreholeCSV_Elevations(t *testing.T) {
	wells, err := ImportBoreholeCSV("testdata/collars.csv", "testdata/survey.csv", "testdata/lithology.csv")
	require.NoError(t, err)
	require.Len(t, wells, 3)

	bh1 := wells[0]
	assert.InDelta(t, 50, bh1.Elevation, 1e-9, "井口高程应取自 collars.csv 的 z")
	require.NotEmpty(t, bh1.Strata)

	first := bh1.Strata[0]
	assert.InDelta(t, 0, first.TopMD, 1e-9)
	assert.InDelta(t, 50, first.BaseMD, 1e-9)
	assert.InDelta(t, 50, first.TopElev, 1e-9, "TopElev = z - TopMD")
	assert.InDelta(t, 0, first.BaseElev, 1e-9, "BaseElev = z - BaseMD")
	assert.Equal(t, 0, first.Index)
}
