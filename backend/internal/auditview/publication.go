package auditview

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"time"
)

// Status and downloads reauthorize current access independently of publication.
func (s *Service) Status(ctx context.Context, p policy.Principal, project, id uuid.UUID) (Export, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return Export{}, e
	}
	defer tx.Rollback()
	if e = s.admit(ctx, tx, p, project); e != nil {
		return Export{}, e
	}
	out := Export{}
	if e = tx.QueryRowContext(ctx, `SELECT id,project_id,state,row_count,expires_at FROM audit_exports WHERE id=$1 AND project_id=$2 AND user_id=$3 AND expires_at>NOW()`, id, project, p.SubjectID).Scan(&out.ID, &out.ProjectID, &out.Status, &out.Rows, &out.ExpiresAt); e != nil {
		return Export{}, ErrDenied
	}
	return out, tx.Commit()
}
func (s *Service) Publish(ctx context.Context, id uuid.UUID) error {
	var user, project, sid uuid.UUID
	if e := s.db.QueryRowContext(ctx, `SELECT user_id,project_id,session_id FROM audit_exports WHERE id=$1`, id).Scan(&user, &project, &sid); e != nil {
		return ErrDenied
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.admit(ctx, tx, policy.Principal{Kind: policy.Human, SubjectID: user, ActorID: user, SessionID: sid}, project); e != nil {
		return e
	}
	var state string
	var expiry time.Time
	if e = tx.QueryRowContext(ctx, `SELECT state,expires_at FROM audit_exports WHERE id=$1 FOR UPDATE`, id).Scan(&state, &expiry); e != nil {
		return e
	}
	if state == "ready" {
		return tx.Commit()
	}
	if state != "pending" || !expiry.After(time.Now()) {
		return ErrDenied
	}
	if _, e = tx.ExecContext(ctx, `UPDATE audit_exports SET state='ready' WHERE id=$1`, id); e != nil {
		return e
	}
	if e = s.audit.CreateTx(tx, &user, &project, "audit.export_ready", "", models.JSONMap{"export_id": id.String()}, ""); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) StepPublication(ctx context.Context, worker uuid.UUID) (bool, error) {
	q := jobs.Queue{DB: s.db}
	job, e := q.ClaimKinds(ctx, worker, 30*time.Second, []string{"audit.export.publish"})
	if errors.Is(e, jobs.ErrNoJob) {
		return s.reconcileFailedPublications(ctx)
	}
	if e != nil {
		return false, e
	}
	var refs struct {
		ID uuid.UUID `json:"export_id"`
	}
	if json.Unmarshal(job.Payload, &refs) != nil || refs.ID == uuid.Nil {
		_ = q.Fail(ctx, *job, false)
		return true, ErrInvalid
	}
	if e = s.Publish(ctx, refs.ID); e != nil {
		if failure := q.Fail(ctx, *job, false); failure != nil {
			return true, errors.Join(e, failure)
		}
		// A denied original authority cannot publish this snapshot. Other errors
		// remain retryable until the persisted job has exhausted its attempts.
		var state string
		if failure := s.db.QueryRowContext(ctx, `SELECT status FROM outbox_jobs WHERE id=$1`, job.ID).Scan(&state); failure != nil {
			return true, errors.Join(e, failure)
		}
		if errors.Is(e, ErrDenied) || state == "failed" || state == "uncertain" {
			return true, errors.Join(e, s.failPublication(ctx, refs.ID))
		}
		return true, e
	}
	return true, q.Ack(ctx, *job)
}

// failPublication only reduces an unpublished projection. The maintenance
// barrier works after session revocation or project archival without granting
// permission to read its payload, and the audit event commits with erasure.
func (s *Service) failPublication(ctx context.Context, id uuid.UUID) error {
	var user, project uuid.UUID
	if e := s.db.QueryRowContext(ctx, `SELECT user_id,project_id FROM audit_exports WHERE id=$1`, id).Scan(&user, &project); e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			return nil
		}
		return e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s'; SET LOCAL statement_timeout='10s'`); e != nil {
		return e
	}
	if e = authority.Postgres(s.db).LockProjectMaintenance(ctx, tx, project, []uuid.UUID{user}); e != nil {
		return e
	}
	var state string
	if e = tx.QueryRowContext(ctx, `SELECT state FROM audit_exports WHERE id=$1 AND user_id=$2 AND project_id=$3 FOR UPDATE`, id, user, project).Scan(&state); e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			return nil
		}
		return e
	}
	if state != "pending" {
		return tx.Commit()
	}
	if _, e = tx.ExecContext(ctx, `UPDATE audit_exports SET state='failed',payload=''::bytea WHERE id=$1`, id); e != nil {
		return e
	}
	if s.audit == nil {
		return ErrDenied
	}
	if e = s.audit.CreateTx(tx, &user, &project, "audit.export_failed", "", models.JSONMap{"export_id": id.String()}, ""); e != nil {
		return e
	}
	return tx.Commit()
}

// Reconcile the crash gap between a terminal queue transition and projection
// cleanup, including attempts exhausted by queue lease recovery.
func (s *Service) reconcileFailedPublications(ctx context.Context) (bool, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT DISTINCT e.id FROM audit_exports e JOIN outbox_jobs j ON j.kind='audit.export.publish' AND j.payload->>'export_id'=e.id::text WHERE e.state='pending' AND j.status IN ('failed','uncertain') ORDER BY e.id LIMIT 100`)
	if e != nil {
		return false, e
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return false, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return false, e
	}
	for _, id := range ids {
		if e = s.failPublication(ctx, id); e != nil {
			return len(ids) > 0, e
		}
	}
	return len(ids) > 0, nil
}

// Cleanup only removes expired projections; expiry already denies downloads.
func (s *Service) Cleanup(ctx context.Context) error {
	_, e := s.db.ExecContext(ctx, `DELETE FROM audit_exports WHERE expires_at<=NOW()`)
	return e
}
