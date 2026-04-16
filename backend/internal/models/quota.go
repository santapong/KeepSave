package models

import (
	"time"

	"github.com/google/uuid"
)

// UsageQuota represents per-organization usage limits.
type UsageQuota struct {
	ID                uuid.UUID `json:"id"`
	OrganizationID    uuid.UUID `json:"organization_id"`
	MaxSecrets        int       `json:"max_secrets"`
	MaxProjects       int       `json:"max_projects"`
	MaxAPIKeys        int       `json:"max_api_keys"`
	MaxRequestsPerDay int       `json:"max_requests_per_day"`
	CurrentSecrets    int       `json:"current_secrets"`
	CurrentProjects   int       `json:"current_projects"`
	CurrentAPIKeys    int       `json:"current_api_keys"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
