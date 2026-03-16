CREATE TABLE IF NOT EXISTS organizations (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    owner_id CHAR(36) NOT NULL,
    created_at DATETIME DEFAULT NOW(),
    updated_at DATETIME DEFAULT NOW(),
    FOREIGN KEY (owner_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS organization_members (
    id CHAR(36) PRIMARY KEY,
    organization_id CHAR(36) NOT NULL,
    user_id CHAR(36) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'viewer',
    created_at DATETIME DEFAULT NOW(),
    updated_at DATETIME DEFAULT NOW(),
    UNIQUE (organization_id, user_id),
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

ALTER TABLE projects ADD COLUMN organization_id CHAR(36),
    ADD FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX idx_projects_organization ON projects(organization_id);

CREATE TABLE IF NOT EXISTS secret_templates (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    stack VARCHAR(100) NOT NULL DEFAULT 'custom',
    `keys` JSON NOT NULL DEFAULT ('[]'),
    created_by CHAR(36) NOT NULL,
    organization_id CHAR(36),
    is_global TINYINT(1) DEFAULT 0,
    created_at DATETIME DEFAULT NOW(),
    updated_at DATETIME DEFAULT NOW(),
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS secret_dependencies (
    id CHAR(36) PRIMARY KEY,
    project_id CHAR(36) NOT NULL,
    environment_id CHAR(36) NOT NULL,
    secret_key VARCHAR(255) NOT NULL,
    depends_on_key VARCHAR(255) NOT NULL,
    reference_pattern VARCHAR(500) NOT NULL,
    created_at DATETIME DEFAULT NOW(),
    UNIQUE (project_id, environment_id, secret_key, depends_on_key),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX idx_secret_deps_project ON secret_dependencies(project_id, environment_id);
CREATE INDEX idx_org_members_user ON organization_members(user_id);
CREATE INDEX idx_org_members_org ON organization_members(organization_id);
CREATE INDEX idx_templates_org ON secret_templates(organization_id);
CREATE INDEX idx_templates_global ON secret_templates(is_global);
