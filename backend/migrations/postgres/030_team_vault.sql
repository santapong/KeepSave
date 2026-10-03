CREATE TABLE vault_lifecycle (
 secret_id UUID PRIMARY KEY,
 project_id UUID NOT NULL,
 responsible_user_id UUID REFERENCES users(id),
 declared_expires_at TIMESTAMPTZ,
 renewal_at TIMESTAMPTZ,
 provenance TEXT NOT NULL DEFAULT '' CHECK(length(provenance)<=500),
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(secret_id,project_id) REFERENCES vault_entries(secret_id,project_id)
);
CREATE TABLE vault_notifications (
 id UUID PRIMARY KEY,
 user_id UUID NOT NULL REFERENCES users(id),
 project_id UUID NOT NULL,
 secret_id UUID NOT NULL,
 lifecycle_revision BIGINT NOT NULL,
 threshold_days INTEGER NOT NULL CHECK(threshold_days IN (30,7,1)),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 read_at TIMESTAMPTZ,
 UNIQUE(secret_id,lifecycle_revision,user_id,threshold_days),
 FOREIGN KEY(secret_id,project_id) REFERENCES vault_entries(secret_id,project_id)
);
CREATE TABLE audit_exports (
 id UUID PRIMARY KEY,
 project_id UUID NOT NULL REFERENCES projects(id),
 user_id UUID NOT NULL REFERENCES users(id),
 from_time TIMESTAMPTZ NOT NULL,
 to_time TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL DEFAULT now()+INTERVAL '1 hour',
 row_count INTEGER NOT NULL CHECK(row_count BETWEEN 0 AND 10000),
 payload BYTEA NOT NULL CHECK(octet_length(payload)<=20971520)
);
