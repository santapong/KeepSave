CREATE TABLE workspace_creation_requests (
 user_id TEXT NOT NULL REFERENCES users(id),
 request_key TEXT NOT NULL CHECK(length(request_key) BETWEEN 1 AND 128),
 request_digest TEXT NOT NULL CHECK(length(request_digest)=64),
 organization_id TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT (datetime('now')),
 PRIMARY KEY(user_id,request_key)
);
