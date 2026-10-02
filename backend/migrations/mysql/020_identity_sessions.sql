-- One atomic MySQL 8 DDL: canonical collisions abort; no email-to-admin conversion.
ALTER TABLE users ADD COLUMN email_canonical VARCHAR(255) GENERATED ALWAYS AS (LOWER(TRIM(email))) STORED, ADD UNIQUE INDEX idx_users_email_canonical (email_canonical);
ALTER TABLE social_auth_flows ADD COLUMN link_session_id CHAR(36), ADD CONSTRAINT fk_social_link_session FOREIGN KEY (link_session_id) REFERENCES session_tokens(id);
CREATE TABLE platform_admin_grants (
    user_id CHAR(36) PRIMARY KEY,
    active TINYINT(1) NOT NULL DEFAULT 1,
    operator_name VARCHAR(255) NOT NULL,
    reason VARCHAR(500) NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
