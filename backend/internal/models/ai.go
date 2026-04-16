package models

import (
	"time"

	"github.com/google/uuid"
)

// Phase 15: AI Intelligence & Smart Operations

// AIProviderConfig represents configuration for an AI provider.
type AIProviderConfig struct {
	Provider    string  `json:"provider"` // claude, openai, gemini, groq, mistral, ollama
	APIKey      string  `json:"api_key,omitempty"`
	Model       string  `json:"model"`
	BaseURL     string  `json:"base_url,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// AIProviderStatus represents the status of an AI provider.
type AIProviderStatus struct {
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
	Model     string `json:"model"`
	Error     string `json:"error,omitempty"`
}

// DriftCheck represents a drift detection run between environments.
type DriftCheck struct {
	ID              uuid.UUID  `json:"id"`
	ProjectID       uuid.UUID  `json:"project_id"`
	SourceEnv       string     `json:"source_env"`
	TargetEnv       string     `json:"target_env"`
	Status          string     `json:"status"` // running, completed, failed
	TotalKeys       int        `json:"total_keys"`
	DriftedKeys     int        `json:"drifted_keys"`
	MissingInSource int        `json:"missing_in_source"`
	MissingInTarget int        `json:"missing_in_target"`
	DriftEntries    JSONMap    `json:"drift_entries,omitempty"`
	Remediation     string     `json:"remediation,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// DriftDetailEntry represents a single drift item between two environments.
type DriftDetailEntry struct {
	Key            string `json:"key"`
	DriftType      string `json:"drift_type"` // value_mismatch, missing_source, missing_target
	SourceExists   bool   `json:"source_exists"`
	TargetExists   bool   `json:"target_exists"`
	ValuesDiffer   bool   `json:"values_differ"`
	Recommendation string `json:"recommendation,omitempty"`
}

// DriftSchedule represents a scheduled drift check.
type DriftSchedule struct {
	ID        uuid.UUID  `json:"id"`
	ProjectID uuid.UUID  `json:"project_id"`
	SourceEnv string     `json:"source_env"`
	TargetEnv string     `json:"target_env"`
	CronExpr  string     `json:"cron_expr"`
	Enabled   bool       `json:"enabled"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Anomaly represents a detected anomaly in secret access patterns.
type Anomaly struct {
	ID             uuid.UUID  `json:"id"`
	ProjectID      *uuid.UUID `json:"project_id,omitempty"`
	APIKeyID       *uuid.UUID `json:"api_key_id,omitempty"`
	AnomalyType    string     `json:"anomaly_type"` // unusual_time, frequency_spike, new_ip, unusual_key
	Severity       string     `json:"severity"`      // low, medium, high, critical
	Description    string     `json:"description"`
	Details        JSONMap    `json:"details,omitempty"`
	Status         string     `json:"status"` // open, acknowledged, resolved, false_positive
	DetectedAt     time.Time  `json:"detected_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// AnomalyRule represents a configurable alert rule for anomaly detection.
type AnomalyRule struct {
	ID        uuid.UUID  `json:"id"`
	ProjectID *uuid.UUID `json:"project_id,omitempty"`
	APIKeyID  *uuid.UUID `json:"api_key_id,omitempty"`
	RuleType  string     `json:"rule_type"` // time_based, frequency, ip_based, key_access
	Config    JSONMap    `json:"config"`
	Enabled   bool       `json:"enabled"`
	CreatedBy uuid.UUID  `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// AccessTimeSeries represents time-series data for access analytics.
type AccessTimeSeries struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"project_id"`
	APIKeyID     *uuid.UUID `json:"api_key_id,omitempty"`
	Bucket       time.Time  `json:"bucket"`
	AccessCount  int        `json:"access_count"`
	UniqueKeys   int        `json:"unique_keys"`
	UniqueIPs    int        `json:"unique_ips"`
	AvgLatencyMs float64    `json:"avg_latency_ms"`
}

// UsageTrend represents aggregated usage data for analytics.
type UsageTrend struct {
	Period      string  `json:"period"` // daily, weekly, monthly
	Date        string  `json:"date"`
	AccessCount int     `json:"access_count"`
	UniqueUsers int     `json:"unique_users"`
	UniqueKeys  int     `json:"unique_keys"`
	Growth      float64 `json:"growth_percent"`
}

// UsageForecast represents a predicted usage value.
type UsageForecast struct {
	Date           string  `json:"date"`
	PredictedCount float64 `json:"predicted_count"`
	LowerBound     float64 `json:"lower_bound"`
	UpperBound     float64 `json:"upper_bound"`
	Confidence     float64 `json:"confidence"`
}

// SecretRecommendation represents an AI-generated recommendation for secrets.
type SecretRecommendation struct {
	ID              uuid.UUID  `json:"id"`
	ProjectID       uuid.UUID  `json:"project_id"`
	RecommType      string     `json:"recomm_type"` // missing_secret, duplicate, rotation_needed, type_mismatch
	Severity        string     `json:"severity"`    // info, warning, critical
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	AffectedKeys    StringList `json:"affected_keys,omitempty"`
	SuggestedAction string     `json:"suggested_action,omitempty"`
	AutoFixable     bool       `json:"auto_fixable"`
	Status          string     `json:"status"` // pending, applied, dismissed
	CreatedAt       time.Time  `json:"created_at"`
}

// NLPQueryResult represents the result of a natural language secret query.
type NLPQueryResult struct {
	Query          string           `json:"query"`
	Intent         string           `json:"intent"` // find_secret, describe_project, list_env, suggest
	MatchedSecrets []NLPSecretMatch `json:"matched_secrets,omitempty"`
	Explanation    string           `json:"explanation"`
	Suggestions    []string         `json:"suggestions,omitempty"`
	Provider       string           `json:"provider"`
	Model          string           `json:"model"`
}

// NLPSecretMatch represents a matched secret from NLP query.
type NLPSecretMatch struct {
	ProjectID   uuid.UUID `json:"project_id"`
	ProjectName string    `json:"project_name"`
	Environment string    `json:"environment"`
	Key         string    `json:"key"`
	Score       float64   `json:"score"`
	Reason      string    `json:"reason"`
}
