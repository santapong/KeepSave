-- ADR-0008: RS256 JWT signing keys. The private key is stored encrypted (under
-- the service sub-key, ADR-0018); only the public key is ever served (JWKS).
-- status: 'signing' (mints tokens, exactly one), 'verifying' (overlap window),
-- 'retired' (excluded from JWKS).
CREATE TABLE jwt_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kid TEXT UNIQUE NOT NULL,
    alg TEXT NOT NULL,
    public_key BYTEA,
    private_key_encrypted BYTEA,
    private_key_nonce BYTEA,
    status TEXT NOT NULL CHECK (status IN ('signing', 'verifying', 'retired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retired_at TIMESTAMPTZ
);

-- At most one signing key at a time.
CREATE UNIQUE INDEX idx_jwt_keys_one_signing ON jwt_keys (status) WHERE status = 'signing';
