package runs

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/automation"
	"github.com/santapong/KeepSave/backend/internal/broker"
	cryptography "github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/harness"
	"github.com/santapong/KeepSave/backend/internal/mcpauth"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/runner"
	"github.com/santapong/KeepSave/backend/migrations"
)

const runsClient = "keepsave-codex-linux-v1"
const runsCommit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const runsTree = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const runsProviderCanary = "synthetic-runs-provider-token-canary"
const runsSource = "---\nname: approved-review\ndescription: Review the approved repository only.\n---\nUse repository_tree and read_file within the bound run.\n"

type runsPGFixture struct {
	s, s2                              *Service
	db                                 *sql.DB
	auth                               *mcpauth.Service
	admin, approver, editor, delegated policy.Principal
	project, organization, environment uuid.UUID
	grant                              Grant
	profile                            Profile
	packageID                          uuid.UUID
	binding                            Binding
	workload                           Workload
	providerCalls                      atomic.Int32
	unwrapCalls                        atomic.Int32
	providerMode                       atomic.Value
	oauthTokens                        map[uuid.UUID]mcpauth.TokenResponse
}

// This observer retains real AEAD custody and only counts entry into decrypt.
type runsCountingCustody struct {
	broker.CryptoCustody
	opens *atomic.Int32
}

func (c runsCountingCustody) Open(ciphertext, nonce []byte) ([]byte, error) {
	c.opens.Add(1)
	return c.CryptoCustody.Open(ciphertext, nonce)
}

func runsPG(t *testing.T) *runsPGFixture {
	t.Helper()
	if os.Getenv("KEEPSAVE_PLATFORM_POSTGRES_TEST") != "1" {
		t.Skip("requires fixed disposable PostgreSQL target; SQLite is not acceptance")
	}
	ctx := context.Background()
	dsn := "postgres://keepsave_platform_test:local-test-only@keepsave-platform-postgres-test:5432/keepsave_platform_test?sslmode=disable"
	admin, _, e := repository.NewDB(dsn)
	if e != nil {
		t.Fatal(e)
	}
	tx, e := admin.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(`SELECT pg_advisory_xact_lock(45883202026)`); e != nil {
		tx.Rollback()
		t.Fatal(e)
	}
	if _, e = tx.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public`); e != nil {
		tx.Rollback()
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	schema := "runs_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, e = admin.Exec("CREATE SCHEMA " + schema); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE"); _ = admin.Close() })
	db, d, e := repository.NewDB(dsn + "&search_path=" + schema + ",public")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(16)
	if e = repository.RunMigrationsFS(db, d, migrations.FS); e != nil {
		t.Fatal(e)
	}
	f := &runsPGFixture{db: db, oauthTokens: map[uuid.UUID]mcpauth.TokenResponse{}}
	f.providerMode.Store("ok")
	cryptoService, e := cryptography.NewService(bytes.Repeat([]byte{0x4a}, 32))
	if e != nil {
		t.Fatal(e)
	}
	audit := repository.NewAuditRepository(db, d)
	audit.SetChainKey(cryptoService.DeriveAuditChainKey())
	makeHuman := func(role string) policy.Principal {
		u, e := repository.NewUserRepository(db, d).Create(uuid.NewString()+"@example.invalid", "synthetic-unusable-password")
		if e != nil {
			t.Fatal(e)
		}
		sid := uuid.New()
		end := time.Now().Add(time.Hour)
		if _, e = db.Exec(`INSERT INTO session_tokens(id,user_id,token_hash,expires_at,revoked) VALUES($1,$2,$3,$4,FALSE)`, sid, u.ID, hash(uuid.NewString()), end); e != nil {
			t.Fatal(e)
		}
		return policy.Principal{Kind: policy.Human, SubjectID: u.ID, ActorID: u.ID, SessionID: sid, ExpiresAt: end}
	}
	f.admin = makeHuman("admin")
	f.approver = makeHuman("promoter")
	f.editor = makeHuman("editor")
	f.organization = uuid.New()
	if _, e = db.Exec(`INSERT INTO organizations(id,name,slug,owner_id) VALUES($1,'Synthetic runs',$2,$3)`, f.organization, uuid.NewString(), f.admin.ActorID); e != nil {
		t.Fatal(e)
	}
	for _, member := range []struct {
		p    policy.Principal
		role string
	}{{f.admin, "admin"}, {f.approver, "promoter"}, {f.editor, "editor"}} {
		if _, e = db.Exec(`INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,$3)`, f.organization, member.p.ActorID, member.role); e != nil {
			t.Fatal(e)
		}
	}
	dek, e := cryptoService.GenerateDEK()
	if e != nil {
		t.Fatal(e)
	}
	encrypted, nonce, e := cryptoService.EncryptDEK(dek)
	cryptography.SecureZero(dek)
	if e != nil {
		t.Fatal(e)
	}
	project, e := repository.NewProjectRepository(db, d).Create("Synthetic runs", "", f.admin.ActorID, encrypted, nonce)
	if e != nil {
		t.Fatal(e)
	}
	f.project = project.ID
	if _, e = db.Exec(`UPDATE projects SET organization_id=$2 WHERE id=$1`, f.project, f.organization); e != nil {
		t.Fatal(e)
	}
	f.environment = uuid.New()
	if _, e = db.Exec(`INSERT INTO environments(id,project_id,name) VALUES($1,$2,'development')`, f.environment, f.project); e != nil {
		t.Fatal(e)
	}
	guard := authority.Postgres(db)
	f.auth, e = mcpauth.New(db, d, audit, guard, mcpauth.Config{Issuer: "https://app.keepsave.example", AppURL: "https://app.keepsave.example", Resource: "https://app.keepsave.example/mcp"})
	if e != nil {
		t.Fatal(e)
	}
	content := []byte("# Permitted synthetic repository content\n")
	blob := sha1.New()
	_, _ = fmt.Fprintf(blob, "blob %d%c", len(content), 0)
	_, _ = blob.Write(content)
	blobSHA := hex.EncodeToString(blob.Sum(nil))
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.providerCalls.Add(1)
		if r.Header.Get("X-GitHub-Api-Version") != broker.APIVersion {
			t.Error("unfixed provider API version")
		}
		if r.URL.Path == "/app/installations/9/access_tokens" {
			var body struct {
				Repositories []int64           `json:"repository_ids"`
				Permissions  map[string]string `json:"permissions"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.Repositories) != 1 || body.Repositories[0] != 42 || len(body.Permissions) != 1 || body.Permissions["contents"] != "read" {
				t.Error("broader provider token requested")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"token": runsProviderCanary, "expires_at": time.Now().Add(time.Minute), "permissions": map[string]string{"contents": "read", "metadata": "read"}, "repositories": []map[string]int{{"id": 42}}})
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+runsProviderCanary || r.Method != "GET" {
			t.Error("provider request escaped read-only Broker")
		}
		switch r.URL.Path {
		case "/repositories/42":
			id := 42
			if f.providerMode.Load().(string) == "wrong_repository" {
				id = 43
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "full_name": "acme/demo"})
		case "/repos/acme/demo/commits/main":
			if f.providerMode.Load().(string) == "expire_check_after_commit" {
				if _, e := f.db.Exec(`UPDATE tool_connection_checks SET deadline=NOW()-INTERVAL '1 second' WHERE project_id=$1 AND state='dispatched'`, f.project); e != nil {
					t.Error("diagnostic deadline fixture failed:", e)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": runsCommit, "commit": map[string]any{"tree": map[string]string{"sha": runsTree}}})
		case "/repos/acme/demo/git/trees/" + runsTree:
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": runsTree, "truncated": false, "tree": []broker.TreeEntry{{Path: "README.md", Type: "blob", Mode: "100644", SHA: blobSHA, Size: int64(len(content))}}, "debug_token": runsProviderCanary})
		case "/repos/acme/demo/git/blobs/" + blobSHA:
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": blobSHA, "encoding": "base64", "size": len(content), "content": base64.StdEncoding.EncodeToString(content), "debug_token": runsProviderCanary})
		default:
			t.Errorf("unexpected provider destination %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(provider.Close)
	b, e := broker.NewFixture(runsCountingCustody{broker.CryptoCustody{Service: cryptoService}, &f.unwrapCalls}, provider.Client(), provider.URL)
	if e != nil {
		t.Fatal(e)
	}
	authorize := func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: d, RequireHumanSession: true}}).Authorize(ctx, p, a, r)
	}
	makeService := func() *Service {
		s := New(db, b, authorize, audit, Flags{Enabled: true, Admission: true, Dispatch: true, Endpoint: "https://app.keepsave.example/mcp"})
		s.PackageExporter = harness.ExportAutomation
		s.LockProjectSubjects = guard.LockProjectSubjects
		s.LockMaintenanceSubjects = guard.LockProjectMaintenance
		s.RequireGrantTx = f.auth.RequireGrantTx
		s.FamilyPrincipalTx = f.auth.FamilyPrincipalTx
		return s
	}
	f.s = makeService()
	f.s2 = makeService()
	artifact, e := f.s.CreateArtifact(ctx, f.admin, f.project, "approved-review", runsSource)
	if e != nil {
		t.Fatal("artifact:", e)
	}
	f.profile, e = f.s.CreateProfile(ctx, f.admin, f.project, artifact.ID)
	if e != nil {
		t.Fatal("profile:", e)
	}
	if e = f.s.ApproveProfile(ctx, f.approver, f.project, f.profile.ID, f.profile.Digest); e != nil {
		t.Fatal("profile approval:", e)
	}
	var pkg automation.Package
	f.packageID, pkg, e = f.s.Package(ctx, f.admin, f.project, f.profile.ID, "codex", "0.153.3")
	if e != nil {
		t.Fatal("package:", e)
	}
	pkgJSON, _ := json.Marshal(pkg)
	if e = f.s.ApprovePackage(ctx, f.approver, f.project, f.packageID, automation.Digest(pkgJSON)); e != nil {
		t.Fatal("package approval:", e)
	}
	rsaKey, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)})
	conn, e := f.s.CreateConnection(ctx, f.admin, f.project, 7, 9, keyPEM)
	cryptography.SecureZero(keyPEM)
	if e != nil {
		t.Fatal("connection:", e)
	}
	f.binding, e = f.s.CreateBinding(ctx, f.admin, f.project, conn, broker.Target{RepositoryID: 42, Owner: "acme", Repository: "demo", Reference: "main"})
	if e != nil {
		t.Fatal("binding:", e)
	}
	f.workload, e = f.s.EnrollWorkload(ctx, f.admin, f.project, strings.Repeat("c", 64), "registry.example/keepsave/connector@sha256:"+strings.Repeat("d", 64))
	if e != nil {
		t.Fatal("workload:", e)
	}
	f.grant, e = f.s.IssueGrant(ctx, f.admin, Grant{ProjectID: f.project, ProfileID: f.profile.ID, PackageID: f.packageID, BindingID: f.binding.ID, WorkloadID: f.workload.ID, ActorID: f.editor.ActorID, ClientID: runsClient, ExpiresAt: time.Now().Add(20 * time.Minute)})
	if e != nil {
		t.Fatal("grant:", e)
	}
	f.delegated = f.oauth(t, runsClient)
	if f.providerCalls.Load() != 0 {
		t.Fatal("management/enrollment performed an external read")
	}
	return f
}

