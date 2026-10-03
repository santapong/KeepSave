-- Fail closed on pre-existing canonical email collisions; never merge accounts.
CREATE UNIQUE INDEX idx_users_email_canonical ON users (LOWER(TRIM(email)));
ALTER TABLE social_auth_flows ADD COLUMN link_session_id UUID REFERENCES session_tokens(id);
CREATE TABLE platform_admin_grants (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    operator_name VARCHAR(255) NOT NULL,
    reason VARCHAR(500) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
