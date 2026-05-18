-- Audit S-H7: per-account login lockout. Tracks consecutive failures by
-- email; a hot threshold (10 failures within 15 min) flips locked_until
-- forward by 15 min, after which the counter resets on first successful
-- login. Per-IP rate-limiting is the existing fast layer; this is the
-- defence against distributed credential stuffing where one email is
-- targeted by many IPs.

CREATE TABLE IF NOT EXISTS auth_login_attempts (
    email           TEXT PRIMARY KEY,
    failed_count    INTEGER NOT NULL DEFAULT 0,
    last_failed_at  TIMESTAMPTZ,
    locked_until    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_login_attempts_locked
    ON auth_login_attempts (locked_until)
    WHERE locked_until IS NOT NULL;
