-- New identity platform remains opt-in and PostgreSQL-only. No authority enters
-- encrypted vault recovery bundles. Existing identities are never email-merged.
ALTER TABLE session_tokens ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'unknown'
 CHECK(auth_method IN ('unknown','password','google','github'));
CREATE TABLE identity_verified_contacts (
 user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 contact VARCHAR(254) NOT NULL,
 verified_at TIMESTAMPTZ NOT NULL,
 revoked_at TIMESTAMPTZ,
 PRIMARY KEY(user_id,contact)
);
CREATE UNIQUE INDEX identity_active_contact ON identity_verified_contacts(contact) WHERE revoked_at IS NULL;
CREATE TABLE identity_proofs (
 id UUID PRIMARY KEY,
 user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 session_id UUID REFERENCES session_tokens(id) ON DELETE RESTRICT,
 purpose TEXT NOT NULL CHECK(purpose IN ('contact_verify','password_reset','invitation')),
 contact VARCHAR(254) NOT NULL,
 proof_hash CHAR(64) NOT NULL UNIQUE,
 expires_at TIMESTAMPTZ NOT NULL,
 consumed BOOLEAN NOT NULL DEFAULT FALSE,
 consumed_at TIMESTAMPTZ,
 revoked BOOLEAN NOT NULL DEFAULT FALSE,
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX identity_proof_budget ON identity_proofs(user_id,purpose,created_at);
CREATE TABLE identity_proof_deliveries (
 id UUID PRIMARY KEY,
 proof_id UUID NOT NULL UNIQUE REFERENCES identity_proofs(id) ON DELETE RESTRICT,
 ciphertext BYTEA NOT NULL,
 nonce BYTEA NOT NULL,
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','dispatched','sent','failed','uncertain','cancelled')),
 reason_code TEXT NOT NULL DEFAULT '',
 dispatched_at TIMESTAMPTZ,
 completed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE identity_invitations (
 id UUID PRIMARY KEY REFERENCES identity_proofs(id) ON DELETE RESTRICT,
 organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
 inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 target_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
 intended_role TEXT NOT NULL CHECK(intended_role IN ('viewer','editor','promoter','admin')),
 accepted_by UUID REFERENCES users(id) ON DELETE RESTRICT,
 accepted_at TIMESTAMPTZ,
 revoked BOOLEAN NOT NULL DEFAULT FALSE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX identity_invitation_scope ON identity_invitations(organization_id,inviter_id,created_at);
CREATE TABLE identity_offboarding_receipts (
 id UUID PRIMARY KEY,
 organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
 actor_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 target_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 request_key VARCHAR(128) NOT NULL,
 request_digest CHAR(64) NOT NULL,
 authority_epoch BIGINT NOT NULL,
 summary JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(organization_id,actor_id,request_key)
);
