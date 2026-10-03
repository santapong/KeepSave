package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// AccessPolicyRepository handles access policy persistence.
type AccessPolicyRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewAccessPolicyRepository(db *sql.DB, dialect Dialect) *AccessPolicyRepository {
	return &AccessPolicyRepository{db: db, dialect: dialect}
}

func (r *AccessPolicyRepository) GetPolicies(projectID uuid.UUID) ([]models.AccessPolicy, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, project_id, policy_type, config, enabled, created_by, created_at, updated_at
		FROM access_policies WHERE project_id = $1 ORDER BY created_at`), projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing access policies: %w", err)
	}
	defer rows.Close()

	var policies []models.AccessPolicy
	for rows.Next() {
		var p models.AccessPolicy
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.PolicyType, &p.Config, &p.Enabled,
			&p.CreatedBy, dbTime(&p.CreatedAt), dbTime(&p.UpdatedAt)); err != nil {
			return nil, fmt.Errorf("scanning access policy: %w", err)
		}
		policies = append(policies, p)
	}
	return policies, nil
}

func (r *AccessPolicyRepository) CreatePolicy(policy *models.AccessPolicy) (*models.AccessPolicy, error) {
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO access_policies (id, project_id, policy_type, config, enabled, created_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, created_at, updated_at`,
			id, policy.ProjectID, policy.PolicyType, policy.Config, policy.Enabled, policy.CreatedBy,
		).Scan(&policy.ID, dbTime(&policy.CreatedAt), dbTime(&policy.UpdatedAt))
		if err != nil {
			return nil, fmt.Errorf("creating access policy: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO access_policies (id, project_id, policy_type, config, enabled, created_by) VALUES ($1, $2, $3, $4, $5, $6)`)
		_, err := r.db.Exec(insertQ, id, policy.ProjectID, policy.PolicyType, policy.Config, policy.Enabled, policy.CreatedBy)
		if err != nil {
			return nil, fmt.Errorf("creating access policy: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, created_at, updated_at FROM access_policies WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&policy.ID, dbTime(&policy.CreatedAt), dbTime(&policy.UpdatedAt))
		if err != nil {
			return nil, fmt.Errorf("reading created access policy: %w", err)
		}
	}
	return policy, nil
}

// DeletePolicy removes an access policy, bound to projectID (the route's :id,
// already access-checked) so a caller cannot delete another project's policy —
// and thereby lift its IP/geo/time restrictions — by guessing its id. Zero rows
// affected → sql.ErrNoRows (handler maps to 404).
func (r *AccessPolicyRepository) DeletePolicy(policyID, projectID uuid.UUID) error {
	res, err := ExecQ(r.db, r.dialect, `DELETE FROM access_policies WHERE id = $1 AND project_id = $2`, policyID, projectID)
	if err != nil {
		return fmt.Errorf("deleting access policy: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
