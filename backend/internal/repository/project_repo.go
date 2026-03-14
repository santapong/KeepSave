package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type ProjectRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewProjectRepository(db *sql.DB, dialect Dialect) *ProjectRepository {
	return &ProjectRepository{db: db, dialect: dialect}
}

func (r *ProjectRepository) Create(name, description string, ownerID uuid.UUID, encryptedDEK, dekNonce []byte) (*models.Project, error) {
	p := &models.Project{}
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id, name, description, owner_id, encrypted_dek, dek_nonce, created_at, updated_at`,
			id, name, description, ownerID, encryptedDEK, dekNonce,
		).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.EncryptedDEK, &p.DEKNonce, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating project: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES ($1, $2, $3, $4, $5, $6)`)
		_, err := r.db.Exec(insertQ, id, name, description, ownerID, encryptedDEK, dekNonce)
		if err != nil {
			return nil, fmt.Errorf("creating project: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, name, description, owner_id, encrypted_dek, dek_nonce, created_at, updated_at FROM projects WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.EncryptedDEK, &p.DEKNonce, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading created project: %w", err)
		}
	}
	return p, nil
}

func (r *ProjectRepository) GetByID(id uuid.UUID) (*models.Project, error) {
	p := &models.Project{}
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT id, name, description, owner_id, encrypted_dek, dek_nonce, created_at, updated_at
		 FROM projects WHERE id = $1`),
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.EncryptedDEK, &p.DEKNonce, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) ListByOwnerID(ownerID uuid.UUID) ([]models.Project, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, name, description, owner_id, encrypted_dek, dek_nonce, created_at, updated_at
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
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.EncryptedDEK, &p.DEKNonce, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepository) Update(id uuid.UUID, name, description string) (*models.Project, error) {
	p := &models.Project{}

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			Q(r.dialect, `UPDATE projects SET name = $2, description = $3, updated_at = NOW()
			 WHERE id = $1
			 RETURNING id, name, description, owner_id, encrypted_dek, dek_nonce, created_at, updated_at`),
			id, name, description,
		).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.EncryptedDEK, &p.DEKNonce, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("updating project: %w", err)
		}
	} else {
		updateQ := Q(r.dialect, `UPDATE projects SET name = $2, description = $3, updated_at = `+r.dialect.Now()+` WHERE id = $1`)
		_, err := r.db.Exec(updateQ, id, name, description)
		if err != nil {
			return nil, fmt.Errorf("updating project: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, name, description, owner_id, encrypted_dek, dek_nonce, created_at, updated_at FROM projects WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.EncryptedDEK, &p.DEKNonce, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading updated project: %w", err)
		}
	}
	return p, nil
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
