package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type SecretRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewSecretRepository(db *sql.DB, dialect Dialect) *SecretRepository {
	return &SecretRepository{db: db, dialect: dialect}
}

func (r *SecretRepository) Create(projectID, environmentID uuid.UUID, key string, encryptedValue, valueNonce []byte) (*models.Secret, error) {
	s := &models.Secret{}
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at`,
			id, projectID, environmentID, key, encryptedValue, valueNonce,
		).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating secret: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce) VALUES ($1, $2, $3, $4, $5, $6)`)
		_, err := r.db.Exec(insertQ, id, projectID, environmentID, key, encryptedValue, valueNonce)
		if err != nil {
			return nil, fmt.Errorf("creating secret: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at FROM secrets WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading created secret: %w", err)
		}
	}
	return s, nil
}

func (r *SecretRepository) GetByID(id uuid.UUID) (*models.Secret, error) {
	s := &models.Secret{}
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at
		 FROM secrets WHERE id = $1`),
		id,
	).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}
	return s, nil
}

func (r *SecretRepository) ListByProjectAndEnv(projectID, environmentID uuid.UUID) ([]models.Secret, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at
		 FROM secrets WHERE project_id = $1 AND environment_id = $2 ORDER BY key`),
		projectID, environmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}
	defer rows.Close()

	var secrets []models.Secret
	for rows.Next() {
		var s models.Secret
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning secret: %w", err)
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

func (r *SecretRepository) Update(id uuid.UUID, encryptedValue, valueNonce []byte) (*models.Secret, error) {
	s := &models.Secret{}

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			Q(r.dialect, `UPDATE secrets SET encrypted_value = $2, value_nonce = $3, updated_at = NOW()
			 WHERE id = $1
			 RETURNING id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at`),
			id, encryptedValue, valueNonce,
		).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("updating secret: %w", err)
		}
	} else {
		updateQ := Q(r.dialect, `UPDATE secrets SET encrypted_value = $2, value_nonce = $3, updated_at = `+r.dialect.Now()+` WHERE id = $1`)
		_, err := r.db.Exec(updateQ, id, encryptedValue, valueNonce)
		if err != nil {
			return nil, fmt.Errorf("updating secret: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at FROM secrets WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading updated secret: %w", err)
		}
	}
	return s, nil
}

func (r *SecretRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(Q(r.dialect, `DELETE FROM secrets WHERE id = $1`), id)
	if err != nil {
		return fmt.Errorf("deleting secret: %w", err)
	}
	return nil
}

func (r *SecretRepository) Upsert(projectID, environmentID uuid.UUID, key string, encryptedValue, valueNonce []byte) (*models.Secret, error) {
	s := &models.Secret{}
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (environment_id, key) DO UPDATE
			 SET encrypted_value = EXCLUDED.encrypted_value, value_nonce = EXCLUDED.value_nonce, updated_at = NOW()
			 RETURNING id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at`,
			id, projectID, environmentID, key, encryptedValue, valueNonce,
		).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("upserting secret: %w", err)
		}
	} else {
		upsertClause := r.dialect.FormatUpsert("environment_id, key",
			"encrypted_value = EXCLUDED.encrypted_value, value_nonce = EXCLUDED.value_nonce, updated_at = "+r.dialect.Now())
		insertQ := Q(r.dialect, `INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce)
			 VALUES ($1, $2, $3, $4, $5, $6) `+upsertClause)
		_, err := r.db.Exec(insertQ, id, projectID, environmentID, key, encryptedValue, valueNonce)
		if err != nil {
			return nil, fmt.Errorf("upserting secret: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at FROM secrets WHERE environment_id = $1 AND key = $2`)
		err = r.db.QueryRow(selectQ, environmentID, key).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading upserted secret: %w", err)
		}
	}
	return s, nil
}

func (r *SecretRepository) ListByProject(projectID uuid.UUID) ([]models.Secret, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at
		 FROM secrets WHERE project_id = $1 ORDER BY key`),
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing secrets by project: %w", err)
	}
	defer rows.Close()

	var secrets []models.Secret
	for rows.Next() {
		var s models.Secret
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning secret: %w", err)
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

func (r *SecretRepository) GetByEnvAndKey(environmentID uuid.UUID, key string) (*models.Secret, error) {
	s := &models.Secret{}
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at
		 FROM secrets WHERE environment_id = $1 AND key = $2`),
		environmentID, key,
	).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting secret by env and key: %w", err)
	}
	return s, nil
}
