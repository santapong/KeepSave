package api

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type gatewayTestEnv struct {
	db        *sql.DB
	cryptoSvc *crypto.Service
	handler   *MCPGatewayHandler
}

func newGatewayTestEnv(t *testing.T) *gatewayTestEnv {
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
			organization_id TEXT,
			encrypted_dek BLOB,
			dek_nonce BLOB,
			allowed_origins TEXT NOT NULL DEFAULT '[]',
			embed_policy_enabled INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE organization_members (
			organization_id TEXT NOT NULL,
			user_id TEXT NOT NULL
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
	// Only the repos/crypto used by resolveSecretEnvVars are wired; the rest
	// (mcpService, builderService, mcpRepo) are not needed for this path.
	h := &MCPGatewayHandler{
		secretRepo:  repository.NewSecretRepository(db, dialect),
		projectRepo: repository.NewProjectRepository(db, dialect),
		envRepo:     repository.NewEnvironmentRepository(db, dialect),
		cryptoSvc:   cryptoSvc,
	}
	return &gatewayTestEnv{db: db, cryptoSvc: cryptoSvc, handler: h}
}

// seedProjectSecret inserts a project owned by ownerID with a fresh DEK and a
// single secret (key=value) under the given environment. Returns the project ID.
func (e *gatewayTestEnv) seedProjectSecret(t *testing.T, ownerID uuid.UUID, env, key, value string) uuid.UUID {
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
	if _, err := e.db.Exec(
		`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES (?, ?, '', ?, ?, ?)`,
		pid.String(), "p-"+pid.String()[:8], ownerID.String(), encDEK, dekNonce,
	); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	envID := uuid.New()
	if _, err := e.db.Exec(
		`INSERT INTO environments (id, project_id, name) VALUES (?, ?, ?)`,
		envID.String(), pid.String(), env,
	); err != nil {
		t.Fatalf("seed environment: %v", err)
	}
	ct, nonce, err := crypto.Encrypt(dek, []byte(value))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := e.db.Exec(
		`INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce) VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), pid.String(), envID.String(), key, ct, nonce,
	); err != nil {
		t.Fatalf("seed secret: %v", err)
	}
	return pid
}

// TestResolveSecretEnvVars_SkipsUnauthorizedProject covers DB-01: a server
// whose EnvMappings reference a project the caller cannot access must NOT have
// that project's secret resolved/decrypted, while the caller's own project's
// secret resolves normally. This is the cross-tenant plaintext-exfiltration
// hole the gateway previously had.
func TestResolveSecretEnvVars_SkipsUnauthorizedProject(t *testing.T) {
	env := newGatewayTestEnv(t)
	userA := uuid.New()
	userB := uuid.New()
	projA := env.seedProjectSecret(t, userA, "alpha", "TOKEN_A", "value-A")
	projB := env.seedProjectSecret(t, userB, "alpha", "TOKEN_B", "value-B")

	server := &models.MCPServerWithTools{}
	server.EnvMappings = models.JSONMap{
		"VAR_A": map[string]interface{}{"project_id": projA.String(), "environment": "alpha", "secret_key": "TOKEN_A"},
		"VAR_B": map[string]interface{}{"project_id": projB.String(), "environment": "alpha", "secret_key": "TOKEN_B"},
	}

	// Caller userA may access projA, never projB.
	envVars, err := env.handler.resolveSecretEnvVars(server, userA)
	if err != nil {
		t.Fatalf("resolveSecretEnvVars: %v", err)
	}
	joined := strings.Join(envVars, "\n")
	if !strings.Contains(joined, "VAR_A=value-A") {
		t.Errorf("authorized secret was not resolved; got %v", envVars)
	}
	if strings.Contains(joined, "value-B") || strings.Contains(joined, "VAR_B=") {
		t.Errorf("DB-01: cross-tenant secret leaked into env vars: %v", envVars)
	}

	// Sanity: the gate scopes by caller, it is not a blanket deny — the owner
	// of projB still resolves VAR_B.
	ownerVars, err := env.handler.resolveSecretEnvVars(server, userB)
	if err != nil {
		t.Fatalf("resolveSecretEnvVars(userB): %v", err)
	}
	if !strings.Contains(strings.Join(ownerVars, "\n"), "VAR_B=value-B") {
		t.Errorf("owner B did not resolve their own secret; got %v", ownerVars)
	}
}
