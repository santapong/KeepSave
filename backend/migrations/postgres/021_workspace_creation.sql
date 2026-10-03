-- Caller-scoped idempotent workspace onboarding. The result ID is retained
-- without an organization FK so deleting an empty workspace never permits its
-- creation request to resurrect it or prevents the explicit deletion itself.
CREATE TABLE workspace_creation_requests (
 user_id UUID NOT NULL REFERENCES users(id),
 request_key VARCHAR(128) NOT NULL,
 request_digest CHAR(64) NOT NULL,
 organization_id UUID NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(user_id,request_key)
);
