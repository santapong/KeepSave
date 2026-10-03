package runs

import (
	"context"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"time"
)

// ReconcileExpired is trusted local maintenance, not an authority grant. It only
// reduces expired authority, clears expired encrypted result spools, and records
// uncertain dispatches. It never decrypts, dispatches or replays provider work.
func (s *Service) ReconcileExpired(ctx context.Context) (int, error) {
	if s == nil || s.db == nil || s.audit == nil {
		return 0, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, e := s.db.QueryContext(ctx, `SELECT DISTINCT r.id FROM tool_runs r LEFT JOIN tool_operations o ON o.run_id=r.id LEFT JOIN tool_resolution_attempts a ON a.run_id=r.id LEFT JOIN tool_result_spool z ON z.operation_id=o.id WHERE(r.expires_at<=NOW() AND r.state IN('preparing','active')) OR(o.status IN('leased','dispatched') AND o.deadline<=NOW()) OR(a.state IN('admitted','dispatched') AND a.deadline<=NOW()) OR z.expires_at<=NOW() ORDER BY r.id LIMIT 100`)
	if e != nil {
		return 0, ErrUnavailable
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
	count, e := s.reconcileChecks(ctx)
	if e != nil {
		return 0, e
	}
	for _, id := range ids {
		changed, e := s.reconcileRun(ctx, id)
		if e != nil {
			return count, e
		}
		if changed {
			count++
		}
	}
	return count, nil
}
func (s *Service) reconcileRun(ctx context.Context, id uuid.UUID) (bool, error) {
	project, subjects, e := s.discoverRun(ctx, id)
	if e != nil {
		return false, e
	}
	if s.LockMaintenanceSubjects == nil {
		return false, ErrUnavailable
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s';SET LOCAL statement_timeout='10s'`); e != nil {
		return false, e
	}
	if e = s.LockMaintenanceSubjects(ctx, tx, project, subjects); e != nil {
		return false, e
	}
	r, e := readRun(ctx, tx, id, true)
	if e != nil {
		return false, e
	}
	p := principalForRun(r)
	p.Kind = policy.Workload // audit attribution remains the immutable original user ID
	changed := false
	if !r.ExpiresAt.After(time.Now()) && (r.State == "active" || r.State == "preparing") {
		if e = cancelRunTx(ctx, tx, id, "revoked"); e != nil {
			return false, e
		}
		changed = true
	}
	ops, e := tx.QueryContext(ctx, `SELECT id FROM tool_operations WHERE run_id=$1 AND status IN('leased','dispatched') AND deadline<=NOW() ORDER BY id`, id)
	if e != nil {
		return false, e
	}
	var opIDs []uuid.UUID
	for ops.Next() {
		var op uuid.UUID
		if e = ops.Scan(&op); e != nil {
			ops.Close()
			return false, e
		}
		opIDs = append(opIDs, op)
	}
	e = ops.Err()
	ops.Close()
	if e != nil {
		return false, e
	}
	for _, opID := range opIDs {
		o, e := readOperation(ctx, tx, opID, true)
		if e != nil {
			return false, e
		}
		if o.Status == "dispatched" {
			if e = s.finishAttemptTx(ctx, tx, p, r, o, "uncertain", "deadline_elapsed", nil); e != nil {
				return false, e
			}
		} else {
			if _, e = tx.ExecContext(ctx, `UPDATE tool_attempts SET state='failed',outcome='pre_dispatch_failure',finished_at=NOW() WHERE operation_id=$1 AND fence=$2 AND state='leased'`, o.ID, o.Fence); e != nil {
				return false, e
			}
			if _, e = tx.ExecContext(ctx, `UPDATE tool_operations SET status='failed',outcome='pre_dispatch_failure',updated_at=NOW() WHERE id=$1`, o.ID); e != nil {
				return false, e
			}
			if _, e = s.receipt(ctx, tx, p, r.ProjectID, r.ID, o.ID, "tool.attempt.expired", "pre_dispatch_failure", models.JSONMap{"fence": o.Fence}); e != nil {
				return false, e
			}
		}
		changed = true
	}
	result, e := tx.ExecContext(ctx, `UPDATE tool_resolution_attempts SET state=CASE WHEN state='dispatched' THEN 'uncertain' ELSE 'failed' END,outcome='deadline_elapsed',finished_at=NOW() WHERE run_id=$1 AND state IN('admitted','dispatched') AND deadline<=NOW()`, id)
	if e != nil {
		return false, e
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		if _, e = tx.ExecContext(ctx, `UPDATE tool_runs SET state='failed' WHERE id=$1 AND state='preparing'`, id); e != nil {
			return false, e
		}
		if _, e = s.receipt(ctx, tx, p, r.ProjectID, id, uuid.Nil, "tool.resolution.expired", "deadline_elapsed", models.JSONMap{}); e != nil {
			return false, e
		}
		changed = true
	}
	result, e = tx.ExecContext(ctx, `DELETE FROM tool_result_spool WHERE operation_id IN(SELECT id FROM tool_operations WHERE run_id=$1) AND expires_at<=NOW()`, id)
	if e != nil {
		return false, e
	}
	n, _ = result.RowsAffected()
	if n > 0 {
		if _, e = s.receipt(ctx, tx, p, r.ProjectID, id, uuid.Nil, "tool.result.expired", "expired", models.JSONMap{"count": n}); e != nil {
			return false, e
		}
		changed = true
	}
	if !changed {
		return false, nil
	}
	return true, tx.Commit()
}

func (s *Service) reconcileChecks(ctx context.Context) (int, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,project_id,actor_id FROM tool_connection_checks WHERE state IN('admitted','dispatched') AND deadline<=NOW() ORDER BY project_id,id LIMIT 100`)
	if e != nil {
		return 0, e
	}
	type target struct{ id, project, actor uuid.UUID }
	var all []target
	for rows.Next() {
		var t target
		if e = rows.Scan(&t.id, &t.project, &t.actor); e != nil {
			rows.Close()
			return 0, e
		}
		all = append(all, t)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return 0, e
	}
	n := 0
	for _, t := range all {
		if s.LockMaintenanceSubjects == nil {
			return n, ErrUnavailable
		}
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return n, e
		}
		if e = s.LockMaintenanceSubjects(ctx, tx, t.project, []uuid.UUID{t.actor}); e != nil {
			tx.Rollback()
			return n, e
		}
		result, e := tx.ExecContext(ctx, `UPDATE tool_connection_checks SET state=CASE WHEN state='dispatched' THEN 'uncertain' ELSE 'failed' END,finished_at=NOW() WHERE id=$1 AND state IN('admitted','dispatched') AND deadline<=NOW()`, t.id)
		if e != nil {
			tx.Rollback()
			return n, e
		}
		changed, _ := result.RowsAffected()
		if changed > 0 {
			p := policy.Principal{Kind: policy.Workload, ActorID: t.actor, SubjectID: t.actor}
			if e = s.event(ctx, tx, p, t.project, "tool.connection.check.expired", models.JSONMap{"check_id": t.id, "outcome": "deadline_elapsed"}); e != nil {
				tx.Rollback()
				return n, e
			}
		}
		if e = tx.Commit(); e != nil {
			return n, e
		}
		n += int(changed)
	}
	return n, nil
}