func (f *runsPGFixture) oauth(t *testing.T, client string) policy.Principal {
	t.Helper()
	ctx := context.Background()
	verifier := strings.Repeat("v", 64)
	h := sha256.Sum256([]byte(verifier))
	callback := "http://127.0.0.1:17701/callback"
	if client == "keepsave-hermes-linux-v1" {
		callback = "http://127.0.0.1:17702/callback"
	}
	r, e := f.auth.BeginAuthorization(ctx, url.Values{"client_id": {client}, "redirect_uri": {callback}, "response_type": {"code"}, "resource": {"https://app.keepsave.example/mcp"}, "scope": {mcpauth.Scope + " offline_access"}, "state": {strings.Repeat("s", 32)}, "code_challenge": {base64.RawURLEncoding.EncodeToString(h[:])}, "code_challenge_method": {"S256"}})
	if e != nil {
		t.Fatal("OAuth request:", e)
	}
	redirect, e := f.auth.Decide(ctx, f.editor.ActorID, f.editor.SessionID, r.ID, true)
	if e != nil {
		t.Fatal("OAuth consent:", e)
	}
	parsed, e := url.Parse(redirect)
	if e != nil {
		t.Fatal(e)
	}
	issued, e := f.auth.Exchange(ctx, mcpauth.TokenRequest{GrantType: "authorization_code", ClientID: client, Code: parsed.Query().Get("code"), RedirectURI: callback, Verifier: verifier, Resource: "https://app.keepsave.example/mcp"})
	if e != nil {
		t.Fatal("OAuth exchange:", e)
	}
	p, e := f.auth.Validate(ctx, issued.AccessToken)
	if e != nil {
		t.Fatal("OAuth validate:", e)
	}
	f.oauthTokens[p.FamilyID] = issued
	return policy.Principal{Kind: policy.OAuthDelegation, SubjectID: p.UserID, ActorID: p.UserID, SessionID: p.SessionID, ParentGrantID: p.FamilyID, TokenID: p.TokenID.String(), ExpiresAt: p.ExpiresAt}
}
func (f *runsPGFixture) run(t *testing.T, key string) Run {
	t.Helper()
	r, e := f.s.CreateRunWithRequest(context.Background(), f.delegated, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: key})
	if e != nil {
		t.Fatal("run:", e)
	}
	if r.State != "active" || r.Commit != runsCommit || r.OwnerID != f.editor.ActorID || r.ExpiresAt.After(f.delegated.ExpiresAt) {
		t.Fatal("unbound/overlong run")
	}
	return r
}
func (f *runsPGFixture) request(t *testing.T, run Run, key string) Operation {
	t.Helper()
	o, e := f.s.Request(context.Background(), f.delegated, runsClient, run.ID, key, "read_file", Arguments{Path: "README.md"})
	if e != nil {
		t.Fatal("operation:", e)
	}
	return o
}
func (f *runsPGFixture) claim(t *testing.T) runner.Ticket {
	t.Helper()
	ticket, e := f.s.Claim(context.Background(), f.workload.CertificateSHA256, f.workload.ImageDigest)
	if e != nil {
		t.Fatal("claim:", e)
	}
	if ticket.Validate(f.workload.ImageDigest, time.Now()) != nil {
		t.Fatal("malformed ticket")
	}
	return ticket
}
func (f *runsPGFixture) execute(t *testing.T, ticket runner.Ticket) runner.ExecuteResponse {
	t.Helper()
	result, e := f.s2.Execute(context.Background(), f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: ticket.TicketID, Token: ticket.Token, Request: ticket.Request()})
	if e != nil {
		t.Fatal("execute:", e)
	}
	if result.Outcome != "succeeded" || result.ReceiptID == uuid.Nil || bytes.Contains(result.Result, []byte(runsProviderCanary)) {
		t.Fatal("unsafe successful response")
	}
	return result
}
func (f *runsPGFixture) count(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if e := f.db.QueryRow(query, args...).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func (f *runsPGFixture) sql(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, e := f.db.Exec(query, args...); e != nil {
		t.Fatal(e)
	}
}
func (f *runsPGFixture) snapshot(t *testing.T) [5]int64 {
	t.Helper()
	return [5]int64{f.count(t, `SELECT COUNT(*) FROM tool_operations`), f.count(t, `SELECT COUNT(*) FROM tool_receipts`), f.count(t, `SELECT COUNT(*) FROM audit_log`), f.count(t, `SELECT COUNT(*) FROM outbox_jobs`), f.count(t, `SELECT revision FROM audit_chain_head WHERE id=1`)}
}

func TestPostgresRunResolutionOperationIdempotencyAndResultCustody(t *testing.T) {
	f := runsPG(t)
	ctx := context.Background()
	r := f.run(t, "run-main")
	if f.providerCalls.Load() != 3 || f.count(t, `SELECT provider_operations FROM tool_runs WHERE id=$1`, r.ID) != 3 {
		t.Fatal("resolution did not charge actual HTTP calls")
	}
	repeat, e := f.s2.CreateRunWithRequest(ctx, f.delegated, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: "run-main"})
	if e != nil || repeat.ID != r.ID || f.providerCalls.Load() != 3 {
		t.Fatal("run retry re-executed provider resolution")
	}
	if _, e = f.s.CreateRunWithRequest(ctx, f.delegated, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: "run-main", DurationSeconds: 300}); !errors.Is(e, ErrConflict) {
		t.Fatal("changed run request idempotency accepted")
	}
	o := f.request(t, r, "read-main")
	same, e := f.s2.Request(ctx, f.delegated, runsClient, r.ID, "read-main", "read_file", Arguments{Path: "README.md"})
	if e != nil || same.ID != o.ID {
		t.Fatal("operation request not idempotent")
	}
	if _, e = f.s.Request(ctx, f.delegated, runsClient, r.ID, "read-main", "repository_tree", Arguments{}); !errors.Is(e, ErrConflict) {
		t.Fatal("changed operation digest accepted")
	}
	ticket := f.claim(t)
	if ticket.OperationID != o.ID || ticket.RunID != r.ID || ticket.GrantID != f.grant.ID || ticket.Fence != 1 || ticket.RequestDigest != runner.RequestDigest("read_file", Arguments{Path: "README.md"}) {
		t.Fatal("unbound ticket")
	}
	if active, e := f.s.TicketStatus(ctx, f.workload.CertificateSHA256, runner.TicketStatusRequest{TicketID: ticket.TicketID, Token: ticket.Token}); e != nil || !active.Active {
		t.Fatal("active ticket status denied")
	}
	dispatched := f.execute(t, ticket)
	if f.providerCalls.Load() != 6 || f.count(t, `SELECT provider_operations FROM tool_runs WHERE id=$1`, r.ID) != 6 {
		t.Fatal("operation did not charge mint/tree/blob")
	}
	status, e := f.s.Status(ctx, f.delegated, runsClient, o.ID, 0)
	if e != nil || status.Status != "succeeded" || len(status.Result) != 0 {
		t.Fatal("metadata status leaked result")
	}
	result, e := f.s2.Result(ctx, f.delegated, runsClient, o.ID)
	if e != nil || !bytes.Equal(result.Result, dispatched.Result) {
		t.Fatal("encrypted result retrieval failed")
	}
	var cipher []byte
	if e = f.db.QueryRow(`SELECT ciphertext FROM tool_result_spool WHERE operation_id=$1`, o.ID).Scan(&cipher); e != nil || bytes.Contains(cipher, []byte("Permitted synthetic")) || bytes.Contains(cipher, result.Result) {
		t.Fatal("result persisted in plaintext")
	}
	if _, e = f.s.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: ticket.TicketID, Token: ticket.Token, Request: ticket.Request()}); e == nil || f.providerCalls.Load() != 6 {
		t.Fatal("ticket replay reached provider")
	}
	for _, action := range []string{"tool.run.preparing", "tool.resolution.admitted", "tool.operation.queued", "tool.attempt.leased", "tool.attempt.dispatched", "tool.broker.admitted", "tool.attempt.completed", "tool.result.admitted"} {
		if f.count(t, `SELECT COUNT(*) FROM audit_log WHERE action=$1`, action) == 0 {
			t.Fatal("missing audit:", action)
		}
	}
}

