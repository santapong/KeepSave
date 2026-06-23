package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeDenylist records jti revocations and treats a fixed set of lease IDs as
// revoked (lease-cascade), mirroring the repository's two revocation legs.
type fakeDenylist struct {
	revokedJTI    map[string]bool
	revokedLeases map[uuid.UUID]bool
	err           error
}

func newFakeDenylist() *fakeDenylist {
	return &fakeDenylist{revokedJTI: map[string]bool{}, revokedLeases: map[uuid.UUID]bool{}}
}

func (f *fakeDenylist) IsTokenRevoked(jti string, leaseID *uuid.UUID) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.revokedJTI[jti] {
		return true, nil
	}
	if leaseID != nil && f.revokedLeases[*leaseID] {
		return true, nil
	}
	return false, nil
}

func TestAgentToken_RoundTripAndClaims(t *testing.T) {
	svc, _ := newRS256Service(t)
	dl := newFakeDenylist()
	svc.EnableDenylist(dl)

	uid, leaseID := uuid.New(), uuid.New()
	tok, jti, exp, err := svc.GenerateAgentToken(uid, "", leaseID, 5*time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAgentToken: %v", err)
	}
	if jti == "" {
		t.Fatal("empty jti")
	}
	if time.Until(exp) > 5*time.Minute+time.Second {
		t.Errorf("expiry %v exceeds requested ttl", time.Until(exp))
	}
	claims, err := svc.ValidateToken(tok)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.TokenType != "agent" {
		t.Errorf("TokenType = %q, want agent", claims.TokenType)
	}
	if claims.LeaseID == nil || *claims.LeaseID != leaseID {
		t.Errorf("LeaseID = %v, want %v", claims.LeaseID, leaseID)
	}
	if claims.ID != jti {
		t.Errorf("jti claim = %q, want %q", claims.ID, jti)
	}
}

func TestAgentToken_TTLCappedAtMax(t *testing.T) {
	svc, _ := newRS256Service(t)
	// Request 1h with a 1h lease remaining; the 15-min hard cap must win.
	_, _, exp, err := svc.GenerateAgentToken(uuid.New(), "", uuid.New(), time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAgentToken: %v", err)
	}
	if time.Until(exp) > maxAgentTokenTTL+time.Second {
		t.Errorf("expiry %v exceeds maxAgentTokenTTL %v", time.Until(exp), maxAgentTokenTTL)
	}
}

func TestAgentToken_TTLClampedToLeaseRemaining(t *testing.T) {
	svc, _ := newRS256Service(t)
	// Lease has only 2 min left; a 15-min request must clamp to ~2 min.
	_, _, exp, err := svc.GenerateAgentToken(uuid.New(), "", uuid.New(), 15*time.Minute, 2*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAgentToken: %v", err)
	}
	if d := time.Until(exp); d > 2*time.Minute+time.Second || d < time.Minute {
		t.Errorf("expiry %v not clamped to lease remaining (~2m)", d)
	}
}

func TestAgentToken_RevokedByJTI(t *testing.T) {
	svc, _ := newRS256Service(t)
	dl := newFakeDenylist()
	svc.EnableDenylist(dl)

	tok, jti, _, err := svc.GenerateAgentToken(uuid.New(), "", uuid.New(), time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAgentToken: %v", err)
	}
	if _, err := svc.ValidateToken(tok); err != nil {
		t.Fatalf("token rejected before revocation: %v", err)
	}
	dl.revokedJTI[jti] = true
	if _, err := svc.ValidateToken(tok); err == nil {
		t.Error("revoked-by-jti token was accepted")
	}
}

func TestAgentToken_RevokedByLeaseCascade(t *testing.T) {
	svc, _ := newRS256Service(t)
	dl := newFakeDenylist()
	svc.EnableDenylist(dl)

	leaseID := uuid.New()
	tok, _, _, err := svc.GenerateAgentToken(uuid.New(), "", leaseID, time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAgentToken: %v", err)
	}
	dl.revokedLeases[leaseID] = true
	if _, err := svc.ValidateToken(tok); err == nil {
		t.Error("token from a revoked lease was accepted")
	}
}

// A user token (no jti) must never touch the denylist — proven by a denylist
// that errors on every call yet leaves user-token validation unaffected.
func TestUserToken_SkipsDenylist(t *testing.T) {
	svc, _ := newRS256Service(t)
	dl := newFakeDenylist()
	dl.err = errTestDenylist
	svc.EnableDenylist(dl)

	tok, err := svc.GenerateToken(uuid.New(), "u@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, err := svc.ValidateToken(tok); err != nil {
		t.Errorf("user token validation consulted the denylist: %v", err)
	}
}

var errTestDenylist = &denylistTestErr{}

type denylistTestErr struct{}

func (*denylistTestErr) Error() string { return "denylist should not be consulted" }
