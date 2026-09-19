package geocore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flywave/go-gocad"
)

func ImportGOCADTriSurf(path string) (*Geometry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ts, err := gocad.Parse(f)
	if err != nil {
		return nil, err
	}

	verts := make([][3]float64, len(ts.Vertices))
	for i, v := range ts.Vertices {
		verts[i] = [3]float64{v[0], v[1], v[2]}
	}
	cells := make([][]uint32, len(ts.Triangles))
	for i, t := range ts.Triangles {
		cells[i] = []uint32{uint32(t[0]), uint32(t[1]), uint32(t[2])}
	}

	g := &Geometry{
		Vertices: verts,
		Cells:    cells,
		Attrs:    make(map[string][]float64),
		Meta:     make(map[string]string),
	}
	g.Meta["name"] = ts.Name
	g.Meta["format"] = "GOCAD TriSurf"
	if ts.Color != [3]float64{0, 0, 0} {
		g.Meta["color"] = fmt.Sprintf("%f %f %f", ts.Color[0], ts.Color[1], ts.Color[2])
	}

	return g, nil
}

func ImportGOCADPLine(path string) (*Geometry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pl, err := gocad.ParsePLine(f)
	if err != nil {
		return nil, err
	}

	verts := make([][3]float64, len(pl.Vertices))
	for i, v := range pl.Vertices {
		verts[i] = [3]float64{v[0], v[1], v[2]}
	}
	cells := make([][]uint32, len(pl.Vertices)-1)
	for i := 0; i < len(pl.Vertices)-1; i++ {
		cells[i] = []uint32{uint32(i), uint32(i + 1)}
	}

	g := &Geometry{
		Vertices: verts,
		Cells:    cells,
		Attrs:    make(map[string][]float64),
		Meta:     make(map[string]string),
	}
	g.Meta["name"] = pl.Metadata["name"]
	g.Meta["format"] = "GOCAD PLine"

	return g, nil
}

// ImportGOCADPLineFaults 把一条 GOCAD PLine 解释为断层迹线（一根断层棍）。
//
// GOCAD 的 PLine 在解释成果里就是断层迹线/剖面线：顶点顺序即沿断层的走向，
// 因此映射到 FaultSet.Sticks（go-geology 的 FaultStickSet → FaultProfile）。
// groupID 是断层编号，同一断层的多条迹线用同一个 groupID 归入同一个 FaultSet。
func ImportGOCADPLineFaults(path, groupID string) (*FaultSet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pl, err := gocad.ParsePLine(f)
	if err != nil {
		return nil, err
	}
	if len(pl.Vertices) < 2 {
		return nil, fmt.Errorf("GOCAD PLine %q 只有 %d 个顶点，无法构成断层迹线（至少需要 2 个）", path, len(pl.Vertices))
	}

	if groupID == "" {
		groupID = pl.Metadata["name"]
	}
	if groupID == "" {
		groupID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	pts := make([][3]float64, len(pl.Vertices))
	for i, v := range pl.Vertices {
		pts[i] = [3]float64{v[0], v[1], v[2]}
	}

	fs := &FaultSet{
		ID:   groupID,
		Meta: map[string]string{"format": "GOCAD PLine", "source": path},
	}
	fs.Sticks = append(fs.Sticks, FaultStick{
		Points:  pts,
		GroupID: groupID,
		Meta:    map[string]string{"name": pl.Metadata["name"]},
	})
	return fs, nil
}

// AddGOCADPLineToProject 把 GOCAD PLine 作为断层迹线挂到 Project 上；
// 已存在同名 FaultSet 时把迹线追加进去（一个断层面通常由多条 PLine 组成）。
func AddGOCADPLineToProject(p *Project, path, groupID string) error {
	fs, err := ImportGOCADPLineFaults(path, groupID)
	if err != nil {
		return err
	}
	for i := range p.FaultSets {
		if p.FaultSets[i].ID == fs.ID {
			p.FaultSets[i].Sticks = append(p.FaultSets[i].Sticks, fs.Sticks...)
			return nil
		}
	}
	p.FaultSets = append(p.FaultSets, *fs)
	return nil
}
