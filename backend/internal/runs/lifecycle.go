package runs

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
)

// These hooks run inside the caller's shared exclusive identity/project barrier
// and required audit/outbox transaction. They never acquire lower identity locks.
func (s *Service) ArchiveProjectTx(ctx context.Context, tx *sql.Tx, project uuid.UUID) error {
	if s == nil || s.db == nil {
		return nil
	}
	for _, table := range []string{"tool_connections", "tool_workloads", "tool_grants", "tool_bindings", "tool_profiles", "tool_artifacts"} {
		if _, e := tx.ExecContext(ctx, `UPDATE `+table+` SET revoked_at=COALESCE(revoked_at,NOW()) WHERE project_id=$1`, project); e != nil {
			return e
		}
	}
	if _, e := tx.ExecContext(ctx, `UPDATE tool_profile_packages SET revoked_at=COALESCE(revoked_at,NOW()) WHERE profile_id IN(SELECT id FROM tool_profiles WHERE project_id=$1)`, project); e != nil {
		return e
	}
	return cancelProjectRunsTx(ctx, tx, `project_id=$1`, project)
}

// Keep this affected-authority selection aligned with the identity preview.
const offboardGrantPredicate = `(g.actor_id=$2 OR g.issued_by=$2 OR g.revoked_at IS NOT NULL OR g.binding_id IN(SELECT b.id FROM tool_bindings b JOIN tool_connections c ON c.id=b.connection_id WHERE c.created_by=$2 OR c.revoked_at IS NOT NULL) OR g.workload_id IN(SELECT id FROM tool_workloads WHERE created_by=$2 OR revoked_at IS NOT NULL) OR g.profile_id IN(SELECT id FROM tool_profiles WHERE approved_by=$2) OR g.package_id IN(SELECT id FROM tool_profile_packages WHERE approved_by=$2))`

func (s *Service) OffboardTx(ctx context.Context, tx *sql.Tx, org, target uuid.UUID) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	affected := `SELECT g.id FROM tool_grants g JOIN projects p ON p.id=g.project_id WHERE p.organization_id=$1 AND ` + offboardGrantPredicate
	rows, e := tx.QueryContext(ctx, `SELECT r.id FROM tool_runs r JOIN projects p ON p.id=r.project_id WHERE p.organization_id=$1 AND r.state IN('preparing','active') AND(r.actor_id=$2 OR r.grant_id IN(`+affected+`)) ORDER BY r.id`, org, target)
	if e != nil {
		return 0, e
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return 0, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return 0, e
	}
	for _, table := range []string{"tool_connections", "tool_workloads"} {
		if _, e = tx.ExecContext(ctx, `UPDATE `+table+` SET revoked_at=COALESCE(revoked_at,NOW()) WHERE created_by=$2 AND project_id IN(SELECT id FROM projects WHERE organization_id=$1)`, org, target); e != nil {
			return 0, e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE tool_grants SET revoked_at=COALESCE(revoked_at,NOW()) WHERE id IN(`+affected+`)`, org, target); e != nil {
		return 0, e
	}
	for _, id := range ids {
		if e = cancelRunTx(ctx, tx, id, "revoked"); e != nil {
			return 0, e
		}
	}
	return len(ids), nil
}
func cancelProjectRunsTx(ctx context.Context, tx *sql.Tx, predicate string, value uuid.UUID) error {
	rows, e := tx.QueryContext(ctx, `SELECT id FROM tool_runs WHERE `+predicate+` ORDER BY id FOR UPDATE`, value)
	if e != nil {
		return e
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if e = cancelRunTx(ctx, tx, id, "revoked"); e != nil {
			return e
		}
	}
	return nil
}

func revokeAffectedRunsTx(ctx context.Context, tx *sql.Tx, project, id uuid.UUID, kind string) error {
	predicates := map[string]string{"artifact": "g.profile_id IN(SELECT id FROM tool_profiles WHERE artifact_id=$2)", "profile": "g.profile_id=$2", "package": "g.package_id=$2", "connection": "g.binding_id IN(SELECT id FROM tool_bindings WHERE connection_id=$2)", "binding": "g.binding_id=$2", "workload": "g.workload_id=$2", "grant": "g.id=$2"}
	predicate, ok := predicates[kind]
	if !ok {
		return ErrInvalid
	}
	if _, e := tx.ExecContext(ctx, `UPDATE tool_grants g SET revoked_at=COALESCE(g.revoked_at,NOW()) WHERE g.project_id=$1 AND(`+predicate+`)`, project, id); e != nil {
		return e
	}
	rows, e := tx.QueryContext(ctx, `SELECT r.id FROM tool_runs r JOIN tool_grants g ON g.id=r.grant_id WHERE r.project_id=$1 AND(`+predicate+`) ORDER BY r.id FOR UPDATE OF r`, project, id)
	if e != nil {
		return e
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var run uuid.UUID
		if e = rows.Scan(&run); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, run)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, run := range ids {
		if e = cancelRunTx(ctx, tx, run, "revoked"); e != nil {
			return e
		}
	}
	return nil
}
