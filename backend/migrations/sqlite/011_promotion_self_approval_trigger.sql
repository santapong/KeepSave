-- ADR-0017 / ADR-0007: enforce the approver≠requester invariant at the DB layer
-- for SQLite, at parity with the Postgres/MySQL CHECK (migration 008). SQLite
-- cannot add a CHECK via ALTER TABLE, so a BEFORE UPDATE trigger aborts any
-- attempt to set approved_by equal to requested_by. The app layer
-- (promotion_service.go ErrSelfApproval) is the primary guard; this is the
-- belt-and-suspenders backstop.
CREATE TRIGGER promotion_no_self_approval
BEFORE UPDATE OF approved_by ON promotion_requests
FOR EACH ROW
WHEN NEW.approved_by IS NOT NULL AND NEW.approved_by = NEW.requested_by
BEGIN
    SELECT RAISE(ABORT, 'requester cannot approve their own promotion');
END;
