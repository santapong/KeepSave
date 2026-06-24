-- ADR-0008: RS256 JWT signing keys. See postgres/013 for rationale. MySQL has
-- no partial unique index, so the single-signing-key invariant is enforced in
-- the application layer (the keystore) rather than by a DB constraint.
CREATE TABLE jwt_keys (
    id CHAR(36) PRIMARY KEY,
    kid VARCHAR(255) UNIQUE NOT NULL,
    alg VARCHAR(20) NOT NULL,
    public_key BLOB,
    private_key_encrypted BLOB,
    private_key_nonce BLOB,
    status VARCHAR(20) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT NOW(),
    retired_at DATETIME NULL
);
