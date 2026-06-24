-- ADR-0017: see postgres/010 for rationale. prior_existed marks whether a
-- snapshot restores an overwritten value (TRUE) or records an added key that
-- rollback must delete (FALSE). Default TRUE keeps existing snapshots correct.
ALTER TABLE secret_snapshots ADD COLUMN prior_existed BOOLEAN NOT NULL DEFAULT TRUE;
