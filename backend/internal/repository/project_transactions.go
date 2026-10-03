package repository

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

func (r *ProjectRepository) Dialect() Dialect { return r.dialect }

// GetByIDTx reads the result under the caller's current authority barrier.
// Authorization and any required project lock belong to the caller.
func (r *ProjectRepository) GetByIDTx(ctx context.Context, tx *sql.Tx, id uuid.UUID) (*models.Project, error) {
	p := new(models.Project)
	err := r.scanProject(p, tx.QueryRowContext(ctx, Q(r.dialect, `SELECT `+projectSelectColumns+` FROM projects WHERE id=$1 AND deleted_at IS NULL`), id))
	if err != nil {
		return nil, err
	}
	return p, nil
}
func (r *ProjectRepository) CreateTx(tx *sql.Tx, name, description string, owner uuid.UUID, cipher, nonce []byte) (*models.Project, error) {
	id := uuid.New()
	if _, err := ExecQ(tx, r.dialect, `INSERT INTO projects(id,name,description,owner_id,encrypted_dek,dek_nonce) VALUES($1,$2,$3,$4,$5,$6)`, id, name, description, owner, cipher, nonce); err != nil {
		return nil, err
	}
	for _, name := range []string{"alpha", "uat", "prod"} {
		if _, err := ExecQ(tx, r.dialect, `INSERT INTO environments(id,project_id,name) VALUES($1,$2,$3)`, uuid.New(), id, name); err != nil {
			return nil, err
		}
	}
	p := new(models.Project)
	err := r.scanProject(p, tx.QueryRow(Q(r.dialect, `SELECT `+projectSelectColumns+` FROM projects WHERE id=$1`), id))
	return p, err
}
func (r *ProjectRepository) IsDeletedOwned(id, user uuid.UUID) (bool, error) {
	var ok bool
	err := r.db.QueryRow(Q(r.dialect, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND owner_id=$2 AND deleted_at IS NOT NULL)`), id, user).Scan(&ok)
	return ok, err
}
func (r *ProjectRepository) TombstoneTx(tx *sql.Tx, id, user uuid.UUID) error {
	var owner uuid.UUID
	query := `SELECT owner_id FROM projects WHERE id=$1 AND deleted_at IS NULL`
	if r.dialect.DBType() == DBTypePostgres || r.dialect.DBType() == DBTypeMySQL {
		query += ` FOR UPDATE`
	}
	if err := tx.QueryRow(Q(r.dialect, query), id).Scan(&owner); err != nil {
		return err
	}
	if owner != user {
		return sql.ErrNoRows
	}
	if _, err := ExecQ(tx, r.dialect, `UPDATE projects SET deleted_at=`+r.dialect.Now()+` WHERE id=$1`, id); err != nil {
		return err
	}
	_, err := ExecQ(tx, r.dialect, `DELETE FROM api_keys WHERE project_id=$1`, id)
	return err
}
