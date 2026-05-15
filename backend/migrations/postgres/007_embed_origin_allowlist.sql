-- ADR-0006: Embed widget origin allow-list
-- Adds per-project allow-list of integrator origins for the embed widget,
-- plus a feature flag so projects opt-in explicitly.

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS allowed_origins TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS embed_policy_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_projects_embed_policy_enabled
    ON projects(embed_policy_enabled)
    WHERE embed_policy_enabled = TRUE;
