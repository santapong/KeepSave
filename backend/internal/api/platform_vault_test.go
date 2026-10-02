package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func platformVault(t *testing.T, f *platformFixture) (*vault.Service, *crypto.Service, policy.Principal) {
	t.Helper()
	if f.d.DBType() != repository.DBTypePostgres {
		t.Skip("PostgreSQL versioned vault contract")
	}
	keys, err := crypto.NewService(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	audit := repository.NewAuditRepository(f.db, f.d)
	audit.SetChainKey(keys.DeriveAuditChainKey())
	authorize := func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d}}).Authorize(ctx, p, a, r)
	}
	v := vault.New(f.db, keys, authorize, audit)
	p := policy.Principal{Kind: policy.Human, SubjectID: f.owner, ActorID: f.owner}
	return v, keys, p
}
func TestPlatformVaultHistoryRotationAndBackup(t *testing.T) {
	f := newPlatformFixture(t)
	v, _, p := platformVault(t, f)
	ctx := context.Background()
	// Retain an existing promotion snapshot through key rotation and backup.
	var env, secret uuid.UUID
	var cipher, nonce []byte
	if err := f.db.QueryRow(`SELECT id,environment_id,encrypted_value,value_nonce FROM secrets WHERE project_id=$1`, f.project).Scan(&secret, &env, &cipher, &nonce); err != nil {
		t.Fatal(err)
	}
	promotions := repository.NewPromotionRepository(f.db, f.d)
	promotion, err := promotions.Create(f.project, "alpha", "uat", f.owner, []string{}, "overwrite", "")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = promotions.CreateSnapshotTx(tx, promotion.ID, env, "DB_URL", cipher, nonce, true); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := v.Enroll(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	var baselineID uuid.UUID
	if err := f.db.QueryRow(`SELECT id FROM secrets WHERE project_id=$1`, f.project).Scan(&baselineID); err != nil {
		t.Fatal(err)
	}
	history, err := v.History(ctx, p, f.project, baselineID)
	if err != nil || len(history) != 1 || history[0].Operation != "baseline" {
		t.Fatal("baseline", err)
	}
	first, err := v.Put(ctx, p, vault.Record{ProjectID: f.project, Environment: "alpha", Key: "NEW", Value: "synthetic-canary-one"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	first.Value = "synthetic-canary-two"
	second, err := v.Put(ctx, p, first, first.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = v.Restore(ctx, p, f.project, first.ID, 1, first.Revision); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("stale restore", err)
	}
	if _, err = v.Rotate(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	read, err := v.Read(ctx, p, f.project, first.ID, 1)
	if err != nil || read.Value != "synthetic-canary-one" {
		t.Fatal("history after rotation", err)
	}
	current, err := v.Read(ctx, p, f.project, first.ID, 0)
	if err != nil || current.Value != "synthetic-canary-two" {
		t.Fatal("current after rotation", err)
	}
	restored, err := v.Restore(ctx, p, f.project, first.ID, 1, current.Revision)
	if err != nil || restored.Revision <= second.Revision {
		t.Fatal("restore is not new revision", err)
	}
	b, err := v.Backup(ctx, p, f.project)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	// The verifier reopens an external file with independently reconstructed
	// recovery material. It has no reference to the source DB or service.
	path := filepath.Join(t.TempDir(), "vault-backup.json")
	if err = os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var external vault.Bundle
	if err = json.Unmarshal(raw, &external); err != nil {
		t.Fatal(err)
	}
	recovery, err := crypto.NewService(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	verified, err := vault.VerifyBackup(external, recovery)
	if err != nil || verified.Keys != 2 || verified.Revisions < 6 || verified.Snapshots != 1 {
		t.Fatal("external backup recovery", verified, err)
	}
	wrongKey := make([]byte, 32)
	wrongKey[0] = 1
	wrong, _ := crypto.NewService(wrongKey)
	if _, err = vault.VerifyBackup(external, wrong); err == nil {
		t.Fatal("wrong recovery key accepted")
	}
	external.Ciphertext[0] ^= 1
	if _, err = vault.VerifyBackup(external, recovery); err == nil {
		t.Fatal("corrupt backup accepted")
	}
	if err = v.Delete(ctx, p, f.project, first.ID, restored.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = v.Restore(ctx, p, f.project, first.ID, 1, restored.Revision+1); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("deleted secret resurrected", err)
	}
	var auditCount, jobCount int
	if err = f.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='secret.restored'`).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err = f.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE payload->>'action'='secret.restored'`).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 || jobCount != 1 {
		t.Fatal("stale/failed restore left successful evidence", auditCount, jobCount)
	}
}
func TestPlatformVaultConcurrentRestoreAndDenial(t *testing.T) {
	f := newPlatformFixture(t)
	v, _, p := platformVault(t, f)
	ctx := context.Background()
	if err := v.Enroll(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	r, err := v.Put(ctx, p, vault.Record{ProjectID: f.project, Environment: "alpha", Key: "RACE", Value: "canary"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	denied := p
	denied.SubjectID = f.other
	denied.ActorID = f.other
	if _, err = v.Read(ctx, denied, f.project, r.ID, 0); !errors.Is(err, vault.ErrDenied) {
		t.Fatal("wrong tenant read", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := v.Restore(ctx, p, f.project, r.ID, 1, 1); results <- err }()
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, vault.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("concurrent revision check", successes, conflicts)
	}
	// Audit failure must abort both the current value and immutable revision.
	if _, err = f.db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT fail_vault_test CHECK(action <> 'secret.updated') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	r.Value = "must-not-commit"
	r.Revision = 2
	if _, err = v.Put(ctx, p, r, 2); err == nil {
		t.Fatal("mutation survived audit failure")
	}
	var revision int64
	if err = f.db.QueryRow(`SELECT revision FROM vault_entries WHERE secret_id=$1`, r.ID).Scan(&revision); err != nil || revision != 2 {
		t.Fatal("partial revision persisted", err)
	}
}
