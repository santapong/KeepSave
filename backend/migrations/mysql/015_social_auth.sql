-- Additive external identity support. Provider tokens are never stored.
CREATE TABLE social_identities (
    provider VARCHAR(16) NOT NULL,
    subject VARBINARY(255) NOT NULL,
    user_id CHAR(36) NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, subject),
    UNIQUE (user_id, provider),
    FOREIGN KEY (user_id) REFERENCES users(id)
);
CREATE TABLE social_auth_flows (
    state_hash CHAR(64) PRIMARY KEY,
    provider VARCHAR(16) NOT NULL,
    challenge CHAR(43) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    link_user_id CHAR(36),
    expires_at BIGINT NOT NULL,
    FOREIGN KEY (link_user_id) REFERENCES users(id)
);
CREATE INDEX idx_social_auth_expiry ON social_auth_flows(expires_at);
