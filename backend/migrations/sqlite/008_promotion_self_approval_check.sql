-- ADR-0007 / audit S-H3. SQLite supports CHECK in CREATE TABLE but not via
-- ALTER TABLE; the app-layer guard in promotion_service.go is the
-- enforcement boundary for the sqlite dialect (used for tests only). This
-- file exists for parity so the schema_migrations row count matches the
-- other dialects.
SELECT 1;
