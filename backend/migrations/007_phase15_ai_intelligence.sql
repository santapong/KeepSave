-- Phase 15: AI Intelligence & Smart Operations

-- Drift detection checks
CREATE TABLE IF NOT EXISTS drift_checks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    source_env TEXT NOT NULL,
    target_env TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',
    total_keys INTEGER NOT NULL DEFAULT 0,
    drifted_keys INTEGER NOT NULL DEFAULT 0,
    missing_in_source INTEGER NOT NULL DEFAULT 0,
    missing_in_target INTEGER NOT NULL DEFAULT 0,
    drift_entries TEXT DEFAULT '{}',
    remediation TEXT DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_drift_checks_project ON drift_checks(project_id);
CREATE INDEX IF NOT EXISTS idx_drift_checks_created ON drift_checks(created_at DESC);

-- Drift check schedules
CREATE TABLE IF NOT EXISTS drift_schedules (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    source_env TEXT NOT NULL,
    target_env TEXT NOT NULL,
    cron_expr TEXT NOT NULL DEFAULT '0 */6 * * *',
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMP,
    next_run_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_drift_schedules_project ON drift_schedules(project_id);

-- Anomaly detection
CREATE TABLE IF NOT EXISTS anomalies (
    id TEXT PRIMARY KEY,
    project_id TEXT,
    api_key_id TEXT,
    anomaly_type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'medium',
    description TEXT NOT NULL,
    details TEXT DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'open',
    detected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    acknowledged_at TIMESTAMP,
    resolved_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_anomalies_project ON anomalies(project_id);
CREATE INDEX IF NOT EXISTS idx_anomalies_status ON anomalies(status);
CREATE INDEX IF NOT EXISTS idx_anomalies_detected ON anomalies(detected_at DESC);

-- Anomaly alert rules
CREATE TABLE IF NOT EXISTS anomaly_rules (
    id TEXT PRIMARY KEY,
    project_id TEXT,
    api_key_id TEXT,
    rule_type TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_by TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_anomaly_rules_project ON anomaly_rules(project_id);

-- Access time-series for analytics
CREATE TABLE IF NOT EXISTS access_timeseries (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    api_key_id TEXT,
    bucket TIMESTAMP NOT NULL,
    access_count INTEGER NOT NULL DEFAULT 0,
    unique_keys INTEGER NOT NULL DEFAULT 0,
    unique_ips INTEGER NOT NULL DEFAULT 0,
    avg_latency_ms REAL NOT NULL DEFAULT 0,
    UNIQUE(project_id, api_key_id, bucket)
);

CREATE INDEX IF NOT EXISTS idx_access_ts_project ON access_timeseries(project_id, bucket);

-- Secret recommendations
CREATE TABLE IF NOT EXISTS secret_recommendations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    recomm_type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'info',
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    affected_keys TEXT DEFAULT '[]',
    suggested_action TEXT DEFAULT '',
    auto_fixable BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_recommendations_project ON secret_recommendations(project_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_status ON secret_recommendations(status);

-- NLP query log for learning and analytics
CREATE TABLE IF NOT EXISTS nlp_query_log (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    query TEXT NOT NULL,
    intent TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    matched_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_nlp_queries_user ON nlp_query_log(user_id);
CREATE INDEX IF NOT EXISTS idx_nlp_queries_created ON nlp_query_log(created_at DESC);

-- AI provider configuration per user
CREATE TABLE IF NOT EXISTS ai_provider_configs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_ai_configs_user ON ai_provider_configs(user_id);
