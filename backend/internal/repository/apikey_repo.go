package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type APIKeyRepository struct {
	db *sql.DB
}

func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func (r *APIKeyRepository) Create(name, hashedKey string, userID, projectID uuid.UUID, scopes []string, environment *string) (*models.APIKey, error) {
	k := &models.APIKey{}
	var env sql.NullString
	if environment != nil {
		env = sql.NullString{String: *environment, Valid: true}
	}

	err := r.db.QueryRow(
		`INSERT INTO api_keys (name, hashed_key, user_id, project_id, scopes, environment)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, hashed_key, user_id, project_id, scopes, environment, expires_at, created_at`,
		name, hashedKey, userID, projectID, pq.Array(scopes), env,
	).Scan(&k.ID, &k.Name, &k.HashedKey, &k.UserID, &k.ProjectID, pq.Array(&k.Scopes), &env, &k.ExpiresAt, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating api key: %w", err)
	}
	if env.Valid {
		k.Environment = &env.String
	}
	return k, nil
}

func (r *APIKeyRepository) GetByHashedKey(hashedKey string) (*models.APIKey, error) {
	k := &models.APIKey{}
	var env sql.NullString
	err := r.db.QueryRow(
		`SELECT id, name, hashed_key, user_id, project_id, scopes, environment, expires_at, created_at
		 FROM api_keys WHERE hashed_key = $1`,
		hashedKey,
	).Scan(&k.ID, &k.Name, &k.HashedKey, &k.UserID, &k.ProjectID, pq.Array(&k.Scopes), &env, &k.ExpiresAt, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting api key by hash: %w", err)
	}
	if env.Valid {
		k.Environment = &env.String
	}
	return k, nil
}

func (r *APIKeyRepository) ListByUserID(userID uuid.UUID) ([]models.APIKey, error) {
	rows, err := r.db.Query(
		`SELECT id, name, user_id, project_id, scopes, environment, expires_at, created_at
		 FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		var k models.APIKey
		var env sql.NullString
		if err := rows.Scan(&k.ID, &k.Name, &k.UserID, &k.ProjectID, pq.Array(&k.Scopes), &env, &k.ExpiresAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning api key: %w", err)
		}
		if env.Valid {
			k.Environment = &env.String
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *APIKeyRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM api_keys WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting api key: %w", err)
	}
	return nil
}
