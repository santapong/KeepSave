-- ADR-0008: RS256 JWT signing keys. See postgres/013 for rationale.
CREATE TABLE jwt_keys (
    id TEXT PRIMARY KEY,
    kid TEXT UNIQUE NOT NULL,
    alg TEXT NOT NULL,
    public_key BLOB,
    private_key_encrypted BLOB,
    private_key_nonce BLOB,
    status TEXT NOT NULL CHECK (status IN ('signing', 'verifying', 'retired')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    retired_at TIMESTAMP
);

CREATE UNIQUE INDEX idx_jwt_keys_one_signing ON jwt_keys (status) WHERE status = 'signing';
