-- ADR-0017: see postgres/010 for rationale. SQLite has no native BOOLEAN, so
-- prior_existed is INTEGER 0/1 (1 = overwrite to restore, 0 = added key to
-- delete on rollback). Default 1 keeps existing (overwrite-only) snapshots
-- correct.
ALTER TABLE secret_snapshots ADD COLUMN prior_existed INTEGER NOT NULL DEFAULT 1;
