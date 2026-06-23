package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

const ddlTokenDenylist = `CREATE TABLE token_denylist (
	jti TEXT PRIMARY KEY,
	lease_id TEXT,
	expires_at TIMESTAMP NOT NULL,
	revoked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	reason TEXT
)`

// TestAgentTokenService_MintRevoke covers the full lifecycle: mint from an
// active lease (audit + validatable token), explicit jti revoke (audit +
// token rejected), and lease-cascade revoke (token rejected once the lease is
// revoked).
func TestAgentTokenService_MintRevoke(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlSecretLeases, ddlTokenDenylist)
	auditRepo := repository.NewAuditRepository(db, dialect)
	leaseSvc := NewLeaseService(db, dialect, auditRepo)
	denylist := repository.NewTokenDenylistRepository(db, dialect)
	jwtSvc := auth.NewJWTService("test-secret")
	jwtSvc.EnableDenylist(denylist)
	ats := NewAgentTokenService(jwtSvc, leaseSvc, denylist, auditRepo)

	actor := uuid.New()
	projectID := uuid.New()
	lease, err := leaseSvc.CreateLease(actor, projectID, "alpha", []string{"DATABASE_URL"}, time.Hour, "10.0.0.1")
	if err != nil {
		t.Fatalf("CreateLease: %v", err)
	}

	// Mint.
	minted, err := ats.MintToken(actor, projectID, lease.ID, 5*time.Minute, "10.0.0.1")
	if err != nil {
		t.Fatalf("MintToken: %v", err)
	}
	if minted.Token == "" || minted.TokenType != "agent" {
		t.Fatalf("bad mint result: %+v", minted)
	}
	if got := auditRowsFor(t, db, "agent.token.minted", &actor); got != 1 {
		t.Fatalf("agent.token.minted rows=%d, want 1", got)
	}

	// The minted token must validate (and through the denylist, which is clean).
	claims, err := jwtSvc.ValidateToken(minted.Token)
	if err != nil {
		t.Fatalf("ValidateToken(minted): %v", err)
	}
	if claims.LeaseID == nil || *claims.LeaseID != lease.ID {
		t.Fatalf("minted token lease = %v, want %v", claims.LeaseID, lease.ID)
	}
	jti := claims.ID

	// Explicit jti revoke → audited and the token is now rejected.
	if err := ats.RevokeToken(actor, projectID, jti, "10.0.0.1"); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	requireAuditRow(t, db, "agent.token.revoked", actor)
	if _, err := jwtSvc.ValidateToken(minted.Token); err == nil {
		t.Error("revoked token still validates")
	}

	// Lease-cascade: a fresh token from the same lease dies when the lease is
	// revoked, without an explicit per-token revoke.
	minted2, err := ats.MintToken(actor, projectID, lease.ID, 5*time.Minute, "10.0.0.1")
	if err != nil {
		t.Fatalf("MintToken#2: %v", err)
	}
	if _, err := jwtSvc.ValidateToken(minted2.Token); err != nil {
		t.Fatalf("second token rejected before lease revoke: %v", err)
	}
	if err := leaseSvc.RevokeLease(lease.ID, projectID, actor, "10.0.0.1"); err != nil {
		t.Fatalf("RevokeLease: %v", err)
	}
	if _, err := jwtSvc.ValidateToken(minted2.Token); err == nil {
		t.Error("token from a revoked lease still validates (no lease-cascade)")
	}
}

// TestAgentTokenService_MintErrors covers the negative paths: a lease in another
// project, and a missing/inactive lease.
func TestAgentTokenService_MintErrors(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlSecretLeases, ddlTokenDenylist)
	auditRepo := repository.NewAuditRepository(db, dialect)
	leaseSvc := NewLeaseService(db, dialect, auditRepo)
	denylist := repository.NewTokenDenylistRepository(db, dialect)
	jwtSvc := auth.NewJWTService("test-secret")
	jwtSvc.EnableDenylist(denylist)
	ats := NewAgentTokenService(jwtSvc, leaseSvc, denylist, auditRepo)

	actor := uuid.New()
	projectID := uuid.New()
	otherProject := uuid.New()
	lease, err := leaseSvc.CreateLease(actor, projectID, "alpha", []string{"K"}, time.Hour, "10.0.0.1")
	if err != nil {
		t.Fatalf("CreateLease: %v", err)
	}

	// Lease exists but belongs to a different project than the caller's scope.
	if _, err := ats.MintToken(actor, otherProject, lease.ID, time.Minute, "10.0.0.1"); !errors.Is(err, ErrLeaseProjectMismatch) {
		t.Errorf("cross-project mint err = %v, want ErrLeaseProjectMismatch", err)
	}

	// Unknown lease.
	if _, err := ats.MintToken(actor, projectID, uuid.New(), time.Minute, "10.0.0.1"); !errors.Is(err, ErrLeaseNotActive) {
		t.Errorf("unknown-lease mint err = %v, want ErrLeaseNotActive", err)
	}

	// Revoked lease.
	if err := leaseSvc.RevokeLease(lease.ID, projectID, actor, "10.0.0.1"); err != nil {
		t.Fatalf("RevokeLease: %v", err)
	}
	if _, err := ats.MintToken(actor, projectID, lease.ID, time.Minute, "10.0.0.1"); !errors.Is(err, ErrLeaseNotActive) {
		t.Errorf("revoked-lease mint err = %v, want ErrLeaseNotActive", err)
	}
}
