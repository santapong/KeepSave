CREATE TABLE secret_snapshots (
    id CHAR(36) PRIMARY KEY,
    promotion_id CHAR(36) NOT NULL,
    environment_id CHAR(36) NOT NULL,
    `key` VARCHAR(255) NOT NULL,
    encrypted_value BLOB NOT NULL,
    value_nonce BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT NOW(),
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX idx_secret_snapshots_promotion ON secret_snapshots(promotion_id);

CREATE TABLE promotion_requests (
    id CHAR(36) PRIMARY KEY,
    project_id CHAR(36) NOT NULL,
    source_environment VARCHAR(50) NOT NULL,
    target_environment VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    requested_by CHAR(36) NOT NULL,
    approved_by CHAR(36),
    keys_filter JSON,
    override_policy VARCHAR(20) NOT NULL DEFAULT 'skip',
    notes TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT NOW(),
    completed_at DATETIME,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (requested_by) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (approved_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_promotion_requests_project ON promotion_requests(project_id, created_at DESC);
CREATE INDEX idx_promotion_requests_status ON promotion_requests(project_id, status);

ALTER TABLE secret_snapshots
    ADD CONSTRAINT fk_snapshots_promotion
    FOREIGN KEY (promotion_id)
    REFERENCES promotion_requests(id)
    ON DELETE CASCADE;
