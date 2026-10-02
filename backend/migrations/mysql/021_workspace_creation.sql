CREATE TABLE workspace_creation_requests (
 user_id CHAR(36) NOT NULL,
 request_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 request_digest CHAR(64) NOT NULL,
 organization_id CHAR(36) NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(user_id,request_key),
 FOREIGN KEY(user_id) REFERENCES users(id)
);
