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
