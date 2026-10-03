-- A membership epoch cannot detect a newly minted grant. An actor-bound,
-- short-lived preview additionally pins the metadata-only affected inventory.
CREATE TABLE identity_offboarding_previews (
 id UUID PRIMARY KEY,
 organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
 actor_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 target_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 authority_epoch BIGINT NOT NULL CHECK(authority_epoch>0),
 impact_digest CHAR(64) NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 consumed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX identity_offboarding_preview_expiry ON identity_offboarding_previews(expires_at);