func TestPostgresRunScopeSnapshotMetadataAndBindingDriftDenial(t *testing.T) {
	f := runsPG(t)
	ctx := context.Background()
	r := f.run(t, "scope-snapshot")
	assertScope := func(got Run) {
		t.Helper()
		if got.ID != r.ID || got.RepositoryID != 42 || got.Repository != "acme/demo" || got.EnvironmentID != f.environment || got.Environment != "development" || got.Reference != "main" || got.InstallationID != 9 || got.Commit != runsCommit {
			t.Fatal("run metadata lost its admitted immutable scope", got)
		}
	}
	assertScope(r)
	assertMetadata := func() {
		t.Helper()
		got, e := f.s2.GetRun(ctx, f.editor, runsClient, r.ID)
		if e != nil {
			t.Fatal(e)
		}
		assertScope(got)
		listed, e := f.s.ListRuns(ctx, f.editor, f.project, runsClient)
		if e != nil || len(listed) != 1 {
			t.Fatal("safe run listing unavailable", e)
		}
		assertScope(listed[0])
	}
	assertMetadata()
	available, e := f.s2.AvailableRuns(ctx, f.delegated, runsClient)
	if e != nil || len(available) != 1 {
		t.Fatal("available run discovery lost safe scope", e)
	}
	assertScope(available[0])
	o := f.request(t, r, "completed-before-drift")
	f.execute(t, f.claim(t))
	beforeCalls, beforeOpens := f.providerCalls.Load(), f.unwrapCalls.Load()
	otherEnvironment := uuid.New()
	f.sql(t, `INSERT INTO environments(id,project_id,name) VALUES($1,$2,'uat')`, otherEnvironment, f.project)
	for _, tc := range []struct {
		name, change, restore string
		args                  []any
	}{
		{"repository", `UPDATE tool_bindings SET repository_id=43 WHERE id=$1`, `UPDATE tool_bindings SET repository_id=42 WHERE id=$1`, []any{f.binding.ID}},
		{"repository_name", `UPDATE tool_bindings SET repository_name='other' WHERE id=$1`, `UPDATE tool_bindings SET repository_name='demo' WHERE id=$1`, []any{f.binding.ID}},
		{"reference", `UPDATE tool_bindings SET reference='other' WHERE id=$1`, `UPDATE tool_bindings SET reference='main' WHERE id=$1`, []any{f.binding.ID}},
		{"installation", `UPDATE tool_connections SET installation_id=10 WHERE id=$1`, `UPDATE tool_connections SET installation_id=9 WHERE id=$1`, []any{f.binding.ConnectionID}},
		{"environment", `UPDATE tool_bindings SET environment_id=$2 WHERE id=$1`, `UPDATE tool_bindings SET environment_id=$3 WHERE id=$1`, []any{f.binding.ID, otherEnvironment, f.environment}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changeArgs := tc.args
			if tc.name == "environment" {
				changeArgs = tc.args[:2]
			}
			f.sql(t, tc.change, changeArgs...)
			assertMetadata()
			if _, e := f.s.Request(ctx, f.delegated, runsClient, r.ID, "drift-"+tc.name, "read_file", Arguments{Path: "README.md"}); e == nil {
				t.Fatal("changed binding expanded an admitted run")
			}
			if _, e := f.s2.Result(ctx, f.delegated, runsClient, o.ID); e == nil {
				t.Fatal("changed binding retrieved protected output")
			}
			available, e := f.s.AvailableRuns(ctx, f.delegated, runsClient)
			if e != nil || len(available) != 0 {
				t.Fatal("changed binding remained discoverable as usable", e)
			}
			if f.providerCalls.Load() != beforeCalls || f.unwrapCalls.Load() != beforeOpens {
				t.Fatal("scope drift reached credential custody or provider")
			}
			restoreArgs := tc.args
			if tc.name == "environment" {
				// Keep placeholders contiguous; PostgreSQL cannot infer an unused
				// second parameter in the restoration statement.
				tc.restore = `UPDATE tool_bindings SET environment_id=$2 WHERE id=$1`
				restoreArgs = []any{f.binding.ID, f.environment}
			}
			f.sql(t, tc.restore, restoreArgs...)
		})
	}
	for _, query := range []string{
		`UPDATE tool_runs SET scope=jsonb_set(scope,'{repository_id}','43') WHERE id=$1`,
		`UPDATE tool_runs SET reference='other' WHERE id=$1`,
	} {
		if _, e := f.db.Exec(query, r.ID); e == nil {
			t.Fatal("database admitted run-scope rewriting")
		}
	}
	assertMetadata()
}

