package api

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auditview"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

func publicationFixture(t *testing.T) (*coreFixture, *auditview.Service, auditview.Export, policy.Principal) {
	t.Helper()
	f := newCoreFixture(t)
	p := teamPrincipal(t, f, f.owner)
	audit := repository.NewAuditRepository(f.db, f.d)
	s := auditview.New(f.db, func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d, RequireHumanSession: true}}).Authorize(ctx, p, a, r)
	}, audit)
	export, err := s.CreateExport(context.Background(), p, f.project, auditview.Filter{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Minute), Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return f, s, export, p
}

func publicationAssertFailed(t *testing.T, f *coreFixture, s *auditview.Service, export auditview.Export, p policy.Principal) {
	t.Helper()
	out, err := s.Status(context.Background(), p, f.project, export.ID)
	if err != nil || out.Status != "failed" {
		t.Fatal("terminal export stayed pending", err)
	}
	if raw, err := s.Download(context.Background(), p, f.project, export.ID); !errors.Is(err, auditview.ErrDenied) || raw != nil {
		t.Fatal("failed snapshot delivered", err)
	}
	var size, count int
	if err := f.db.QueryRow(`SELECT octet_length(payload) FROM audit_exports WHERE id=$1`, export.ID).Scan(&size); err != nil || size != 0 {
		t.Fatal("failed snapshot retained", err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='audit.export_failed' AND details->>'export_id'=$1`, export.ID.String()).Scan(&count); err != nil || count != 1 {
		t.Fatal("missing or duplicate terminal audit", err)
	}
}

func TestPlatformAuditPublicationParentRevocationIsTerminal(t *testing.T) {
	f, s, export, old := publicationFixture(t)
	ctx := context.Background()
	audit := repository.NewAuditRepository(f.db, f.d)
	sessions := service.NewSessionService(f.db, f.d, f.jwt, audit)
	user, err := repository.NewUserRepository(f.db, f.d).GetByID(f.owner)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	token, err := sessions.IssueTx(ctx, tx, user, "publication-new-session", "", "")
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	f.tokens[f.owner] = token
	current := teamPrincipal(t, f, f.owner)
	if err := sessions.Revoke(ctx, f.owner, current.SessionID, old.SessionID, ""); err != nil {
		t.Fatal(err)
	}
	if worked, err := s.StepPublication(ctx, uuid.New()); !worked || !errors.Is(err, auditview.ErrDenied) {
		t.Fatal("revoked parent published", err)
	}
	publicationAssertFailed(t, f, s, export, current)
	if _, err := s.Status(ctx, old, f.project, export.ID); !errors.Is(err, auditview.ErrDenied) {
		t.Fatal("old parent can read status", err)
	}
}

func TestPlatformAuditPublicationExhaustionErasesProjection(t *testing.T) {
	f, s, export, p := publicationFixture(t)
	if _, err := f.db.Exec(`CREATE FUNCTION reject_export_ready() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='audit.export_ready' THEN RAISE EXCEPTION 'synthetic publication failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_export_ready BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION reject_export_ready(); UPDATE outbox_jobs SET max_attempts=1 WHERE kind='audit.export.publish'`); err != nil {
		t.Fatal(err)
	}
	if worked, err := s.StepPublication(context.Background(), uuid.New()); !worked || err == nil {
		t.Fatal("publication failure missing", err)
	}
	publicationAssertFailed(t, f, s, export, p)
	var state string
	if err := f.db.QueryRow(`SELECT status FROM outbox_jobs WHERE kind='audit.export.publish'`).Scan(&state); err != nil || state != "failed" {
		t.Fatal("exhausted publication retried", err)
	}
}

func TestPlatformAuditPublicationRetryPreservesPendingSnapshot(t *testing.T) {
	f, s, export, p := publicationFixture(t)
	if _, err := f.db.Exec(`CREATE FUNCTION reject_export_ready() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='audit.export_ready' THEN RAISE EXCEPTION 'synthetic publication failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_export_ready BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION reject_export_ready()`); err != nil {
		t.Fatal(err)
	}
	if worked, err := s.StepPublication(context.Background(), uuid.New()); !worked || err == nil {
		t.Fatal("publication failure missing", err)
	}
	out, err := s.Status(context.Background(), p, f.project, export.ID)
	if err != nil || out.Status != "pending" {
		t.Fatal("retryable failure discarded snapshot", err)
	}
	if _, err := f.db.Exec(`DROP TRIGGER reject_export_ready ON audit_log; UPDATE outbox_jobs SET available_at=NOW() WHERE kind='audit.export.publish'`); err != nil {
		t.Fatal(err)
	}
	if worked, err := s.StepPublication(context.Background(), uuid.New()); !worked || err != nil {
		t.Fatal("retry did not publish", err)
	}
	out, err = s.Status(context.Background(), p, f.project, export.ID)
	if err != nil || out.Status != "ready" {
		t.Fatal("retry did not become ready", err)
	}
	if raw, err := s.Download(context.Background(), p, f.project, export.ID); err != nil || len(raw) == 0 {
		t.Fatal("ready snapshot unavailable", err)
	}
}

func TestPlatformAuditPublicationRecoversExpiredFinalLease(t *testing.T) {
	f, s, export, p := publicationFixture(t)
	if _, err := f.db.Exec(`UPDATE outbox_jobs SET status='leased',worker_id=$1,attempt=max_attempts,lease_until=NOW()-INTERVAL '1 second' WHERE kind='audit.export.publish'`, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if worked, err := s.StepPublication(context.Background(), uuid.New()); !worked || err != nil {
		t.Fatal("final lease crash not recovered", err)
	}
	publicationAssertFailed(t, f, s, export, p)
	if worked, err := s.StepPublication(context.Background(), uuid.New()); worked || err != nil {
		t.Fatal("terminal cleanup repeated", err)
	}
}
