package service

import (
	"database/sql"
	"sync"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// promoEnv wires a real PromotionService over in-memory SQLite so the
// transaction / claim / snapshot / rollback paths (ADR-0017) run end-to-end.
type promoEnv struct {
	db            *sql.DB
	cryptoSvc     *crypto.Service
	secretRepo    *repository.SecretRepository
	promotionRepo *repository.PromotionRepository
	svc           *PromotionService
	dialect       repository.Dialect
}

func newPromoEnv(t *testing.T) *promoEnv {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	db.SetMaxOpenConns(1) // ":memory:" is per-connection; pin to one DB
	t.Cleanup(func() { _ = db.Close() })
	for _, ddl := range []string{
		`CREATE TABLE projects (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT, owner_id TEXT NOT NULL,
			encrypted_dek BLOB, dek_nonce BLOB,
			allowed_origins TEXT NOT NULL DEFAULT '[]', embed_policy_enabled INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE environments (
			id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE secrets (
			id TEXT PRIMARY KEY, project_id TEXT NOT NULL, environment_id TEXT NOT NULL, key TEXT NOT NULL,
			encrypted_value BLOB, value_nonce BLOB,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (environment_id, key))`,
		`CREATE TABLE promotion_requests (
			id TEXT PRIMARY KEY, project_id TEXT NOT NULL, source_environment TEXT NOT NULL, target_environment TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending', requested_by TEXT NOT NULL, approved_by TEXT,
			keys_filter TEXT NOT NULL DEFAULT '[]', override_policy TEXT NOT NULL DEFAULT 'skip', notes TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, completed_at TIMESTAMP)`,
		`CREATE TABLE secret_snapshots (
			id TEXT PRIMARY KEY, promotion_id TEXT NOT NULL, environment_id TEXT NOT NULL, key TEXT NOT NULL,
			encrypted_value BLOB NOT NULL, value_nonce BLOB NOT NULL, prior_existed INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT, user_id TEXT, project_id TEXT, action TEXT, environment TEXT,
			details TEXT, ip_address TEXT, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 3)
	}
	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	dialect := repository.NewDialect(repository.DBTypeSQLite)
	secretRepo := repository.NewSecretRepository(db, dialect)
	promotionRepo := repository.NewPromotionRepository(db, dialect)
	svc := NewPromotionService(
		promotionRepo, secretRepo,
		repository.NewProjectRepository(db, dialect),
		repository.NewEnvironmentRepository(db, dialect),
		repository.NewAuditRepository(db, dialect),
		cryptoSvc,
	)
	return &promoEnv{db: db, cryptoSvc: cryptoSvc, secretRepo: secretRepo, promotionRepo: promotionRepo, svc: svc, dialect: dialect}
}

// seedProject creates a project with a DEK and the named environments, returning
// the project id, the plaintext DEK, and a name->envID map.
func (e *promoEnv) seedProject(t *testing.T, owner uuid.UUID, envNames ...string) (uuid.UUID, []byte, map[string]uuid.UUID) {
	t.Helper()
	dek, err := e.cryptoSvc.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	encDEK, dekNonce, err := e.cryptoSvc.EncryptDEK(dek)
	if err != nil {
		t.Fatalf("EncryptDEK: %v", err)
	}
	pid := uuid.New()
	if _, err := e.db.Exec(`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES (?,?,'',?,?,?)`,
		pid.String(), "p", owner.String(), encDEK, dekNonce); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	envs := map[string]uuid.UUID{}
	for _, name := range envNames {
		eid := uuid.New()
		if _, err := e.db.Exec(`INSERT INTO environments (id, project_id, name) VALUES (?,?,?)`, eid.String(), pid.String(), name); err != nil {
			t.Fatalf("seed env: %v", err)
		}
		envs[name] = eid
	}
	return pid, dek, envs
}

func (e *promoEnv) seedSecret(t *testing.T, dek []byte, pid, envID uuid.UUID, key, value string) {
	t.Helper()
	ct, nonce, err := crypto.Encrypt(dek, []byte(value))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := e.db.Exec(`INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce) VALUES (?,?,?,?,?,?)`,
		uuid.New().String(), pid.String(), envID.String(), key, ct, nonce); err != nil {
		t.Fatalf("seed secret: %v", err)
	}
}

// secretValue returns the decrypted value of a key in an environment, or ("",
// false) when the key is absent.
func (e *promoEnv) secretValue(t *testing.T, dek []byte, envID uuid.UUID, key string) (string, bool) {
	t.Helper()
	var ct, nonce []byte
	err := e.db.QueryRow(`SELECT encrypted_value, value_nonce FROM secrets WHERE environment_id=? AND key=?`, envID.String(), key).Scan(&ct, &nonce)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		t.Fatalf("read secret %s: %v", key, err)
	}
	pt, err := crypto.Decrypt(dek, ct, nonce)
	if err != nil {
		t.Fatalf("decrypt %s: %v", key, err)
	}
	return string(pt), true
}

func (e *promoEnv) status(t *testing.T, promoID uuid.UUID) string {
	t.Helper()
	var s string
	if err := e.db.QueryRow(`SELECT status FROM promotion_requests WHERE id=?`, promoID.String()).Scan(&s); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return s
}

// TestPromote_CopiesOverwriteAndAdd checks the happy path and that snapshots
// record prior state correctly: an overwritten key keeps prior_existed=1 with
// its old value; an added key gets prior_existed=0.
func TestPromote_CopiesOverwriteAndAdd(t *testing.T) {
	e := newPromoEnv(t)
	owner := uuid.New()
	pid, dek, envs := e.seedProject(t, owner, "alpha", "uat")
	e.seedSecret(t, dek, pid, envs["alpha"], "A", "va")
	e.seedSecret(t, dek, pid, envs["alpha"], "B", "vb")
	e.seedSecret(t, dek, pid, envs["uat"], "B", "old_b")

	promo, err := e.svc.Promote(pid, "alpha", "uat", nil, "overwrite", "", owner, "127.0.0.1")
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if promo.Status != "completed" {
		t.Fatalf("status = %s, want completed", promo.Status)
	}

	if v, ok := e.secretValue(t, dek, envs["uat"], "A"); !ok || v != "va" {
		t.Errorf("uat A = %q,%v want va,true", v, ok)
	}
	if v, ok := e.secretValue(t, dek, envs["uat"], "B"); !ok || v != "vb" {
		t.Errorf("uat B = %q,%v want vb,true", v, ok)
	}

	snaps, err := e.promotionRepo.GetSnapshotsByPromotionID(promo.ID)
	if err != nil {
		t.Fatalf("snapshots: %v", err)
	}
	byKey := map[string]bool{}
	for _, s := range snaps {
		byKey[s.Key] = s.PriorExisted
	}
	if pe, ok := byKey["B"]; !ok || !pe {
		t.Errorf("snapshot B prior_existed = %v (present=%v), want true", pe, ok)
	}
	if pe, ok := byKey["A"]; !ok || pe {
		t.Errorf("snapshot A prior_existed = %v (present=%v), want false", pe, ok)
	}
}

// TestRollback_RestoresAndDeletes verifies P-03/P-04: rollback restores an
// overwritten key to its old value and deletes a key the promotion added, then
// moves the request to rolled_back.
func TestRollback_RestoresAndDeletes(t *testing.T) {
	e := newPromoEnv(t)
	owner := uuid.New()
	pid, dek, envs := e.seedProject(t, owner, "alpha", "uat")
	e.seedSecret(t, dek, pid, envs["alpha"], "A", "va")
	e.seedSecret(t, dek, pid, envs["alpha"], "B", "vb")
	e.seedSecret(t, dek, pid, envs["uat"], "B", "old_b")

	promo, err := e.svc.Promote(pid, "alpha", "uat", nil, "overwrite", "", owner, "127.0.0.1")
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if err := e.svc.Rollback(promo.ID, owner, "127.0.0.1"); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if v, ok := e.secretValue(t, dek, envs["uat"], "B"); !ok || v != "old_b" {
		t.Errorf("after rollback uat B = %q,%v want old_b,true", v, ok)
	}
	if _, ok := e.secretValue(t, dek, envs["uat"], "A"); ok {
		t.Errorf("after rollback uat A still present; added key not deleted")
	}
	if s := e.status(t, promo.ID); s != "rolled_back" {
		t.Errorf("status = %s, want rolled_back", s)
	}
}

// TestPromote_RollsBackOnFailure verifies P-01: a failure mid-copy leaves the
// target environment untouched (no half-promotion). A corrupt source secret
// makes decrypt fail after a valid key was already upserted in the tx.
func TestPromote_RollsBackOnFailure(t *testing.T) {
	e := newPromoEnv(t)
	owner := uuid.New()
	pid, dek, envs := e.seedProject(t, owner, "alpha", "uat")
	e.seedSecret(t, dek, pid, envs["alpha"], "AAA", "good") // processed first (ORDER BY key)
	// ZZZ has ciphertext that won't decrypt under the DEK.
	if _, err := e.db.Exec(`INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce) VALUES (?,?,?,?,?,?)`,
		uuid.New().String(), pid.String(), envs["alpha"].String(), "ZZZ", []byte("garbage-ciphertext"), []byte("badnonce0000")); err != nil {
		t.Fatal(err)
	}

	promo, err := e.svc.Promote(pid, "alpha", "uat", nil, "overwrite", "", owner, "127.0.0.1")
	if err == nil {
		t.Fatalf("Promote succeeded, want failure on corrupt secret (promo=%v)", promo)
	}

	// AAA must NOT have been written to uat — the whole tx rolled back.
	if _, ok := e.secretValue(t, dek, envs["uat"], "AAA"); ok {
		t.Error("uat AAA present after failed promotion; transaction did not roll back")
	}
}

// TestCompareAndSetStatusTx_ClaimsOnce is the deterministic guard for the
// approve TOCTOU (P-02): only the first pending->completed claim wins.
func TestCompareAndSetStatusTx_ClaimsOnce(t *testing.T) {
	e := newPromoEnv(t)
	owner := uuid.New()
	pid, _, _ := e.seedProject(t, owner, "uat", "prod")
	requester := uuid.New()
	approver := uuid.New()
	promo, err := e.svc.Promote(pid, "uat", "prod", nil, "skip", "", requester, "127.0.0.1")
	if err != nil {
		t.Fatalf("Promote(prod) should create pending: %v", err)
	}

	tx1, _ := e.db.Begin()
	ok1, err := e.promotionRepo.CompareAndSetStatusTx(tx1, promo.ID, "pending", "completed", &approver)
	if err != nil {
		t.Fatalf("CAS1: %v", err)
	}
	_ = tx1.Commit()

	tx2, _ := e.db.Begin()
	ok2, err := e.promotionRepo.CompareAndSetStatusTx(tx2, promo.ID, "pending", "completed", &approver)
	if err != nil {
		t.Fatalf("CAS2: %v", err)
	}
	_ = tx2.Commit()

	if !ok1 || ok2 {
		t.Errorf("CAS claims = (%v,%v), want (true,false)", ok1, ok2)
	}
}

// TestApprove_DoubleApproveExecutesOnce fires two concurrent approvers at a
// pending PROD promotion and asserts exactly one succeeds and the copy runs
// once (one promotion_completed row), proving four-eyes can't double-apply.
func TestApprove_DoubleApproveExecutesOnce(t *testing.T) {
	e := newPromoEnv(t)
	owner := uuid.New()
	pid, dek, envs := e.seedProject(t, owner, "uat", "prod")
	e.seedSecret(t, dek, pid, envs["uat"], "K", "v")
	requester := uuid.New()
	promo, err := e.svc.Promote(pid, "uat", "prod", nil, "overwrite", "", requester, "127.0.0.1")
	if err != nil {
		t.Fatalf("Promote(prod): %v", err)
	}
	if promo.Status != "pending" {
		t.Fatalf("prod promotion status = %s, want pending", promo.Status)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for _, approver := range []uuid.UUID{uuid.New(), uuid.New()} {
		wg.Add(1)
		go func(a uuid.UUID) {
			defer wg.Done()
			if _, err := e.svc.ApprovePromotion(promo.ID, a, "127.0.0.1"); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}(approver)
	}
	wg.Wait()

	if successes != 1 {
		t.Errorf("approve successes = %d, want 1", successes)
	}
	if v, ok := e.secretValue(t, dek, envs["prod"], "K"); !ok || v != "v" {
		t.Errorf("prod K = %q,%v want v,true", v, ok)
	}
	var completed int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='promotion_completed' AND project_id=?`, pid.String()).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if completed != 1 {
		t.Errorf("promotion_completed rows = %d, want 1", completed)
	}
}
