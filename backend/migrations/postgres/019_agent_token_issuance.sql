-- Persist the issuing lease and actor so a project cannot revoke another
-- project's token merely by supplying its JTI. Existing tokens remain bounded
-- by lease/parent revocation and expiry; only newly recorded JTIs are addressable.
CREATE TABLE agent_token_issuance (
 jti VARCHAR(255) PRIMARY KEY,
 lease_id UUID NOT NULL REFERENCES secret_leases(id) ON DELETE CASCADE,
 project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX agent_token_issuance_lease ON agent_token_issuance(lease_id);
