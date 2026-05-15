-- ADR-0006: Embed widget origin allow-list (SQLite)
-- Adds per-project allow-list of integrator origins for the embed widget,
-- plus a feature flag so projects opt-in explicitly.
-- Idempotency is enforced by the schema_migrations table; this file is
-- only ever executed once per database. SQLite supports IF NOT EXISTS
-- on ALTER TABLE ADD COLUMN since 3.35.5.

ALTER TABLE projects ADD COLUMN allowed_origins TEXT NOT NULL DEFAULT '[]';

ALTER TABLE projects ADD COLUMN embed_policy_enabled INTEGER NOT NULL DEFAULT 0;
