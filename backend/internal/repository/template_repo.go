package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type TemplateRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewTemplateRepository(db *sql.DB, dialect Dialect) *TemplateRepository {
	return &TemplateRepository{db: db, dialect: dialect}
}

func (r *TemplateRepository) Create(name, description, stack string, keys models.JSONMap, createdBy uuid.UUID, orgID *uuid.UUID, isGlobal bool) (*models.SecretTemplate, error) {
	t := &models.SecretTemplate{}
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO secret_templates (id, name, description, stack, keys, created_by, organization_id, is_global)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING id, name, description, stack, keys, created_by, organization_id, is_global, created_at, updated_at`,
			id, name, description, stack, keys, createdBy, orgID, isGlobal,
		).Scan(&t.ID, &t.Name, &t.Description, &t.Stack, &t.Keys, &t.CreatedBy, &t.OrganizationID, &t.IsGlobal, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating template: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO secret_templates (id, name, description, stack, keys, created_by, organization_id, is_global) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`)
		_, err := r.db.Exec(insertQ, id, name, description, stack, keys, createdBy, orgID, isGlobal)
		if err != nil {
			return nil, fmt.Errorf("creating template: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, name, description, stack, keys, created_by, organization_id, is_global, created_at, updated_at FROM secret_templates WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&t.ID, &t.Name, &t.Description, &t.Stack, &t.Keys, &t.CreatedBy, &t.OrganizationID, &t.IsGlobal, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading created template: %w", err)
		}
	}
	return t, nil
}

func (r *TemplateRepository) GetByID(id uuid.UUID) (*models.SecretTemplate, error) {
	t := &models.SecretTemplate{}
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT id, name, description, stack, keys, created_by, organization_id, is_global, created_at, updated_at
		 FROM secret_templates WHERE id = $1`),
		id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.Stack, &t.Keys, &t.CreatedBy, &t.OrganizationID, &t.IsGlobal, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting template: %w", err)
	}
	return t, nil
}

func (r *TemplateRepository) ListGlobal() ([]models.SecretTemplate, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, name, description, stack, keys, created_by, organization_id, is_global, created_at, updated_at
		 FROM secret_templates WHERE is_global = `+r.dialect.BoolLiteral(true)+` ORDER BY stack, name`),
	)
	if err != nil {
		return nil, fmt.Errorf("listing global templates: %w", err)
	}
	defer rows.Close()
	return r.scanTemplates(rows)
}

func (r *TemplateRepository) ListByOrganization(orgID uuid.UUID) ([]models.SecretTemplate, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, name, description, stack, keys, created_by, organization_id, is_global, created_at, updated_at
		 FROM secret_templates WHERE organization_id = $1 OR is_global = `+r.dialect.BoolLiteral(true)+` ORDER BY stack, name`),
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing organization templates: %w", err)
	}
	defer rows.Close()
	return r.scanTemplates(rows)
}

func (r *TemplateRepository) ListByUser(userID uuid.UUID) ([]models.SecretTemplate, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, name, description, stack, keys, created_by, organization_id, is_global, created_at, updated_at
		 FROM secret_templates WHERE created_by = $1 OR is_global = `+r.dialect.BoolLiteral(true)+` ORDER BY stack, name`),
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing user templates: %w", err)
	}
	defer rows.Close()
	return r.scanTemplates(rows)
}

func (r *TemplateRepository) Update(id uuid.UUID, name, description, stack string, keys models.JSONMap) (*models.SecretTemplate, error) {
	t := &models.SecretTemplate{}

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			Q(r.dialect, `UPDATE secret_templates SET name = $2, description = $3, stack = $4, keys = $5, updated_at = NOW()
			 WHERE id = $1
			 RETURNING id, name, description, stack, keys, created_by, organization_id, is_global, created_at, updated_at`),
			id, name, description, stack, keys,
		).Scan(&t.ID, &t.Name, &t.Description, &t.Stack, &t.Keys, &t.CreatedBy, &t.OrganizationID, &t.IsGlobal, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("updating template: %w", err)
		}
	} else {
		updateQ := Q(r.dialect, `UPDATE secret_templates SET name = $2, description = $3, stack = $4, keys = $5, updated_at = `+r.dialect.Now()+` WHERE id = $1`)
		_, err := r.db.Exec(updateQ, id, name, description, stack, keys)
		if err != nil {
			return nil, fmt.Errorf("updating template: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, name, description, stack, keys, created_by, organization_id, is_global, created_at, updated_at FROM secret_templates WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&t.ID, &t.Name, &t.Description, &t.Stack, &t.Keys, &t.CreatedBy, &t.OrganizationID, &t.IsGlobal, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading updated template: %w", err)
		}
	}
	return t, nil
}

func (r *TemplateRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(Q(r.dialect, `DELETE FROM secret_templates WHERE id = $1`), id)
	if err != nil {
		return fmt.Errorf("deleting template: %w", err)
	}
	return nil
}

func (r *TemplateRepository) scanTemplates(rows *sql.Rows) ([]models.SecretTemplate, error) {
	var templates []models.SecretTemplate
	for rows.Next() {
		var t models.SecretTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Stack, &t.Keys, &t.CreatedBy, &t.OrganizationID, &t.IsGlobal, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning template: %w", err)
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}