func TestPostgresRunClientFamilyTenantRepositoryAndApprovalBoundaries(t *testing.T) {
	f := runsPG(t)
	ctx := context.Background()
	assertApprovalMetadata := func(approved bool) {
		t.Helper()
		profiles, e := f.s.ListProfiles(ctx, f.editor, f.project)
		if e != nil || len(profiles) != 1 || profiles[0].ID != f.profile.ID || profiles[0].Approved != approved {
			t.Fatal("profile metadata misrepresented current approval:", e)
		}
		packages, e := f.s2.ListPackages(ctx, f.editor, f.project)
		if e != nil || len(packages) != 1 || packages[0].ID != f.packageID || packages[0].Approved != approved {
			t.Fatal("package metadata misrepresented current approval:", e)
		}
	}
	assertApprovalMetadata(true)
	r := f.run(t, "bound-run")
	o := f.request(t, r, "bound-read")
	otherFamily := f.oauth(t, runsClient)
	before := f.providerCalls.Load()
	if _, e := f.s.CreateRunWithRequest(ctx, otherFamily, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: "bound-run"}); !errors.Is(e, ErrConflict) && !errors.Is(e, ErrDenied) || f.providerCalls.Load() != before {
		t.Fatal("another family reused run idempotency key or read externally")
	}
	if _, e := f.s.Request(ctx, otherFamily, runsClient, r.ID, "foreign-family", "read_file", Arguments{Path: "README.md"}); e == nil {
		t.Fatal("another family adopted run")
	}
	if _, e := f.s.Status(ctx, f.editor, "keepsave-hermes-linux-v1", o.ID, 0); e == nil {
		t.Fatal("wrong client inspected run")
	}
	foreign := f.delegated
	foreign.TenantID = uuid.New()
	if _, e := f.s.Result(ctx, foreign, runsClient, o.ID); e == nil {
		t.Fatal("wrong tenant admitted")
	}
	if _, e := f.s.CreateRunWithRequest(ctx, f.delegated, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: "wrong-ref", Reference: "other-branch"}); e == nil || f.providerCalls.Load() != before {
		t.Fatal("unapproved reference reached provider")
	}
	if _, e := f.s.CreateProjectRun(ctx, f.editor, uuid.New(), runsClient, CreateRunRequest{FamilyID: f.delegated.ParentGrantID, GrantID: f.grant.ID, RequestKey: "foreign-project"}); e == nil {
		t.Fatal("grant adopted foreign project")
	}
	if e := f.s.ApproveProfile(ctx, f.admin, f.project, f.profile.ID, f.profile.Digest); e == nil {
		t.Fatal("creator self-approved")
	}
	if _, e := f.s.CreateBindingInEnvironment(ctx, f.admin, f.project, f.binding.ConnectionID, "production", f.binding.Target); e == nil || f.providerCalls.Load() != before {
		t.Fatal("production environment admitted or read externally")
	}
	human, e := f.s.CreateRunWithRequest(ctx, f.editor, runsClient, CreateRunRequest{FamilyID: f.delegated.ParentGrantID, GrantID: f.grant.ID, RequestKey: "browser-run"})
	if e != nil || human.Commit != runsCommit {
		t.Fatal("owned browser family mapping failed:", e)
	}
	if _, e = f.s.CreateRunWithRequest(ctx, f.editor, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: "browser-no-family"}); e == nil {
		t.Fatal("human run omitted exact family")
	}
	ticket := f.claim(t)
	for _, change := range []func(*runner.ExecuteRequest){func(r *runner.ExecuteRequest) { r.Request.RunID = uuid.New() }, func(r *runner.ExecuteRequest) { r.Request.GrantID = uuid.New() }, func(r *runner.ExecuteRequest) { r.Request.Fence++; r.Request.Attempt++ }, func(r *runner.ExecuteRequest) { r.Request.Nonce = strings.Repeat("a", 64) }, func(r *runner.ExecuteRequest) {
		r.Request.Arguments.Path = "other.md"
		r.Request.RequestDigest = runner.RequestDigest(r.Request.Kind, r.Request.Arguments)
	}} {
		req := runner.ExecuteRequest{TicketID: ticket.TicketID, Token: ticket.Token, Request: ticket.Request()}
		change(&req)
		if _, e = f.s.Execute(ctx, f.workload.CertificateSHA256, req); e == nil {
			t.Fatal("altered ticket executed")
		}
	}
	if _, e = f.s.Execute(ctx, strings.Repeat("e", 64), runner.ExecuteRequest{TicketID: ticket.TicketID, Token: ticket.Token, Request: ticket.Request()}); e == nil {
		t.Fatal("different certificate redeemed ticket")
	}
	f.execute(t, ticket)
	f.sql(t, `UPDATE organization_members SET role='editor' WHERE organization_id=$1 AND user_id=$2`, f.organization, f.approver.ActorID)
	assertApprovalMetadata(false)
	f.sql(t, `UPDATE organization_members SET role='promoter' WHERE organization_id=$1 AND user_id=$2`, f.organization, f.approver.ActorID)
	assertApprovalMetadata(false)
	if _, e = f.s.Result(ctx, f.editor, runsClient, o.ID); e == nil {
		t.Fatal("restored role revived stale approval epochs")
	}
	if e := f.s.Revoke(ctx, f.admin, f.project, f.profile.ArtifactID, "artifact"); e != nil {
		t.Fatal("artifact revoke:", e)
	}
	if profiles, e := f.s.ListProfiles(ctx, f.editor, f.project); e != nil || len(profiles) != 0 {
		t.Fatal("revoked artifact remained in profile metadata:", e)
	}
	if packages, e := f.s2.ListPackages(ctx, f.editor, f.project); e != nil || len(packages) != 0 {
		t.Fatal("revoked artifact remained in package metadata:", e)
	}
}

