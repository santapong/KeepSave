package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type EnvironmentRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewEnvironmentRepository(db *sql.DB, dialect Dialect) *EnvironmentRepository {
	return &EnvironmentRepository{db: db, dialect: dialect}
}

func (r *EnvironmentRepository) CreateDefaultsForProject(projectID uuid.UUID) ([]models.Environment, error) {
	names := []string{"alpha", "uat", "prod"}
	var envs []models.Environment

	for _, name := range names {
		env := models.Environment{}
		id := uuid.New()

		if r.dialect.SupportsReturning() {
			err := r.db.QueryRow(
				`INSERT INTO environments (id, project_id, name) VALUES ($1, $2, $3)
				 RETURNING id, project_id, name, created_at`,
				id, projectID, name,
			).Scan(&env.ID, &env.ProjectID, &env.Name, &env.CreatedAt)
			if err != nil {
				return nil, fmt.Errorf("creating environment %s: %w", name, err)
			}
		} else {
			insertQ := Q(r.dialect, `INSERT INTO environments (id, project_id, name) VALUES ($1, $2, $3)`)
			_, err := r.db.Exec(insertQ, id, projectID, name)
			if err != nil {
				return nil, fmt.Errorf("creating environment %s: %w", name, err)
			}
			selectQ := Q(r.dialect, `SELECT id, project_id, name, created_at FROM environments WHERE id = $1`)
			err = r.db.QueryRow(selectQ, id).Scan(&env.ID, &env.ProjectID, &env.Name, &env.CreatedAt)
			if err != nil {
				return nil, fmt.Errorf("reading created environment %s: %w", name, err)
			}
		}
		envs = append(envs, env)
	}
	return envs, nil
}

func (r *EnvironmentRepository) ListByProjectID(projectID uuid.UUID) ([]models.Environment, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, project_id, name, created_at FROM environments WHERE project_id = $1 ORDER BY name`),
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing environments: %w", err)
	}
	defer rows.Close()

	var envs []models.Environment
	for rows.Next() {
		var e models.Environment
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Name, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning environment: %w", err)
		}
		envs = append(envs, e)
	}
	return envs, rows.Err()
}

func (r *EnvironmentRepository) GetByProjectAndName(projectID uuid.UUID, name string) (*models.Environment, error) {
	env := &models.Environment{}
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT id, project_id, name, created_at FROM environments WHERE project_id = $1 AND name = $2`),
		projectID, name,
	).Scan(&env.ID, &env.ProjectID, &env.Name, &env.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting environment: %w", err)
	}
	return env, nil
}
