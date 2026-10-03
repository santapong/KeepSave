package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// Organization mutations use one transaction for authority, data and required
// audit. Locks serialize membership changes with workspace administration.
func (r *OrganizationRepository) WithTx(fn func(*sql.Tx) error) error {
	return runInTx(r.db, fn)
}

func (r *OrganizationRepository) SupportsDurableJobs() bool {
	return r.dialect.DBType() == DBTypePostgres
}

func (r *OrganizationRepository) LockUserTx(tx *sql.Tx, userID uuid.UUID) error {
	if r.dialect.DBType() == DBTypeSQLite {
		if _, err := ExecQ(tx, r.dialect, `UPDATE users SET id=id WHERE id=$1`, userID); err != nil {
			return err
		}
	}
	query := `SELECT id FROM users WHERE id=$1`
	if r.dialect.DBType() != DBTypeSQLite {
		query += ` FOR UPDATE`
	}
	var stored uuid.UUID
	return QueryRowQ(tx, r.dialect, query, userID).Scan(&stored)
}

func scanOrganization(row rowScanner) (*models.Organization, error) {
	o := &models.Organization{}
	err := row.Scan(&o.ID, &o.Name, &o.Slug, &o.OwnerID, dbTime(&o.CreatedAt), dbTime(&o.UpdatedAt))
	return o, err
}

func (r *OrganizationRepository) GetByIDTx(tx *sql.Tx, id uuid.UUID) (*models.Organization, error) {
	return scanOrganization(QueryRowQ(tx, r.dialect,
		`SELECT id,name,slug,owner_id,created_at,updated_at FROM organizations WHERE id=$1`, id))
}

func (r *OrganizationRepository) LockOrganizationTx(tx *sql.Tx, id uuid.UUID) (*models.Organization, error) {
	if r.dialect.DBType() == DBTypeSQLite {
		if _, err := ExecQ(tx, r.dialect, `UPDATE organizations SET id=id WHERE id=$1`, id); err != nil {
			return nil, err
		}
	}
	query := `SELECT id,name,slug,owner_id,created_at,updated_at FROM organizations WHERE id=$1`
	if r.dialect.DBType() != DBTypeSQLite {
		query += ` FOR UPDATE`
	}
	return scanOrganization(QueryRowQ(tx, r.dialect, query, id))
}

func (r *OrganizationRepository) CreateTx(tx *sql.Tx, id uuid.UUID, name, slug string, ownerID uuid.UUID) (*models.Organization, error) {
	if _, err := ExecQ(tx, r.dialect, `INSERT INTO organizations(id,name,slug,owner_id) VALUES($1,$2,$3,$4)`, id, name, slug, ownerID); err != nil {
		return nil, fmt.Errorf("creating organization: %w", err)
	}
	return r.GetByIDTx(tx, id)
}

func (r *OrganizationRepository) UpdateTx(tx *sql.Tx, id uuid.UUID, name string) (*models.Organization, error) {
	if _, err := ExecQ(tx, r.dialect, `UPDATE organizations SET name=$2,updated_at=NOW() WHERE id=$1`, id, name); err != nil {
		return nil, err
	}
	return r.GetByIDTx(tx, id)
}

func (r *OrganizationRepository) GetMemberTx(tx *sql.Tx, orgID, userID uuid.UUID) (*models.OrgMember, error) {
	m := &models.OrgMember{}
	err := QueryRowQ(tx, r.dialect, `SELECT id,organization_id,user_id,role,created_at,updated_at FROM organization_members WHERE organization_id=$1 AND user_id=$2`, orgID, userID).
		Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, dbTime(&m.CreatedAt), dbTime(&m.UpdatedAt))
	return m, err
}

func (r *OrganizationRepository) AddMemberTx(tx *sql.Tx, orgID, userID uuid.UUID, role string) (*models.OrgMember, error) {
	clause := r.dialect.FormatUpsert("organization_id, user_id", "role = EXCLUDED.role, updated_at = "+r.dialect.Now())
	if _, err := ExecQ(tx, r.dialect, `INSERT INTO organization_members(id,organization_id,user_id,role) VALUES($1,$2,$3,$4) `+clause, uuid.New(), orgID, userID, role); err != nil {
		return nil, err
	}
	return r.GetMemberTx(tx, orgID, userID)
}

