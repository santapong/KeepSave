package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlatformBackupWorkerScheduleRestartAndRetention(t *testing.T) {
	f := newPlatformFixture(t)
	v, _, p := platformVault(t, f)
	ctx := context.Background()
	if err := v.Enroll(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "private-backups")
	m := vault.Maintenance{Vault: v, Directory: directory, WorkerID: uuid.New()}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	if err := m.Schedule(ctx, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='vault.backup'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("unverified recovery enabled schedule", err)
	}
	m.RecoveryVerified = true
	if err := m.Schedule(ctx, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='vault.backup'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("scheduled before2UTC", err)
	}
	if err := m.Schedule(ctx, now); err != nil {
		t.Fatal(err)
	}
	if err := m.Schedule(ctx, now); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='vault.backup'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate daily schedule", err)
	}
	// A new worker instance consumes durable scheduled work after restart.
	m.WorkerID = uuid.New()
	for i := 0; i < 100; i++ {
		err := m.Step(ctx)
		if errors.Is(err, jobs.ErrNoJob) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	records, err := v.ListBackups(ctx, p, f.project)
	if err != nil || len(records) != 1 || records[0].State != "verified" {
		t.Fatal("worker catalog", err)
	}
	if _, err = os.Stat(filepath.Join(directory, records[0].ID.String()+".json")); err != nil {
		t.Fatal("external bundle absent", err)
	}
	// Simulate a crash after dispatch/file/catalog commit but before acknowledgment.
	var jobID uuid.UUID
	if err = f.db.QueryRow(`SELECT id FROM outbox_jobs WHERE kind='vault.backup'`).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE outbox_jobs SET status='leased',lease_until=NOW()-INTERVAL '1 second' WHERE id=$1`, jobID); err != nil {
		t.Fatal(err)
	}
	m.WorkerID = uuid.New()
	if err = m.Step(ctx); err != nil {
		t.Fatal(err)
	}
	records, err = v.ListBackups(ctx, p, f.project)
	if err != nil || len(records) != 1 {
		t.Fatal("restart duplicated durable bundle", err)
	}
	// Retention never deletes manual bundles, and records an unlink failure.
	manual := uuid.New()
	failed := uuid.New()
	for _, item := range []struct {
		id        uuid.UUID
		scheduled bool
		at        time.Time
	}{{manual, false, now.Add(-72 * time.Hour)}, {failed, true, now.Add(-48 * time.Hour)}, {uuid.New(), true, now.Add(-24 * time.Hour)}} {
		path := filepath.Join(directory, item.id.String()+".json")
		if item.id == failed {
			if err = os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(path, "blocked"), []byte("metadata-fixture"), 0600); err != nil {
				t.Fatal(err)
			}
		} else {
			if err = os.WriteFile(path, []byte("metadata-fixture"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = f.db.Exec(`INSERT INTO backup_catalog(id,project_id,filename,bundle_sha256,byte_size,scheduled,verified_at,created_at) VALUES($1,$2,$3,'fixture-only',1,$4,NOW(),$5)`, item.id, f.project, item.id.String()+".json", item.scheduled, item.at); err != nil {
			t.Fatal(err)
		}
	}
	m.RecoveryVerified = false
	if err = m.Retain(ctx, p, f.project, 2); err != nil {
		t.Fatal(err)
	}
	var pausedState string
	if err = f.db.QueryRow(`SELECT state FROM backup_catalog WHERE id=$1`, failed).Scan(&pausedState); err != nil || pausedState != "verified" {
		t.Fatal("retention proceeded without verified recovery", pausedState, err)
	}
	m.RecoveryVerified = true
	if err = m.Retain(ctx, p, f.project, 2); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = f.db.QueryRow(`SELECT state FROM backup_catalog WHERE id=$1`, failed).Scan(&state); err != nil || state != "retention_failed" {
		t.Fatal("unlink failure falsely succeeded", state, err)
	}
	if _, err = os.Stat(filepath.Join(directory, manual.String()+".json")); err != nil {
		t.Fatal("manual bundle purged", err)
	}
	if err = f.db.QueryRow(`SELECT COUNT(*) FROM backup_catalog WHERE project_id=$1 AND scheduled AND state='verified'`, f.project).Scan(&count); err != nil || count < 2 {
		t.Fatal("fewer than2 verified bundles retained", err)
	}
	// If required audit fails, pending state prevents an object being removed.
	if _, err = f.db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT fail_retention CHECK(action<>'backup.retention.requested') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err = m.Retain(ctx, p, f.project, 2); err == nil {
		t.Fatal("retention survived required audit failure")
	}
	if _, err = os.Stat(filepath.Join(directory, failed.String()+".json", "blocked")); err != nil {
		t.Fatal("filesystem changed before auditcommit", err)
	}
}

// Reusing a durable object after restart still requires live authorization and
// required audit admission before the encrypted recovery material is checked.
func TestPlatformBackupWorkerReusedBundleRequiresAdmission(t *testing.T) {
	f := newPlatformFixture(t)
	v, _, p := platformVault(t, f)
	ctx := context.Background()
	if err := v.Enroll(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "private-backups")
	m := vault.Maintenance{Vault: v, Directory: directory, WorkerID: uuid.New()}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	bundle, err := v.Backup(ctx, p, f.project)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, id.String()+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"project_id": f.project.String(), "actor_id": f.owner.String()})
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = jobs.EnqueueTx(ctx, tx, id, "vault.backup", payload, "local", 5); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT reject_reused_admission CHECK(action<>'backup.verified') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	rejected := false
	for i := 0; i < 100; i++ {
		err = m.Step(ctx)
		if errors.Is(err, jobs.ErrNoJob) {
			break
		}
		if err != nil {
			rejected = true
			break
		}
	}
	if !rejected {
		t.Fatal("reused object skipped required admission")
	}
	var count int
	if err = f.db.QueryRow(`SELECT COUNT(*) FROM backup_catalog WHERE id=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatal("unaudited reused object cataloged", count, err)
	}
}
