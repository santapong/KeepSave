CREATE TABLE backup_catalog (
 id UUID PRIMARY KEY,
 project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 filename TEXT NOT NULL UNIQUE,
 bundle_sha256 TEXT NOT NULL,
 byte_size BIGINT NOT NULL CHECK(byte_size>0),
 scheduled BOOLEAN NOT NULL,
 source_day DATE,
 state TEXT NOT NULL DEFAULT 'verified' CHECK(state IN ('verified','delete_pending','deleted','retention_failed')),
 verified_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(project_id,source_day)
);
CREATE INDEX backup_catalog_retention ON backup_catalog(project_id,state,scheduled,created_at);
