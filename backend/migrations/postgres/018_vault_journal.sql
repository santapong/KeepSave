-- Opt-in vault journal. Legacy paths remain unchanged until the project is
-- enrolled through the authorized application service. Never fabricate history.
CREATE TABLE vault_keys (
 id UUID PRIMARY KEY,
 project_id UUID NOT NULL REFERENCES projects(id),
 encrypted_key BYTEA NOT NULL,
 nonce BYTEA NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(id,project_id)
);
CREATE TABLE vault_projects (
 project_id UUID PRIMARY KEY REFERENCES projects(id),
 current_key_id UUID NOT NULL,
 enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 FOREIGN KEY(current_key_id,project_id) REFERENCES vault_keys(id,project_id)
);
CREATE TABLE vault_entries (
 secret_id UUID PRIMARY KEY,
 project_id UUID NOT NULL REFERENCES vault_projects(project_id),
 environment_id UUID NOT NULL REFERENCES environments(id),
 secret_key VARCHAR(255) NOT NULL,
 revision BIGINT NOT NULL CHECK(revision>0),
 key_id UUID NOT NULL,
 deleted BOOLEAN NOT NULL DEFAULT FALSE,
 FOREIGN KEY(key_id,project_id) REFERENCES vault_keys(id,project_id)
);
CREATE UNIQUE INDEX vault_entries_active_name ON vault_entries(environment_id,secret_key) WHERE NOT deleted;
CREATE TABLE vault_revisions (
 secret_id UUID NOT NULL REFERENCES vault_entries(secret_id),
 project_id UUID NOT NULL,
 revision BIGINT NOT NULL CHECK(revision>0),
 key_id UUID NOT NULL,
 encrypted_value BYTEA NOT NULL,
 nonce BYTEA NOT NULL,
 operation VARCHAR(40) NOT NULL,
 actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(secret_id,revision),
 FOREIGN KEY(key_id,project_id) REFERENCES vault_keys(id,project_id)
);
CREATE INDEX vault_revisions_project ON vault_revisions(project_id,secret_id,revision);
CREATE TABLE vault_snapshot_keys (
 snapshot_id UUID PRIMARY KEY REFERENCES secret_snapshots(id),
 project_id UUID NOT NULL,
 key_id UUID NOT NULL,
 FOREIGN KEY(key_id,project_id) REFERENCES vault_keys(id,project_id)
);
