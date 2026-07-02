package api

import (
	"database/sql"
	"errors"
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

// TestScrubSecrets covers NEW-9: any decrypted secret value a subprocess echoes
// back on stdout must be redacted before the response reaches the caller.
func TestScrubSecrets(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		secretValues []string
		want         string
	}{
		{
			name:         "no secrets configured",
			input:        "hello world",
			secretValues: nil,
			want:         "hello world",
		},
		{
			name:         "single secret redacted",
			input:        "token is s3cr3t here",
			secretValues: []string{"s3cr3t"},
			want:         "token is [REDACTED] here",
		},
		{
			name:         "multiple secrets redacted",
			input:        "a=alpha b=bravo",
			secretValues: []string{"alpha", "bravo"},
			want:         "a=[REDACTED] b=[REDACTED]",
		},
		{
			name:         "overlapping substring values both redacted",
			input:        "supersecret and secret",
			secretValues: []string{"supersecret", "secret"},
			want:         "[REDACTED] and [REDACTED]",
		},
		{
			name:         "empty value ignored",
			input:        "unchanged output",
			secretValues: []string{""},
			want:         "unchanged output",
		},
		{
			name:         "empty value mixed with real secret",
			input:        "leak PA55 here",
			secretValues: []string{"", "PA55"},
			want:         "leak [REDACTED] here",
		},
		{
			name:         "value appearing in JSON field is redacted",
			input:        `{"result":{"content":[{"type":"text","text":"API_KEY=hunter2"}]}}`,
			secretValues: []string{"hunter2"},
			want:         `{"result":{"content":[{"type":"text","text":"API_KEY=[REDACTED]"}]}}`,
		},
		{
			name:         "repeated occurrences all redacted",
			input:        "pw pw pw",
			secretValues: []string{"pw"},
			want:         "[REDACTED] [REDACTED] [REDACTED]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(scrubSecrets([]byte(tc.input), tc.secretValues))
			if got != tc.want {
				t.Fatalf("scrubSecrets() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestParseToolOutput covers NEW-9 structured-output validation: a conforming
// JSON-RPC object is parsed and its result returned, while non-conforming tool
// stdout (arbitrary text, JSON array, bare literal, empty) is rejected with a
// typed error and NEVER returned verbatim — so a tool cannot smuggle arbitrary
// bytes past the gateway even if they survived scrubbing.
func TestParseToolOutput(t *testing.T) {
	t.Run("well-formed result passes through", func(t *testing.T) {
		out := []byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"ok"}]}}`)
		got, err := parseToolOutput(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m, ok := got.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map result, got %T", got)
		}
		if _, ok := m["content"]; !ok {
			t.Fatalf("expected result content preserved, got %v", got)
		}
	})

	t.Run("json object without result returns whole object", func(t *testing.T) {
		out := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"boom"}}`)
		got, err := parseToolOutput(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := got.(map[string]interface{}); !ok {
			t.Fatalf("expected map, got %T", got)
		}
	})

	nonConforming := []struct {
		name string
		out  string
	}{
		{"arbitrary text", "API_KEY=hunter2 leaked to stdout"},
		{"json array not object", `["not","an","object"]`},
		{"bare string literal", `"just a string"`},
		{"bare number", `42`},
		{"empty output", ``},
		{"partial json", `{"result":`},
	}
	for _, tc := range nonConforming {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			got, err := parseToolOutput([]byte(tc.out))
			if !errors.Is(err, errNonConformingToolOutput) {
				t.Fatalf("expected errNonConformingToolOutput, got err=%v", err)
			}
			if got != nil {
				t.Fatalf("non-conforming output must not be returned; got %v", got)
			}
			// The tool's own bytes must never appear in what we hand back.
			if strings.Contains(errNonConformingToolOutput.Error(), tc.out) && tc.out != "" {
				t.Fatalf("error text leaked tool output")
			}
		})
	}
}

// TestSecretValuesFromEnvVars asserts the VALUE side is extracted from each
// "NAME=value" env string that resolveSecretEnvVars builds, including values
// that themselves contain '='.
func TestSecretValuesFromEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		envVars []string
		want    []string
	}{
		{"empty", nil, []string{}},
		{"simple", []string{"NAME=value"}, []string{"value"}},
		{"value contains equals", []string{"TOKEN=a=b=c"}, []string{"a=b=c"}},
		{"multiple", []string{"A=1", "B=2"}, []string{"1", "2"}},
		{"empty value preserved", []string{"EMPTY="}, []string{""}},
		{"no equals skipped", []string{"MALFORMED"}, []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := secretValuesFromEnvVars(tc.envVars)
			if len(got) != len(tc.want) {
				t.Fatalf("secretValuesFromEnvVars() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("secretValuesFromEnvVars()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestRedactedArgKeys proves the CWE-312 fix: the persisted gateway-log payload
// keeps the argument KEY names but never the VALUES, which may hold credentials.
func TestRedactedArgKeys(t *testing.T) {
	args := map[string]interface{}{
		"token":    "super-secret-token",
		"password": "hunter2",
		"limit":    50,
	}
	got := redactedArgKeys(args)
	if len(got) != len(args) {
		t.Fatalf("redactedArgKeys len = %d, want %d", len(got), len(args))
	}
	for k := range args {
		if got[k] != "[redacted]" {
			t.Errorf("redactedArgKeys[%q] = %v, want [redacted]", k, got[k])
		}
	}
	// No original plaintext value survives anywhere in the redacted map.
	for _, v := range got {
		if v == "super-secret-token" || v == "hunter2" {
			t.Errorf("redacted map still contains a plaintext value: %v", v)
		}
	}
}
