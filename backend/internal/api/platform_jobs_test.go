package api

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"sync"
	"testing"
	"time"
)

func TestPlatformJobFencingAndUncertainty(t *testing.T) {
	f := newPlatformFixture(t)
	if f.d.DBType() != repository.DBTypePostgres {
		t.Skip("PostgreSQL job contract")
	}
	// This scenario controls its queue; project creation now contributes events.
	if _, err := f.db.Exec(`DELETE FROM outbox_jobs`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	q := jobs.Queue{DB: f.db}
	enqueue := func(effect string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		tx, err := f.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err = jobs.EnqueueTx(ctx, tx, id, "test", []byte(`{}`), effect, 3); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		return id
	}
	expire := func(id uuid.UUID) {
		t.Helper()
		if _, err := f.db.Exec(`UPDATE outbox_jobs SET lease_until=NOW()-INTERVAL '1 second' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	id := enqueue("local")
	var wg sync.WaitGroup
	results := make(chan *jobs.Job, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); j, err := q.Claim(ctx, uuid.New(), time.Minute); results <- j; errs <- err }()
	}
	wg.Wait()
	close(results)
	close(errs)
	var first *jobs.Job
	for j := range results {
		if j != nil {
			if first != nil {
				t.Fatal("duplicate claim")
			}
			first = j
		}
	}
	for err := range errs {
		if err != nil && !errors.Is(err, jobs.ErrNoJob) {
			t.Fatal(err)
		}
	}
	if first == nil || first.ID != id {
		t.Fatal("job missing")
	}
	expire(id)
	second, err := q.Claim(ctx, uuid.New(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err = q.Ack(ctx, *first); !errors.Is(err, jobs.ErrFenced) {
		t.Fatal("stale worker acknowledged", err)
	}
	if second.Fence <= first.Fence {
		t.Fatal("fence did not advance")
	}
	if err = q.Ack(ctx, *second); err != nil {
		t.Fatal(err)
	}
	external := enqueue("external")
	j, err := q.Claim(ctx, uuid.New(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err = q.Dispatch(ctx, *j); err != nil {
		t.Fatal(err)
	}
	expire(external)
	if _, err = q.Claim(ctx, uuid.New(), time.Minute); !errors.Is(err, jobs.ErrNoJob) {
		t.Fatal("ambiguous external effect replayed", err)
	}
	var status string
	if err = f.db.QueryRow(`SELECT status FROM outbox_jobs WHERE id=$1`, external).Scan(&status); err != nil || status != "uncertain" {
		t.Fatal(status, err)
	}
	forged := enqueue("external")
	j, err = q.Claim(ctx, uuid.New(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err = q.Dispatch(ctx, *j); err != nil {
		t.Fatal(err)
	}
	j.Effect = "local"
	if err = q.Fail(ctx, *j, false); err != nil {
		t.Fatal(err)
	}
	if err = f.db.QueryRow(`SELECT status FROM outbox_jobs WHERE id=$1`, forged).Scan(&status); err != nil || status != "uncertain" {
		t.Fatal("worker rewrote persisted effect semantics", status, err)
	}

}
