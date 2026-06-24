-- ADR-0021: explicit revocation list for short-lived agent tokens. See
-- postgres/014 for rationale.
CREATE TABLE token_denylist (
    jti TEXT PRIMARY KEY,
    lease_id TEXT,
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reason TEXT
);

CREATE INDEX idx_token_denylist_expires ON token_denylist (expires_at);
