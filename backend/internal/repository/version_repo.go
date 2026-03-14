package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type SecretVersionRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewSecretVersionRepository(db *sql.DB, dialect Dialect) *SecretVersionRepository {
	return &SecretVersionRepository{db: db, dialect: dialect}
}

func (r *SecretVersionRepository) CreateVersion(secretID, projectID, envID uuid.UUID, version int, encryptedValue, valueNonce []byte, createdBy *uuid.UUID) (*models.SecretVersion, error) {
	sv := &models.SecretVersion{}
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO secret_versions (id, secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING id, secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by, created_at`,
			id, secretID, projectID, envID, version, encryptedValue, valueNonce, createdBy,
		).Scan(&sv.ID, &sv.SecretID, &sv.ProjectID, &sv.EnvironmentID, &sv.Version, &sv.EncryptedValue, &sv.ValueNonce, &sv.CreatedBy, &sv.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating secret version: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO secret_versions (id, secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`)
		_, err := r.db.Exec(insertQ, id, secretID, projectID, envID, version, encryptedValue, valueNonce, createdBy)
		if err != nil {
			return nil, fmt.Errorf("creating secret version: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by, created_at FROM secret_versions WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&sv.ID, &sv.SecretID, &sv.ProjectID, &sv.EnvironmentID, &sv.Version, &sv.EncryptedValue, &sv.ValueNonce, &sv.CreatedBy, &sv.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading created secret version: %w", err)
		}
	}
	return sv, nil
}

func (r *SecretVersionRepository) ListVersions(secretID uuid.UUID) ([]models.SecretVersion, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by, created_at
		 FROM secret_versions WHERE secret_id = $1 ORDER BY version DESC`),
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

func (r *SecretVersionRepository) GetLatestVersion(secretID uuid.UUID) (int, error) {
	var version int
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT COALESCE(MAX(version), 0) FROM secret_versions WHERE secret_id = $1`),
		secretID,
	).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("getting latest version: %w", err)
	}
	return version, nil
}

func (r *SecretVersionRepository) GetVersion(secretID uuid.UUID, version int) (*models.SecretVersion, error) {
	sv := &models.SecretVersion{}
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT id, secret_id, project_id, environment_id, version, encrypted_value, value_nonce, created_by, created_at
		 FROM secret_versions WHERE secret_id = $1 AND version = $2`),
		secretID, version,
	).Scan(&sv.ID, &sv.SecretID, &sv.ProjectID, &sv.EnvironmentID, &sv.Version, &sv.EncryptedValue, &sv.ValueNonce, &sv.CreatedBy, &sv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting secret version: %w", err)
	}
	return sv, nil
}

func (r *SecretVersionRepository) PruneOldVersions(secretID uuid.UUID, keepN int) (int64, error) {
	// Use a subquery approach that works across all databases
	var query string
	if r.dialect.DBType() == DBTypeMySQL {
		// MySQL doesn't support LIMIT in subqueries with IN, use a different approach
		query = Q(r.dialect, `DELETE FROM secret_versions
		 WHERE secret_id = $1 AND id NOT IN (
		   SELECT id FROM (SELECT id FROM secret_versions WHERE secret_id = $1 ORDER BY version DESC LIMIT $2) AS keep_versions
		 )`)
	} else {
		query = Q(r.dialect, `DELETE FROM secret_versions
		 WHERE secret_id = $1 AND version NOT IN (
		   SELECT version FROM secret_versions WHERE secret_id = $1 ORDER BY version DESC LIMIT $2
		 )`)
	}
	result, err := r.db.Exec(query, secretID, keepN)
	if err != nil {
		return 0, fmt.Errorf("pruning old versions: %w", err)
	}
	return result.RowsAffected()
}
