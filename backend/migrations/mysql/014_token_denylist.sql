-- ADR-0021: explicit revocation list for short-lived agent tokens. See
-- postgres/014 for rationale.
CREATE TABLE token_denylist (
    jti VARCHAR(255) PRIMARY KEY,
    lease_id CHAR(36),
    expires_at DATETIME NOT NULL,
    revoked_at DATETIME NOT NULL DEFAULT NOW(),
    reason TEXT
);

CREATE INDEX idx_token_denylist_expires ON token_denylist (expires_at);
