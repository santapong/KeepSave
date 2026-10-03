// Package jobs provides the PostgreSQL durable outbox. It does not execute
// connector processes or infer whether an ambiguous external effect succeeded.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"time"
)

var ErrFenced = errors.New("job lease no longer belongs to worker")
var ErrNoJob = errors.New("no job ready")

type Job struct {
	ID         uuid.UUID       `json:"id"`
	Kind       string          `json:"kind"`
	Payload    json.RawMessage `json:"-"`
	Effect     string          `json:"effect"`
	Attempt    int             `json:"attempt"`
	Fence      int64           `json:"fence"`
	LeaseUntil time.Time       `json:"lease_until"`
	WorkerID   uuid.UUID       `json:"-"`
}
type Queue struct{ DB *sql.DB }

// EnqueueTx writes work in the same transaction as its domain mutation and
// audit event. Payloads must contain references/metadata, never credentials.
func EnqueueTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, kind string, payload json.RawMessage, effect string, maxAttempts int) error {
	if id == uuid.Nil || kind == "" || len(kind) > 100 || len(payload) > 65536 || !json.Valid(payload) || (effect != "local" && effect != "external") || maxAttempts < 1 || maxAttempts > 20 {
		return errors.New("invalid job")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,effect,max_attempts) VALUES($1,$2,$3,$4,$5)`, id, kind, []byte(payload), effect, maxAttempts)
	return err
}

// Claim is atomic across workers. A crashed external dispatch is left uncertain;
// only local work and work that was never dispatched may be claimed again.
func (q Queue) Claim(ctx context.Context, worker uuid.UUID, lease time.Duration) (*Job, error) {
	return q.ClaimKinds(ctx, worker, lease, nil)
}

// ClaimKinds prevents a trusted worker from claiming capabilities it cannot execute.
func (q Queue) ClaimKinds(ctx context.Context, worker uuid.UUID, lease time.Duration, kinds []string) (*Job, error) {
	if kinds == nil {
		kinds = []string{}
	}
	if worker == uuid.Nil || lease < time.Second || lease > 5*time.Minute {
		return nil, errors.New("invalid job lease")
	}
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='uncertain',outcome='dispatch_outcome_unknown',updated_at=NOW() WHERE status='dispatched' AND effect='external' AND lease_until<=NOW()`); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',outcome='attempts_exhausted',updated_at=NOW() WHERE status IN ('leased','dispatched') AND lease_until<=NOW() AND attempt>=max_attempts`); err != nil {
		return nil, err
	}
	j := &Job{WorkerID: worker}
	err = tx.QueryRowContext(ctx, `WITH candidate AS (
 SELECT id FROM outbox_jobs WHERE attempt<max_attempts AND available_at<=NOW() AND (cardinality($3::text[])=0 OR kind=ANY($3::text[])) AND
 (status='pending' OR (status IN ('leased','dispatched') AND lease_until<=NOW() AND (status='leased' OR effect='local')))
 ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1)
 UPDATE outbox_jobs j SET status='leased',worker_id=$1,lease_until=NOW()+$2*INTERVAL '1 millisecond',
 attempt=attempt+1,fence=fence+1,updated_at=NOW() FROM candidate c WHERE j.id=c.id
 RETURNING j.id,j.kind,j.payload,j.effect,j.attempt,j.fence,j.lease_until`, worker, lease.Milliseconds(), pq.Array(kinds)).Scan(&j.ID, &j.Kind, &j.Payload, &j.Effect, &j.Attempt, &j.Fence, &j.LeaseUntil)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, ErrNoJob
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return j, nil
}
func (q Queue) Dispatch(ctx context.Context, j Job) error {
	return q.transition(ctx, j, `status='dispatched'`, "leased")
}
func (q Queue) Ack(ctx context.Context, j Job) error {
	return q.transition(ctx, j, `status='done',outcome='completed'`, "leased", "dispatched")
}

// Fail never trusts the caller's copy of Effect to authorize a replay. An
// external dispatch is uncertain even if a worker reports it as local work.
func (q Queue) Fail(ctx context.Context, j Job, uncertain bool) error {
	state := "CASE WHEN status='dispatched' AND effect='external' THEN 'uncertain' WHEN attempt>=max_attempts THEN 'failed' ELSE 'pending' END"
	outcome := "CASE WHEN status='dispatched' AND effect='external' THEN 'dispatch_outcome_unknown' ELSE 'failed' END"
	if uncertain {
		state = "'uncertain'"
		outcome = "'dispatch_outcome_unknown'"
	}
	return q.transition(ctx, j, `status=`+state+`,outcome=`+outcome+`,available_at=NOW()+LEAST(attempt*attempt,300)*INTERVAL '1 second'`, "leased", "dispatched")
}
func (q Queue) transition(ctx context.Context, j Job, set string, statuses ...string) error {
	if j.ID == uuid.Nil || j.WorkerID == uuid.Nil {
		return ErrFenced
	}
	// All SQL fragments and status names originate above, never from a request.
	allowed := ""
	for i, s := range statuses {
		if i > 0 {
			allowed += ","
		}
		allowed += "'" + s + "'"
	}
	result, err := q.DB.ExecContext(ctx, `UPDATE outbox_jobs SET `+set+`,updated_at=NOW() WHERE id=$1 AND worker_id=$2 AND fence=$3 AND lease_until>NOW() AND status IN (`+allowed+`)`, j.ID, j.WorkerID, j.Fence)
	if err != nil {
		return fmt.Errorf("updating job: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrFenced
	}
	return nil
}
