package geocore

import (
	"fmt"
	"os"

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
