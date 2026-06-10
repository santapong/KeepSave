package service

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

func TestKeyRotationServiceNew(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("failed to create crypto service: %v", err)
	}

	svc := NewKeyRotationService(nil, nil, nil, nil, cryptoSvc)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.cryptoSvc != cryptoSvc {
		t.Error("crypto service not set correctly")
	}
}

func TestRotationResultFields(t *testing.T) {
	result := RotationResult{
		ProjectID:        uuid.New(),
		SecretsRotated:   15,
		EnvironmentsUsed: 3,
	}

	if result.SecretsRotated != 15 {
		t.Errorf("SecretsRotated = %d, want 15", result.SecretsRotated)
	}
	if result.EnvironmentsUsed != 3 {
		t.Errorf("EnvironmentsUsed = %d, want 3", result.EnvironmentsUsed)
	}
	if result.ProjectID == uuid.Nil {
		t.Error("ProjectID should not be nil")
	}
}

func TestKeyRotationEncryptDecryptCycle(t *testing.T) {
	// Simulate the crypto operations that happen during key rotation
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 5)
	}

	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("failed to create crypto service: %v", err)
	}

	// Generate and encrypt old DEK
	oldDEK, err := cryptoSvc.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK() error = %v", err)
	}

	encryptedOldDEK, oldDEKNonce, err := cryptoSvc.EncryptDEK(oldDEK)
	if err != nil {
		t.Fatalf("EncryptDEK() error = %v", err)
	}

	// Encrypt a secret with old DEK
	plaintext := []byte("super-secret-value-123")
	secretCiphertext, secretNonce, err := crypto.Encrypt(oldDEK, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Simulate rotation: decrypt old DEK
	decryptedOldDEK, err := cryptoSvc.DecryptDEK(encryptedOldDEK, oldDEKNonce)
	if err != nil {
		t.Fatalf("DecryptDEK() error = %v", err)
	}

	// Decrypt secret with old DEK
	decryptedSecret, err := crypto.Decrypt(decryptedOldDEK, secretCiphertext, secretNonce)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	// Generate new DEK
	newDEK, err := cryptoSvc.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK() error = %v", err)
	}

	// Re-encrypt secret with new DEK
	newCiphertext, newNonce, err := crypto.Encrypt(newDEK, decryptedSecret)
	if err != nil {
		t.Fatalf("Encrypt() with new DEK error = %v", err)
	}

	// Encrypt new DEK with master key
	encryptedNewDEK, newDEKNonce, err := cryptoSvc.EncryptDEK(newDEK)
	if err != nil {
		t.Fatalf("EncryptDEK() error = %v", err)
	}

	// Verify: decrypt new DEK, then decrypt secret
	finalDEK, err := cryptoSvc.DecryptDEK(encryptedNewDEK, newDEKNonce)
	if err != nil {
		t.Fatalf("DecryptDEK() error = %v", err)
	}

	finalPlaintext, err := crypto.Decrypt(finalDEK, newCiphertext, newNonce)
	if err != nil {
		t.Fatalf("Decrypt() with new DEK error = %v", err)
	}

	if string(finalPlaintext) != string(plaintext) {
		t.Errorf("after rotation, got %q, want %q", string(finalPlaintext), string(plaintext))
	}
}

func TestKeyRotationDEKsAreDifferent(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 10)
	}

	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("failed to create crypto service: %v", err)
	}

	dek1, _ := cryptoSvc.GenerateDEK()
	dek2, _ := cryptoSvc.GenerateDEK()

	if string(dek1) == string(dek2) {
		t.Error("consecutive DEKs should be different")
	}
}

// rotationTestEnv wires real repositories over in-memory SQLite so
// RotateProjectKey runs end-to-end (decrypt -> re-encrypt -> UpdateDEK ->
// audit emit). Hand-rolled DDL per the convention in
// api/negative_auth_test.go - the migration runner is avoided in tests.
type rotationTestEnv struct {
	db          *sql.DB
	cryptoSvc   *crypto.Service
	projectRepo *repository.ProjectRepository
	svc         *KeyRotationService
}

func newRotationTestEnv(t *testing.T) *rotationTestEnv {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
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
		masterKey[i] = byte(i + 21)
	}
	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("crypto service: %v", err)
	}

	dialect := repository.NewDialect(repository.DBTypeSQLite)
	projectRepo := repository.NewProjectRepository(db, dialect)
	svc := NewKeyRotationService(
		projectRepo,
		repository.NewSecretRepository(db, dialect),
		repository.NewEnvironmentRepository(db, dialect),
		repository.NewAuditRepository(db, dialect),
		cryptoSvc,
	)
	return &rotationTestEnv{db: db, cryptoSvc: cryptoSvc, projectRepo: projectRepo, svc: svc}
}

