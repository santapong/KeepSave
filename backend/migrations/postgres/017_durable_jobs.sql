CREATE TABLE outbox_jobs (
 id UUID PRIMARY KEY,
 kind VARCHAR(100) NOT NULL,
 payload JSONB NOT NULL,
 effect VARCHAR(20) NOT NULL CHECK(effect IN ('local','external')),
 status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','leased','dispatched','done','failed','uncertain')),
 available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 lease_until TIMESTAMPTZ,
 worker_id UUID,
 attempt INTEGER NOT NULL DEFAULT 0,
 max_attempts INTEGER NOT NULL CHECK(max_attempts BETWEEN 1 AND 20),
 fence BIGINT NOT NULL DEFAULT 0,
 outcome VARCHAR(100) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX outbox_jobs_admission ON outbox_jobs(status,available_at,lease_until);
