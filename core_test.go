package geocore

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProject(t *testing.T) {
	p := NewProject("test")
	assert.Equal(t, "test", p.Name)
	assert.Empty(t, p.Geometry)
	assert.Empty(t, p.Wells)
}

func TestAddGeometry(t *testing.T) {
	p := NewProject("test")
	g := p.AddGeometry(
		[][3]float64{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		[][]uint32{{0, 1, 2}},
		nil,
	)
	assert.NotNil(t, g)
	assert.Equal(t, 3, g.VertexCount())
	assert.Equal(t, 1, g.CellCount())
	assert.Equal(t, CellTriangle, g.CellType())
}

func TestCellTypeDetection(t *testing.T) {
	tests := []struct {
		cells [][]uint32
		want  CellType
	}{
		{nil, CellPoint},
		{[][]uint32{{0, 1}}, CellLine},
		{[][]uint32{{0, 1, 2}}, CellTriangle},
		{[][]uint32{{0, 1, 2, 3}}, CellTetra},
	}
	for _, tt := range tests {
		g := Geometry{Cells: tt.cells}
		assert.Equal(t, tt.want, g.CellType())
	}
}

func TestBounds(t *testing.T) {
	g := Geometry{
		Vertices: [][3]float64{{0, 0, 0}, {5, 10, 15}, {-2, 3, 8}},
	}
	min, max := g.Bounds()
	assert.InDelta(t, -2, min[0], 0.01)
	assert.InDelta(t, 5, max[0], 0.01)
	assert.InDelta(t, 0, min[1], 0.01)
	assert.InDelta(t, 10, max[1], 0.01)
}

func TestGridIndex(t *testing.T) {
	g := Grid{Dims: [3]int{10, 20, 30}}
	idx := g.Index(1, 2, 3)
	assert.Equal(t, 1+2*10+3*10*20, idx)
}

func TestAddWell(t *testing.T) {
	p := NewProject("wells")
	w := p.AddWell("BH1", [3]float64{100, 200, 50})
	assert.Equal(t, "BH1", w.ID)
	assert.Equal(t, float64(100), w.Location[0])
}

func TestEmptyBounds(t *testing.T) {
	g := Geometry{}
	min, max := g.Bounds()
	assert.Equal(t, [3]float64{}, min)
	assert.Equal(t, [3]float64{}, max)
}

func TestAddGrid(t *testing.T) {
	p := NewProject("grid")
	g := p.AddGrid([3]float64{0, 0, 0}, [3]float64{1, 1, 1}, [3]int{10, 20, 30})
	assert.Equal(t, 6000, g.CellCount())
}

// ─── File import tests ──────────────────────────────────────

func TestImportLAS(t *testing.T) {
	if _, err := os.Stat("../go-las/testdata/sample_well.las"); os.IsNotExist(err) {
		t.Skip("LAS test data not found")
	}
	w, err := ImportLAS("../go-las/testdata/sample_well.las")
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.NotEmpty(t, w.ID)
	assert.NotEmpty(t, w.Logs)
	for _, c := range w.Logs {
		assert.NotNil(t, c)
		assert.Greater(t, len(c.Points), 0)
		assert.NotEmpty(t, c.Mnemonic)
	}
}

func TestImportSEGY(t *testing.T) {
	if _, err := os.Stat("../go-segy/testdata/test.segy"); os.IsNotExist(err) {
		t.Skip("SEGY test data not found")
	}
	grid, geom, err := ImportSEGY("../go-segy/testdata/test.segy")
	require.NoError(t, err)
	assert.NotNil(t, grid)
	assert.NotNil(t, geom)
	assert.Greater(t, grid.Dims[2], 0)
}

func TestImportGOCAD(t *testing.T) {
	if _, err := os.Stat("../go-gocad/testdata/cube.ts"); os.IsNotExist(err) {
		t.Skip("GOCAD test data not found")
	}
	g, err := ImportGOCADTriSurf("../go-gocad/testdata/cube.ts")
	require.NoError(t, err)
	assert.Equal(t, 8, g.VertexCount())
	assert.Equal(t, 12, g.CellCount())
}

func TestImportGOCADPLine(t *testing.T) {
	if _, err := os.Stat("../go-gocad/testdata/line.pl"); os.IsNotExist(err) {
		t.Skip("GOCAD line test data not found")
	}
	g, err := ImportGOCADPLine("../go-gocad/testdata/line.pl")
	require.NoError(t, err)
	assert.Greater(t, g.VertexCount(), 0)
}

func TestImportOMF(t *testing.T) {
	if _, err := os.Stat("../go-omf/testdata/square.omf"); os.IsNotExist(err) {
		t.Skip("OMF test data not found")
	}
	p, err := ImportOMF("../go-omf/testdata/square.omf")
	require.NoError(t, err)
	require.Len(t, p.Geometry, 1)
	assert.Equal(t, CellTriangle, p.Geometry[0].CellType())
	assert.Equal(t, 4, p.Geometry[0].VertexCount())
}

func TestImportOMFMulti(t *testing.T) {
	if _, err := os.Stat("../go-omf/testdata/multi.omf"); os.IsNotExist(err) {
		t.Skip("OMF multi test data not found")
	}
	p, err := ImportOMF("../go-omf/testdata/multi.omf")
	require.NoError(t, err)
	assert.Len(t, p.Geometry, 3)
}

func TestWellCurve(t *testing.T) {
	w := &Well{
		Logs: map[string]*LogCurve{
			"GR": {Mnemonic: "GR", Points: []LogSample{{Depth: 100, Value: 45}}},
		},
	}
	c := w.Curve("GR")
	require.NotNil(t, c)
	assert.InDelta(t, 45, c.Points[0].Value, 0.01)
	assert.Nil(t, w.Curve("NONEXIST"))
}
