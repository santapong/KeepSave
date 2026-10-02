-- Unique-index failure preserves conflicting accounts for operator resolution.
CREATE UNIQUE INDEX idx_users_email_canonical ON users (LOWER(TRIM(email)));
ALTER TABLE social_auth_flows ADD COLUMN link_session_id TEXT REFERENCES session_tokens(id);
CREATE TABLE platform_admin_grants (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    active INTEGER NOT NULL DEFAULT 1,
    operator_name TEXT NOT NULL,
    reason TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
