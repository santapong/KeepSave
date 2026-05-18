-- Audit S-H7: per-account login lockout. SQLite variant - used in tests
-- and embedded deployments. Production uses postgres/.

CREATE TABLE IF NOT EXISTS auth_login_attempts (
    email           TEXT PRIMARY KEY,
    failed_count    INTEGER NOT NULL DEFAULT 0,
    last_failed_at  TIMESTAMP,
    locked_until    TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_auth_login_attempts_locked
    ON auth_login_attempts (locked_until);
