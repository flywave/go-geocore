# Data Mapping: go-geocore Schema → go-geology

## Quick Reference

| go-geocore (DB) | go-geocore (Go) | go-geology | Consumption |
|-----------------|-----------------|-----------|-------------|
| `wells` + `well_surveys` + `well_strata` | `Well` | `Borehole` | **Main input** to `GenerateStratum()`. Strata → GTP model. |
| `well_log_curves` | `LogCurve` | `Borehole.LogCurves` | Seismic-well tie, property assignment. |
| `meshes` (triangles) | `Geometry{Cells: N×3}` | `TINMesh` | Imported structural surfaces, embedded bodies for CSG. |
| `meshes` (lines) | `Geometry{Cells: N×2}` | `FaultStickSet` | Fault interpretation sticks → `ToFaultProfile()`. |
| `meshes` (points) | `Geometry{Cells: nil}` | `[]vec3d.T` | Initial borehole positions, TIN constraints. |
| `grids` + `grid_data` | `Grid` | `SeismicCube` | Seismic constraint via `SeismicBoreholeIntegrator`. |
| `fault_sets` + `fault_sticks` | `FaultSet` | `FaultProfile` | Fault displacement + GTP splitting. |
| `sections` | `VerticalSection` | `SectionProfile` | 2D cross-section extraction. |

## Detailed Consumption Paths

### Path A: Wells → Borehole → GTP Stratum Mesh

```
wells ──────────→ Borehole{X, Y, Elevation, Depth}
well_surveys ───→ Borehole.Trajectory[] (deviated well path)
well_strata ────→ Borehole.Stratums[]:
                     TopElevation/BaseElevation → GTP prism top/base
                     Lithology → Stratum.Lithology → ColorMap
                     props (JSONB) → Stratum.Properties → PropertySet
                            │
                            ▼
                    BuildTriangularPrismModel()
                            │
                            ▼
                    BuildStratumVolumesFromTriangularPrism()
                            │
                            ▼
                    StratumMesh.Layers[] → OBJ export
```

Every well → one Borehole. Strata intervals define layer surfaces across all boreholes
via Delaunay TIN interpolation. Each TIN triangle + each stratum → one GTP (triangular prism).

### Path B: Fault Sticks → FaultProfile → Fault Surface + Stratum Offset

```
meshes (lines, geom_type=2)
       │
       ▼
FaultStickSet{
    Stick{Points: [][3]float64},  ← each stick is one line mesh
    Strike, Dip, Throw             ← estimated from point cloud
}
       │ ToFaultProfile()
       ▼
FaultProfile{
    Points: []FaultPoint,   ← all vertices from all sticks
    Surface: *TINMesh        ← triangulated surface (built by buildSurface())
}
       │
       ├──→ GenerateFaultIntersections(boreholes)
       │         │
       │         ▼
       │    Inserts IsFault stratum markers into each borehole at
       │    the intersection Z of the borehole ray with the fault TIN.
       │
       ├──→ ApplyFaultWithOverlapResolution(bh)
       │         │
       │         ▼
       │    Displaces top/base elevations on hanging wall/foot wall.
       │    Then ResolveAllStrataOverlaps() fixes any overlaps.
       │
       └──→ GTP Pipeline
                 │
                 ▼
            detectFaultSplit(gtp, bhs, ...)
                 │
                 ▼
            Records which GTP voxel vertices are above/below
            the fault plane → FaultSplit{AboveVerts, BelowVerts}.
            splitGTPByFault() splits the prism at the fault plane
            into two separate polyhedra.
```

The full pipeline:
```
FaultStickSet → ToFaultProfile() → FaultProfile{Surface TINMesh}
                                          │
                              (sample.go) │ (gen.go)
                                          ▼
                              GenerateFaultIntersections()
                              ApplyFaultWithOverlapResolution()
                                          │
                                          ▼
                              BuildTriangularPrismModel()
                              detectFaultSplit()
                              splitGTPByFault()
                                          │
                                          ▼
                              StratumMesh.Faults[] → OBJ group
```

### Path C: Seismic Grid → SeismicCube → Kriging Constraint

```
grids + grid_data ──→ SeismicCube{
    InlineCount, CrosslineCount, SampleCount,
    CornerTL/TR/BL/BR (mapped from Grid.Origin + Spacing),
    Data[nx*ny*nz] float64 amplitude
}
       │
       ├──→ SeismicBoreholeIntegrator
       │         │
       │         ▼
       │    ConstrainStratumPrediction(strata, x, y)
       │         │ Uses GetHorizonDepthAt() → bilinear grid interpolation
       │         │ Uses getAmplitudeFactor() → amplitude-weighted confidence
       │         │ ConstrainKrigingPrediction() → kriging*(1-c) + seismic*c
       │         ▼
       │    Adjusted stratum elevations (blended with kriging)
       │
       └──→ CreateVirtualBorehole(x, y, elev)
                 │ Converts horizon depths → stratum list
                 │ Merged into InterpolationBHs at weight c
                 ▼
            Virtual boreholes for denser TIN triangulation
```

### Path D: Triangle Mesh → TINMesh → Structural Surfaces

```
meshes (triangles, geom_type=3) ──→ TINMesh{Vertices, Triangles}
       │
       ├──→ DEM terrain (note: DEM is handled by a separate module,
       │                    not via go-geocore meshes)
       │
       ├──→ FaultProfile.Surface (from pre-triangulated fault surface)
       │         │
       │         ▼
       │    Used in GenerateFaultIntersections() BVH ray-triangle
       │    intersection queries to find fault-borehole crossings.
       │
       ├──→ MeshBody (embedded 3D bodies)
       │         │
       │         ▼
       │    ProcessCollapses() → CSG subtraction on stratum voxels
       │    isVoxelFullyContainedBody() / isVoxelIntersectedBody()
       │
       └──→ Exported as fault surface OBJ groups
```

## About DEM

DEM (Digital Elevation Model) is **not** stored in go-geocore's `meshes` table.
It is handled by a dedicated raster/DEM module via `StratumOptions.DEM *Raster`.
The DEM contributes:
- Ground surface elevation at each borehole location
- `GenerateDEMGridBoreholes()` creates surface-matching virtual boreholes
- `ResampleHeight` option converts all strata to absolute elevation

## About Well Log Curves

`well_log_curves.data` (packed BYTEA) maps to `Borehole.LogCurves map[string][]LogCurvePoint`.
These curves are consumed by `GenerateSampleSeismicData()` for synthetic seismogram
generation: `LogCurves["Vp"]` + `LogCurves["DEN"]` → acoustic impedance → reflection coefficients.
They can also drive rock property interpolation via KrigingManager.

## Notes

- `StratumVolume` / `StratumVoxel` / `CollapsePillar` are **outputs** of the pipeline,
  not inputs. They are generated during modeling.
- `SeismicHorizon` is derived from `SeismicCube` data (peak tracking) or imported separately.
  It can be stored as a `mesh` (triangulated horizon surface), but is not a native type.
- `StratumOptions` is the main configuration struct that assembles all inputs for
  `GenerateStratum()`. The go-geocore → go-geology converter builds this struct.
