-- ADR-0007 / audit S-H3 - mirror of the postgres migration. MySQL 8+
-- enforces CHECK constraints; for older versions this is parsed but
-- silently ignored - the app-layer guard in promotion_service.go still
-- holds.

ALTER TABLE promotion_requests
  ADD CONSTRAINT promotion_requests_no_self_approval
  CHECK (
    requested_by IS NULL
    OR approved_by IS NULL
    OR requested_by <> approved_by
  );
