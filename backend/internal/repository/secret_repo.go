package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type SecretRepository struct {
	db *sql.DB
}

func NewSecretRepository(db *sql.DB) *SecretRepository {
	return &SecretRepository{db: db}
}

func (r *SecretRepository) Create(projectID, environmentID uuid.UUID, key string, encryptedValue, valueNonce []byte) (*models.Secret, error) {
	s := &models.Secret{}
	err := r.db.QueryRow(
		`INSERT INTO secrets (project_id, environment_id, key, encrypted_value, value_nonce)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at`,
		projectID, environmentID, key, encryptedValue, valueNonce,
	).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating secret: %w", err)
	}
	return s, nil
}

func (r *SecretRepository) GetByID(id uuid.UUID) (*models.Secret, error) {
	s := &models.Secret{}
	err := r.db.QueryRow(
		`SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at
		 FROM secrets WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}
	return s, nil
}

func (r *SecretRepository) ListByProjectAndEnv(projectID, environmentID uuid.UUID) ([]models.Secret, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at
		 FROM secrets WHERE project_id = $1 AND environment_id = $2 ORDER BY key`,
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
	err := r.db.QueryRow(
		`UPDATE secrets SET encrypted_value = $2, value_nonce = $3, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at`,
		id, encryptedValue, valueNonce,
	).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating secret: %w", err)
	}
	return s, nil
}

func (r *SecretRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM secrets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting secret: %w", err)
	}
	return nil
}

func (r *SecretRepository) Upsert(projectID, environmentID uuid.UUID, key string, encryptedValue, valueNonce []byte) (*models.Secret, error) {
	s := &models.Secret{}
	err := r.db.QueryRow(
		`INSERT INTO secrets (project_id, environment_id, key, encrypted_value, value_nonce)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (environment_id, key) DO UPDATE
		 SET encrypted_value = EXCLUDED.encrypted_value, value_nonce = EXCLUDED.value_nonce, updated_at = NOW()
		 RETURNING id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at`,
		projectID, environmentID, key, encryptedValue, valueNonce,
	).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting secret: %w", err)
	}
	return s, nil
}

func (r *SecretRepository) GetByEnvAndKey(environmentID uuid.UUID, key string) (*models.Secret, error) {
	s := &models.Secret{}
	err := r.db.QueryRow(
		`SELECT id, project_id, environment_id, key, encrypted_value, value_nonce, created_at, updated_at
		 FROM secrets WHERE environment_id = $1 AND key = $2`,
		environmentID, key,
	).Scan(&s.ID, &s.ProjectID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting secret by env and key: %w", err)
	}
	return s, nil
}
