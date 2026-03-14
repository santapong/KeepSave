package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Project struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	OwnerID      uuid.UUID `json:"owner_id"`
	EncryptedDEK []byte    `json:"-"`
	DEKNonce     []byte    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Environment struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Secret struct {
	ID             uuid.UUID `json:"id"`
	ProjectID      uuid.UUID `json:"project_id"`
	EnvironmentID  uuid.UUID `json:"environment_id"`
	Key            string    `json:"key"`
	EncryptedValue []byte    `json:"-"`
	ValueNonce     []byte    `json:"-"`
	Value          string    `json:"value,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type APIKey struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	HashedKey   string     `json:"-"`
	UserID      uuid.UUID  `json:"user_id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	Scopes      StringList `json:"scopes"`
	Environment *string    `json:"environment,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type AuditEntry struct {
	ID          uuid.UUID  `json:"id"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	Action      string     `json:"action"`
	Environment string     `json:"environment,omitempty"`
	Details     JSONMap    `json:"details"`
	IPAddress   string     `json:"ip_address"`
	CreatedAt   time.Time  `json:"created_at"`
}

// JSONMap is a map that implements sql.Scanner and driver.Valuer for JSONB columns.
type JSONMap map[string]interface{}

func (j JSONMap) Value() (interface{}, error) {
	if j == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(src interface{}) error {
	if src == nil {
		*j = make(JSONMap)
		return nil
	}
	source, ok := src.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(source, j)
}

// PromotionRequest represents a request to promote secrets between environments.
type PromotionRequest struct {
	ID                uuid.UUID  `json:"id"`
	ProjectID         uuid.UUID  `json:"project_id"`
	SourceEnvironment string     `json:"source_environment"`
	TargetEnvironment string     `json:"target_environment"`
	Status            string     `json:"status"` // pending, approved, rejected, completed
	RequestedBy       uuid.UUID  `json:"requested_by"`
	ApprovedBy        *uuid.UUID `json:"approved_by,omitempty"`
	KeysFilter        StringList `json:"keys_filter,omitempty"`
	OverridePolicy    string     `json:"override_policy"` // skip, overwrite
	Notes             string     `json:"notes,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

// SecretSnapshot stores a snapshot of a secret value before promotion for rollback.
type SecretSnapshot struct {
	ID             uuid.UUID `json:"id"`
	PromotionID    uuid.UUID `json:"promotion_id"`
	EnvironmentID  uuid.UUID `json:"environment_id"`
	Key            string    `json:"key"`
	EncryptedValue []byte    `json:"-"`
	ValueNonce     []byte    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
}

// SecretVersion stores a historical version of a secret value.
type SecretVersion struct {
	ID             uuid.UUID  `json:"id"`
	SecretID       uuid.UUID  `json:"secret_id"`
	ProjectID      uuid.UUID  `json:"project_id"`
	EnvironmentID  uuid.UUID  `json:"environment_id"`
	Version        int        `json:"version"`
	EncryptedValue []byte     `json:"-"`
	ValueNonce     []byte     `json:"-"`
	Value          string     `json:"value,omitempty"`
	CreatedBy      *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// DiffEntry represents a single key difference between two environments.
type DiffEntry struct {
	Key          string `json:"key"`
	Action       string `json:"action"` // add, update, no_change
	SourceValue  string `json:"source_value,omitempty"`
	TargetValue  string `json:"target_value,omitempty"`
	SourceExists bool   `json:"source_exists"`
	TargetExists bool   `json:"target_exists"`
}

// StringList handles PostgreSQL TEXT[] arrays.
type StringList []string

// Organization represents a multi-tenant organization account.
type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	OwnerID   uuid.UUID `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OrgMember represents a user's membership and role in an organization.
type OrgMember struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`
	Role           string    `json:"role"` // viewer, editor, admin, promoter
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TemplateKey represents a single key definition within a secret template.
type TemplateKey struct {
	Key          string `json:"key"`
	Description  string `json:"description,omitempty"`
	DefaultValue string `json:"default_value,omitempty"`
	Required     bool   `json:"required"`
}

// SecretTemplate represents a predefined set of secret keys for common stacks.
type SecretTemplate struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Stack          string     `json:"stack"`
	Keys           JSONMap    `json:"keys"`
	CreatedBy      uuid.UUID  `json:"created_by"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	IsGlobal       bool       `json:"is_global"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// SecretDependency represents a reference relationship between secrets.
type SecretDependency struct {
	ID               uuid.UUID `json:"id"`
	ProjectID        uuid.UUID `json:"project_id"`
	EnvironmentID    uuid.UUID `json:"environment_id"`
	SecretKey        string    `json:"secret_key"`
	DependsOnKey     string    `json:"depends_on_key"`
	ReferencePattern string    `json:"reference_pattern"`
	CreatedAt        time.Time `json:"created_at"`
}

// DependencyNode represents a node in the secrets dependency graph.
type DependencyNode struct {
	Key          string   `json:"key"`
	DependsOn    []string `json:"depends_on"`
	ReferencedBy []string `json:"referenced_by"`
}

// EnvFile represents a parsed .env file for import/export.
type EnvFile struct {
	Environment string            `json:"environment"`
	Variables   map[string]string `json:"variables"`
}

// Phase 9: Enterprise Features

// SSOConfig represents an SSO provider configuration for an organization.
type SSOConfig struct {
	ID                    uuid.UUID `json:"id"`
	OrganizationID        uuid.UUID `json:"organization_id"`
	Provider              string    `json:"provider"` // oidc, saml
	IssuerURL             string    `json:"issuer_url"`
	ClientID              string    `json:"client_id"`
	ClientSecretEncrypted []byte    `json:"-"`
	ClientSecretNonce     []byte    `json:"-"`
	Metadata              JSONMap   `json:"metadata,omitempty"`
	Enabled               bool      `json:"enabled"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// IPAllowlistEntry represents an IP allowlist entry.
type IPAllowlistEntry struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	ProjectID      *uuid.UUID `json:"project_id,omitempty"`
	CIDR           string     `json:"cidr"`
	Description    string     `json:"description"`
	CreatedBy      uuid.UUID  `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
}

// SecretPolicy defines lifecycle policies for secrets in a project.
type SecretPolicy struct {
	ID                   uuid.UUID `json:"id"`
	ProjectID            uuid.UUID `json:"project_id"`
	MaxAgeDays           int       `json:"max_age_days"`
	RotationReminderDays int       `json:"rotation_reminder_days"`
	RequireRotation      bool      `json:"require_rotation"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// ComplianceReport represents a generated compliance report.
type ComplianceReport struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	ReportType     string     `json:"report_type"` // soc2, gdpr, pci
	Status         string     `json:"status"`      // pending, generating, completed, failed
	Data           JSONMap    `json:"data,omitempty"`
	GeneratedBy    uuid.UUID  `json:"generated_by"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// BackupSnapshot represents an encrypted backup of project data.
type BackupSnapshot struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     *uuid.UUID `json:"project_id,omitempty"`
	SnapshotType  string     `json:"snapshot_type"` // full, incremental
	EncryptedData []byte     `json:"-"`
	DataNonce     []byte     `json:"-"`
	SizeBytes     int64      `json:"size_bytes"`
	CreatedBy     uuid.UUID  `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Phase 10: Security Hardening

// SecurityEvent represents a logged security event.
type SecurityEvent struct {
	ID        uuid.UUID  `json:"id"`
	EventType string     `json:"event_type"` // auth_failure, rate_limit, suspicious_access
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	IPAddress string     `json:"ip_address"`
	UserAgent string     `json:"user_agent"`
	Details   JSONMap    `json:"details,omitempty"`
	Severity  string     `json:"severity"` // info, warning, critical
	CreatedAt time.Time  `json:"created_at"`
}

// SessionToken represents a tracked session for token revocation.
type SessionToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

// Phase 11: AI Agent Experience

// SecretLease represents a time-limited access grant for an agent.
type SecretLease struct {
	ID          uuid.UUID  `json:"id"`
	APIKeyID    uuid.UUID  `json:"api_key_id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	Environment string     `json:"environment"`
	SecretKeys  StringList `json:"secret_keys"`
	GrantedAt   time.Time  `json:"granted_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	Revoked     bool       `json:"revoked"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

// AgentActivity represents a logged agent action.
type AgentActivity struct {
	ID          uuid.UUID `json:"id"`
	APIKeyID    uuid.UUID `json:"api_key_id"`
	ProjectID   uuid.UUID `json:"project_id"`
	Action      string    `json:"action"`
	Environment string    `json:"environment,omitempty"`
	SecretKey   string    `json:"secret_key,omitempty"`
	IPAddress   string    `json:"ip_address"`
	CreatedAt   time.Time `json:"created_at"`
}

// Phase 12: Platform Ecosystem

// Event represents an event in the event log.
type Event struct {
	ID          uuid.UUID `json:"id"`
	EventType   string    `json:"event_type"`
	AggregateID uuid.UUID `json:"aggregate_id"`
	Payload     JSONMap   `json:"payload"`
	Published   bool      `json:"published"`
	CreatedAt   time.Time `json:"created_at"`
}

// AccessPolicy defines access restrictions for a project.
type AccessPolicy struct {
	ID         uuid.UUID `json:"id"`
	ProjectID  uuid.UUID `json:"project_id"`
	PolicyType string    `json:"policy_type"` // time_window, ip_restriction, geo_restriction
	Config     JSONMap   `json:"config"`
	Enabled    bool      `json:"enabled"`
	CreatedBy  uuid.UUID `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Plugin represents a registered plugin.
type Plugin struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	PluginType string    `json:"plugin_type"` // secret_provider, notification, validation
	Version    string    `json:"version"`
	Config     JSONMap   `json:"config,omitempty"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
