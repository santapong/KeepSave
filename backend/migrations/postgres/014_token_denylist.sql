-- ADR-0021: explicit revocation list for short-lived agent tokens. A token is
-- revoked if its jti appears here OR (handled in the repository) the lease it
-- was minted from is revoked/expired. Rows are pruned once expires_at passes —
-- after that the token is unusable anyway.
CREATE TABLE token_denylist (
    jti TEXT PRIMARY KEY,
    lease_id UUID,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason TEXT
);

-- Prune scans by expiry.
CREATE INDEX idx_token_denylist_expires ON token_denylist (expires_at);
