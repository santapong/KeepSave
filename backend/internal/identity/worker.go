package identity

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// StepDelivery is a trusted worker entry point. The queue dispatch commits
// before proof dispatch, and both commit before SMTP. No external replay occurs.
// worked=false means there was no eligible proof-delivery job.
func (s *Service) StepDelivery(ctx context.Context, worker uuid.UUID, sender Sender) (worked bool, err error) {
	if !s.Enabled() || sender == nil || worker == uuid.Nil {
		return false, ErrUnavailable
	}
	if _, err = s.SweepDeliveries(ctx); err != nil {
		return false, err
	}
	queue := jobs.Queue{DB: s.db}
	job, err := queue.ClaimKinds(ctx, worker, time.Minute, []string{"identity.proof_delivery"})
	if errors.Is(err, jobs.ErrNoJob) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var payload struct {
		DeliveryID uuid.UUID `json:"delivery_id"`
	}
	decoder := json.NewDecoder(bytes.NewReader(job.Payload))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&payload); err != nil || payload.DeliveryID == uuid.Nil || decoder.Decode(new(any)) != io.EOF || job.Effect != "external" {
		if err = queue.Fail(ctx, *job, false); err != nil {
			return true, err
		}
		return true, ErrInvalid
	}
	var settled string
	if err = s.db.QueryRowContext(ctx, `SELECT state FROM identity_proof_deliveries WHERE id=$1`, payload.DeliveryID).Scan(&settled); err != nil {
		_ = queue.Fail(ctx, *job, false)
		return true, err
	}
	if settled == "sent" || settled == "failed" || settled == "cancelled" {
		return true, queue.Ack(ctx, *job)
	}
	if settled == "uncertain" {
		return true, queue.Fail(ctx, *job, true)
	}
	if err = queue.Dispatch(ctx, *job); err != nil {
		return true, err
	}
	outcome, deliverErr := s.Deliver(ctx, payload.DeliveryID, sender)
	if deliverErr != nil {
		if err = s.db.QueryRowContext(ctx, `SELECT state FROM identity_proof_deliveries WHERE id=$1`, payload.DeliveryID).Scan(&settled); err == nil && (settled == "sent" || settled == "failed" || settled == "cancelled") {
			return true, queue.Ack(ctx, *job)
		}
	}
	if deliverErr != nil || outcome.State == "uncertain" {
		if err = queue.Fail(ctx, *job, true); err != nil {
			return true, err
		}
		return true, deliverErr
	}
	if outcome.State != "sent" && outcome.State != "failed" && outcome.State != "cancelled" {
		if err = queue.Fail(ctx, *job, true); err != nil {
			return true, err
		}
		return true, ErrConflict
	}
	return true, queue.Ack(ctx, *job)
}

// SweepDeliveries conservatively settles bounded expired/crashed dispatches.
// A queue dispatch with a still-pending proof is uncertain after its lease, even
// if the process might have died before opening custody. It is never resent.
func (s *Service) SweepDeliveries(ctx context.Context) (int, error) {
	if !s.Enabled() {
		return 0, ErrUnavailable
	}
	rows, err := s.db.QueryContext(ctx, `SELECT d.id FROM identity_proof_deliveries d JOIN identity_proofs p ON p.id=d.proof_id WHERE
 (d.state='pending' AND (p.expires_at<=NOW() OR p.revoked OR p.consumed OR EXISTS(SELECT 1 FROM outbox_jobs j WHERE j.kind='identity.proof_delivery' AND j.payload->>'delivery_id'=d.id::text AND (j.status IN ('uncertain','failed') OR (j.status='dispatched' AND j.lease_until<=NOW())))))
 OR (d.state='dispatched' AND d.dispatched_at<NOW()-INTERVAL '1 minute') ORDER BY d.created_at,d.id LIMIT 100`)
	if err != nil {
		return 0, err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, id := range ids {
		changed := false
		err = s.transaction(ctx, func(tx *sql.Tx) error {
			var user uuid.UUID
			if e := tx.QueryRowContext(ctx, `SELECT p.user_id FROM identity_proof_deliveries d JOIN identity_proofs p ON p.id=d.proof_id WHERE d.id=$1`, id).Scan(&user); e != nil {
				return e
			}
			if e := s.guard.LockSubjects(ctx, tx, []uuid.UUID{user}, false); e != nil {
				return e
			}
			var state string
			var expired, stale, queueUnknown, queueFailed bool
			if e := tx.QueryRowContext(ctx, `SELECT d.state,(p.expires_at<=NOW() OR p.revoked OR p.consumed),COALESCE(d.dispatched_at<NOW()-INTERVAL '1 minute',FALSE),
 EXISTS(SELECT 1 FROM outbox_jobs j WHERE j.kind='identity.proof_delivery' AND j.payload->>'delivery_id'=d.id::text AND (j.status='uncertain' OR (j.status='dispatched' AND j.lease_until<=NOW()))),
 EXISTS(SELECT 1 FROM outbox_jobs j WHERE j.kind='identity.proof_delivery' AND j.payload->>'delivery_id'=d.id::text AND j.status='failed')
 FROM identity_proof_deliveries d JOIN identity_proofs p ON p.id=d.proof_id WHERE d.id=$1 FOR UPDATE OF d`, id).Scan(&state, &expired, &stale, &queueUnknown, &queueFailed); e != nil {
				return e
			}
			outcome := DeliveryOutcome{}
			if state == "dispatched" && stale || state == "pending" && queueUnknown {
				outcome = DeliveryOutcome{State: "uncertain", Reason: "worker_outcome_unknown"}
			} else if state == "pending" && expired {
				outcome = DeliveryOutcome{State: "cancelled", Reason: "proof_inactive"}
			} else if state == "pending" && queueFailed {
				outcome = DeliveryOutcome{State: "failed", Reason: "worker_abandoned"}
			} else {
				return nil
			}
			if _, e := tx.ExecContext(ctx, `UPDATE identity_proof_deliveries SET state=$1,reason_code=$2,ciphertext=''::bytea,nonce=''::bytea,completed_at=NOW() WHERE id=$3`, outcome.State, outcome.Reason, id); e != nil {
				return e
			}
			if e := s.audit.CreateTx(tx, &user, nil, "identity.delivery_recovered", "", models.JSONMap{"delivery_id": id.String(), "state": outcome.State, "reason_code": outcome.Reason}, ""); e != nil {
				return e
			}
			changed = true
			return nil
		})
		if err != nil {
			return count, err
		}
		if changed {
			count++
		}
	}
	return count, nil
}
