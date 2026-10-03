package repository

import (
	"database/sql"
	"sort"

	"github.com/google/uuid"
)

func (r *OrganizationRepository) LockSubjectsTx(tx *sql.Tx, ids ...uuid.UUID) error {
	set := map[uuid.UUID]bool{}
	for _, id := range ids {
		if id == uuid.Nil {
			return sql.ErrNoRows
		}
		set[id] = true
	}
	ordered := make([]uuid.UUID, 0, len(set))
	for id := range set {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	for _, id := range ordered {
		if err := r.LockUserTx(tx, id); err != nil {
			return err
		}
	}
	return nil
}
func (r *OrganizationRepository) ProjectAssignmentTx(tx *sql.Tx, project uuid.UUID) (ProjectAssignment, error) {
	var result ProjectAssignment
	err := QueryRowQ(tx, r.dialect, `SELECT owner_id,organization_id,deleted_at FROM projects WHERE id=$1`, project).Scan(&result.OwnerID, &result.OrganizationID, dbTime(&result.DeletedAt))
	return result, err
}

// Member mutations hold the target epoch before checking the caller session.
// The migration trigger alone advances the epoch when membership changes.
func (r *OrganizationRepository) LockMemberStatesTx(tx *sql.Tx, org uuid.UUID, ids ...uuid.UUID) error {
	if r.dialect.DBType() != DBTypePostgres {
		return nil
	}
	set := map[uuid.UUID]bool{}
	for _, id := range ids {
		set[id] = true
	}
	ordered := make([]uuid.UUID, 0, len(set))
	for id := range set {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	for _, id := range ordered {
		if _, err := tx.Exec(`INSERT INTO member_authority_state(organization_id,user_id,active,epoch) VALUES($1,$2,FALSE,1) ON CONFLICT DO NOTHING`, org, id); err != nil {
			return err
		}
		var epoch int64
		if err := tx.QueryRow(`SELECT epoch FROM member_authority_state WHERE organization_id=$1 AND user_id=$2 FOR UPDATE`, org, id).Scan(&epoch); err != nil {
			return err
		}
	}
	return nil
}
