package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

func vaultAdapters(f *platformFixture, v *vault.Service, keys *crypto.Service) (*service.SecretService, *service.EnvFileService, *service.TemplateService, *service.KeyRotationService) {
	sr := repository.NewSecretRepository(f.db, f.d)
	pr := repository.NewProjectRepository(f.db, f.d)
	er := repository.NewEnvironmentRepository(f.db, f.d)
	ar := repository.NewAuditRepository(f.db, f.d)
	ss := service.NewSecretService(sr, pr, er, ar, keys)
	ss.EnableVault(v)
	ef := service.NewEnvFileService(sr, pr, er, ar, keys)
	ef.EnableVault(v)
	ts := service.NewTemplateService(repository.NewTemplateRepository(f.db, f.d), sr, pr, er, ar, keys)
	ts.EnableVault(v)
	kr := service.NewKeyRotationService(pr, sr, er, ar, keys)
	kr.EnableVault(v)
	return ss, ef, ts, kr
}
func TestPlatformVaultIntegratedMutationsAndScopes(t *testing.T) {
	f := newPlatformFixture(t)
	v, keys, p := platformVault(t, f)
	ctx := context.Background()
	if err := v.Enroll(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	ss, ef, ts, kr := vaultAdapters(f, v, keys)
	created, err := ss.CreateAuthorized(ctx, p, f.project, "alpha", "PUBLIC", "first", "")
	if err != nil {
		t.Fatal(err)
	}
	if created.Revision != 1 || created.EnvironmentID == uuid.Nil {
		t.Fatal("missing revision/environment metadata")
	}
	expected := created.Revision
	updated, err := ss.UpdateAuthorized(ctx, p, f.project, created.ID, "second", "", &expected)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ss.UpdateAuthorized(ctx, p, f.project, created.ID, "stale", "", &expected); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("stale update", err)
	}
	result, err := ef.ImportAuthorized(ctx, p, f.project, "alpha", "PUBLIC=imported\nPRIVATE_TOKEN=private-canary\nALIAS=${PRIVATE_TOKEN}\nEMPTY=\nMULTILINE=\"a\\nb\"\n", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 || len(result.Created) != 4 {
		t.Fatal("import result", result)
	}
	tmpl, err := ts.Create("Integration", "", "synthetic", models.JSONMap{"keys": []any{map[string]any{"key": "PUBLIC", "default_value": "templated"}, map[string]any{"key": "TEMPLATE_NEW", "default_value": "new"}}}, f.owner, nil, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ts.ApplyTemplateAuthorized(ctx, p, tmpl.ID, f.project, "alpha"); err != nil {
		t.Fatal(err)
	}
	history, err := v.History(ctx, p, f.project, created.ID)
	if err != nil || len(history) != 4 {
		t.Fatal("missing mutation history", history, err)
	}
	if history[0].Operation != "template" || history[1].Operation != "import" {
		t.Fatal("mutation provenance", history)
	}
	before, err := ef.ExportAuthorized(ctx, p, f.project, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(before, `MULTILINE="a\nb"`) || !strings.Contains(before, "EMPTY=\n") {
		t.Fatal("export roundtrip escaping")
	}
	rotation, err := kr.RotateProjectKeyAuthorized(ctx, p, f.project, "")
	if err != nil || rotation.SecretsRotated < 7 {
		t.Fatal("rotation", rotation, err)
	}
	historical, err := v.Read(ctx, p, f.project, created.ID, updated.Revision)
	if err != nil || historical.Value != "second" {
		t.Fatal("historical key continuity", err)
	}
	_, writeID := f.key([]string{"write:PUBLIC"}, nil, nil)
	writeOnly := policy.Principal{Kind: policy.APIKey, SubjectID: writeID, ActorID: f.owner}
	if _, err = ss.GetMetadataAuthorized(ctx, writeOnly, f.project, created.ID, policy.WriteSecret); err != nil {
		t.Fatal("write-only metadata denied", err)
	}
	if _, err = ss.GetByIDAuthorized(ctx, writeOnly, f.project, created.ID); !errors.Is(err, vault.ErrDenied) {
		t.Fatal("write-only value access", err)
	}
	_, readID := f.key([]string{"read:PUBLIC", "read:ALIAS"}, nil, nil)
	scoped := policy.Principal{Kind: policy.APIKey, SubjectID: readID, ActorID: f.owner}
	allowed, missing, err := ss.BatchAuthorized(ctx, scoped, f.project, "alpha", []string{"PUBLIC", "PRIVATE_TOKEN", "MISSING", "PUBLIC"})
	if err != nil || len(allowed) != 1 || len(missing) != 2 {
		t.Fatal("batch scope/missing parity", len(allowed), missing, err)
	}
	if missing[0] != "PRIVATE_TOKEN" || missing[1] != "MISSING" {
		t.Fatal("batch ordering", missing)
	}
	if _, err = ss.ListResolvedAuthorized(ctx, scoped, f.project, "alpha"); !errors.Is(err, vault.ErrDenied) {
		t.Fatal("reference escaped key scope", err)
	}
	exported, err := ef.ExportAuthorized(ctx, scoped, f.project, "alpha")
	if err != nil || strings.Contains(exported, "private-canary") {
		t.Fatal("scoped export leaked", err)
	}
	_, deleteID := f.key([]string{"delete:TEMPLATE_NEW"}, nil, nil)
	deleteOnly := policy.Principal{Kind: policy.APIKey, SubjectID: deleteID, ActorID: f.owner}
	records, err := ss.ListAuthorized(ctx, p, f.project, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	var deleteSecret uuid.UUID
	for _, r := range records {
		if r.Key == "TEMPLATE_NEW" {
			deleteSecret = r.ID
		}
	}
	if err = ss.DeleteAuthorized(ctx, deleteOnly, f.project, deleteSecret, "", nil); err != nil {
		t.Fatal("legacy delete-only scope", err)
	}
	if _, err = v.List(ctx, p, f.project, "absent"); !errors.Is(err, vault.ErrNotFound) {
		t.Fatal("unknown environment appeared successful", err)
	}
}

func TestPlatformVaultIntegratedImportFailureAtomic(t *testing.T) {
	f := newPlatformFixture(t)
	v, keys, p := platformVault(t, f)
	ctx := context.Background()
	if err := v.Enroll(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	_, ef, _, _ := vaultAdapters(f, v, keys)
	if _, err := f.db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT fail_later_import CHECK(COALESCE(details->>'key','')<>'Z_FAILURE') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err := ef.ImportAuthorized(ctx, p, f.project, "alpha", "A_PARTIAL=must-rollback\nZ_FAILURE=must-rollback\n", true, ""); err == nil {
		t.Fatal("import succeeded through audit failure")
	}
	for _, table := range []string{"secrets", "vault_entries"} {
		var count int
		column := "key"
		if table == "vault_entries" {
			column = "secret_key"
		}
		if err := f.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s IN ('A_PARTIAL','Z_FAILURE')", table, column)).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial imported data", table, count, err)
		}
	}
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE details->>'key'='A_PARTIAL'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("rolled back import left success evidence", count, err)
	}
	if _, err := ef.ImportAuthorized(ctx, p, f.project, "alpha", "VALID=x\nthis is malformed\n", true, ""); !errors.Is(err, vault.ErrInvalid) {
		t.Fatal("malformed input partly imported", err)
	}
	if _, err := ef.ImportAuthorized(ctx, p, f.project, "alpha", "DUP=a\nDUP=b\n", true, ""); !errors.Is(err, vault.ErrInvalid) {
		t.Fatal("duplicate keys silently lost", err)
	}
}

func TestPlatformVaultJournalGuard(t *testing.T) {
	f := newPlatformFixture(t)
	v, _, p := platformVault(t, f)
	ctx := context.Background()
	if err := v.Enroll(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	if err := f.db.QueryRow(`SELECT id FROM secrets WHERE project_id=$1`, f.project).Scan(&id); err != nil {
		t.Fatal(err)
	}
	before, err := v.Read(ctx, p, f.project, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE secrets SET encrypted_value='forged'::bytea WHERE id=$1`, id); err == nil {
		t.Fatal("unjournaled legacy writer accepted")
	}
	if _, err = f.db.Exec(`UPDATE vault_revisions SET operation='forged' WHERE secret_id=$1`, id); err == nil {
		t.Fatal("immutable history rewritten")
	}
	if _, err = f.db.Exec(`DELETE FROM vault_revisions WHERE secret_id=$1`, id); err == nil {
		t.Fatal("immutable history deleted")
	}
	if _, err = f.db.Exec(`UPDATE vault_keys SET encrypted_key='forged'::bytea WHERE project_id=$1`, f.project); err == nil {
		t.Fatal("retained key material rewritten")
	}
	if _, err = f.db.Exec(`DELETE FROM vault_keys WHERE project_id=$1`, f.project); err == nil {
		t.Fatal("retained key material deleted")
	}
	if _, err = f.db.Exec(`UPDATE vault_entries SET revision=revision+1 WHERE secret_id=$1`, id); err == nil {
		t.Fatal("current journal pointer changed without a revision")
	}
	after, err := v.Read(ctx, p, f.project, id, 0)
	if err != nil || after.Value != before.Value || after.Revision != before.Revision {
		t.Fatal("failed bypass changed current value", err)
	}
	foreign := policy.Principal{Kind: policy.Human, SubjectID: f.other, ActorID: f.other}
	if _, err = v.List(ctx, foreign, f.foreign, "alpha"); !errors.Is(err, vault.ErrNotEnrolled) {
		t.Fatal("unenrolled project returned empty success", err)
	}
}

func TestPlatformVaultIndependentRecoveryAndSelectedRestore(t *testing.T) {
	f := newPlatformFixture(t)
	v, _, p := platformVault(t, f)
	ctx := context.Background()
	var env uuid.UUID
	var cipher, nonce []byte
	if err := f.db.QueryRow(`SELECT environment_id,encrypted_value,value_nonce FROM secrets WHERE project_id=$1`, f.project).Scan(&env, &cipher, &nonce); err != nil {
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
	if err = v.Enroll(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	first, err := v.Put(ctx, p, vault.Record{ProjectID: f.project, Environment: "alpha", Key: "RECOVERY", Value: "old-canary"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	first.Value = "new-canary"
	second, err := v.Put(ctx, p, first, first.Revision)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := v.Put(ctx, p, vault.Record{ProjectID: f.project, Environment: "alpha", Key: "DELETED", Value: "removed-canary"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = v.Delete(ctx, p, f.project, removed.ID, removed.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = v.Rotate(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	bundle, err := v.Backup(ctx, p, f.project)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "canary") {
		t.Fatal("backup contains plaintext")
	}
	path := filepath.Join(t.TempDir(), "external-backup.json")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	externalBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var external vault.Bundle
	if err = json.Unmarshal(externalBytes, &external); err != nil {
		t.Fatal(err)
	}
	recoveryKeys, err := crypto.NewService(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	// A separate migrated schema, account and wrapping key prove independence
	// from the source DB and prevent any authority records moving with the vault.
	g := newPlatformFixture(t)
	targetMaterial := make([]byte, 32)
	targetMaterial[0] = 7
	targetKeys, err := crypto.NewService(targetMaterial)
	if err != nil {
		t.Fatal(err)
	}
	targetAudit := repository.NewAuditRepository(g.db, g.d)
	targetAudit.SetChainKey(targetKeys.DeriveAuditChainKey())
	targetAuthorize := func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: g.d}}).Authorize(ctx, p, a, r)
	}
	targetVault := vault.New(g.db, targetKeys, targetAuthorize, targetAudit)
	targetPrincipal := policy.Principal{Kind: policy.Human, SubjectID: g.other, ActorID: g.other}
	verified, err := targetVault.RecoverToEmptyProject(ctx, targetPrincipal, g.foreign, external, recoveryKeys)
	if err != nil || verified.Keys != 2 || verified.Snapshots != 1 {
		t.Fatal("independent database recovery", verified, err)
	}
	old, err := targetVault.Read(ctx, targetPrincipal, g.foreign, first.ID, 1)
	if err != nil || old.Value != "old-canary" {
		t.Fatal("recovered history", err)
	}
	current, err := targetVault.Read(ctx, targetPrincipal, g.foreign, first.ID, 0)
	if err != nil || current.Value != "new-canary" {
		t.Fatal("recovered current value", err)
	}
	var grantCount, promotionCount int
	if err = g.db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE project_id=$1`, g.foreign).Scan(&grantCount); err != nil {
		t.Fatal(err)
	}
	if err = g.db.QueryRow(`SELECT COUNT(*) FROM promotion_requests WHERE project_id=$1`, g.foreign).Scan(&promotionCount); err != nil {
		t.Fatal(err)
	}
	if grantCount != 0 || promotionCount != 0 {
		t.Fatal("recovery resurrected authority")
	}
	if _, err = targetVault.RecoverToEmptyProject(ctx, targetPrincipal, g.foreign, external, recoveryKeys); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("nonempty recovery target accepted", err)
	}
	recoveredBundle, err := targetVault.Backup(ctx, targetPrincipal, g.foreign)
	if err != nil {
		t.Fatal(err)
	}
	again, err := vault.VerifyBackup(recoveredBundle, targetKeys)
	if err != nil || again.Snapshots != 1 || again.Revisions != verified.Revisions {
		t.Fatal("recovered retained dependencies", again, err)
	}
	preview, err := v.PreviewRestore(ctx, p, f.project, external)
	if err != nil || len(preview.Records) != 3 {
		t.Fatal("metadata preview", preview, err)
	}
	plainPreview, _ := json.Marshal(preview)
	if strings.Contains(string(plainPreview), "canary") {
		t.Fatal("preview contains plaintext")
	}
	live, err := v.Read(ctx, p, f.project, first.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	selected := []vault.RestoreSelection{{SecretID: first.ID, BackupRevision: 1, ExpectedRevision: live.Revision}}
	restored, err := v.RestoreSelected(ctx, p, f.project, external, selected)
	if err != nil || len(restored) != 1 || restored[0].Revision <= second.Revision || restored[0].Value != "" {
		t.Fatal("selected restore", restored, err)
	}
	if _, err = v.RestoreSelected(ctx, p, f.project, external, selected); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("stale selected restore", err)
	}
	if _, err = v.RestoreSelected(ctx, p, f.project, external, []vault.RestoreSelection{{SecretID: removed.ID, BackupRevision: 1, ExpectedRevision: 2}}); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("selected restore resurrected deleted record", err)
	}
}

func TestPlatformVaultVersionHandlersMetadataAndRestore(t *testing.T) {
	f := newPlatformFixture(t)
	v, keys, p := platformVault(t, f)
	ctx := context.Background()
	if err := v.Enroll(ctx, p, f.project); err != nil {
		t.Fatal(err)
	}
	record, err := v.Put(ctx, p, vault.Record{ProjectID: f.project, Environment: "alpha", Key: "VERSION", Value: "explicit-history-canary"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewVersionHandler(repository.NewSecretVersionRepository(f.db, f.d), repository.NewSecretRepository(f.db, f.d), repository.NewProjectRepository(f.db, f.d), keys)
	handler.EnableVault(v)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", f.owner); c.Next() })
	base := "/projects/:id/secrets/:secretId/versions"
	r.GET(base, handler.ListVersions)
	r.GET(base+"/:version", handler.GetVersion)
	r.POST(base+"/:version/restore", handler.RestoreVersion)
	prefix := "/projects/" + f.project.String() + "/secrets/" + record.ID.String() + "/versions"
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	list := call("GET", prefix, "")
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), "explicit-history-canary") {
		t.Fatal("history metadata leaked", list.Code, list.Body.String())
	}
	explicit := call("GET", prefix+"/1", "")
	if explicit.Code != http.StatusOK || !strings.Contains(explicit.Body.String(), "explicit-history-canary") {
		t.Fatal("explicit history read", explicit.Code)
	}
	missing := call("POST", prefix+"/1/restore", `{}`)
	if missing.Code != http.StatusBadRequest {
		t.Fatal("restore missing revision accepted", missing.Code)
	}
	restore := call("POST", prefix+"/1/restore", `{"expected_current_revision":1}`)
	if restore.Code != http.StatusOK || strings.Contains(restore.Body.String(), "explicit-history-canary") {
		t.Fatal("restore wire response", restore.Code, restore.Body.String())
	}
	stale := call("POST", prefix+"/1/restore", `{"expected_current_revision":1}`)
	if stale.Code != http.StatusConflict {
		t.Fatal("stale restore handler", stale.Code)
	}
}

// A deliberately corrupt denied ciphertext proves the compatibility adapter
// filters before decryption rather than decrypting all values and trimming later.
func TestPlatformLegacyVaultScopedReadBeforeDecryption(t *testing.T) {
	f := newPlatformFixture(t)
	ctx := context.Background()
	keys, err := crypto.NewService(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	sr := repository.NewSecretRepository(f.db, f.d)
	pr := repository.NewProjectRepository(f.db, f.d)
	er := repository.NewEnvironmentRepository(f.db, f.d)
	ar := repository.NewAuditRepository(f.db, f.d)
	ss := service.NewSecretService(sr, pr, er, ar, keys)
	ef := service.NewEnvFileService(sr, pr, er, ar, keys)
	private, err := ss.Create(f.project, "alpha", "PRIVATE_TOKEN", "must-never-decrypt", f.owner, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ss.Create(f.project, "alpha", "PUBLIC", "allowed", f.owner, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = ss.Create(f.project, "alpha", "ALIAS", "${PRIVATE_TOKEN}", f.owner, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(repository.Q(f.d, `UPDATE secrets SET encrypted_value=$1 WHERE id=$2`), []byte("deliberately-invalid-ciphertext"), private.ID); err != nil {
		t.Fatal(err)
	}
	raw, id := f.key([]string{"read:PUBLIC", "read:ALIAS"}, nil, nil)
	p := policy.Principal{Kind: policy.APIKey, SubjectID: id, ActorID: f.owner}
	records, err := ss.ListAuthorized(ctx, p, f.project, "alpha")
	if err != nil || len(records) != 2 {
		t.Fatal("denied ciphertext was decrypted", len(records), err)
	}
	if _, err = ss.GetByIDAuthorized(ctx, p, f.project, private.ID); !errors.Is(err, vault.ErrDenied) {
		t.Fatal("denied direct read reached decryption", err)
	}
	selected, missing, err := ss.BatchAuthorized(ctx, p, f.project, "alpha", []string{"PUBLIC", "PRIVATE_TOKEN", "ABSENT"})
	if err != nil || len(selected) != 1 || len(missing) != 2 {
		t.Fatal("legacy bounded batch", len(selected), missing, err)
	}
	if _, err = ss.ListResolvedAuthorized(ctx, p, f.project, "alpha"); !errors.Is(err, vault.ErrDenied) {
		t.Fatal("legacy reference expansion reached denied value", err)
	}
	exported, err := ef.ExportAuthorized(ctx, p, f.project, "alpha")
	if err != nil || strings.Contains(exported, "must-never-decrypt") {
		t.Fatal("legacy scoped export", err)
	}
	w := f.call("POST", "/api/v1/projects/"+f.project.String()+"/secrets/batch", `{"environment":"alpha","keys":["PUBLIC","PRIVATE_TOKEN","ABSENT"]}`, raw, uuid.Nil)
	if w.Code != http.StatusOK {
		t.Fatal("legacy SDK batch route", w.Code)
	}
	privatePath := "/api/v1/projects/" + f.project.String() + "/secrets/" + private.ID.String()
	if denied := f.call("GET", privatePath, "", raw, uuid.Nil); denied.Code != http.StatusNotFound {
		t.Fatal("denied ciphertext route reached decryption", denied.Code)
	}
	fullRead, _ := f.key([]string{"read:PRIVATE_TOKEN"}, nil, nil)
	failedRead := f.call("GET", privatePath, "", fullRead, uuid.Nil)
	if failedRead.Code != http.StatusInternalServerError || strings.Contains(failedRead.Body.String(), "deliberately-invalid-ciphertext") || strings.Contains(failedRead.Body.String(), "must-never-decrypt") {
		t.Fatal("authorized corruption was hidden or leaked", failedRead.Code)
	}
}
