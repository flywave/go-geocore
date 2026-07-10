-- go-geocore Server Schema
-- Generic storage for geophysical/geological intermediate data.
-- Designed to map 1:1 from go-geocore types and serve go-geology visualization.
--
-- PostgreSQL + PostGIS recommended for spatial indexing and tiling.

-- ============================================================
-- 1. Project — top-level container
-- ============================================================
CREATE TABLE projects (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL DEFAULT '',
    meta        JSONB DEFAULT '{}',
    bbox        BOX2D,               -- computed bounding box of all children
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

-- ============================================================
-- 2. Well — borehole head location and metadata
--    Maps to go-geocore.Well / go-geology.Borehole
-- ============================================================
CREATE TABLE wells (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    well_id     TEXT NOT NULL,        -- business ID (e.g. "BH-01")
    x           DOUBLE PRECISION,
    y           DOUBLE PRECISION,
    elev        DOUBLE PRECISION,     -- wellhead elevation
    depth       DOUBLE PRECISION,     -- total drilled depth
    azimuth     DOUBLE PRECISION,     -- overall azimuth
    inclination DOUBLE PRECISION,     -- overall inclination
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_wells_project ON wells(project_id);
CREATE INDEX idx_wells_geom ON wells USING GIST (st_makepoint(x, y));
CREATE UNIQUE INDEX idx_wells_biz ON wells(project_id, well_id);

-- ============================================================
-- 3. Well Survey — borehole trajectory stations
--    Maps to go-geocore.SurveyPoint / go-geology.TrajectoryPoint
-- ============================================================
CREATE TABLE well_surveys (
    id          BIGSERIAL PRIMARY KEY,
    well_id     BIGINT NOT NULL REFERENCES wells(id) ON DELETE CASCADE,
    md          DOUBLE PRECISION,     -- measured depth
    x           DOUBLE PRECISION,
    y           DOUBLE PRECISION,
    z           DOUBLE PRECISION,
    azimuth     DOUBLE PRECISION,
    inclination DOUBLE PRECISION,
    idx         INT NOT NULL DEFAULT 0  -- sequence order
);
CREATE INDEX idx_surveys_well ON well_surveys(well_id);
CREATE INDEX idx_surveys_md ON well_surveys(well_id, md);

-- ============================================================
-- 4. Well Stratum — stratigraphic/lithological intervals
--    Maps to go-geocore.StratumInterval / go-geology.Stratum
-- ============================================================
CREATE TABLE well_strata (
    id           BIGSERIAL PRIMARY KEY,
    well_id      BIGINT NOT NULL REFERENCES wells(id) ON DELETE CASCADE,
    stratum_id   TEXT NOT NULL DEFAULT '',  -- formation ID (e.g. "S1")
    idx          INT NOT NULL DEFAULT 0,    -- order from surface
    lithology    TEXT NOT NULL DEFAULT '',
    top_md       DOUBLE PRECISION,
    base_md      DOUBLE PRECISION,
    top_elev     DOUBLE PRECISION,
    base_elev    DOUBLE PRECISION,
    thickness    DOUBLE PRECISION
);
CREATE INDEX idx_strata_well ON well_strata(well_id);
CREATE INDEX idx_strata_order ON well_strata(well_id, idx);

-- ============================================================
-- 5. Well Log Curve — curve definition
--    Maps to go-geocore.LogCurve / go-geology.LogCurvePoint
-- ============================================================
CREATE TABLE well_log_curves (
    id          BIGSERIAL PRIMARY KEY,
    well_id     BIGINT NOT NULL REFERENCES wells(id) ON DELETE CASCADE,
    mnemonic    TEXT NOT NULL,         -- curve name (GR, RT, DT, DEN...)
    unit        TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_log_curves_well ON well_log_curves(well_id);
CREATE UNIQUE INDEX idx_log_curves_mnem ON well_log_curves(well_id, mnemonic);

-- ============================================================
-- 6. Well Log Sample — individual measurements
-- ============================================================
CREATE TABLE well_log_samples (
    id          BIGSERIAL PRIMARY KEY,
    curve_id    BIGINT NOT NULL REFERENCES well_log_curves(id) ON DELETE CASCADE,
    depth       DOUBLE PRECISION,
    value       DOUBLE PRECISION
);
CREATE INDEX idx_log_samples_curve ON well_log_samples(curve_id);
CREATE INDEX idx_log_samples_depth ON well_log_samples(curve_id, depth);

-- ============================================================
-- 7. Geometry — unstructured 3D geometry container
--    cell_type: 0=points, 2=lines, 3=triangles, 4=tetrahedra
--    Maps to go-geocore.Geometry / go-geology.TINMesh/FaultStickSet
-- ============================================================
CREATE TABLE geometries (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    geom_type   INT NOT NULL DEFAULT 0,   -- CellType enum
    vertex_count INT NOT NULL DEFAULT 0,
    cell_count  INT NOT NULL DEFAULT 0,
    bbox        BOX3D,
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_geometries_project ON geometries(project_id);
CREATE INDEX idx_geometries_type ON geometries(geom_type);
CREATE INDEX idx_geometries_bbox ON geometries USING GIST (bbox);

-- ============================================================
-- 8. Geometry Vertex — individual vertex positions
-- ============================================================
CREATE TABLE geometry_vertices (
    id          BIGSERIAL PRIMARY KEY,
    geometry_id BIGINT NOT NULL REFERENCES geometries(id) ON DELETE CASCADE,
    idx         INT NOT NULL,          -- vertex index (0-based)
    x           DOUBLE PRECISION,
    y           DOUBLE PRECISION,
    z           DOUBLE PRECISION
);
CREATE INDEX idx_gv_geom ON geometry_vertices(geometry_id);
CREATE INDEX idx_gv_idx ON geometry_vertices(geometry_id, idx);

-- ============================================================
-- 9. Geometry Cell — connectivity (triangles, lines, tetrahedra)
-- ============================================================
CREATE TABLE geometry_cells (
    id          BIGSERIAL PRIMARY KEY,
    geometry_id BIGINT NOT NULL REFERENCES geometries(id) ON DELETE CASCADE,
    idx         INT NOT NULL,          -- cell index (0-based)
    v0          INT NOT NULL,
    v1          INT NOT NULL,
    v2          INT,                   -- NULL for lines
    v3          INT                    -- NULL for triangles, lines; set for tetrahedra
);
CREATE INDEX idx_gc_geom ON geometry_cells(geometry_id);
CREATE INDEX idx_gc_idx ON geometry_cells(geometry_id, idx);

-- ============================================================
-- 10. Geometry Attribute — per-vertex or per-cell attributes
-- ============================================================
CREATE TABLE geometry_attrs (
    id          BIGSERIAL PRIMARY KEY,
    geometry_id BIGINT NOT NULL REFERENCES geometries(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,          -- attribute name (elevation, temperature...)
    idx         INT NOT NULL,           -- vertex/cell index
    value       DOUBLE PRECISION
);
CREATE INDEX idx_ga_geom ON geometry_attrs(geometry_id);
CREATE INDEX idx_ga_name ON geometry_attrs(geometry_id, name, idx);

-- ============================================================
-- 11. Grid — structured 3D regular grid
--      Maps to go-geocore.Grid / go-geology.SeismicCube
-- ============================================================
CREATE TABLE grids (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    ox          DOUBLE PRECISION,      -- origin X
    oy          DOUBLE PRECISION,      -- origin Y
    oz          DOUBLE PRECISION,      -- origin Z
    sx          DOUBLE PRECISION,      -- spacing X
    sy          DOUBLE PRECISION,      -- spacing Y
    sz          DOUBLE PRECISION,      -- spacing Z
    nx          INT NOT NULL,          -- dimension X
    ny          INT NOT NULL,          -- dimension Y
    nz          INT NOT NULL,          -- dimension Z
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_grids_project ON grids(project_id);

-- ============================================================
-- 12. Grid Data Array — named 3D arrays
-- ============================================================
CREATE TABLE grid_data (
    id          BIGSERIAL PRIMARY KEY,
    grid_id     BIGINT NOT NULL REFERENCES grids(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,          -- "amplitude", "velocity", "density"...
    -- store as chunked or indexed array depending on size.
    -- For small grids (<10K cells), store raw JSON array.
    -- For large grids, use a blob/tile approach.
    data        BYTEA,                 -- binary float64 array
    data_size   INT NOT NULL DEFAULT 0 -- number of float64 values
);
CREATE INDEX idx_gd_grid ON grid_data(grid_id);

-- ============================================================
-- 13. Fault Set — fault interpretation group
--      Maps to go-geocore.FaultSet / go-geology.FaultProfile
-- ============================================================
CREATE TABLE fault_sets (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    strike      DOUBLE PRECISION,
    dip         DOUBLE PRECISION,
    throw       DOUBLE PRECISION,
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_fault_sets_project ON fault_sets(project_id);

-- ============================================================
-- 14. Fault Stick — individual interpretation line
-- ============================================================
CREATE TABLE fault_sticks (
    id          BIGSERIAL PRIMARY KEY,
    fault_set_id BIGINT NOT NULL REFERENCES fault_sets(id) ON DELETE CASCADE,
    group_id    TEXT NOT NULL DEFAULT '',  -- stick group ID
    idx         INT NOT NULL               -- point order within stick
);
CREATE INDEX idx_fault_sticks_set ON fault_sticks(fault_set_id);

CREATE TABLE fault_stick_points (
    id          BIGSERIAL PRIMARY KEY,
    stick_id    BIGINT NOT NULL REFERENCES fault_sticks(id) ON DELETE CASCADE,
    idx         INT NOT NULL,
    x           DOUBLE PRECISION,
    y           DOUBLE PRECISION,
    z           DOUBLE PRECISION
);
CREATE INDEX idx_fsp_stick ON fault_stick_points(stick_id);

-- ============================================================
-- 15. Section — 2D vertical cross-section
--      Maps to go-geocore.VerticalSection / go-geology.SectionProfile
-- ============================================================
CREATE TABLE sections (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    start_x     DOUBLE PRECISION,
    start_y     DOUBLE PRECISION,
    end_x       DOUBLE PRECISION,
    end_y       DOUBLE PRECISION,
    well_ids    TEXT[] NOT NULL DEFAULT '{}',  -- well IDs on this section
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_sections_project ON sections(project_id);

-- ============================================================
-- Schema summary
-- ============================================================
-- projects         1:N  wells
-- wells            1:N  well_surveys
-- wells            1:N  well_strata
-- wells            1:N  well_log_curves  1:N  well_log_samples
-- projects         1:N  geometries       1:N  geometry_vertices
--                                       1:N  geometry_cells
--                                       1:N  geometry_attrs
-- projects         1:N  grids            1:N  grid_data
-- projects         1:N  fault_sets       1:N  fault_sticks  1:N  fault_stick_points
-- projects         1:N  sections
