package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auditview"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

func teamPrincipal(t *testing.T, f *coreFixture, user uuid.UUID) policy.Principal {
	t.Helper()
	claims, e := f.jwt.ValidateToken(f.tokens[user])
	if e != nil {
		t.Fatal(e)
	}
	sid, e := uuid.Parse(claims.SessionID)
	if e != nil {
		t.Fatal(e)
	}
	return policy.Principal{Kind: policy.Human, SubjectID: user, ActorID: user, SessionID: sid, ExpiresAt: claims.ExpiresAt.Time}
}
func TestPlatformTeamLifecycleRemindersAndRecovery(t *testing.T) {
	f := newCoreFixture(t)
	ctx := context.Background()
	p := teamPrincipal(t, f, f.owner)
	r := f.put("alpha", "RENEW_ME", "credential-canary")
	due := time.Now().UTC().Add(2 * 24 * time.Hour).Truncate(time.Microsecond)
	change := vault.LifecycleChange{ResponsibleUserID: &f.owner, DeclaredExpiresAt: &due, Provenance: "operator declaration", ExpectedRevision: 0}
	life, e := f.v.UpdateLifecycle(ctx, p, f.project, r.ID, change)
	if e != nil || life.Revision != 1 {
		t.Fatal(life, e)
	}
	history, e := f.v.History(ctx, p, f.project, r.ID)
	if e != nil || len(history) != 1 {
		t.Fatal("lifecycle changed value revisions", history, e)
	}
	if _, e = f.v.UpdateLifecycle(ctx, p, f.project, r.ID, change); !errors.Is(e, vault.ErrConflict) {
		t.Fatal("stale metadata accepted", e)
	}
	change.ExpectedRevision = 1
	change.ResponsibleUserID = &f.other
	if _, e = f.v.UpdateLifecycle(ctx, p, f.project, r.ID, change); !errors.Is(e, vault.ErrDenied) {
		t.Fatal("foreign owner accepted", e)
	}
	if n, e := f.v.GenerateReminders(ctx, time.Now().UTC()); e != nil || n != 2 {
		t.Fatal("expected 30/7 day reminders", n, e)
	}
	if n, e := f.v.GenerateReminders(ctx, time.Now().UTC()); e != nil || n != 0 {
		t.Fatal("duplicate reminders", n, e)
	}
	notes, e := f.v.Notifications(ctx, p)
	if e != nil || len(notes) != 2 {
		t.Fatal(notes, e)
	}
	other := teamPrincipal(t, f, f.other)
	notes, e = f.v.Notifications(ctx, other)
	if e != nil || len(notes) != 0 {
		t.Fatal("foreign reminders disclosed", notes, e)
	}
	bundle, e := f.v.Backup(ctx, p, f.project)
	if e != nil || bundle.Format != vault.BackupFormat {
		t.Fatal(bundle.Format, e)
	}
	_, keys, _ := platformVault(t, f.platformFixture)
	verified, e := vault.VerifyBackup(bundle, keys)
	if e != nil || verified.LifecycleRecords != 1 {
		t.Fatal("bundle lifecycle missing", verified, e)
	}
	change.ResponsibleUserID = &f.owner
	change.DeclaredExpiresAt = nil
	change.ExpectedRevision = 1
	if _, e = f.v.UpdateLifecycle(ctx, p, f.project, r.ID, change); e != nil {
		t.Fatal(e)
	}
	preview, e := f.v.PreviewRestore(ctx, p, f.project, bundle)
	if e != nil {
		t.Fatal(e)
	}
	var metadata *vault.RestoreDiff
	for i := range preview.Records {
		if preview.Records[i].SecretID == r.ID {
			metadata = &preview.Records[i]
			break
		}
	}
	if metadata == nil || metadata.BackupLifecycle == nil || metadata.BackupLifecycle.Revision != 1 || metadata.CurrentMetadataRevision != 2 || metadata.BackupLifecycle.ResponsibleUserID == nil || *metadata.BackupLifecycle.ResponsibleUserID != f.owner {
		t.Fatal("lifecycle metadata preview incomplete")
	}
	selection := []vault.RestoreSelection{{SecretID: r.ID, BackupRevision: 1, ExpectedRevision: 1, RestoreLifecycle: true, ExpectedMetadataRevision: 2}}
	if _, e = f.v.RestoreSelected(ctx, p, f.project, bundle, selection); !errors.Is(e, vault.ErrInvalid) {
		t.Fatal("missing explicit owner mapping", e)
	}
	selection[0].MappedResponsibleUserID = &f.owner
	if _, e = f.v.RestoreSelected(ctx, p, f.project, bundle, selection); e != nil {
		t.Fatal(e)
	}
	life, e = f.v.Lifecycle(ctx, p, f.project, r.ID)
	if e != nil || life.Revision != 3 || life.DeclaredExpiresAt == nil || !life.DeclaredExpiresAt.Equal(due) {
		t.Fatal("lifecycle not restored", life, e)
	}
	current, e := f.v.Read(ctx, p, f.project, r.ID, 0)
	if e != nil || current.Value != "credential-canary" || current.Revision != 2 {
		t.Fatal(current, e)
	}
	// Same bundle verifies externally, but a selected restore never imports roles.
	var grants int
	if e = f.db.QueryRow(`SELECT COUNT(*) FROM platform_admin_grants`).Scan(&grants); e != nil || grants != 0 {
		t.Fatal("restored authority", grants, e)
	}
}
func TestPlatformAuditSnapshotVisibilityAndExpiry(t *testing.T) {
	f := newCoreFixture(t)
	ctx := context.Background()
	p := teamPrincipal(t, f, f.owner)
	audit := repository.NewAuditRepository(f.db, f.d)
	ref := uuid.New()
	for i := 0; i < 3; i++ {
		if e := audit.Create(&f.owner, &f.project, "test.safe", "alpha", models.JSONMap{"secret_id": ref.String(), "credential": "never-export-this", "upstream_url": "https://credential.invalid"}, ""); e != nil {
			t.Fatal(e)
		}
	}
	s := auditview.New(f.db, func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d, RequireHumanSession: true}}).Authorize(ctx, p, a, r)
	}, audit)
	filter := auditview.Filter{Action: "test.safe", From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Minute), Limit: 2}
	page, e := s.Search(ctx, p, f.project, filter)
	if e != nil || len(page.Entries) != 2 || page.NextCursor == "" {
		t.Fatal(page, e)
	}
	filter.Cursor = page.NextCursor
	next, e := s.Search(ctx, p, f.project, filter)
	if e != nil || len(next.Entries) != 1 || next.Entries[0].ID == page.Entries[0].ID {
		t.Fatal("unstable cursor", next, e)
	}
	bytes, _ := json.Marshal(page)
	if strings.Contains(string(bytes), "never-export-this") || strings.Contains(string(bytes), "credential.invalid") {
		t.Fatal("unsafe audit projection")
	}
	other := teamPrincipal(t, f, f.other)
	if _, e = s.Search(ctx, other, f.project, filter); !errors.Is(e, auditview.ErrDenied) {
		t.Fatal("foreign audit", e)
	}
	exp, e := s.CreateExport(ctx, p, f.project, filter)
	if e != nil || exp.Rows != 3 || exp.Status != "pending" {
		t.Fatal(exp, e)
	}
	if e = audit.Create(&f.owner, &f.project, "test.safe", "alpha", models.JSONMap{}, ""); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Download(ctx, p, f.project, exp.ID); !errors.Is(e, auditview.ErrDenied) {
		t.Fatal("unpublished snapshot available", e)
	}
	if e = s.Publish(ctx, exp.ID); e != nil {
		t.Fatal("publication failed", e)
	}
	raw, e := s.Download(ctx, p, f.project, exp.ID)
	if e != nil || strings.Contains(string(raw), "never-export-this") {
		t.Fatal("unsafe export", e)
	}
	var result struct{ Entries []auditview.Entry }
	if e = json.Unmarshal(raw, &result); e != nil || len(result.Entries) != 3 {
		t.Fatal("export is not materialized snapshot", len(result.Entries), e)
	}
	if _, e = s.Download(ctx, other, f.project, exp.ID); !errors.Is(e, auditview.ErrDenied) {
		t.Fatal("stolen export handle", e)
	}
	if _, e = f.db.Exec(`UPDATE audit_exports SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, exp.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Download(ctx, p, f.project, exp.ID); !errors.Is(e, auditview.ErrDenied) {
		t.Fatal("expired export", e)
	}
}
func TestPlatformTeamMetadataAndSnapshotRollback(t *testing.T) {
	f := newCoreFixture(t)
	ctx := context.Background()
	p := teamPrincipal(t, f, f.owner)
	r := f.put("alpha", "ROLLBACK_META", "value")
	if _, e := f.db.Exec(`CREATE FUNCTION reject_team_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action IN ('secret.lifecycle_updated','audit.export_created') THEN RAISE EXCEPTION 'synthetic audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_team_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION reject_team_audit()`); e != nil {
		t.Fatal(e)
	}
	if _, e := f.v.UpdateLifecycle(ctx, p, f.project, r.ID, vault.LifecycleChange{ResponsibleUserID: &f.owner}); e == nil {
		t.Fatal("audit failure ignored")
	}
	l, e := f.v.Lifecycle(ctx, p, f.project, r.ID)
	if e != nil || l.Known {
		t.Fatal("partial metadata committed", l, e)
	}
	views := auditview.New(f.db, func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d, RequireHumanSession: true}}).Authorize(ctx, p, a, r)
	}, repository.NewAuditRepository(f.db, f.d))
	if _, err := views.CreateExport(ctx, p, f.project, auditview.Filter{From: time.Now().Add(-time.Hour), To: time.Now(), Limit: 100}); err == nil {
		t.Fatal("export ignored required audit failure")
	}
	var exports int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM audit_exports`).Scan(&exports); err != nil || exports != 0 {
		t.Fatal("partial export committed", exports, err)
	}
	var count int
	if e = f.db.QueryRow(`SELECT COUNT(*) FROM vault_lifecycle WHERE secret_id=$1`, r.ID).Scan(&count); e != nil || count != 0 {
		t.Fatal("partial update", count, e)
	}
}

func TestPlatformTeamRouterContractsAndSafeMetadata(t *testing.T) {
	f := newCoreFixture(t)
	r := f.put("alpha", "LIFECYCLE_ONLY", "must-not-be-in-metadata")
	base := "/api/v1/projects/" + f.project.String()
	w := f.request("GET", base+"/secret-lifecycle", "", f.owner, "")
	assertCoreResponse(t, "/projects/{id}/secret-lifecycle", "GET", w, 200)
	if strings.Contains(w.Body.String(), "must-not-be-in-metadata") {
		t.Fatal("lifecycle decrypted a credential")
	}
	change := `{"responsible_user_id":"` + f.owner.String() + `","declared_expires_at":"2026-10-03T00:00:00Z","renewal_at":null,"provenance":"declared","expected_revision":0}`
	path := base + "/secrets/" + r.ID.String() + "/lifecycle"
	assertCoreResponse(t, "/projects/{id}/secrets/{secretId}/lifecycle", "PUT", f.request("PUT", path, change, f.owner, ""), 200)
	assertCoreResponse(t, "/projects/{id}/secrets/{secretId}/lifecycle", "GET", f.request("GET", path, "", f.owner, ""), 200)
	if w = f.request("GET", path, "", f.other, ""); w.Code != 403 {
		t.Fatal("foreign lifecycle disclosed", w.Code)
	}
	assertCoreResponse(t, "/account/notifications", "GET", f.request("GET", "/api/v1/account/notifications", "", f.owner, ""), 200)
	assertCoreResponse(t, "/projects/{id}/audit", "GET", f.request("GET", base+"/audit", "", f.owner, ""), 200)
	ar := repository.NewAuditRepository(f.db, f.d)
	operators := repository.NewPlatformAdminRepository(f.db, f.d, ar)
	if w = f.request("GET", "/api/v1/operator/readiness", "", f.owner, ""); w.Code != 403 {
		t.Fatal("workspace owner obtained operator readiness", w.Code)
	}
	if e := operators.SetGrant(context.Background(), f.owner, true, "synthetic-operator", "read-only diagnostic fixture"); e != nil {
		t.Fatal(e)
	}
	assertCoreResponse(t, "/operator/readiness", "GET", f.request("GET", "/api/v1/operator/readiness", "", f.owner, ""), 200)
	claims, e := f.jwt.ValidateToken(f.tokens[f.owner])
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE session_tokens SET revoked=TRUE WHERE id=$1`, claims.SessionID); e != nil {
		t.Fatal(e)
	}
	if w = f.request("GET", base+"/secret-lifecycle", "", f.owner, ""); w.Code != 401 {
		t.Fatal("revoked human still read metadata", w.Code)
	}
}
