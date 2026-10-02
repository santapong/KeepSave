package repository

import (
	"database/sql"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

const templateColumns = `id,name,description,stack,keys,created_by,organization_id,is_global,created_at,updated_at`

func (r *TemplateRepository) WithTx(fn func(*sql.Tx) error) error { return runInTx(r.db, fn) }
func (r *TemplateRepository) Dialect() Dialect                    { return r.dialect }

func templateScan(row rowScanner) (*models.SecretTemplate, error) {
	t := &models.SecretTemplate{}
	err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Stack, &t.Keys, &t.CreatedBy, &t.OrganizationID, &t.IsGlobal, dbTime(&t.CreatedAt), dbTime(&t.UpdatedAt))
	return t, err
}
func (r *TemplateRepository) GetByIDTx(tx *sql.Tx, id uuid.UUID) (*models.SecretTemplate, error) {
	q := `SELECT ` + templateColumns + ` FROM secret_templates WHERE id=$1`
	if r.dialect.DBType() != DBTypeSQLite {
		q += ` FOR UPDATE`
	}
	return templateScan(QueryRowQ(tx, r.dialect, q, id))
}
func (r *TemplateRepository) CreateTx(tx *sql.Tx, id uuid.UUID, name, description, stack string, keys models.JSONMap, actor uuid.UUID, org *uuid.UUID, global bool) (*models.SecretTemplate, error) {
	if _, err := ExecQ(tx, r.dialect, `INSERT INTO secret_templates(id,name,description,stack,keys,created_by,organization_id,is_global) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, name, description, stack, keys, actor, org, global); err != nil {
		return nil, err
	}
	return r.GetByIDTx(tx, id)
}
func (r *TemplateRepository) UpdateTx(tx *sql.Tx, id uuid.UUID, name, description, stack string, keys models.JSONMap) (*models.SecretTemplate, error) {
	_, err := ExecQ(tx, r.dialect, `UPDATE secret_templates SET name=$2,description=$3,stack=$4,keys=$5,updated_at=`+r.dialect.Now()+` WHERE id=$1`, id, name, description, stack, keys)
	if err != nil {
		return nil, err
	}
	// The row was locked before mutation. MySQL can report zero changed rows for
	// a same-value update, so the authoritative read also verifies its existence.
	return r.GetByIDTx(tx, id)
}
func (r *TemplateRepository) DeleteTx(tx *sql.Tx, id uuid.UUID) error {
	res, err := ExecQ(tx, r.dialect, `DELETE FROM secret_templates WHERE id=$1`, id)
	if err != nil {
		return err
	}
	return requireAffectedRow(res)
}
func (r *TemplateRepository) HasOrganizationRole(user, org uuid.UUID, required string) (bool, error) {
	var role string
	err := r.db.QueryRow(Q(r.dialect, `SELECT m.role FROM organization_members m JOIN organizations o ON o.id=m.organization_id WHERE m.organization_id=$1 AND m.user_id=$2`), org, user).Scan(&role)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return policy.RoleAllows(role, required), err
}
func (r *TemplateRepository) HasOrganizationRoleTx(tx *sql.Tx, user, org uuid.UUID, required string) (bool, error) {
	var role string
	q := `SELECT m.role FROM organization_members m WHERE m.organization_id=$1 AND m.user_id=$2 AND EXISTS(SELECT 1 FROM organizations o WHERE o.id=m.organization_id)`
	if r.dialect.DBType() == DBTypePostgres {
		q += ` FOR UPDATE OF m`
	} else if r.dialect.DBType() == DBTypeMySQL {
		q += ` FOR UPDATE`
	}
	err := QueryRowQ(tx, r.dialect, q, org, user).Scan(&role)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return policy.RoleAllows(role, required), err
}
