package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"github.com/santapong/KeepSave/backend/migrations"
)

// The child executes the actual CLI with only synthetic, explicitly supplied
// configuration. No operator environment, credential file or provider is read.
func TestPlatformRecoveryLifecycleCLIHelper(t *testing.T) {
	if os.Getenv("KEEPSAVE_RECOVERY_LIFECYCLE_HELPER") != "1" {
		t.Skip("subprocess helper")
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"keepsave-vault"}, os.Args[i+1:]...)
			flag.CommandLine = flag.NewFlagSet("keepsave-vault", flag.ExitOnError)
			main()
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func TestPlatformRecoveryLifecycleFreshDatabaseCLI(t *testing.T) {
	if os.Getenv("KEEPSAVE_PLATFORM_POSTGRES_TEST") != "1" {
		t.Skip("fixed disposable PostgreSQL harness required; SQLite is not acceptance")
	}
	const fixture = "postgres://keepsave_platform_test:local-test-only@keepsave-platform-postgres-test:5432/keepsave_platform_test?sslmode=disable"
	admin, _, err := repository.NewDB(fixture)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	fresh := func(label string) (*sql.DB, repository.Dialect, string) {
		t.Helper()
		name := "lifecycle_" + label + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(fixture)
		if err != nil {
			t.Fatal(err)
		}
		u.Path = "/" + name
		db, dialect, err := repository.NewDB(u.String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = db.Close()
			if _, err := admin.Exec(`DROP DATABASE ` + name); err != nil {
				t.Error(err)
			}
		})
		return db, dialect, u.String()
	}
	count := func(db *sql.DB, query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	ctx := context.Background()
	source, dialect, _ := fresh("source")
	if err := repository.RunMigrationsFS(source, dialect, migrations.FS); err != nil {
		t.Fatal(err)
	}
	material := bytes.Repeat([]byte{0x52}, 32)
	keys, err := crypto.NewService(material)
	if err != nil {
		t.Fatal(err)
	}
	audit := repository.NewAuditRepository(source, dialect)
	audit.SetChainKey(keys.DeriveAuditChainKey())
	owner, err := repository.NewUserRepository(source, dialect).Create("source-custodian@example.invalid", "!")
	if err != nil {
		t.Fatal(err)
	}
	projects := service.NewProjectService(repository.NewProjectRepository(source, dialect), repository.NewEnvironmentRepository(source, dialect), audit, keys)
	project, err := projects.Create("Synthetic lifecycle recovery source", "", owner.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	operator := policy.Principal{Kind: policy.Human, SubjectID: owner.ID, ActorID: owner.ID}
	authorize := func(db *sql.DB, d repository.Dialect) vault.AuthorizeTx {
		return func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
			return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: d}}).Authorize(ctx, p, a, r)
		}
	}
	v := vault.New(source, keys, authorize(source, dialect), audit)
	if err := v.Enroll(ctx, operator, project.ID); err != nil {
		t.Fatal(err)
	}
	record, err := v.Put(ctx, operator, vault.Record{ProjectID: project.ID, Environment: "alpha", Key: "LIFECYCLE_RECOVERY", Value: "synthetic-lifecycle-old-value"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	record.Value = "synthetic-lifecycle-current-value"
	if record, err = v.Put(ctx, operator, record, record.Revision); err != nil {
		t.Fatal(err)
	}
	due := time.Now().UTC().Add(2 * 24 * time.Hour).Truncate(time.Microsecond)
	life, err := v.UpdateLifecycle(ctx, operator, project.ID, record.ID, vault.LifecycleChange{ResponsibleUserID: &owner.ID, DeclaredExpiresAt: &due, ExpectedRevision: 0, Provenance: "synthetic source declaration"})
	if err != nil {
		t.Fatal(err)
	}
	if life, err = v.UpdateLifecycle(ctx, operator, project.ID, record.ID, vault.LifecycleChange{ResponsibleUserID: &owner.ID, DeclaredExpiresAt: &due, ExpectedRevision: life.Revision, Provenance: "synthetic source revision two"}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Rotate(ctx, operator, project.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := v.GenerateReminders(ctx, time.Now().UTC()); err != nil || n != 2 {
		t.Fatal("source reminder fixture", n, err)
	}
	// Populate real source authority that the encrypted vault must never restore.
	if _, err := service.NewOrganizationService(repository.NewOrganizationRepository(source, dialect), audit).CreateWorkspace("Source authority only", owner.ID, "", uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(`INSERT INTO api_keys(id,name,hashed_key,user_id,project_id,scopes) VALUES($1,'source-only',$2,$3,$4,ARRAY['read'])`, uuid.New(), uuid.NewString(), owner.ID, project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(`INSERT INTO session_tokens(id,user_id,token_hash,expires_at) VALUES($1,$2,$3,NOW()+INTERVAL '1 hour')`, uuid.New(), owner.ID, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	if err := repository.NewPlatformAdminRepository(source, dialect, audit).SetGrant(ctx, owner.ID, true, "synthetic operator", "source-only authority"); err != nil {
		t.Fatal(err)
	}
	bundle, err := v.Backup(ctx, operator, project.ID)
	if err != nil || bundle.Format != "keepsave.encrypted-vault.v2" {
		t.Fatal("v2 source backup", bundle.Format, err)
	}
	before, err := vault.VerifyBackup(bundle, keys)
	if err != nil || before.Keys != 2 || before.Revisions != 3 || before.LifecycleRecords != 1 {
		t.Fatal("source retained dependencies", before, err)
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("synthetic-lifecycle-")) {
		t.Fatal("external bundle contains plaintext")
	}
	path := filepath.Join(t.TempDir(), "external-v2-lifecycle.json")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	recoverCLI := func(dsn, mapping string) ([]byte, error) {
		t.Helper()
		childCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		args := []string{"-test.run=^TestPlatformRecoveryLifecycleCLIHelper$", "--", "-action=recover-isolated", "-bundle=" + path, "-target-name=Synthetic isolated custodian", "-confirm-empty-isolated-database"}
		if mapping != "" {
			args = append(args, "-map-lifecycle-owners-to-isolated-custodian="+mapping)
		}
		cmd := exec.CommandContext(childCtx, executable, args...)
		cmd.Env = []string{"DATABASE_URL=" + dsn, "MASTER_KEY=" + base64.StdEncoding.EncodeToString(material), "JWT_SECRET=synthetic-recovery-command-signing-key", "KEEPSAVE_ENV=development", "KEEPSAVE_KEY_PROVIDER=env", "KEEPSAVE_RECOVERY_LIFECYCLE_HELPER=1", "GOMAXPROCS=2"}
		// Migration diagnostics are on stderr; the CLI's stdout is its JSON
		// contract. Keep diagnostics for failed subprocesses without mixing
		// successful migration logs into the verification response.
		output, err := cmd.Output()
		if failure, ok := err.(*exec.ExitError); ok {
			output = append(output, failure.Stderr...)
		}
		return output, err
	}
	for _, missing := range []string{"", uuid.NewString()} {
		target, _, dsn := fresh("refused")
		if output, err := recoverCLI(dsn, missing); err == nil {
			t.Fatal("CLI accepted absent/wrong lifecycle owner mapping", string(output))
		}
		if count(target, `SELECT COUNT(*) FROM vault_entries`)+count(target, `SELECT COUNT(*) FROM vault_keys`)+count(target, `SELECT COUNT(*) FROM vault_lifecycle`)+count(target, `SELECT COUNT(*) FROM vault_revisions`) != 0 {
			t.Fatal("refused recovery committed vault data")
		}
		if count(target, `SELECT COUNT(*) FROM users WHERE id=$1`, owner.ID) != 0 {
			t.Fatal("refusal imported source user")
		}
	}
	target, targetDialect, dsn := fresh("mapped")
	output, err := recoverCLI(dsn, owner.ID.String())
	if err != nil {
		t.Fatal("explicit mapped CLI recovery", err, string(output))
	}
	var result vault.Verification
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal("CLI verification response", err)
	}
	if result.ProjectID == project.ID || result.Keys != before.Keys || result.Revisions != before.Revisions || result.LifecycleRecords != 1 {
		t.Fatal("isolated recovery dependencies", result)
	}
	var custodian uuid.UUID
	var password string
	if err := target.QueryRow(`SELECT id,password_hash FROM users`).Scan(&custodian, &password); err != nil || password != "!" || custodian == owner.ID {
		t.Fatal("custodian identity", err)
	}
	if count(target, `SELECT COUNT(*) FROM users`) != 1 || count(target, `SELECT COUNT(*) FROM projects WHERE id=$1 AND owner_id=$2 AND organization_id IS NULL`, result.ProjectID, custodian) != 1 {
		t.Fatal("isolated ownership imported source authority")
	}
	targetAudit := repository.NewAuditRepository(target, targetDialect)
	targetAudit.SetChainKey(keys.DeriveAuditChainKey())
	targetVault := vault.New(target, keys, authorize(target, targetDialect), targetAudit)
	currentOperator := policy.Principal{Kind: policy.Human, SubjectID: custodian, ActorID: custodian}
	recovered, err := targetVault.Lifecycle(ctx, currentOperator, result.ProjectID, record.ID)
	if err != nil || recovered.ResponsibleUserID == nil || *recovered.ResponsibleUserID != custodian || recovered.Revision != life.Revision || recovered.DeclaredExpiresAt == nil || !recovered.DeclaredExpiresAt.Equal(due) || recovered.Provenance != life.Provenance {
		t.Fatal("explicit lifecycle remap", recovered, err)
	}
	for _, version := range []struct {
		n     int64
		value string
	}{{1, "synthetic-lifecycle-old-value"}, {2, "synthetic-lifecycle-current-value"}, {0, "synthetic-lifecycle-current-value"}} {
		r, err := targetVault.Read(ctx, currentOperator, result.ProjectID, record.ID, version.n)
		if err != nil || r.Value != version.value {
			t.Fatal("retained revision/key read", version.n, err)
		}
	}
	if _, err := targetVault.Read(ctx, operator, result.ProjectID, record.ID, 0); err == nil {
		t.Fatal("source user authority survived recovery")
	}
	keyIDs := func(db *sql.DB, project uuid.UUID) []uuid.UUID {
		t.Helper()
		rows, err := db.Query(`SELECT id FROM vault_keys WHERE project_id=$1 ORDER BY id`, project)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		ids := []uuid.UUID{}
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return ids
	}
	if !reflect.DeepEqual(keyIDs(source, project.ID), keyIDs(target, result.ProjectID)) || count(target, `SELECT COUNT(DISTINCT key_id) FROM vault_revisions WHERE project_id=$1`, result.ProjectID) != 2 {
		t.Fatal("retained key continuity lost")
	}
	for _, table := range []string{"organizations", "organization_members", "member_authority_state", "session_tokens", "api_keys", "secret_leases", "platform_admin_grants", "mcp_oauth_consents", "mcp_oauth_families", "mcp_oauth_tokens", "identity_verified_contacts", "identity_proofs", "identity_invitations", "tool_grants", "tool_connections", "tool_workloads", "tool_runs", "promotion_requests", "vault_notifications"} {
		if count(target, `SELECT COUNT(*) FROM `+table) != 0 {
			t.Fatal("source authority or reminders imported", table)
		}
	}
	if broken, err := targetAudit.VerifyChain(); err != nil || broken != nil {
		t.Fatal("recovery audit chain", err)
	}
	again, err := targetVault.Backup(ctx, currentOperator, result.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	verifiedAgain, err := vault.VerifyBackup(again, keys)
	if err != nil || verifiedAgain.Keys != 2 || verifiedAgain.Revisions != 3 || verifiedAgain.LifecycleRecords != 1 {
		t.Fatal("recovered v2 bundle verification", verifiedAgain, err)
	}
}