// seedProject inserts a project with a fresh DEK plus the given
// environment names, each holding one secret encrypted under that DEK.
// Returns the project ID and the plaintext seeded per secret key.
func (e *rotationTestEnv) seedProject(t *testing.T, ownerID uuid.UUID, envNames []string) (uuid.UUID, map[string]string) {
	t.Helper()
	dek, err := e.cryptoSvc.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	encryptedDEK, dekNonce, err := e.cryptoSvc.EncryptDEK(dek)
	if err != nil {
		t.Fatalf("EncryptDEK: %v", err)
	}
	pid := uuid.New()
	if _, err := e.db.Exec(
		`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES (?, ?, '', ?, ?, ?)`,
		pid.String(), "p-"+pid.String()[:8], ownerID.String(), encryptedDEK, dekNonce,
	); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	plaintexts := map[string]string{}
	for _, envName := range envNames {
		envID := uuid.New()
		if _, err := e.db.Exec(
			`INSERT INTO environments (id, project_id, name) VALUES (?, ?, ?)`,
			envID.String(), pid.String(), envName,
		); err != nil {
			t.Fatalf("seed environment: %v", err)
		}
		key := "TOKEN_" + envName
		value := "secret-value-" + envName
		ciphertext, nonce, err := crypto.Encrypt(dek, []byte(value))
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		if _, err := e.db.Exec(
			`INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce) VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), pid.String(), envID.String(), key, ciphertext, nonce,
		); err != nil {
			t.Fatalf("seed secret: %v", err)
		}
		plaintexts[key] = value
	}
	return pid, plaintexts
}

// TestRotateProjectKey_EmitsAudit covers the CLAUDE.md audit rule for the
// key-rotation mutation: rotation succeeds end-to-end, secrets decrypt
// under the new DEK, and exactly one key.dek_rotated row is written with
// the actor, project, and rotation counts.
func TestRotateProjectKey_EmitsAudit(t *testing.T) {
	env := newRotationTestEnv(t)
	owner := uuid.New()
	actor := uuid.New()
	pid, plaintexts := env.seedProject(t, owner, []string{"alpha", "uat"})

	result, err := env.svc.RotateProjectKey(pid, actor, "127.0.0.1")
	if err != nil {
		t.Fatalf("RotateProjectKey: %v", err)
	}
	if result.SecretsRotated != 2 {
		t.Errorf("SecretsRotated = %d, want 2", result.SecretsRotated)
	}
	if result.EnvironmentsUsed != 2 {
		t.Errorf("EnvironmentsUsed = %d, want 2", result.EnvironmentsUsed)
	}

	// Secrets must decrypt under the NEW DEK.
	project, err := env.projectRepo.GetByID(pid)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	newDEK, err := env.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		t.Fatalf("DecryptDEK after rotation: %v", err)
	}
	rows, err := env.db.Query(`SELECT key, encrypted_value, value_nonce FROM secrets WHERE project_id = ?`, pid.String())
	if err != nil {
		t.Fatalf("query secrets: %v", err)
	}
	defer rows.Close()
	decrypted := 0
	for rows.Next() {
		var key string
		var ciphertext, nonce []byte
		if err := rows.Scan(&key, &ciphertext, &nonce); err != nil {
			t.Fatalf("scan secret: %v", err)
		}
		plaintext, err := crypto.Decrypt(newDEK, ciphertext, nonce)
		if err != nil {
			t.Fatalf("decrypt %s under new DEK: %v", key, err)
		}
		if string(plaintext) != plaintexts[key] {
			t.Errorf("secret %s = %q after rotation, want %q", key, plaintext, plaintexts[key])
		}
		decrypted++
	}
	if decrypted != 2 {
		t.Errorf("decrypted %d secrets, want 2", decrypted)
	}

	// Exactly one audit row, attributed to the actor and project.
	var count int
	var gotActor, gotProject, gotIP, details string
	if err := env.db.QueryRow(
		`SELECT COUNT(*) FROM audit_log WHERE action = 'key.dek_rotated'`,
	).Scan(&count); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("got %d key.dek_rotated rows, want 1", count)
	}
	if err := env.db.QueryRow(
		`SELECT user_id, project_id, ip_address, details FROM audit_log WHERE action = 'key.dek_rotated'`,
	).Scan(&gotActor, &gotProject, &gotIP, &details); err != nil {
		t.Fatalf("read audit row: %v", err)
	}
	if gotActor != actor.String() {
		t.Errorf("audit user_id = %q, want %q", gotActor, actor.String())
	}
	if gotProject != pid.String() {
		t.Errorf("audit project_id = %q, want %q", gotProject, pid.String())
	}
	if gotIP != "127.0.0.1" {
		t.Errorf("audit ip_address = %q, want 127.0.0.1", gotIP)
	}
	var detailMap map[string]interface{}
	if err := json.Unmarshal([]byte(details), &detailMap); err != nil {
		t.Fatalf("details %q is not JSON: %v", details, err)
	}
	if got, ok := detailMap["secrets_rotated"].(float64); !ok || int(got) != 2 {
		t.Errorf("details.secrets_rotated = %v, want 2", detailMap["secrets_rotated"])
	}
	if got, ok := detailMap["environments"].(float64); !ok || int(got) != 2 {
		t.Errorf("details.environments = %v, want 2", detailMap["environments"])
	}
}

// TestRotateAllProjects_EmitsAuditPerProject pins the bulk-rotation audit
// shape: one key.dek_rotated row per project, no summary event.
func TestRotateAllProjects_EmitsAuditPerProject(t *testing.T) {
	env := newRotationTestEnv(t)
	owner := uuid.New()
	pid1, _ := env.seedProject(t, owner, []string{"alpha"})
	pid2, _ := env.seedProject(t, owner, []string{"alpha"})

	results, err := env.svc.RotateAllProjects(owner, "10.0.0.9")
	if err != nil {
		t.Fatalf("RotateAllProjects: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("rotated %d projects, want 2", len(results))
	}

	for _, pid := range []uuid.UUID{pid1, pid2} {
		var count int
		if err := env.db.QueryRow(
			`SELECT COUNT(*) FROM audit_log WHERE action = 'key.dek_rotated' AND project_id = ? AND user_id = ?`,
			pid.String(), owner.String(),
		).Scan(&count); err != nil {
			t.Fatalf("count audit rows for %s: %v", pid, err)
		}
		if count != 1 {
			t.Errorf("project %s has %d key.dek_rotated rows, want 1", pid, count)
		}
	}
}
