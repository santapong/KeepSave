CREATE TABLE promotion_requests (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_environment TEXT NOT NULL,
    target_environment TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    requested_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    approved_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    keys_filter TEXT,
    override_policy TEXT NOT NULL DEFAULT 'skip',
    notes TEXT DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT
);

CREATE TABLE secret_snapshots (
    id TEXT PRIMARY KEY,
    promotion_id TEXT NOT NULL REFERENCES promotion_requests(id) ON DELETE CASCADE,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    encrypted_value BLOB NOT NULL,
    value_nonce BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_secret_snapshots_promotion ON secret_snapshots(promotion_id);
