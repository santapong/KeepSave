-- ADR-0017: rollback must also undo keys the promotion *added* (not only the
-- ones it overwrote). prior_existed marks each snapshot: TRUE = the target had
-- a value to restore; FALSE = the promotion added the key and rollback deletes
-- it. Default TRUE keeps existing (overwrite-only) snapshots correct.
ALTER TABLE secret_snapshots ADD COLUMN prior_existed BOOLEAN NOT NULL DEFAULT TRUE;
