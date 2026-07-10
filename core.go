package geocore

import (
)

type CellType int

const (
	CellPoint    CellType = 0
	CellLine     CellType = 2
	CellTriangle CellType = 3
	CellTetra    CellType = 4
)

type Geometry struct {
	Vertices [][3]float64
	Cells    [][]uint32
	Attrs    map[string][]float64
	Meta     map[string]string
}

type Grid struct {
	Origin  [3]float64
	Spacing [3]float64
	Dims    [3]int
	Data    map[string][]float64
	Meta    map[string]string
}

type SurveyPoint struct {
	Depth       float64
	X, Y, Z     float64
	Azimuth     float64
	Inclination float64
}

type Well struct {
	ID       string
	Location [3]float64
	Surveys  []SurveyPoint
	Logs     map[string]*LogCurve
	Meta     map[string]string
}

type LogCurve struct {
	Mnemonic string
	Unit     string
	Points   []LogSample
}

type LogSample struct {
	Depth float64
	Value float64
}

type Project struct {
	Name     string
	Geometry []Geometry
	Grids    []Grid
	Wells    []Well
	Meta     map[string]string
}

func (g *Geometry) CellType() CellType {
	if len(g.Cells) == 0 {
		return CellPoint
	}
	cols := 0
	for _, c := range g.Cells {
		if len(c) > cols {
			cols = len(c)
		}
	}
	if cols <= 2 {
		return CellLine
	}
	return CellType(cols)
}

func (g *Geometry) VertexCount() int { return len(g.Vertices) }
func (g *Geometry) CellCount() int   { return len(g.Cells) }

func (g *Geometry) Bounds() (min, max [3]float64) {
	if len(g.Vertices) == 0 {
		return
	}
	min, max = g.Vertices[0], g.Vertices[0]
	for _, v := range g.Vertices[1:] {
		for i := 0; i < 3; i++ {
			if v[i] < min[i] {
				min[i] = v[i]
			}
			if v[i] > max[i] {
				max[i] = v[i]
			}
		}
	}
	return
}

func (g *Grid) CellCount() int {
	return g.Dims[0] * g.Dims[1] * g.Dims[2]
}

func (g *Grid) Index(i, j, k int) int {
	return i + j*g.Dims[0] + k*g.Dims[0]*g.Dims[1]
}

func (w *Well) Curve(mnemonic string) *LogCurve {
	return w.Logs[mnemonic]
}

func (p *Project) AddGeometry(verts [][3]float64, cells [][]uint32, attrs map[string][]float64) *Geometry {
	g := Geometry{
		Vertices: verts,
		Cells:    cells,
		Attrs:    attrs,
		Meta:     make(map[string]string),
	}
	if g.Attrs == nil {
		g.Attrs = make(map[string][]float64)
	}
	p.Geometry = append(p.Geometry, g)
	return &p.Geometry[len(p.Geometry)-1]
}

func (p *Project) AddGrid(origin, spacing [3]float64, dims [3]int) *Grid {
	g := Grid{
		Origin:  origin,
		Spacing: spacing,
		Dims:    dims,
		Data:    make(map[string][]float64),
		Meta:    make(map[string]string),
	}
	p.Grids = append(p.Grids, g)
	return &p.Grids[len(p.Grids)-1]
}

func (p *Project) AddWell(id string, loc [3]float64) *Well {
	w := Well{
		ID:       id,
		Location: loc,
		Logs:     make(map[string]*LogCurve),
		Meta:     make(map[string]string),
	}
	p.Wells = append(p.Wells, w)
	return &p.Wells[len(p.Wells)-1]
}

func NewProject(name string) *Project {
	return &Project{
		Name:     name,
		Geometry: make([]Geometry, 0),
		Grids:    make([]Grid, 0),
		Wells:    make([]Well, 0),
		Meta:     make(map[string]string),
	}
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
