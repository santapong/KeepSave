-- Audit S-H7: per-account login lockout. Mirror of the postgres migration.

CREATE TABLE IF NOT EXISTS auth_login_attempts (
    email           VARCHAR(320) PRIMARY KEY,
    failed_count    INTEGER NOT NULL DEFAULT 0,
    last_failed_at  TIMESTAMP NULL,
    locked_until    TIMESTAMP NULL,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE INDEX idx_auth_login_attempts_locked ON auth_login_attempts (locked_until);
