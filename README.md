# go-geocore

Go library providing unified data model for geophysical/geological data exchange.

Bridges format-specific readers (go-las, go-segy, go-gocad, go-omf) to a common intermediate representation consumed by go-geology.

## Core types

- Geometry — unstructured mesh (triangles, lines, tetrahedra)
- Grid — structured 3D grid (seismic cubes, voxel models)
- Well — borehole with surveys, logs, and attributes
- Project — top-level container

## Import functions

- ImportLAS(path) → Well
- ImportSEGY(path) → Grid + Geometry
- ImportGOCADTriSurf(path) / ImportGOCADPLine(path) → Geometry
- ImportOMF(path) → Project