func TestPostgresRunConcurrentClaimFencesAndCancellation(t *testing.T) {
	f := runsPG(t)
	ctx := context.Background()
	r := f.run(t, "claim-run")
	o := f.request(t, r, "claim-read")
	if _, e := f.s.Claim(ctx, strings.Repeat("e", 64), f.workload.ImageDigest); e == nil {
		t.Fatal("unenrolled certificate claimed work")
	}
	if _, e := f.s.Claim(ctx, f.workload.CertificateSHA256, "registry.example/connector@sha256:"+strings.Repeat("e", 64)); e == nil {
		t.Fatal("unapproved image claimed work")
	}
	if f.count(t, `SELECT COUNT(*) FROM tool_tickets`) != 0 || f.providerCalls.Load() != 3 {
		t.Fatal("denied enrollment created ticket or called provider")
	}
	var won atomic.Int32
	var ticket runner.Ticket
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := f.s
			if i%2 == 1 {
				s = f.s2
			}
			got, e := s.Claim(ctx, f.workload.CertificateSHA256, f.workload.ImageDigest)
			if e == nil {
				won.Add(1)
				mu.Lock()
				ticket = got
				mu.Unlock()
			} else if !errors.Is(e, ErrNoWork) {
				t.Errorf("claim failed: %v", e)
			}
		}(i)
	}
	wg.Wait()
	if won.Load() != 1 || ticket.OperationID != o.ID {
		t.Fatal("duplicate claim")
	}
	f.sql(t, `UPDATE tool_operations SET lease_until=NOW()-INTERVAL '1 second' WHERE id=$1`, o.ID)
	second := f.claim(t)
	if second.Fence != ticket.Fence+1 {
		t.Fatal("attempt not fenced after pre-dispatch expiry")
	}
	if _, e := f.s.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: ticket.TicketID, Token: ticket.Token, Request: ticket.Request()}); e == nil {
		t.Fatal("stale fence executed")
	}
	if e := f.s.CancelOperation(ctx, f.delegated, runsClient, o.ID); e != nil {
		t.Fatal(e)
	}
	if active, e := f.s.TicketStatus(ctx, f.workload.CertificateSHA256, runner.TicketStatusRequest{TicketID: second.TicketID, Token: second.Token}); e == nil && active.Active {
		t.Fatal("cancelled ticket active")
	}
	if _, e := f.s.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: second.TicketID, Token: second.Token, Request: second.Request()}); e == nil || f.providerCalls.Load() != 3 {
		t.Fatal("cancelled attempt dispatched")
	}
}

func TestPostgresRunRevocationDeniesResultAndNewDispatch(t *testing.T) {
	for _, mode := range []string{"grant", "oauth_family", "session", "actor_epoch", "connection", "binding", "workload", "package", "profile", "artifact", "environment_prod", "token_rotation", "operation_cancel", "run_cancel"} {
		t.Run(mode, func(t *testing.T) {
			f := runsPG(t)
			ctx := context.Background()
			r := f.run(t, "revoke-run")
			o := f.request(t, r, "completed")
			ticket := f.claim(t)
			f.execute(t, ticket)
			queued := f.request(t, r, "queued")
			next := f.claim(t)
			before, opens := f.providerCalls.Load(), f.unwrapCalls.Load()
			switch mode {
			case "grant":
				if e := f.s.Revoke(ctx, f.admin, f.project, f.grant.ID, "grant"); e != nil {
					t.Fatal(e)
				}
			case "oauth_family":
				if e := f.auth.RevokeDelegation(ctx, f.editor, f.delegated.ParentGrantID); e != nil {
					t.Fatal(e)
				}
			case "session":
				f.sql(t, `UPDATE session_tokens SET revoked=TRUE WHERE id=$1`, f.editor.SessionID)
			case "actor_epoch":
				f.sql(t, `UPDATE organization_members SET role='viewer' WHERE organization_id=$1 AND user_id=$2`, f.organization, f.editor.ActorID)
			case "connection":
				if e := f.s.Revoke(ctx, f.admin, f.project, f.binding.ConnectionID, "connection"); e != nil {
					t.Fatal(e)
				}
			case "binding":
				if e := f.s.Revoke(ctx, f.admin, f.project, f.binding.ID, "binding"); e != nil {
					t.Fatal(e)
				}
			case "workload":
				if e := f.s.Revoke(ctx, f.admin, f.project, f.workload.ID, "workload"); e != nil {
					t.Fatal(e)
				}
			case "package":
				if e := f.s.RevokePackage(ctx, f.admin, f.project, f.packageID); e != nil {
					t.Fatal(e)
				}
			case "profile":
				if e := f.s.Revoke(ctx, f.admin, f.project, f.profile.ID, "profile"); e != nil {
					t.Fatal(e)
				}
			case "artifact":
				if e := f.s.Revoke(ctx, f.admin, f.project, f.profile.ArtifactID, "artifact"); e != nil {
					t.Fatal(e)
				}
			case "environment_prod":
				f.sql(t, `UPDATE environments SET name='production' WHERE id=$1`, f.environment)
			case "token_rotation":
				issued := f.oauthTokens[f.delegated.ParentGrantID]
				rotated, e := f.auth.Exchange(ctx, mcpauth.TokenRequest{GrantType: "refresh_token", ClientID: runsClient, Resource: "https://app.keepsave.example/mcp", RefreshToken: issued.RefreshToken})
				if e != nil {
					t.Fatal("actual token rotation:", e)
				}
				current, e := f.auth.Validate(ctx, rotated.AccessToken)
				if e != nil || current.FamilyID != f.delegated.ParentGrantID || current.TokenID.String() == f.delegated.TokenID {
					t.Fatal("rotation fixture did not preserve family and replace token")
				}
				rotatedPrincipal := policy.Principal{Kind: policy.OAuthDelegation, SubjectID: current.UserID, ActorID: current.UserID, SessionID: current.SessionID, ParentGrantID: current.FamilyID, TokenID: current.TokenID.String(), ExpiresAt: current.ExpiresAt}
				if _, e := f.s.CreateRunWithRequest(ctx, rotatedPrincipal, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: "revoke-run"}); !errors.Is(e, ErrConflict) && !errors.Is(e, ErrDenied) || f.providerCalls.Load() != before {
					t.Fatal("new token reused another token's run idempotency key")
				}
			case "operation_cancel":
				if e := f.s.CancelOperation(ctx, f.delegated, runsClient, o.ID); e != nil {
					t.Fatal(e)
				}
				if e := f.s.CancelOperation(ctx, f.delegated, runsClient, queued.ID); e != nil {
					t.Fatal(e)
				}
			case "run_cancel":
				if e := f.s.CancelRun(ctx, f.delegated, runsClient, r.ID); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := f.s2.Result(ctx, f.editor, runsClient, o.ID); e == nil {
				t.Fatal("human bypassed revoked parent/result authority")
			}
			if f.unwrapCalls.Load() != opens {
				t.Fatal("revoked result decrypted before current authority admission")
			}
			if _, e := f.s2.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: next.TicketID, Token: next.Token, Request: next.Request()}); e == nil || f.providerCalls.Load() != before {
				t.Fatal("revoked attempt reached provider")
			}
			if f.unwrapCalls.Load() != opens {
				t.Fatal("revoked attempt unwrapped provider credential")
			}
		})
	}
}

