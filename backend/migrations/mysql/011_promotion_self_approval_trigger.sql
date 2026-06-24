-- ADR-0017 / ADR-0007 parity. MySQL already enforces approver≠requester via the
-- CHECK constraint added in migration 008; this file is a no-op so the migration
-- version count stays aligned with the SQLite dialect.
SELECT 1;
