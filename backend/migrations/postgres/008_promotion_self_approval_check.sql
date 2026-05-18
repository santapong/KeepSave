-- ADR-0007 / audit S-H3: a single compromised account cannot move secrets
-- to PROD. App layer rejects self-approval; this CHECK is the DB-level
-- belt-and-suspenders that catches any code path the app validator misses.

ALTER TABLE promotion_requests
  ADD CONSTRAINT promotion_requests_no_self_approval
  CHECK (
    requested_by IS NULL
    OR approved_by IS NULL
    OR requested_by <> approved_by
  );
