-- go-geocore Server Schema v2
-- Generic storage for geophysical/geological intermediate data.
-- Maps 1:1 from go-geocore types and serves go-geology visualization.
--
-- Design principle: large geometry/array data is stored as binary blobs (meshes, volumes)
-- referenced by ID, not normalized into row-per-vertex tables.
-- PostgreSQL + PostGIS recommended.

-- ============================================================
-- 1. Project — top-level container
-- ============================================================
CREATE TABLE projects (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL DEFAULT '',
    meta        JSONB DEFAULT '{}',
    bbox        BOX3D,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

-- ============================================================
-- 2. Mesh — binary storage for geometry vertices + cells + attrs
--    Stores the complete unstructured geometry as a binary blob.
--    The mesh format is a simple concatenation:
--      [header][float64 vertices][uint32 cells][float64 attrs]
--    or references an external file path.
-- ============================================================
CREATE TABLE meshes (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    geom_type   INT NOT NULL DEFAULT 0,   -- 0=points, 2=lines, 3=triangles, 4=tetra
    vertex_count INT NOT NULL DEFAULT 0,
    cell_count  INT NOT NULL DEFAULT 0,
    data        BYTEA,                    -- packed binary: [vertices][cells][attrs]
    file_path   TEXT,                     -- external file, if data is too large for DB
    bbox        BOX3D,
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_meshes_project ON meshes(project_id);
CREATE INDEX idx_meshes_type ON meshes(geom_type);
CREATE INDEX idx_meshes_bbox ON meshes USING GIST (bbox);

-- ============================================================
-- 3. Well — borehole head location and metadata
--    Maps to go-geocore.Well / go-geology.Borehole
-- ============================================================
CREATE TABLE wells (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    well_id     TEXT NOT NULL,            -- business ID (e.g. "BH-01")
    x           DOUBLE PRECISION,
    y           DOUBLE PRECISION,
    elev        DOUBLE PRECISION,        -- wellhead elevation
    depth       DOUBLE PRECISION,        -- total drilled depth
    azimuth     DOUBLE PRECISION,
    inclination DOUBLE PRECISION,
    mesh_id     BIGINT REFERENCES meshes(id),  -- optional 3D tube mesh for the well
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_wells_project ON wells(project_id);
CREATE INDEX idx_wells_geom ON wells USING GIST (st_makepoint(x, y));
CREATE UNIQUE INDEX idx_wells_biz ON wells(project_id, well_id);

-- ============================================================
-- 4. Well Survey — borehole trajectory stations
--    Maps to go-geocore.SurveyPoint / go-geology.TrajectoryPoint
-- ============================================================
CREATE TABLE well_surveys (
    id          BIGSERIAL PRIMARY KEY,
    well_id     BIGINT NOT NULL REFERENCES wells(id) ON DELETE CASCADE,
    idx         INT NOT NULL DEFAULT 0,   -- sequence order
    md          DOUBLE PRECISION,         -- measured depth
    x           DOUBLE PRECISION,
    y           DOUBLE PRECISION,
    z           DOUBLE PRECISION,
    azimuth     DOUBLE PRECISION,
    inclination DOUBLE PRECISION
);
CREATE INDEX idx_surveys_well ON well_surveys(well_id);
CREATE INDEX idx_surveys_md ON well_surveys(well_id, md);

-- ============================================================
-- 5. Well Stratum — stratigraphic/lithological intervals
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
    thickness    DOUBLE PRECISION,
    props        JSONB DEFAULT '{}'        -- rock properties (RQD, density, etc.)
);
CREATE INDEX idx_strata_well ON well_strata(well_id);
CREATE INDEX idx_strata_order ON well_strata(well_id, idx);

-- ============================================================
-- 6. Well Log Curve — curve definition
--    Maps to go-geocore.LogCurve / go-geology.LogCurvePoint
-- ============================================================
CREATE TABLE well_log_curves (
    id          BIGSERIAL PRIMARY KEY,
    well_id     BIGINT NOT NULL REFERENCES wells(id) ON DELETE CASCADE,
    mnemonic    TEXT NOT NULL,             -- curve name (GR, RT, DT, DEN...)
    unit        TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    data        BYTEA,                    -- packed float64: [depth0,val0, depth1,val1, ...]
    sample_count INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_log_curves_well ON well_log_curves(well_id);
CREATE UNIQUE INDEX idx_log_curves_mnem ON well_log_curves(well_id, mnemonic);

-- ============================================================
-- 7. Grid — structured 3D regular grid
--    Maps to go-geocore.Grid / go-geology.SeismicCube
-- ============================================================
CREATE TABLE grids (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    ox          DOUBLE PRECISION,          -- origin X
    oy          DOUBLE PRECISION,          -- origin Y
    oz          DOUBLE PRECISION,          -- origin Z (time/depth)
    sx          DOUBLE PRECISION,          -- spacing X
    sy          DOUBLE PRECISION,          -- spacing Y
    sz          DOUBLE PRECISION,          -- spacing Z
    nx          INT NOT NULL,              -- dimension X
    ny          INT NOT NULL,              -- dimension Y
    nz          INT NOT NULL,              -- dimension Z
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_grids_project ON grids(project_id);

-- ============================================================
-- 8. Grid Data Array — named 3D arrays stored as blobs
--    Each row = one named attribute volume (amplitude, velocity, etc.)
--    Data is a flat float64 binary array (nx*ny*nz values).
-- ============================================================
CREATE TABLE grid_data (
    id          BIGSERIAL PRIMARY KEY,
    grid_id     BIGINT NOT NULL REFERENCES grids(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,             -- "amplitude", "velocity", "density"...
    data        BYTEA,                    -- float64 array, length = nx*ny*nz
    data_size   INT NOT NULL DEFAULT 0,    -- number of float64 values
    file_path   TEXT                       -- external file for large volumes
);
CREATE INDEX idx_gd_grid ON grid_data(grid_id);
CREATE UNIQUE INDEX idx_gd_name ON grid_data(grid_id, name);

-- ============================================================
-- 9. Fault Set — fault interpretation group
--    Maps to go-geocore.FaultSet / go-geology.FaultProfile
-- ============================================================
CREATE TABLE fault_sets (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    strike      DOUBLE PRECISION,
    dip         DOUBLE PRECISION,
    throw       DOUBLE PRECISION,
    mesh_id     BIGINT REFERENCES meshes(id),  -- triangulated fault surface mesh
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_fault_sets_project ON fault_sets(project_id);

-- ============================================================
-- 10. Fault Stick — individual interpretation line
-- ============================================================
CREATE TABLE fault_sticks (
    id          BIGSERIAL PRIMARY KEY,
    fault_set_id BIGINT NOT NULL REFERENCES fault_sets(id) ON DELETE CASCADE,
    group_id    TEXT NOT NULL DEFAULT '',   -- stick group ID
    mesh_id     BIGINT REFERENCES meshes(id)  -- stick as a line mesh
);
CREATE INDEX idx_fault_sticks_set ON fault_sticks(fault_set_id);

-- ============================================================
-- 11. Section — 2D vertical cross-section
--     Maps to go-geocore.VerticalSection / go-geology.SectionProfile
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
    mesh_id     BIGINT REFERENCES meshes(id),  -- section geometry as a mesh
    meta        JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_sections_project ON sections(project_id);

-- ============================================================
-- Entity relationship summary
-- ============================================================
-- projects ──1:N── meshes        (binary geometry blobs)
-- projects ──1:N── wells ──1:N── well_surveys
--                       1:N── well_strata
--                       1:N── well_log_curves  (data packed in BYTEA)
-- projects ──1:N── grids        1:N── grid_data (data packed in BYTEA)
-- projects ──1:N── fault_sets   1:N── fault_sticks
-- projects ──1:N── sections
--
-- meshes is referenced by: wells(mesh_id), fault_sets(mesh_id),
--                          fault_sticks(mesh_id), sections(mesh_id)
