package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type DependencyRepository struct {
	db *sql.DB
}

func NewDependencyRepository(db *sql.DB) *DependencyRepository {
	return &DependencyRepository{db: db}
}

func (r *DependencyRepository) Create(projectID, envID uuid.UUID, secretKey, dependsOnKey, pattern string) (*models.SecretDependency, error) {
	d := &models.SecretDependency{}
	err := r.db.QueryRow(
		`INSERT INTO secret_dependencies (project_id, environment_id, secret_key, depends_on_key, reference_pattern)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (project_id, environment_id, secret_key, depends_on_key) DO UPDATE
		 SET reference_pattern = EXCLUDED.reference_pattern
		 RETURNING id, project_id, environment_id, secret_key, depends_on_key, reference_pattern, created_at`,
		projectID, envID, secretKey, dependsOnKey, pattern,
	).Scan(&d.ID, &d.ProjectID, &d.EnvironmentID, &d.SecretKey, &d.DependsOnKey, &d.ReferencePattern, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating dependency: %w", err)
	}
	return d, nil
}

func (r *DependencyRepository) ListByProjectAndEnv(projectID, envID uuid.UUID) ([]models.SecretDependency, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, environment_id, secret_key, depends_on_key, reference_pattern, created_at
		 FROM secret_dependencies WHERE project_id = $1 AND environment_id = $2
		 ORDER BY secret_key, depends_on_key`,
		projectID, envID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing dependencies: %w", err)
	}
	defer rows.Close()

	var deps []models.SecretDependency
	for rows.Next() {
		var d models.SecretDependency
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.EnvironmentID, &d.SecretKey, &d.DependsOnKey, &d.ReferencePattern, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning dependency: %w", err)
		}
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

func (r *DependencyRepository) DeleteByProjectAndEnv(projectID, envID uuid.UUID) error {
	_, err := r.db.Exec(
		`DELETE FROM secret_dependencies WHERE project_id = $1 AND environment_id = $2`,
		projectID, envID,
	)
	if err != nil {
		return fmt.Errorf("deleting dependencies: %w", err)
	}
	return nil
}

func (r *DependencyRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM secret_dependencies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting dependency: %w", err)
	}
	return nil
}
