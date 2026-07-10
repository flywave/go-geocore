package geocore

type CellType int

const (
	CellPoint    CellType = 0
	CellLine     CellType = 2
	CellTriangle CellType = 3
	CellTetra    CellType = 4
)

// Geometry holds any unstructured 3D geometry (points/lines/triangles/tetrahedra).
// Cell type is inferred from Cells column count.
type Geometry struct {
	Vertices [][3]float64
	Cells    [][]uint32
	Attrs    map[string][]float64
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
func (g *Geometry) CellCount() int  { return len(g.Cells) }
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

// Grid represents a structured 3D regular grid (seismic cubes, voxel models).
type Grid struct {
	Origin  [3]float64
	Spacing [3]float64
	Dims    [3]int
	Data    map[string][]float64
	Meta    map[string]string
}

func (g *Grid) CellCount() int { return g.Dims[0] * g.Dims[1] * g.Dims[2] }
func (g *Grid) Index(i, j, k int) int {
	return i + j*g.Dims[0] + k*g.Dims[0]*g.Dims[1]
}

// SurveyPoint represents a single measured depth point along a wellbore trajectory.
type SurveyPoint struct {
	MD          float64 // measured depth
	X, Y, Z     float64 // calculated coordinates
	Azimuth     float64
	Inclination float64
}

// StratumInterval represents a stratigraphic/lithological unit in a borehole.
type StratumInterval struct {
	Name      string
	TopMD     float64 // top measured depth
	BaseMD    float64 // base measured depth
	TopElev   float64 // top elevation
	BaseElev  float64 // base elevation
	Thickness float64
	Lithology string
}

// LogCurve holds a continuous well log curve (GR, RT, DT, DEN, etc.).
type LogCurve struct {
	Mnemonic string
	Unit     string
	Points   []LogSample
}

type LogSample struct {
	Depth float64
	Value float64
}

// Well represents a borehole with trajectory, strata, and log curves.
// Corresponds to subsurface BoreholeSet (Collars + Survey + lithology).
type Well struct {
	ID        string
	Location  [3]float64 // wellhead X, Y, elevation
	Elevation float64
	Surveys   []SurveyPoint
	Strata    []StratumInterval
	Logs      map[string]*LogCurve
	Meta      map[string]string
}

func (w *Well) Curve(mnemonic string) *LogCurve { return w.Logs[mnemonic] }
func (w *Well) Depth() float64 {
	if len(w.Strata) > 0 {
		return w.Strata[len(w.Strata)-1].BaseMD
	}
	return 0
}

// FaultStick represents a single fault interpretation line (stick).
// Multiple sticks with the same GroupID form a FaultStickSet.
type FaultStick struct {
	Points  [][3]float64
	GroupID string
	Meta    map[string]string
}

// FaultStickSet holds a collection of fault sticks forming a fault surface.
// Corresponds to subsurface FaultSticks.
type FaultSet struct {
	ID     string
	Sticks []FaultStick
	Meta   map[string]string
}
func (fs *FaultSet) AllPoints() [][3]float64 {
	var pts [][3]float64
	for _, s := range fs.Sticks {
		pts = append(pts, s.Points...)
	}
	return pts
}

// VerticalSection represents a 2D geological cross-section along a profile line.
type VerticalSection struct {
	Name      string
	StartLine [2]float64
	EndLine   [2]float64
	Wells     []string // well IDs on this section
	Geometry  []Geometry
	Meta      map[string]string
}

// Project is the top-level container for all geophysical/geological data.
type Project struct {
	Name      string
	Geometry  []Geometry
	Grids     []Grid
	Wells     []Well
	FaultSets []FaultSet
	Sections  []VerticalSection
	Meta      map[string]string
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

func (p *Project) AddGeometry(verts [][3]float64, cells [][]uint32, attrs map[string][]float64) *Geometry {
	g := Geometry{Vertices: verts, Cells: cells, Attrs: attrs, Meta: make(map[string]string)}
	if g.Attrs == nil {
		g.Attrs = make(map[string][]float64)
	}
	p.Geometry = append(p.Geometry, g)
	return &p.Geometry[len(p.Geometry)-1]
}

func (p *Project) AddGrid(origin, spacing [3]float64, dims [3]int) *Grid {
	g := Grid{Origin: origin, Spacing: spacing, Dims: dims, Data: make(map[string][]float64), Meta: make(map[string]string)}
	p.Grids = append(p.Grids, g)
	return &p.Grids[len(p.Grids)-1]
}

func (p *Project) AddWell(id string, loc [3]float64) *Well {
	w := Well{ID: id, Location: loc, Logs: make(map[string]*LogCurve), Meta: make(map[string]string)}
	p.Wells = append(p.Wells, w)
	return &p.Wells[len(p.Wells)-1]
}

func (p *Project) AddFaultSet(id string) *FaultSet {
	fs := FaultSet{ID: id, Meta: make(map[string]string)}
	p.FaultSets = append(p.FaultSets, fs)
	return &p.FaultSets[len(p.FaultSets)-1]
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
