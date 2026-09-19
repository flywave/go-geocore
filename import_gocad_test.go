package geocore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gocadPLineRel = "../go-gocad/testdata/line.pl"

// TestImportGOCADPLineFaults 真实 GOCAD PLine（断层迹线）→ FaultSet
func TestImportGOCADPLineFaults(t *testing.T) {
	if _, err := os.Stat(gocadPLineRel); os.IsNotExist(err) {
		t.Skip("GOCAD line test data not found")
	}

	fs, err := ImportGOCADPLineFaults(gocadPLineRel, "")
	require.NoError(t, err)
	require.NotNil(t, fs)

	// groupID 缺省取 HEADER 里的 name
	assert.Equal(t, "fault-trace", fs.ID)
	require.Len(t, fs.Sticks, 1)

	stick := fs.Sticks[0]
	assert.Equal(t, "fault-trace", stick.GroupID)
	require.Len(t, stick.Points, 4)
	assert.Equal(t, [3]float64{0, 0, 0}, stick.Points[0])
	assert.Equal(t, [3]float64{10, 5, 20}, stick.Points[1])
	assert.Equal(t, [3]float64{30, 15, 60}, stick.Points[3])

	// 断层几何必须落在 Project 的 FaultSets 上（stratum 只消费 Wells/FaultSets）
	p := NewProject("gocad")
	require.NoError(t, AddGOCADPLineToProject(p, gocadPLineRel, "F1"))
	require.Len(t, p.FaultSets, 1)
	assert.Equal(t, "F1", p.FaultSets[0].ID)
	assert.Len(t, p.FaultSets[0].AllPoints(), 4)
}

// TestAddGOCADPLineToProject_MergesSticks 同一断层的多条 PLine 归入同一个 FaultSet
func TestAddGOCADPLineToProject_MergesSticks(t *testing.T) {
	if _, err := os.Stat(gocadPLineRel); os.IsNotExist(err) {
		t.Skip("GOCAD line test data not found")
	}

	p := NewProject("gocad")
	require.NoError(t, AddGOCADPLineToProject(p, gocadPLineRel, "F1"))
	require.NoError(t, AddGOCADPLineToProject(p, gocadPLineRel, "F1"))

	require.Len(t, p.FaultSets, 1)
	assert.Len(t, p.FaultSets[0].Sticks, 2)
	assert.Len(t, p.FaultSets[0].AllPoints(), 8)
}

// TestImportGOCADPLineFaults_TooFewPoints 单点 PLine 无法构成断层迹线，必须报错
func TestImportGOCADPLineFaults_TooFewPoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "single.pl")
	content := "GOCAD PLine 1.0\nHEADER {\nname: single\n}\nPVRTX 1 0 0 0\nEND\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	_, err := ImportGOCADPLineFaults(path, "F1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "至少需要 2 个")
}

// TestImportGOCADPLineFaults_IDFromFileName 头部没有 name 时用文件名作断层编号
func TestImportGOCADPLineFaults_IDFromFileName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fault_a.pl")
	content := "GOCAD PLine 1.0\nPVRTX 1 0 0 0\nPVRTX 2 10 0 20\nEND\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	fs, err := ImportGOCADPLineFaults(path, "")
	require.NoError(t, err)
	assert.Equal(t, "fault_a", fs.ID)
}
