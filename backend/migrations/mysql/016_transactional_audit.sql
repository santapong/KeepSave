-- Persisted serialization point for the existing HMAC audit format.
-- Bootstrap from exactly one existing tip; application verification rejects forks.
CREATE TABLE audit_chain_head (
    id INTEGER PRIMARY KEY,
    entry_hash VARCHAR(64) NOT NULL,
    revision BIGINT NOT NULL DEFAULT 0,
    CHECK (id = 1)
);
INSERT INTO audit_chain_head(id,entry_hash)
SELECT 1, COALESCE((SELECT a.entry_hash FROM audit_log a WHERE a.entry_hash IS NOT NULL
 AND NOT EXISTS (SELECT 1 FROM audit_log b WHERE b.prev_hash=a.entry_hash)), '');
