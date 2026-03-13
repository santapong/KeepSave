package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// SecretVersionRepository manages secret version history.
type SecretVersionRepository struct {
	db *sql.DB
}

// NewSecretVersionRepository creates a new secret version repository.
func NewSecretVersionRepository(db *sql.DB) *SecretVersionRepository {
	return &SecretVersionRepository{db: db}
}

// CreateVersion stores a new version of a secret.
func (r *SecretVersionRepository) CreateVersion(secretID, projectID, envID uuid.UUID, version int, encryptedValue, valueNonce []byte, createdBy *uuid.UUID) (*models.SecretVersion, error) {
	sv := &models.SecretVersion{}
	err := r.db.QueryRow(
		`INSERT INTO secret_versions (secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by, created_at`,
		secretID, projectID, envID, version, encryptedValue, valueNonce, createdBy,
	).Scan(&sv.ID, &sv.SecretID, &sv.ProjectID, &sv.EnvironmentID, &sv.Version, &sv.EncryptedValue, &sv.ValueNonce, &sv.CreatedBy, &sv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating secret version: %w", err)
	}
	return sv, nil
}

// ListVersions returns all versions of a secret, newest first.
func (r *SecretVersionRepository) ListVersions(secretID uuid.UUID) ([]models.SecretVersion, error) {
	rows, err := r.db.Query(
		`SELECT id, secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by, created_at
		 FROM secret_versions WHERE secret_id = $1 ORDER BY version DESC`,
		secretID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing secret versions: %w", err)
	}
	defer rows.Close()

	var versions []models.SecretVersion
	for rows.Next() {
		var sv models.SecretVersion
		if err := rows.Scan(&sv.ID, &sv.SecretID, &sv.ProjectID, &sv.EnvironmentID, &sv.Version, &sv.EncryptedValue, &sv.ValueNonce, &sv.CreatedBy, &sv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning secret version: %w", err)
		}
		versions = append(versions, sv)
	}
	return versions, rows.Err()
}

// GetLatestVersion returns the latest version number for a secret.
func (r *SecretVersionRepository) GetLatestVersion(secretID uuid.UUID) (int, error) {
	var version int
	err := r.db.QueryRow(
		`SELECT COALESCE(MAX(version), 0) FROM secret_versions WHERE secret_id = $1`,
		secretID,
	).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("getting latest version: %w", err)
	}
	return version, nil
}

// GetVersion returns a specific version of a secret.
func (r *SecretVersionRepository) GetVersion(secretID uuid.UUID, version int) (*models.SecretVersion, error) {
	sv := &models.SecretVersion{}
	err := r.db.QueryRow(
		`SELECT id, secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by, created_at
		 FROM secret_versions WHERE secret_id = $1 AND version = $2`,
		secretID, version,
	).Scan(&sv.ID, &sv.SecretID, &sv.ProjectID, &sv.EnvironmentID, &sv.Version, &sv.EncryptedValue, &sv.ValueNonce, &sv.CreatedBy, &sv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting secret version: %w", err)
	}
	return sv, nil
}

// PruneOldVersions keeps only the N most recent versions for a secret.
func (r *SecretVersionRepository) PruneOldVersions(secretID uuid.UUID, keepN int) (int64, error) {
	result, err := r.db.Exec(
		`DELETE FROM secret_versions
		 WHERE secret_id = $1 AND version NOT IN (
		   SELECT version FROM secret_versions WHERE secret_id = $1 ORDER BY version DESC LIMIT $2
		 )`,
		secretID, keepN,
	)
	if err != nil {
		return 0, fmt.Errorf("pruning old versions: %w", err)
	}
	return result.RowsAffected()
}
