-- Phase 14: Application Dashboard (integrated from application-dashboard project)
-- Provides centralized service/application management within KeepSave

CREATE TABLE IF NOT EXISTS applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    url VARCHAR(2048) NOT NULL,
    description TEXT DEFAULT '',
    icon VARCHAR(2048) DEFAULT '🚀',
    category VARCHAR(100) DEFAULT 'General',
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_applications_owner ON applications(owner_id);
CREATE INDEX IF NOT EXISTS idx_applications_category ON applications(category);

CREATE TABLE IF NOT EXISTS application_favorites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (user_id, application_id)
);

CREATE INDEX IF NOT EXISTS idx_app_favorites_user ON application_favorites(user_id);
