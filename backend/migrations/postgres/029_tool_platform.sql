-- Default-off PostgreSQL tool platform. Result spools and authority are excluded
-- from vault recovery. New versions append; artifact contents cannot be edited.
CREATE TABLE tool_artifacts (
 id UUID PRIMARY KEY,project_id UUID NOT NULL REFERENCES projects(id),name VARCHAR(64) NOT NULL,
 digest CHAR(64) NOT NULL,source TEXT NOT NULL,created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),revoked_at TIMESTAMPTZ,
 UNIQUE(id,project_id),UNIQUE(project_id,digest)
);
CREATE TABLE tool_profiles (
 id UUID PRIMARY KEY,project_id UUID NOT NULL REFERENCES projects(id),artifact_id UUID NOT NULL,
 digest CHAR(64) NOT NULL,manifest JSONB NOT NULL,created_by UUID NOT NULL REFERENCES users(id),
 approved_by UUID REFERENCES users(id),approver_epoch BIGINT,approved_at TIMESTAMPTZ,approved_until TIMESTAMPTZ,revoked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),UNIQUE(id,project_id),
 CHECK(approved_by IS NULL OR approved_by<>created_by),
 FOREIGN KEY(artifact_id,project_id) REFERENCES tool_artifacts(id,project_id)
);
CREATE TABLE tool_profile_packages (
 id UUID PRIMARY KEY,profile_id UUID NOT NULL REFERENCES tool_profiles(id),harness VARCHAR(32) NOT NULL,
 version VARCHAR(64) NOT NULL,format VARCHAR(64) NOT NULL,digest CHAR(64) NOT NULL,
 content JSONB NOT NULL,compatibility JSONB NOT NULL,created_by UUID NOT NULL REFERENCES users(id),
 approved_by UUID REFERENCES users(id),approver_epoch BIGINT,approved_at TIMESTAMPTZ,approved_until TIMESTAMPTZ,revoked_at TIMESTAMPTZ,
 CHECK(approved_by IS NULL OR approved_by<>created_by),UNIQUE(profile_id,harness,version,digest)
);
CREATE TABLE tool_connections (
 id UUID PRIMARY KEY,project_id UUID NOT NULL REFERENCES projects(id),app_id BIGINT NOT NULL CHECK(app_id>0),
 installation_id BIGINT NOT NULL CHECK(installation_id>0),ciphertext BYTEA NOT NULL,nonce BYTEA NOT NULL,
 created_by UUID NOT NULL REFERENCES users(id),created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),revoked_at TIMESTAMPTZ,
 UNIQUE(id,project_id)
);
CREATE TABLE tool_bindings (
 id UUID PRIMARY KEY,project_id UUID NOT NULL REFERENCES projects(id),connection_id UUID NOT NULL,
 environment_id UUID NOT NULL REFERENCES environments(id),repository_id BIGINT NOT NULL CHECK(repository_id>0),owner_name VARCHAR(100) NOT NULL,repository_name VARCHAR(100) NOT NULL,
 reference VARCHAR(200) NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),revoked_at TIMESTAMPTZ,
 UNIQUE(id,project_id),FOREIGN KEY(connection_id,project_id) REFERENCES tool_connections(id,project_id)
);
CREATE TABLE tool_workloads (
 id UUID PRIMARY KEY,project_id UUID NOT NULL REFERENCES projects(id),certificate_sha256 CHAR(64) NOT NULL UNIQUE,
 image_digest VARCHAR(255) NOT NULL,created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),revoked_at TIMESTAMPTZ,UNIQUE(id,project_id)
);
CREATE TABLE tool_grants (
 id UUID PRIMARY KEY,project_id UUID NOT NULL REFERENCES projects(id),profile_id UUID NOT NULL,
 package_id UUID NOT NULL REFERENCES tool_profile_packages(id),profile_digest CHAR(64) NOT NULL,package_digest CHAR(64) NOT NULL,
 binding_id UUID NOT NULL,workload_id UUID NOT NULL,actor_id UUID NOT NULL REFERENCES users(id),
 client_id VARCHAR(128) NOT NULL REFERENCES mcp_oauth_clients(client_id),issued_by UUID NOT NULL REFERENCES users(id),
 membership_epoch BIGINT NOT NULL,issuer_epoch BIGINT NOT NULL,expires_at TIMESTAMPTZ NOT NULL,revoked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),UNIQUE(id,project_id),
 FOREIGN KEY(profile_id,project_id) REFERENCES tool_profiles(id,project_id),
 FOREIGN KEY(binding_id,project_id) REFERENCES tool_bindings(id,project_id),
 FOREIGN KEY(workload_id,project_id) REFERENCES tool_workloads(id,project_id)
);
CREATE TABLE tool_runs (
 id UUID PRIMARY KEY,project_id UUID NOT NULL REFERENCES projects(id),grant_id UUID NOT NULL,
 actor_id UUID NOT NULL REFERENCES users(id),session_id UUID NOT NULL REFERENCES session_tokens(id),client_id VARCHAR(128) NOT NULL,
 parent_kind VARCHAR(32) NOT NULL CHECK(parent_kind IN ('human','oauth_delegation')),
 parent_family UUID REFERENCES mcp_oauth_families(id),parent_token UUID REFERENCES mcp_oauth_tokens(id),
 authority_expires_at TIMESTAMPTZ NOT NULL,state VARCHAR(16) NOT NULL DEFAULT 'preparing' CHECK(state IN('preparing','active','failed','cancelled','revoked')),
 commit_sha CHAR(40),tree_sha CHAR(40),reference VARCHAR(200) NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 provider_operations INT NOT NULL DEFAULT 0 CHECK(provider_operations>=0 AND provider_operations<=100),
 result_bytes BIGINT NOT NULL DEFAULT 0 CHECK(result_bytes>=0 AND result_bytes<=33554432),
 request_key VARCHAR(128) NOT NULL,request_digest CHAR(64) NOT NULL,
 UNIQUE(actor_id,client_id,request_key),FOREIGN KEY(grant_id,project_id) REFERENCES tool_grants(id,project_id)
);
CREATE TABLE tool_resolution_attempts (
 id UUID PRIMARY KEY,run_id UUID NOT NULL REFERENCES tool_runs(id),state VARCHAR(16) NOT NULL CHECK(state IN('admitted','dispatched','succeeded','failed','uncertain')),
 deadline TIMESTAMPTZ NOT NULL,outcome VARCHAR(64) NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),finished_at TIMESTAMPTZ
);
CREATE TABLE tool_operations (
 id UUID PRIMARY KEY,run_id UUID NOT NULL REFERENCES tool_runs(id),kind VARCHAR(32) NOT NULL CHECK(kind IN('repository_tree','read_file')),
 arguments JSONB NOT NULL,request_key VARCHAR(128) NOT NULL,request_digest CHAR(64) NOT NULL,
 parent_kind VARCHAR(32) NOT NULL CHECK(parent_kind IN('human','oauth_delegation')),parent_family UUID REFERENCES mcp_oauth_families(id),
 parent_token UUID REFERENCES mcp_oauth_tokens(id),authority_expires_at TIMESTAMPTZ NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'queued' CHECK(status IN('queued','leased','dispatched','succeeded','failed','uncertain','cancelled')),
 outcome VARCHAR(64) NOT NULL DEFAULT '',cancel_requested BOOLEAN NOT NULL DEFAULT FALSE,
 fence BIGINT NOT NULL DEFAULT 0,workload_id UUID REFERENCES tool_workloads(id),lease_until TIMESTAMPTZ,
 deadline TIMESTAMPTZ,expires_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(run_id,request_key)
);
CREATE INDEX tool_operations_pending ON tool_operations(status,created_at);
CREATE TABLE tool_tickets (
 id UUID PRIMARY KEY,operation_id UUID NOT NULL REFERENCES tool_operations(id),workload_id UUID NOT NULL REFERENCES tool_workloads(id),
 token_hash CHAR(64) NOT NULL UNIQUE,fence BIGINT NOT NULL,request_digest CHAR(64) NOT NULL,image_digest VARCHAR(255) NOT NULL,
 nonce CHAR(64) NOT NULL,expires_at TIMESTAMPTZ NOT NULL,consumed_at TIMESTAMPTZ
);
CREATE TABLE tool_attempts (
 operation_id UUID NOT NULL REFERENCES tool_operations(id),fence BIGINT NOT NULL,ticket_id UUID NOT NULL REFERENCES tool_tickets(id),
 workload_id UUID NOT NULL REFERENCES tool_workloads(id),state VARCHAR(16) NOT NULL CHECK(state IN('leased','dispatched','succeeded','failed','uncertain','cancelled')),
 deadline TIMESTAMPTZ NOT NULL,outcome VARCHAR(64) NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),finished_at TIMESTAMPTZ,
 PRIMARY KEY(operation_id,fence)
);
CREATE TABLE tool_receipts (
 id UUID PRIMARY KEY,run_id UUID NOT NULL REFERENCES tool_runs(id),operation_id UUID REFERENCES tool_operations(id),
 action VARCHAR(64) NOT NULL,outcome VARCHAR(64) NOT NULL,details JSONB NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE tool_result_spool (
 operation_id UUID PRIMARY KEY REFERENCES tool_operations(id),ciphertext BYTEA NOT NULL,nonce BYTEA NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX tool_result_spool_expiry ON tool_result_spool(expires_at);
CREATE FUNCTION keepsave_tool_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (to_jsonb(NEW)-ARRAY['revoked_at','approved_by','approved_at','approved_until','approver_epoch']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['revoked_at','approved_by','approved_at','approved_until','approver_epoch']) THEN RAISE EXCEPTION 'immutable tool version'; END IF;
 IF to_jsonb(OLD)->>'approved_at' IS NOT NULL AND (to_jsonb(NEW)-'revoked_at') IS DISTINCT FROM (to_jsonb(OLD)-'revoked_at') THEN RAISE EXCEPTION 'immutable approval'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER tool_artifact_immutable BEFORE UPDATE ON tool_artifacts FOR EACH ROW EXECUTE FUNCTION keepsave_tool_immutable();
CREATE TRIGGER tool_profile_immutable BEFORE UPDATE ON tool_profiles FOR EACH ROW EXECUTE FUNCTION keepsave_tool_immutable();
CREATE TRIGGER tool_package_immutable BEFORE UPDATE ON tool_profile_packages FOR EACH ROW EXECUTE FUNCTION keepsave_tool_immutable();
-- Explicit diagnostics have durable, non-replayable provider outcomes too.
CREATE TABLE tool_connection_checks (
 id UUID PRIMARY KEY,project_id UUID NOT NULL REFERENCES projects(id),connection_id UUID NOT NULL REFERENCES tool_connections(id),binding_id UUID NOT NULL REFERENCES tool_bindings(id),actor_id UUID NOT NULL REFERENCES users(id),session_id UUID NOT NULL REFERENCES session_tokens(id),
 deadline TIMESTAMPTZ NOT NULL,state VARCHAR(16) NOT NULL DEFAULT 'admitted' CHECK(state IN('admitted','dispatched','succeeded','failed','uncertain')),
 provider_operations INT NOT NULL DEFAULT 0 CHECK(provider_operations BETWEEN 0 AND 3),created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),finished_at TIMESTAMPTZ
);