func TestPostgresRunBudgetsAndWrongRepositoryResolution(t *testing.T) {
	f := runsPG(t)
	ctx := context.Background()
	f.providerMode.Store("wrong_repository")
	if _, e := f.s.CreateRunWithRequest(ctx, f.delegated, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: "wrong-repository"}); e == nil {
		t.Fatal("provider repository mismatch accepted")
	}
	f.providerMode.Store("ok")
	r := f.run(t, "budget-run")
	o := f.request(t, r, "last-provider-call")
	ticket := f.claim(t)
	f.sql(t, `UPDATE tool_runs SET provider_operations=99 WHERE id=$1`, r.ID)
	before := f.providerCalls.Load()
	if _, e := f.s.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: ticket.TicketID, Token: ticket.Token, Request: ticket.Request()}); e == nil || f.providerCalls.Load() != before+1 || f.count(t, `SELECT provider_operations FROM tool_runs WHERE id=$1`, r.ID) != 100 {
		t.Fatal("provider count budget not enforced per HTTP call")
	}
	if _, e := f.s.Request(ctx, f.delegated, runsClient, r.ID, "over-provider-budget", "read_file", Arguments{Path: "README.md"}); e == nil {
		t.Fatal("provider budget allowed request")
	}
	if f.count(t, `SELECT COUNT(*) FROM tool_result_spool WHERE operation_id=$1`, o.ID) != 0 {
		t.Fatal("failed budget dispatch persisted result")
	}
	bytesRun := f.run(t, "byte-run")
	f.sql(t, `UPDATE tool_runs SET result_bytes=33554432 WHERE id=$1`, bytesRun.ID)
	before = f.providerCalls.Load()
	if _, e := f.s.Request(ctx, f.delegated, runsClient, bytesRun.ID, "over-byte-budget", "read_file", Arguments{Path: "README.md"}); e == nil || f.providerCalls.Load() != before {
		t.Fatal("32MiB budget allowed request")
	}
	near := f.run(t, "byte-near-limit")
	nearOp := f.request(t, near, "too-large-completion")
	nearTicket := f.claim(t)
	f.sql(t, `UPDATE tool_runs SET result_bytes=33554431 WHERE id=$1`, near.ID)
	if _, e := f.s.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: nearTicket.TicketID, Token: nearTicket.Token, Request: nearTicket.Request()}); e == nil || f.count(t, `SELECT COUNT(*) FROM tool_result_spool WHERE operation_id=$1`, nearOp.ID) != 0 {
		t.Fatal("completion overflow delivered/spooled")
	}
	if _, e := f.s.RetryOperation(ctx, f.delegated, runsClient, nearOp.ID); e == nil {
		t.Fatal("possibly dispatched outcome blindly retried")
	}
}

func TestPostgresRunAuditFailureRollsBackAdmissionAndDispatch(t *testing.T) {
	f := runsPG(t)
	ctx := context.Background()
	r := f.run(t, "audit-run")
	before := f.snapshot(t)
	f.sql(t, `ALTER TABLE audit_log ADD CONSTRAINT synthetic_tool_audit_failure CHECK(action!='tool.operation.queued') NOT VALID`)
	if _, e := f.s.Request(ctx, f.delegated, runsClient, r.ID, "failed-admission", "read_file", Arguments{Path: "README.md"}); e == nil {
		t.Fatal("audit failure admitted operation")
	}
	if f.snapshot(t) != before {
		t.Fatal("failed audit left operation/receipt/outbox/chain rows")
	}
	f.sql(t, `ALTER TABLE audit_log DROP CONSTRAINT synthetic_tool_audit_failure`)
	o := f.request(t, r, "dispatch-audit")
	ticket := f.claim(t)
	calls := f.providerCalls.Load()
	f.sql(t, `ALTER TABLE audit_log ADD CONSTRAINT synthetic_tool_audit_failure CHECK(action!='tool.attempt.dispatched') NOT VALID`)
	if _, e := f.s.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: ticket.TicketID, Token: ticket.Token, Request: ticket.Request()}); e == nil {
		t.Fatal("unaudited dispatch accepted")
	}
	if f.providerCalls.Load() != calls || f.count(t, `SELECT COUNT(*) FROM tool_tickets WHERE id=$1 AND consumed_at IS NOT NULL`, ticket.TicketID) != 0 || f.count(t, `SELECT COUNT(*) FROM tool_operations WHERE id=$1 AND status='leased'`, o.ID) != 1 {
		t.Fatal("dispatch audit failure consumed ticket or reached provider")
	}
	f.sql(t, `ALTER TABLE audit_log DROP CONSTRAINT synthetic_tool_audit_failure`)
	f.execute(t, ticket)
}

