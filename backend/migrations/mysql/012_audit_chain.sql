-- ADR-0019: tamper-evident audit hash chain. See postgres/012 for rationale.
ALTER TABLE audit_log ADD COLUMN prev_hash VARCHAR(64);
ALTER TABLE audit_log ADD COLUMN entry_hash VARCHAR(64);
