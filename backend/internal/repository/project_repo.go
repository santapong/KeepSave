package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type ProjectRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewProjectRepository(db *sql.DB, dialect Dialect) *ProjectRepository {
	return &ProjectRepository{db: db, dialect: dialect}
}

// projectSelectColumns is the canonical column list for SELECT queries.
// Keep in sync with scanProject and any future Project field additions.
const projectSelectColumns = `id, name, description, owner_id, encrypted_dek, dek_nonce, allowed_origins, embed_policy_enabled, created_at, updated_at`

// scanProject populates p from a row source matching projectSelectColumns.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func (r *ProjectRepository) scanProject(p *models.Project, row rowScanner) error {
	// For PostgreSQL, allowed_origins is a native TEXT[] which pq.Array can
	// decode robustly (including URLs with characters that pg-arrays quote).
	// For SQLite/MySQL it is JSON/TEXT and StringList.Scan handles it.
	if r.dialect.DBType() == DBTypePostgres {
		var origins []string
		if err := row.Scan(
			&p.ID, &p.Name, &p.Description, &p.OwnerID,
			&p.EncryptedDEK, &p.DEKNonce,
			pq.Array(&origins), &p.EmbedPolicyEnabled,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return err
		}
		p.AllowedOrigins = models.StringList(origins)
		return nil
	}
	return row.Scan(
		&p.ID, &p.Name, &p.Description, &p.OwnerID,
		&p.EncryptedDEK, &p.DEKNonce,
		&p.AllowedOrigins, &p.EmbedPolicyEnabled,
		&p.CreatedAt, &p.UpdatedAt,
	)
}

func (r *ProjectRepository) Create(name, description string, ownerID uuid.UUID, encryptedDEK, dekNonce []byte) (*models.Project, error) {
	p := &models.Project{}
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.scanProject(p, r.db.QueryRow(
			`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING `+projectSelectColumns,
			id, name, description, ownerID, encryptedDEK, dekNonce,
		))
		if err != nil {
			return nil, fmt.Errorf("creating project: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES ($1, $2, $3, $4, $5, $6)`)
		_, err := r.db.Exec(insertQ, id, name, description, ownerID, encryptedDEK, dekNonce)
		if err != nil {
			return nil, fmt.Errorf("creating project: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT `+projectSelectColumns+` FROM projects WHERE id = $1`)
		if err := r.scanProject(p, r.db.QueryRow(selectQ, id)); err != nil {
			return nil, fmt.Errorf("reading created project: %w", err)
		}
	}
	return p, nil
}

func (r *ProjectRepository) GetByID(id uuid.UUID) (*models.Project, error) {
	p := &models.Project{}
	err := r.scanProject(p, r.db.QueryRow(
		Q(r.dialect, `SELECT `+projectSelectColumns+` FROM projects WHERE id = $1`),
		id,
	))
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) ListByOwnerID(ownerID uuid.UUID) ([]models.Project, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT `+projectSelectColumns+`
		 FROM projects WHERE owner_id = $1 ORDER BY created_at DESC`),
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := r.scanProject(&p, rows); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepository) Update(id uuid.UUID, name, description string) (*models.Project, error) {
	p := &models.Project{}

	if r.dialect.SupportsReturning() {
		err := r.scanProject(p, r.db.QueryRow(
			Q(r.dialect, `UPDATE projects SET name = $2, description = $3, updated_at = NOW()
			 WHERE id = $1
			 RETURNING `+projectSelectColumns),
			id, name, description,
		))
		if err != nil {
			return nil, fmt.Errorf("updating project: %w", err)
		}
	} else {
		updateQ := Q(r.dialect, `UPDATE projects SET name = $2, description = $3, updated_at = `+r.dialect.Now()+` WHERE id = $1`)
		_, err := r.db.Exec(updateQ, id, name, description)
		if err != nil {
			return nil, fmt.Errorf("updating project: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT `+projectSelectColumns+` FROM projects WHERE id = $1`)
		if err := r.scanProject(p, r.db.QueryRow(selectQ, id)); err != nil {
			return nil, fmt.Errorf("reading updated project: %w", err)
		}
	}
	return p, nil
}

// UpdateEmbedConfig persists the embed-widget allow-list and feature flag for a project.
// Per ADR-0006, this is the ONLY way to set allowed_origins; the API validates the
// wildcard sentinel before calling this method.
func (r *ProjectRepository) UpdateEmbedConfig(id uuid.UUID, allowedOrigins []string, embedPolicyEnabled bool) error {
	arrParam, err := r.dialect.ArrayParam(allowedOrigins)
	if err != nil {
		return fmt.Errorf("encoding allowed_origins: %w", err)
	}
	updateQ := Q(r.dialect, `UPDATE projects SET allowed_origins = $2, embed_policy_enabled = $3, updated_at = `+r.dialect.Now()+` WHERE id = $1`)
	if _, err := r.db.Exec(updateQ, id, arrParam, embedPolicyEnabled); err != nil {
		return fmt.Errorf("updating embed config: %w", err)
	}
	return nil
}

func (r *ProjectRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(Q(r.dialect, `DELETE FROM projects WHERE id = $1`), id)
	if err != nil {
		return fmt.Errorf("deleting project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) UpdateDEK(id uuid.UUID, encryptedDEK, dekNonce []byte) error {
	updateQ := Q(r.dialect, `UPDATE projects SET encrypted_dek = $2, dek_nonce = $3, updated_at = `+r.dialect.Now()+` WHERE id = $1`)
	_, err := r.db.Exec(updateQ, id, encryptedDEK, dekNonce)
	if err != nil {
		return fmt.Errorf("updating project DEK: %w", err)
	}
	return nil
}

func (r *ProjectRepository) ListByOwner(ownerID uuid.UUID) ([]models.Project, error) {
	return r.ListByOwnerID(ownerID)
}

// UserHasAccess returns (true, nil) when userID owns projectID OR is a
// member of the organization the project belongs to. Returns (false, nil)
// when the project exists but the user has no access path. Returns
// (false, sql.ErrNoRows) when the project does not exist — callers should
// surface this as 404 to avoid project-existence enumeration via 403/404
// differential.
func (r *ProjectRepository) UserHasAccess(userID, projectID uuid.UUID) (bool, error) {
	// Single round-trip: covers owner OR org-member. organization_id may be
	// NULL when the project is not assigned to an org (single-user case);
	// in that path only the owner check matches.
	query := Q(r.dialect, `
		SELECT EXISTS (
			SELECT 1 FROM projects p
			WHERE p.id = $1 AND (
				p.owner_id = $2
				OR (
					p.organization_id IS NOT NULL
					AND EXISTS (
						SELECT 1 FROM organization_members om
						WHERE om.organization_id = p.organization_id
						AND om.user_id = $2
					)
				)
			)
		)
	`)
	var allowed bool
	if err := r.db.QueryRow(query, projectID, userID).Scan(&allowed); err != nil {
		return false, fmt.Errorf("checking project access: %w", err)
	}
	if !allowed {
		// Distinguish "no row" from "no access" so the middleware can decide
		// the right status. A separate existence check keeps the security
		// model honest: leaking existence is itself a finding.
		var exists bool
		if err := r.db.QueryRow(Q(r.dialect, `SELECT EXISTS (SELECT 1 FROM projects WHERE id = $1)`), projectID).Scan(&exists); err != nil {
			return false, fmt.Errorf("checking project existence: %w", err)
		}
		if !exists {
			return false, sql.ErrNoRows
		}
	}
	return allowed, nil
}