func TestPostgresConnectionCheckRequiresAcknowledgedReadsAndTokenMint(t *testing.T) {
	f := runsPG(t)
	ctx := context.Background()
	for _, ack := range [][2]bool{{false, false}, {false, true}, {true, false}} {
		if _, e := f.s.CheckConnection(ctx, f.admin, f.project, f.binding.ConnectionID, f.binding.ID, ack[0], ack[1]); e == nil {
			t.Fatal("diagnostic accepted omitted external effect acknowledgement")
		}
	}
	if _, e := f.s.CheckConnection(ctx, f.editor, f.project, f.binding.ConnectionID, f.binding.ID, true, true); e == nil {
		t.Fatal("repository diagnostic bypassed connection authority")
	}
	if _, e := f.s.CheckConnection(ctx, f.admin, f.project, f.binding.ConnectionID, uuid.New(), true, true); e == nil {
		t.Fatal("diagnostic adopted an unbound repository")
	}
	if f.providerCalls.Load() != 0 || f.unwrapCalls.Load() != 0 || f.count(t, `SELECT COUNT(*) FROM tool_connection_checks`) != 0 {
		t.Fatal("denied diagnostic unwrapped or read externally")
	}
	check, e := f.s.CheckConnection(ctx, f.admin, f.project, f.binding.ConnectionID, f.binding.ID, true, true)
	if e != nil || check.ID == uuid.Nil || check.Status != "succeeded" || check.RepositoryID != 42 || check.Commit != runsCommit || !check.ExternalRead || !check.AuthorizationTokenMint || check.RepositoryWrite {
		t.Fatal("explicit connection diagnostic failed or overstated effects:", e)
	}
	if f.providerCalls.Load() != 3 || f.unwrapCalls.Load() != 1 || f.count(t, `SELECT provider_operations FROM tool_connection_checks WHERE id=$1`, check.ID) != 3 {
		t.Fatal("diagnostic did not charge precisely mint, repository and commit reads")
	}
	if f.count(t, `SELECT COUNT(*) FROM tool_connection_checks WHERE id=$1 AND state='succeeded' AND finished_at IS NOT NULL AND finished_at<deadline`, check.ID) != 1 {
		t.Fatal("successful diagnostic lacked one unexpired durable completion")
	}
	metadata, e := json.Marshal(check)
	if e != nil || bytes.Contains(metadata, []byte(runsProviderCanary)) || bytes.Contains(metadata, []byte("PRIVATE KEY")) || bytes.Contains(metadata, []byte("Permitted synthetic")) {
		t.Fatal("diagnostic published credential or repository contents")
	}
	for _, action := range []string{"tool.connection.check.admitted", "tool.connection.check.access", "tool.connection.check.completed"} {
		if f.count(t, `SELECT COUNT(*) FROM audit_log WHERE action=$1`, action) == 0 {
			t.Fatal("missing diagnostic audit:", action)
		}
	}
	// Advance only this diagnostic's persisted deadline after its third read.
	// The returned provider metadata must not bypass durable completion checks.
	f.providerMode.Store("expire_check_after_commit")
	if _, e := f.s.CheckConnection(ctx, f.admin, f.project, f.binding.ConnectionID, f.binding.ID, true, true); !errors.Is(e, ErrDenied) {
		t.Fatal("expired completion published diagnostic metadata:", e)
	}
	if f.providerCalls.Load() != 6 || f.unwrapCalls.Load() != 2 || f.count(t, `SELECT COUNT(*) FROM tool_connection_checks WHERE state='succeeded'`) != 1 || f.count(t, `SELECT COUNT(*) FROM audit_log WHERE action='tool.connection.check.completed'`) != 1 {
		t.Fatal("expired diagnostic claimed a completed row or completed audit")
	}
}

func TestPostgresInstructionVersionsAndApprovalsCannotBeMutated(t *testing.T) {
	f := runsPG(t)
	for _, attempt := range []struct {
		query string
		id    uuid.UUID
	}{
		{`UPDATE tool_artifacts SET source=source||' changed' WHERE id=$1`, f.profile.ArtifactID},
		{`UPDATE tool_profiles SET digest=REPEAT('e',64) WHERE id=$1`, f.profile.ID},
		{`UPDATE tool_profiles SET manifest=manifest||'{"max_seconds":999}'::jsonb WHERE id=$1`, f.profile.ID},
		{`UPDATE tool_profiles SET approved_until=approved_until+INTERVAL '1 day' WHERE id=$1`, f.profile.ID},
		{`UPDATE tool_profile_packages SET content=content||'{"injected":true}'::jsonb WHERE id=$1`, f.packageID},
		{`UPDATE tool_profile_packages SET approver_epoch=approver_epoch+1 WHERE id=$1`, f.packageID},
	} {
		if _, e := f.db.Exec(attempt.query, attempt.id); e == nil {
			t.Fatal("database accepted mutation of reviewed immutable content or approval")
		}
	}
	r := f.run(t, "immutable-review")
	f.request(t, r, "preserved-source")
	f.execute(t, f.claim(t))
}

