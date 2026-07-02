package service

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// TestDetectDrift_EmitsAudit covers the CLAUDE.md audit hard-gate for the drift
// path (CWE-778): DetectDrift decrypts every secret in both environments, so a
// run MUST write a drift.detected audit row — and that row must carry only
// project/env/counts, never a decrypted secret key or value.
func TestDetectDrift_EmitsAudit(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	db.SetMaxOpenConns(1) // ":memory:" is per-connection
	t.Cleanup(func() { _ = db.Close() })

	for _, ddl := range []string{
		`CREATE TABLE projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			owner_id TEXT NOT NULL,
			encrypted_dek BLOB,
			dek_nonce BLOB,
			allowed_origins TEXT NOT NULL DEFAULT '[]',
			embed_policy_enabled INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE environments (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE secrets (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			environment_id TEXT NOT NULL,
			key TEXT NOT NULL,
			encrypted_value BLOB,
			value_nonce BLOB,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE drift_checks (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			source_env TEXT,
			target_env TEXT,
			status TEXT,
			total_keys INTEGER,
			drifted_keys INTEGER,
			missing_in_source INTEGER,
			missing_in_target INTEGER,
			drift_entries TEXT,
			remediation TEXT,
			created_at TIMESTAMP,
			completed_at TIMESTAMP
		)`,
		`CREATE TABLE audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT,
			project_id TEXT,
			action TEXT,
			environment TEXT,
			details TEXT,
			ip_address TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 7)
	}
	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("crypto service: %v", err)
	}

	dialect := repository.NewDialect(repository.DBTypeSQLite)
	svc := NewDriftService(
		db,
		dialect,
		repository.NewSecretRepository(db, dialect),
		repository.NewProjectRepository(db, dialect),
		repository.NewEnvironmentRepository(db, dialect),
		repository.NewAuditRepository(db, dialect),
		cryptoSvc,
		nil, // no AI provider — remediation branch is skipped
	)

	// Seed a project with a fresh DEK and two environments whose shared secret
	// has different values (so drift is detected).
	dek, err := cryptoSvc.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	encDEK, dekNonce, err := cryptoSvc.EncryptDEK(dek)
	if err != nil {
		t.Fatalf("EncryptDEK: %v", err)
	}
	owner := uuid.New()
	pid := uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES (?,?,'',?,?,?)`,
		pid.String(), "p", owner.String(), encDEK, dekNonce); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	const plaintextAlpha = "alpha-plaintext-value"
	seedSecret := func(envName, key, value string) {
		envID := uuid.New()
		if _, err := db.Exec(`INSERT INTO environments (id, project_id, name) VALUES (?,?,?)`, envID.String(), pid.String(), envName); err != nil {
			t.Fatalf("seed env: %v", err)
		}
		ct, nonce, err := crypto.Encrypt(dek, []byte(value))
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce) VALUES (?,?,?,?,?,?)`,
			uuid.New().String(), pid.String(), envID.String(), key, ct, nonce); err != nil {
			t.Fatalf("seed secret: %v", err)
		}
	}
	seedSecret("alpha", "DATABASE_URL", plaintextAlpha)
	seedSecret("uat", "DATABASE_URL", "uat-plaintext-value")

	check, err := svc.DetectDrift(pid, owner, "alpha", "uat")
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	if check.DriftedKeys != 1 {
		t.Errorf("DriftedKeys = %d, want 1", check.DriftedKeys)
	}

	// Exactly one drift.detected row, attributed to the actor and project.
	var count int
	var gotActor, gotProject, details string
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = 'drift.detected'`).Scan(&count); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("got %d drift.detected rows, want 1", count)
	}
	if err := db.QueryRow(`SELECT user_id, project_id, details FROM audit_log WHERE action = 'drift.detected'`).
		Scan(&gotActor, &gotProject, &details); err != nil {
		t.Fatalf("read audit row: %v", err)
	}
	if gotActor != owner.String() {
		t.Errorf("audit user_id = %q, want %q", gotActor, owner.String())
	}
	if gotProject != pid.String() {
		t.Errorf("audit project_id = %q, want %q", gotProject, pid.String())
	}
	// The audit details MUST NOT contain any decrypted secret value or key.
	if strings.Contains(details, plaintextAlpha) || strings.Contains(details, "uat-plaintext-value") {
		t.Errorf("audit details leaked a plaintext secret value: %s", details)
	}
	if strings.Contains(details, "DATABASE_URL") {
		t.Errorf("audit details leaked a secret key: %s", details)
	}
}
