-- Secret Snapshots (for rollback support)
CREATE TABLE secret_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promotion_id UUID NOT NULL,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    value_nonce BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_secret_snapshots_promotion ON secret_snapshots(promotion_id);

-- Promotion Requests
CREATE TABLE promotion_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_environment VARCHAR(50) NOT NULL,
    target_environment VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    requested_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    keys_filter TEXT[],
    override_policy VARCHAR(20) NOT NULL DEFAULT 'skip',
    notes TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_promotion_requests_project ON promotion_requests(project_id, created_at DESC);
CREATE INDEX idx_promotion_requests_status ON promotion_requests(project_id, status);

-- Add FK from snapshots to promotion_requests (deferred to avoid circular issues)
ALTER TABLE secret_snapshots
    ADD CONSTRAINT fk_snapshots_promotion
    FOREIGN KEY (promotion_id)
    REFERENCES promotion_requests(id)
    ON DELETE CASCADE;