func TestPostgresExpiredAttemptRecoveryNeverReplaysDispatchedWork(t *testing.T) {
	f := runsPG(t)
	ctx := context.Background()
	r := f.run(t, "recovery-run")
	completed := f.request(t, r, "completed-before-recovery")
	f.execute(t, f.claim(t))
	leased := f.request(t, r, "expired-before-dispatch")
	leasedTicket := f.claim(t)
	dispatched := f.request(t, r, "interrupted-after-dispatch")
	dispatchedTicket := f.claim(t)
	// Persist the two crash boundaries directly. This verifies durable recovery;
	// it does not assert an actual process crash or an upstream execution.
	f.sql(t, `UPDATE tool_operations SET deadline=NOW()-INTERVAL '1 second' WHERE id=$1`, leased.ID)
	f.sql(t, `UPDATE tool_operations SET status='dispatched',deadline=NOW()-INTERVAL '1 second' WHERE id=$1`, dispatched.ID)
	f.sql(t, `UPDATE tool_attempts SET state='dispatched',deadline=NOW()-INTERVAL '1 second' WHERE operation_id=$1`, dispatched.ID)
	f.sql(t, `UPDATE tool_tickets SET consumed_at=NOW() WHERE id=$1`, dispatchedTicket.TicketID)
	f.sql(t, `UPDATE tool_result_spool SET expires_at=NOW()-INTERVAL '1 second' WHERE operation_id=$1`, completed.ID)
	calls, opens := f.providerCalls.Load(), f.unwrapCalls.Load()
	changed, e := f.s2.ReconcileExpired(ctx)
	if e != nil || changed != 1 {
		t.Fatal("durable recovery failed:", e)
	}
	if f.providerCalls.Load() != calls || f.unwrapCalls.Load() != opens || f.count(t, `SELECT COUNT(*) FROM tool_result_spool WHERE operation_id=$1`, completed.ID) != 0 {
		t.Fatal("maintenance dispatched, decrypted or preserved expired result")
	}
	if f.count(t, `SELECT COUNT(*) FROM tool_operations WHERE id=$1 AND status='failed' AND outcome='pre_dispatch_failure'`, leased.ID) != 1 || f.count(t, `SELECT COUNT(*) FROM tool_operations WHERE id=$1 AND status='uncertain' AND outcome='deadline_elapsed'`, dispatched.ID) != 1 {
		t.Fatal("recovery confused pre-dispatch failure and uncertain dispatch")
	}
	if _, e := f.s.Claim(ctx, f.workload.CertificateSHA256, f.workload.ImageDigest); !errors.Is(e, ErrNoWork) {
		t.Fatal("recovery automatically replayed failed or uncertain work")
	}
	if _, e := f.s.RetryOperation(ctx, f.delegated, runsClient, dispatched.ID); e == nil {
		t.Fatal("uncertain upstream outcome retried")
	}
	if _, e := f.s.Result(ctx, f.delegated, runsClient, completed.ID); e == nil {
		t.Fatal("expired result returned after recovery")
	}
	if retried, e := f.s.RetryOperation(ctx, f.delegated, runsClient, leased.ID); e != nil || retried.ID != leased.ID || retried.Status != "queued" {
		t.Fatal("explicit pre-dispatch retry denied:", e)
	}
	retry := f.claim(t)
	if retry.Fence != leasedTicket.Fence+1 || retry.OperationID != leased.ID {
		t.Fatal("explicit retry did not advance fence")
	}
	if _, e := f.s.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: leasedTicket.TicketID, Token: leasedTicket.Token, Request: leasedTicket.Request()}); e == nil || f.providerCalls.Load() != calls {
		t.Fatal("recovered stale ticket reached provider")
	}
	f.sql(t, `UPDATE tool_runs SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, r.ID)
	if n, e := f.s.ReconcileExpired(ctx); e != nil || n != 1 {
		t.Fatal("run expiry did not reduce outstanding authority:", e)
	}
	if _, e := f.s.RetryOperation(ctx, f.delegated, runsClient, leased.ID); e == nil {
		t.Fatal("expired run allowed retry")
	}
	if active, e := f.s.TicketStatus(ctx, f.workload.CertificateSHA256, runner.TicketStatusRequest{TicketID: retry.TicketID, Token: retry.Token}); e == nil && active.Active {
		t.Fatal("expired run retained active ticket")
	}
	if f.providerCalls.Load() != calls || f.unwrapCalls.Load() != opens {
		t.Fatal("recovery contacted provider or decrypted results")
	}
}

func TestPostgresRunKillSwitchPreservesRevocationAndMetadata(t *testing.T) {
	assertMetadata := func(t *testing.T, f *runsPGFixture, r Run, o Operation) {
		t.Helper()
		ctx := context.Background()
		status, e := f.s2.Status(ctx, f.editor, runsClient, o.ID, 0)
		if e != nil || status.ID != o.ID || len(status.Result) != 0 {
			t.Fatal("current editor lost metadata status or received protected output:", e)
		}
		if got, e := f.s.GetRun(ctx, f.editor, runsClient, r.ID); e != nil || got.ID != r.ID {
			t.Fatal("current editor lost run metadata:", e)
		}
		receipts, e := f.s2.Receipts(ctx, f.editor, runsClient, r.ID)
		if e != nil || len(receipts) == 0 {
			t.Fatal("current editor lost durable receipts:", e)
		}
		data, e := json.Marshal(receipts)
		if e != nil || bytes.Contains(data, []byte(runsProviderCanary)) || bytes.Contains(data, []byte("Permitted synthetic")) {
			t.Fatal("metadata receipts leaked a provider token or result")
		}
	}
	t.Run("kill_switch", func(t *testing.T) {
		f := runsPG(t)
		ctx := context.Background()
		r := f.run(t, "switch-run")
		completed := f.request(t, r, "completed")
		f.execute(t, f.claim(t))
		pending := f.request(t, r, "cancel-while-disabled")
		pendingTicket := f.claim(t)
		expired := f.request(t, r, "expire-while-disabled")
		f.claim(t)
		f.sql(t, `UPDATE tool_operations SET deadline=NOW()-INTERVAL '1 second' WHERE id=$1`, expired.ID)
		calls, opens := f.providerCalls.Load(), f.unwrapCalls.Load()
		f.s.flags.Enabled, f.s2.flags.Enabled = false, false
		if f.s.Enabled() || !f.s.Ready() || f.s2.Enabled() || !f.s2.Ready() {
			t.Fatal("kill switch removed configured control readiness")
		}
		if _, e := f.s.Request(ctx, f.delegated, runsClient, r.ID, "disabled-request", "read_file", Arguments{Path: "README.md"}); !errors.Is(e, ErrUnavailable) {
			t.Fatal("disabled admission accepted a new operation:", e)
		}
		if _, e := f.s2.Result(ctx, f.editor, runsClient, completed.ID); !errors.Is(e, ErrUnavailable) {
			t.Fatal("disabled result publication decrypted output:", e)
		}
		if _, e := f.s.Claim(ctx, f.workload.CertificateSHA256, f.workload.ImageDigest); !errors.Is(e, ErrUnavailable) {
			t.Fatal("disabled supervisor claimed work:", e)
		}
		if _, e := f.s.CreateRunWithRequest(ctx, f.delegated, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: "disabled-run"}); !errors.Is(e, ErrUnavailable) {
			t.Fatal("disabled admission resolved a new run:", e)
		}
		if _, e := f.s2.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: pendingTicket.TicketID, Token: pendingTicket.Token, Request: pendingTicket.Request()}); !errors.Is(e, ErrUnavailable) {
			t.Fatal("disabled dispatch executed upstream:", e)
		}
		assertMetadata(t, f, r, completed)
		if changed, e := f.s2.ReconcileExpired(ctx); e != nil || changed != 1 || f.count(t, `SELECT COUNT(*) FROM tool_receipts WHERE operation_id=$1 AND action='tool.attempt.expired'`, expired.ID) != 1 {
			t.Fatal("disabled maintenance lost durable expiry:", e)
		}
		if e := f.s.CancelOperation(ctx, f.editor, runsClient, pending.ID); e != nil {
			t.Fatal("disabled operation cancellation failed:", e)
		}
		if e := f.s2.CancelRun(ctx, f.editor, runsClient, r.ID); e != nil {
			t.Fatal("disabled run cancellation failed:", e)
		}
		if e := f.s.Revoke(ctx, f.admin, f.project, f.grant.ID, "grant"); e != nil {
			t.Fatal("disabled grant revocation failed:", e)
		}
		if f.count(t, `SELECT COUNT(*) FROM tool_grants WHERE id=$1 AND revoked_at IS NOT NULL`, f.grant.ID) != 1 {
			t.Fatal("kill switch failed to persist grant revocation")
		}
		f.s.flags.Enabled, f.s2.flags.Enabled = true, true
		if _, e := f.s2.CreateRunWithRequest(ctx, f.delegated, runsClient, CreateRunRequest{GrantID: f.grant.ID, RequestKey: "reenabled-revoked-grant"}); e == nil {
			t.Fatal("reenabling resurrected a revoked grant")
		}
		if _, e := f.s.Result(ctx, f.editor, runsClient, completed.ID); e == nil {
			t.Fatal("reenabling published cancelled/revoked output")
		}
		assertMetadata(t, f, r, completed)
		if f.providerCalls.Load() != calls || f.unwrapCalls.Load() != opens {
			t.Fatal("disabled/revoked controls contacted provider or decrypted output")
		}
	})
	for _, departed := range []string{"issuer", "approver"} {
		t.Run(departed+"_membership_departure", func(t *testing.T) {
			f := runsPG(t)
			ctx := context.Background()
			r := f.run(t, "ancestor-departure")
			completed := f.request(t, r, "completed")
			f.execute(t, f.claim(t))
			pending := f.request(t, r, "pending")
			ticket := f.claim(t)
			calls, opens := f.providerCalls.Load(), f.unwrapCalls.Load()
			ancestor := f.admin.ActorID
			if departed == "approver" {
				ancestor = f.approver.ActorID
			}
			f.sql(t, `DELETE FROM organization_members WHERE organization_id=$1 AND user_id=$2`, f.organization, ancestor)
			// The feature stays enabled here: denial comes from departed current
			// authority, while the editor retains metadata and cancel controls.
			assertMetadata(t, f, r, completed)
			if _, e := f.s2.Result(ctx, f.editor, runsClient, completed.ID); e == nil {
				t.Fatal("departed grant ancestor allowed protected result")
			}
			if _, e := f.s.Execute(ctx, f.workload.CertificateSHA256, runner.ExecuteRequest{TicketID: ticket.TicketID, Token: ticket.Token, Request: ticket.Request()}); e == nil {
				t.Fatal("departed grant ancestor allowed provider dispatch")
			}
			if e := f.s.CancelOperation(ctx, f.editor, runsClient, pending.ID); e != nil {
				t.Fatal("ancestor departure blocked editor operation cancellation:", e)
			}
			if e := f.s2.CancelRun(ctx, f.editor, runsClient, r.ID); e != nil {
				t.Fatal("ancestor departure blocked editor run cancellation:", e)
			}
			assertMetadata(t, f, r, completed)
			if f.providerCalls.Load() != calls || f.unwrapCalls.Load() != opens {
				t.Fatal("departed ancestor caused credential unwrap or result decryption")
			}
		})
	}
}
