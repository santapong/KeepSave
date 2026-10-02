ALTER TABLE projects ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX projects_active_owner ON projects(owner_id,deleted_at);
-- Historical audit identities are immutable values, not mutable live links.
ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_user_id_fkey;
ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_project_id_fkey;
