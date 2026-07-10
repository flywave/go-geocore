# go-geocore

Unified data model for geophysical and geological data exchange. Bridges format-specific readers to a common intermediate representation consumed by `go-geology`.

Design inspiration: [subsurface](https://github.com/SoftwareUnderground/subsurface) (Python DataHub). Unlike subsurface, go-geocore **only** handles geophysical/exploration data (boreholes, seismic, faults). General mesh, GIS, raster, point cloud data are covered by other [flywave](https://github.com/flywave) libraries.

## Architecture

```
┌─ Format Readers ──────┐   ┌─ go-geocore ───────┐   ┌─ Consumer ──────┐
│                        │   │                     │   │                  │
│  go-las ───────────────┤   │                     │   │                  │
│  go-segy ──────────────┤   │  Project             │   │  go-geology      │
│  go-gocad ─────────────┼──→│    ├─ Geometry       │──→│  (geological     │
│  go-omf ───────────────┤   │    ├─ Grid           │   │   modeling)      │
│  CSV (collars/survey/  │   │    ├─ Well           │   │                  │
│       lithology) ──────┤   │    ├─ FaultSet       │   │                  │
│                        │   │    └─ VerticalSection│   │                  │
└────────────────────────┘   └─────────────────────┘   └──────────────────┘
```

## Core Types

### Geometry — Unstructured 3D geometry

Holds vertices and connectivity for points, lines, triangles, and tetrahedra. Cell type is inferred from `Cells` column count.

| Cells shape | Geometry type |
|-------------|---------------|
| `nil` or empty | Point set |
| `N×2` | Lines / polylines |
| `N×3` | Triangle mesh |
| `N×4` | Tetrahedral mesh |

```go
g := Geometry{
    Vertices: [][3]float64{{0,0,0}, {1,0,0}, {1,1,0}, {0,1,0}},
    Cells:    [][]uint32{{0,1,2}, {0,2,3}},
    Attrs:    map[string][]float64{"elevation": {0, 0, 0, 0}},
}
g.CellType() // → CellTriangle
```

### Grid — Structured 3D regular grid

For seismic cubes, voxel models, gravity/magnetic grids, and other regularly sampled volumes.

```go
g := Grid{
    Origin:  [3]float64{0, 0, 0},
    Spacing: [3]float64{25, 25, 4}, // inline, crossline, time (ms)
    Dims:    [3]int{200, 300, 500},
    Data:    map[string][]float64{"amplitude": make([]float64, 200*300*500)},
}
idx := g.Index(il, xl, t) // inline/crossline/time → flat index
```

### Well — Borehole data

Combines wellhead location, survey trajectory, stratigraphic intervals, and continuous log curves. Maps to go-geology: `Borehole`.

```go
w := Well{
    ID:        "BH-01",
    X:         512345.6,
    Y:         4567890.1,
    Elevation: 125.0,
    Depth:     350.0,
    Azimuth:   15.3,  // overall azimuth (straight-hole default)
    Surveys:   []SurveyPoint{
        {MD: 0, Azimuth: 0, Inclination: 0},
        {MD: 100, Azimuth: 15.3, Inclination: 2.1},
    },
    Strata:   []StratumInterval{
        {ID: "S1", Index: 0, TopMD: 0, BaseMD: 5, Lithology: "clay"},
        {ID: "S2", Index: 1, TopMD: 5, BaseMD: 35, Lithology: "sandstone"},
    },
    Logs: map[string]*LogCurve{
        "GR": {Mnemonic: "GR", Unit: "API", Points: []LogSample{
            {Depth: 0, Value: 45.2},
            {Depth: 0.5, Value: 52.1},
        }},
    },
}
w.Curve("GR")    // → *LogCurve
```

### FaultSet — Fault interpretation sticks

Groups of fault stick lines forming fault surfaces.

```go
fs := FaultSet{ID: "F1"}
fs.Sticks = append(fs.Sticks, FaultStick{
    GroupID: "stick_1",
    Points:  [][3]float64{{0,0,100}, {10,0,110}, {20,0,120}},
})
fs.AllPoints() // flattened [][3]float64
```

## Import Functions

| Function | Source | Output | Description |
|----------|--------|--------|-------------|
| `ImportBoreholeCSV(collar, survey, lith)` | CSV files | `[]*Well` | Collars + survey trajectory + lithology intervals |
| `ImportLAS(path)` | `go-las` | `*Well` | Well log curves (GR, RT, DT, DEN, CNL...) |
| `ImportSEGY(path)` | `go-segy` | `*Grid, *Geometry` | Seismic amplitude volume + trace lines |
| `ImportGOCADTriSurf(path)` | `go-gocad` | `*Geometry` | GOCAD triangulated surface (.ts) |
| `ImportGOCADPLine(path)` | `go-gocad` | `*Geometry` | GOCAD polyline (.pl) |
| `ImportOMF(path)` | `go-omf` | `*Project` | OMF HDF5 project (multi-element) |

## Project — Top-level container

Aggregates all data types into a single exportable unit.

```go
p := NewProject("my-area")
p.AddGeometry(verts, cells, nil)       // returns *Geometry
p.AddGrid(origin, spacing, dims)        // returns *Grid
p.AddWell("BH-01", [3]float64{x, y, z}) // returns *Well
p.AddFaultSet("F1")                     // returns *FaultSet

// Import LAS into project
well, _ := ImportLAS("well.las")
p.Wells = append(p.Wells, *well)
```

## Type Mapping to go-geology

| go-geocore | go-geology | Conversion |
|-----------|-----------|------------|
| `Geometry` (triangles) | `TINMesh` | `Vertices → TINMesh.Vertices`, `Cells → TINMesh.Triangles` |
| `Geometry` (points) | `[]vec3d.T` | Direct vertex array mapping |
| `Grid` | `SeismicCube` | `Dims/Spacing → Inline/Crossline/SampleCount` |
| `Well` | `Borehole` | `X,Y,Elevation → Borehole.X,Y,Elevation` |
| `Well.Surveys` | `Borehole.Trajectory` | `MD,X,Y,Z,Azimuth,Inclination → TrajectoryPoint` |
| `Well.Strata[]` | `Borehole.Stratums[]` | `TopMD/BaseMD → Top/Base`, `Lithology → Lithology` |
| `Well.Logs` | `Borehole.LogCurves` | Named log curves by mnemonic |
| `FaultSet` | `FaultProfile` / `FaultStickSet` | `Strike/Dip/Throw → FaultProfile`, sticks → `ToFaultProfile()` |
| `VerticalSection` | `SectionProfile` | Well IDs + geometry along profile line |

## What go-geocore does NOT cover

These data types are handled by dedicated flywave libraries:

| Data type | Library |
|-----------|---------|
| GIS vector (SHP, GeoJSON, GPKG) | [go-vector](https://github.com/flywave/go-vector) |
| General mesh (OBJ, STL, GLB, DXF) | go-obj, go-stl, go-3dasset, go-assimp |
| Point cloud (LAS/LAZ, PCD, PLY) | go-pcd, flywave-pointcloud |
| Raster/DEM (GeoTIFF) | go-cog, flywave-gdal |
| 3D tiles (i3s, 3dtiles) | go-i3s, go-3dtile |
| 2D/3D rendering | flywave-mapnik, flywave-topovis |
| Geological modeling | [go-geology](https://github.com/flywave/go-geology) |

## Related Repositories

- [go-geology](https://github.com/flywave/go-geology) — 3D geological modeling and analysis
- [go-las](https://github.com/flywave/go-las) — LAS well log file reader/writer
- [go-segy](https://github.com/flywave/go-segy) — SEG-Y seismic data reader/writer
- [go-gocad](https://github.com/flywave/go-gocad) — GOCAD TriSurf/PLine reader/writer
- [go-omf](https://github.com/flywave/go-omf) — Open Mining Format HDF5 reader/writer
- [go-hdf5](https://github.com/flywave/hdf5) — Pure Go HDF5 library (fork)
