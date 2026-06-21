-- ADR-0019: tamper-evident audit hash chain. See postgres/012 for rationale.
ALTER TABLE audit_log ADD COLUMN prev_hash TEXT;
ALTER TABLE audit_log ADD COLUMN entry_hash TEXT;
