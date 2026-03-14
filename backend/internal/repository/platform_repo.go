package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// AccessPolicyRepository handles access policy persistence.
type AccessPolicyRepository struct {
	db *sql.DB
}

// NewAccessPolicyRepository creates a new access policy repository.
func NewAccessPolicyRepository(db *sql.DB) *AccessPolicyRepository {
	return &AccessPolicyRepository{db: db}
}

// GetPolicies returns access policies for a project.
func (r *AccessPolicyRepository) GetPolicies(projectID uuid.UUID) ([]models.AccessPolicy, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, policy_type, config, enabled, created_by, created_at, updated_at
		FROM access_policies WHERE project_id = $1 ORDER BY created_at`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing access policies: %w", err)
	}
	defer rows.Close()

	var policies []models.AccessPolicy
	for rows.Next() {
		var p models.AccessPolicy
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.PolicyType, &p.Config, &p.Enabled,
			&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning access policy: %w", err)
		}
		policies = append(policies, p)
	}
	return policies, nil
}

// CreatePolicy creates a new access policy.
func (r *AccessPolicyRepository) CreatePolicy(policy *models.AccessPolicy) (*models.AccessPolicy, error) {
	err := r.db.QueryRow(
		`INSERT INTO access_policies (project_id, policy_type, config, enabled, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		policy.ProjectID, policy.PolicyType, policy.Config, policy.Enabled, policy.CreatedBy,
	).Scan(&policy.ID, &policy.CreatedAt, &policy.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating access policy: %w", err)
	}
	return policy, nil
}

// DeletePolicy removes an access policy.
func (r *AccessPolicyRepository) DeletePolicy(policyID uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM access_policies WHERE id = $1`, policyID)
	if err != nil {
		return fmt.Errorf("deleting access policy: %w", err)
	}
	return nil
}
