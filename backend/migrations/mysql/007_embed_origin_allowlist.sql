-- ADR-0006: Embed widget origin allow-list (MySQL)
-- Adds per-project allow-list of integrator origins for the embed widget,
-- plus a feature flag so projects opt-in explicitly.
-- Idempotency is enforced by the schema_migrations table; this file is
-- only ever executed once per database.

ALTER TABLE projects ADD COLUMN allowed_origins JSON;

ALTER TABLE projects ADD COLUMN embed_policy_enabled BOOLEAN NOT NULL DEFAULT 0;

UPDATE projects SET allowed_origins = JSON_ARRAY() WHERE allowed_origins IS NULL;
