package repository

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"time"
)

func (r *APIKeyRepository) Dialect() Dialect                    { return r.dialect }
func (r *APIKeyRepository) WithTx(fn func(*sql.Tx) error) error { return runInTx(r.db, fn) }
func (r *APIKeyRepository) CreateTx(ctx context.Context, tx *sql.Tx, name, hash string, user, project uuid.UUID, scopes []string, environment *string, expiry time.Time) (*models.APIKey, error) {
	id := uuid.New()
	_, err := tx.ExecContext(ctx, Q(r.dialect, `INSERT INTO api_keys(id,name,hashed_key,user_id,project_id,scopes,environment,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`), id, name, hash, user, project, r.arrayParam(scopes), environment, expiry)
	if err != nil {
		return nil, err
	}
	return r.GetByIDTx(ctx, tx, id)
}
func (r *APIKeyRepository) GetByIDTx(ctx context.Context, tx *sql.Tx, id uuid.UUID) (*models.APIKey, error) {
	k := &models.APIKey{}
	var env sql.NullString
	err := tx.QueryRowContext(ctx, Q(r.dialect, `SELECT id,name,hashed_key,user_id,project_id,scopes,environment,expires_at,created_at FROM api_keys WHERE id=$1`), id).Scan(&k.ID, &k.Name, &k.HashedKey, &k.UserID, &k.ProjectID, &k.Scopes, &env, dbTime(&k.ExpiresAt), dbTime(&k.CreatedAt))
	if env.Valid {
		k.Environment = &env.String
	}
	return k, err
}
func (r *APIKeyRepository) DeleteTx(ctx context.Context, tx *sql.Tx, id, user uuid.UUID) error {
	result, err := tx.ExecContext(ctx, Q(r.dialect, `DELETE FROM api_keys WHERE id=$1 AND user_id=$2`), id, user)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}
func (r *ProjectRepository) RequireOwnedTx(ctx context.Context, tx *sql.Tx, id, user uuid.UUID) error {
	query := `SELECT owner_id FROM projects WHERE id=$1 AND deleted_at IS NULL AND (organization_id IS NULL OR EXISTS(SELECT 1 FROM organization_members om WHERE om.organization_id=projects.organization_id AND om.user_id=$2 AND om.role='admin'))`
	if r.dialect.DBType() != DBTypeSQLite {
		query += ` FOR UPDATE`
	}
	var owner uuid.UUID
	if err := tx.QueryRowContext(ctx, Q(r.dialect, query), id, user).Scan(&owner); err != nil {
		return err
	}
	if owner != user {
		return sql.ErrNoRows
	}
	return nil
}
func (r *ProjectRepository) UpdateMetadataTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, name, description string) error {
	_, err := tx.ExecContext(ctx, Q(r.dialect, `UPDATE projects SET name=$1,description=$2,updated_at=`+r.dialect.Now()+` WHERE id=$3 AND deleted_at IS NULL`), name, description, id)
	return err
}
func (r *ProjectRepository) UpdateEmbedConfigTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, origins []string, enabled bool) error {
	value, err := r.dialect.ArrayParam(origins)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, Q(r.dialect, `UPDATE projects SET allowed_origins=$1,embed_policy_enabled=$2,updated_at=`+r.dialect.Now()+` WHERE id=$3 AND deleted_at IS NULL`), value, enabled, id)
	return err
}
func (r *ProjectRepository) EmbedOriginsTx(ctx context.Context, tx *sql.Tx, id uuid.UUID) ([]string, error) {
	var origins models.StringList
	err := tx.QueryRowContext(ctx, Q(r.dialect, `SELECT allowed_origins FROM projects WHERE id=$1 AND deleted_at IS NULL`), id).Scan(&origins)
	return []string(origins), err
}
