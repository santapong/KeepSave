-- ADR-0019: tamper-evident audit hash chain. entry_hash is the hex HMAC-SHA256
-- of the previous row's entry_hash plus this row's canonical fields; prev_hash
-- is that previous entry_hash. Both NULL on pre-chain rows (epoch start), so
-- this migration is additive and inert until the chain key is configured.
ALTER TABLE audit_log ADD COLUMN prev_hash VARCHAR(64);
ALTER TABLE audit_log ADD COLUMN entry_hash VARCHAR(64);
