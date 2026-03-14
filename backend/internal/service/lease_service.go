package service

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// LeaseService manages just-in-time secret leases for agents.
type LeaseService struct {
	db *sql.DB
}

// NewLeaseService creates a new lease service.
func NewLeaseService(db *sql.DB) *LeaseService {
	return &LeaseService{db: db}
}

// CreateLease grants time-limited access to specific secrets.
func (s *LeaseService) CreateLease(apiKeyID, projectID uuid.UUID, environment string, secretKeys []string, duration time.Duration) (*models.SecretLease, error) {
	lease := &models.SecretLease{}
	expiresAt := time.Now().Add(duration)

	err := s.db.QueryRow(
		`INSERT INTO secret_leases (api_key_id, project_id, environment, secret_keys, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, api_key_id, project_id, environment, secret_keys, granted_at, expires_at, revoked`,
		apiKeyID, projectID, environment, models.StringList(secretKeys), expiresAt,
	).Scan(&lease.ID, &lease.APIKeyID, &lease.ProjectID, &lease.Environment,
		&lease.SecretKeys, &lease.GrantedAt, &lease.ExpiresAt, &lease.Revoked)
	if err != nil {
		return nil, fmt.Errorf("creating lease: %w", err)
	}
	return lease, nil
}

// GetActiveLease returns an active (non-expired, non-revoked) lease.
func (s *LeaseService) GetActiveLease(leaseID uuid.UUID) (*models.SecretLease, error) {
	lease := &models.SecretLease{}
	err := s.db.QueryRow(
		`SELECT id, api_key_id, project_id, environment, secret_keys, granted_at, expires_at, revoked, revoked_at
		FROM secret_leases WHERE id = $1 AND revoked = FALSE AND expires_at > NOW()`, leaseID,
	).Scan(&lease.ID, &lease.APIKeyID, &lease.ProjectID, &lease.Environment,
		&lease.SecretKeys, &lease.GrantedAt, &lease.ExpiresAt, &lease.Revoked, &lease.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("getting active lease: %w", err)
	}
	return lease, nil
}

// ListActiveLeases returns all active leases for an API key.
func (s *LeaseService) ListActiveLeases(apiKeyID uuid.UUID) ([]models.SecretLease, error) {
	rows, err := s.db.Query(
		`SELECT id, api_key_id, project_id, environment, secret_keys, granted_at, expires_at, revoked
		FROM secret_leases WHERE api_key_id = $1 AND revoked = FALSE AND expires_at > NOW()
		ORDER BY granted_at DESC`, apiKeyID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing active leases: %w", err)
	}
	defer rows.Close()

	var leases []models.SecretLease
	for rows.Next() {
		var l models.SecretLease
		if err := rows.Scan(&l.ID, &l.APIKeyID, &l.ProjectID, &l.Environment,
			&l.SecretKeys, &l.GrantedAt, &l.ExpiresAt, &l.Revoked); err != nil {
			return nil, fmt.Errorf("scanning lease: %w", err)
		}
		leases = append(leases, l)
	}
	return leases, nil
}

// RevokeLease revokes an active lease.
func (s *LeaseService) RevokeLease(leaseID uuid.UUID) error {
	_, err := s.db.Exec(
		`UPDATE secret_leases SET revoked = TRUE, revoked_at = NOW() WHERE id = $1`, leaseID,
	)
	if err != nil {
		return fmt.Errorf("revoking lease: %w", err)
	}
	return nil
}

// AgentAnalyticsService tracks and analyzes agent activity.
type AgentAnalyticsService struct {
	db *sql.DB
}

// NewAgentAnalyticsService creates a new agent analytics service.
func NewAgentAnalyticsService(db *sql.DB) *AgentAnalyticsService {
	return &AgentAnalyticsService{db: db}
}

// LogActivity records an agent action.
func (s *AgentAnalyticsService) LogActivity(apiKeyID, projectID uuid.UUID, action, environment, secretKey, ipAddress string) error {
	_, err := s.db.Exec(
		`INSERT INTO agent_activities (api_key_id, project_id, action, environment, secret_key, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		apiKeyID, projectID, action, environment, secretKey, ipAddress,
	)
	if err != nil {
		return fmt.Errorf("logging agent activity: %w", err)
	}
	return nil
}

// GetActivitySummary returns activity counts grouped by action for an API key.
func (s *AgentAnalyticsService) GetActivitySummary(apiKeyID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT action, COUNT(*) as count, MAX(created_at) as last_used
		FROM agent_activities WHERE api_key_id = $1
		GROUP BY action ORDER BY count DESC`, apiKeyID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting activity summary: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var action string
		var count int
		var lastUsed time.Time
		if err := rows.Scan(&action, &count, &lastUsed); err != nil {
			return nil, fmt.Errorf("scanning activity: %w", err)
		}
		results = append(results, map[string]interface{}{
			"action":    action,
			"count":     count,
			"last_used": lastUsed,
		})
	}
	return results, nil
}

// GetRecentActivity returns recent agent activities.
func (s *AgentAnalyticsService) GetRecentActivity(projectID uuid.UUID, limit int) ([]models.AgentActivity, error) {
	rows, err := s.db.Query(
		`SELECT id, api_key_id, project_id, action, environment, secret_key, ip_address, created_at
		FROM agent_activities WHERE project_id = $1
		ORDER BY created_at DESC LIMIT $2`, projectID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("getting recent activity: %w", err)
	}
	defer rows.Close()

	var activities []models.AgentActivity
	for rows.Next() {
		var a models.AgentActivity
		if err := rows.Scan(&a.ID, &a.APIKeyID, &a.ProjectID, &a.Action,
			&a.Environment, &a.SecretKey, &a.IPAddress, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning activity: %w", err)
		}
		activities = append(activities, a)
	}
	return activities, nil
}

// GetAccessHeatmap returns secret access frequency data.
func (s *AgentAnalyticsService) GetAccessHeatmap(projectID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT secret_key, environment, COUNT(*) as access_count,
		DATE_TRUNC('hour', created_at) as hour
		FROM agent_activities
		WHERE project_id = $1 AND secret_key != '' AND created_at > NOW() - INTERVAL '7 days'
		GROUP BY secret_key, environment, hour
		ORDER BY access_count DESC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting access heatmap: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var secretKey, environment string
		var count int
		var hour time.Time
		if err := rows.Scan(&secretKey, &environment, &count, &hour); err != nil {
			return nil, fmt.Errorf("scanning heatmap: %w", err)
		}
		results = append(results, map[string]interface{}{
			"secret_key":   secretKey,
			"environment":  environment,
			"access_count": count,
			"hour":         hour,
		})
	}
	return results, nil
}
