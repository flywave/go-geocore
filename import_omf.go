package geocore

import (
	"github.com/flywave/go-omf"
)

func ImportOMF(path string) (*Project, error) {
	p, err := omf.Open(path)
	if err != nil {
		return nil, err
	}

	proj := NewProject(p.Name)
	for _, e := range p.Elements {
		verts := make([][3]float64, len(e.Vertices))
		for i, v := range e.Vertices {
			verts[i] = [3]float64{v[0], v[1], v[2]}
		}

		var cells [][]uint32
		switch e.Type {
		case omf.ElementTriSurf:
			for i := 0; i+2 < len(e.Indices); i += 3 {
				cells = append(cells, []uint32{e.Indices[i], e.Indices[i+1], e.Indices[i+2]})
			}
		case omf.ElementPolyLine:
			for i := 0; i+1 < len(e.Indices); i += 2 {
				cells = append(cells, []uint32{e.Indices[i], e.Indices[i+1]})
			}
		case omf.ElementTetraMesh:
			for i := 0; i+3 < len(e.Indices); i += 4 {
				cells = append(cells, []uint32{e.Indices[i], e.Indices[i+1], e.Indices[i+2], e.Indices[i+3]})
			}
		}

		g := proj.AddGeometry(verts, cells, nil)
		g.Meta["omf_name"] = e.Name
		g.Meta["omf_type"] = omfTypeName(e.Type)
	}

	return proj, nil
}

func omfTypeName(t omf.ElementType) string {
	switch t {
	case omf.ElementPointSet:
		return "PointSet"
	case omf.ElementPolyLine:
		return "PolyLine"
	case omf.ElementTriSurf:
		return "TriSurf"
	case omf.ElementTetraMesh:
		return "TetraMesh"
	case omf.ElementVolume:
		return "Volume"
	}
	return "Unknown"
}
