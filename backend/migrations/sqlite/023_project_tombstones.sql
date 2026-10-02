ALTER TABLE projects ADD COLUMN deleted_at TEXT;
CREATE INDEX projects_active_owner ON projects(owner_id,deleted_at);