func (r *OrganizationRepository) UpdateMemberRoleTx(tx *sql.Tx, orgID, userID uuid.UUID, role string) (*models.OrgMember, error) {
	_, err := ExecQ(tx, r.dialect, `UPDATE organization_members SET role=$3,updated_at=NOW() WHERE organization_id=$1 AND user_id=$2`, orgID, userID, role)
	if err != nil {
		return nil, err
	}
	return r.GetMemberTx(tx, orgID, userID)
}

func (r *OrganizationRepository) RemoveMemberTx(tx *sql.Tx, orgID, userID uuid.UUID) error {
	result, err := ExecQ(tx, r.dialect, `DELETE FROM organization_members WHERE organization_id=$1 AND user_id=$2`, orgID, userID)
	if err != nil {
		return err
	}
	if err := requireAffectedRow(result); err != nil {
		return err
	}
	return nil
}

func (r *OrganizationRepository) CountProjectsTx(tx *sql.Tx, orgID uuid.UUID) (int, error) {
	var count int
	err := QueryRowQ(tx, r.dialect, `SELECT COUNT(*) FROM projects WHERE organization_id=$1`, orgID).Scan(&count)
	return count, err
}

func (r *OrganizationRepository) DeleteTx(tx *sql.Tx, id uuid.UUID) error {
	result, err := ExecQ(tx, r.dialect, `DELETE FROM organizations WHERE id=$1`, id)
	if err != nil {
		return err
	}
	return requireAffectedRow(result)
}

// ProjectAssignment contains stored authority only, never credential material.
type ProjectAssignment struct {
	OwnerID        uuid.UUID
	OrganizationID uuid.NullUUID
	DeletedAt      sql.NullTime
}

func (r *OrganizationRepository) LockProjectAssignmentTx(tx *sql.Tx, projectID uuid.UUID) (ProjectAssignment, error) {
	if r.dialect.DBType() == DBTypeSQLite {
		if _, err := ExecQ(tx, r.dialect, `UPDATE projects SET id=id WHERE id=$1`, projectID); err != nil {
			return ProjectAssignment{}, err
		}
	}
	query := `SELECT owner_id,organization_id,deleted_at FROM projects WHERE id=$1`
	if r.dialect.DBType() != DBTypeSQLite {
		query += ` FOR UPDATE`
	}
	var p ProjectAssignment
	err := QueryRowQ(tx, r.dialect, query, projectID).Scan(&p.OwnerID, &p.OrganizationID, dbTime(&p.DeletedAt))
	return p, err
}

func (r *OrganizationRepository) AssignPersonalProjectTx(tx *sql.Tx, projectID, ownerID, orgID uuid.UUID) error {
	result, err := ExecQ(tx, r.dialect, `UPDATE projects SET organization_id=$3,updated_at=NOW() WHERE id=$1 AND owner_id=$2 AND organization_id IS NULL AND deleted_at IS NULL`, projectID, ownerID, orgID)
	if err != nil {
		return err
	}
	return requireAffectedRow(result)
}

// Changing tenancy ends source-scope delegated access. Deleting parent keys
// cascades their leases/issuances; token validation rejects the missing parent.
func (r *OrganizationRepository) RevokeProjectDelegationTx(tx *sql.Tx, projectID uuid.UUID) (int64, error) {
	if _, err := ExecQ(tx, r.dialect, `UPDATE secret_leases SET revoked=`+r.dialect.BoolLiteral(true)+`,revoked_at=NOW() WHERE project_id=$1 AND revoked=`+r.dialect.BoolLiteral(false), projectID); err != nil {
		return 0, err
	}
	result, err := ExecQ(tx, r.dialect, `DELETE FROM api_keys WHERE project_id=$1`, projectID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *OrganizationRepository) GetWorkspaceRequestTx(tx *sql.Tx, userID uuid.UUID, key string) (string, uuid.UUID, error) {
	var digest string
	var orgID uuid.UUID
	err := QueryRowQ(tx, r.dialect, `SELECT request_digest,organization_id FROM workspace_creation_requests WHERE user_id=$1 AND request_key=$2`, userID, key).Scan(&digest, &orgID)
	return digest, orgID, err
}

func (r *OrganizationRepository) CreateWorkspaceRequestTx(tx *sql.Tx, userID uuid.UUID, key, digest string, orgID uuid.UUID) error {
	_, err := ExecQ(tx, r.dialect, `INSERT INTO workspace_creation_requests(user_id,request_key,request_digest,organization_id) VALUES($1,$2,$3,$4)`, userID, key, digest, orgID)
	return err
}

func requireAffectedRow(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}
