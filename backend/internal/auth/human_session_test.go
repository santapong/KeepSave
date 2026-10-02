package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type recordingHumanSessions struct {
	calls  int
	ctx    context.Context
	claims *Claims
	raw    string
	err    error
}

func (v *recordingHumanSessions) CheckHumanSession(ctx context.Context, claims *Claims, raw string) error {
	v.calls++
	v.ctx, v.claims, v.raw = ctx, claims, raw
	return v.err
}

func TestHumanSessionClaimsAndAuthoritativeContext(t *testing.T) {
	svc := NewJWTService("synthetic-human-session-test")
	validator := &recordingHumanSessions{}
	svc.EnableHumanSessions(validator)
	// A human session is not an agent grant, even though both carry a jti.
	dl := newFakeDenylist()
	dl.err = errors.New("agent checker must not run for humans")
	svc.EnableDenylist(dl)
	uid, sid := uuid.New(), uuid.New()
	expires := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	raw, err := svc.GenerateSessionToken(uid, "synthetic@example.invalid", sid, expires)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	claims, err := svc.ValidateTokenContext(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	if claims.TokenType != "human" || claims.SessionID != sid.String() || claims.ID != sid.String() || claims.UserID != uid || claims.ExpiresAt == nil || !claims.ExpiresAt.Time.Equal(expires) {
		t.Fatalf("incorrect human session claims: %#v", claims)
	}
	if validator.calls != 1 || validator.ctx != ctx || validator.claims != claims || validator.raw != raw {
		t.Fatal("authoritative validator did not receive the request context and exact token")
	}
}

func TestHumanSessionMintRejectsInvalidIdentityAndLifetime(t *testing.T) {
	svc := NewJWTService("synthetic-human-session-test")
	uid, sid := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name    string
		uid     uuid.UUID
		sid     uuid.UUID
		expires time.Time
	}{
		{"missing user", uuid.Nil, sid, time.Now().Add(time.Hour)},
		{"missing session", uid, uuid.Nil, time.Now().Add(time.Hour)},
		{"expired", uid, sid, time.Now().Add(-time.Minute)},
		{"beyond maximum", uid, sid, time.Now().Add(25 * time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if raw, err := svc.GenerateSessionToken(tc.uid, "", tc.sid, tc.expires); err == nil || raw != "" {
				t.Fatal("invalid human session was minted")
			}
		})
	}
}

func TestHumanSessionRejectsMalformedAndLegacyClaimsBeforeDB(t *testing.T) {
	svc := NewJWTService("synthetic-human-session-test")
	validator := &recordingHumanSessions{}
	svc.EnableHumanSessions(validator)
	for _, tc := range []struct {
		name   string
		mutate func(*Claims)
	}{
		{"legacy cutover", func(c *Claims) { c.TokenType, c.SessionID, c.ID = "", "", "" }},
		{"missing session", func(c *Claims) { c.SessionID = "" }},
		{"missing jti", func(c *Claims) { c.ID = "" }},
		{"mismatched jti", func(c *Claims) { c.ID = uuid.NewString() }},
		{"missing user", func(c *Claims) { c.UserID = uuid.Nil }},
		{"missing expiry", func(c *Claims) { c.ExpiresAt = nil }},
		{"unknown principal type", func(c *Claims) { c.TokenType = "operator" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sid := uuid.NewString()
			claims := &Claims{UserID: uuid.New(), TokenType: "human", SessionID: sid, RegisteredClaims: jwt.RegisteredClaims{ID: sid, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
			tc.mutate(claims)
			raw, err := svc.signClaims(claims)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.ValidateToken(raw); err == nil {
				t.Fatal("malformed human claims were accepted")
			}
			if validator.calls != 0 {
				t.Fatal("malformed token reached the authoritative validator")
			}
		})
	}
}

func TestHumanSessionValidatorFailuresRemainFailClosed(t *testing.T) {
	svc := NewJWTService("synthetic-human-session-test")
	validator := &recordingHumanSessions{}
	svc.EnableHumanSessions(validator)
	raw, err := svc.GenerateSessionToken(uuid.New(), "", uuid.New(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []error{ErrSessionInvalid, ErrSessionUnavailable} {
		validator.err = failure
		claims, err := svc.ValidateToken(raw)
		if claims != nil || !errors.Is(err, failure) {
			t.Fatalf("authoritative failure was bypassed or hidden: claims=%v err=%v", claims, err)
		}
	}
	if validator.calls != 2 {
		t.Fatal("current authority was not checked on every request")
	}
}

func TestAgentTokenDoesNotInheritHumanSessionValidation(t *testing.T) {
	svc := NewJWTService("synthetic-human-session-test")
	validator := &recordingHumanSessions{err: ErrSessionUnavailable}
	svc.EnableHumanSessions(validator)
	dl := newFakeDenylist()
	svc.EnableDenylist(dl)
	lease, project := uuid.New(), uuid.New()
	raw, jti, _, err := svc.GenerateAgentToken(uuid.New(), "", lease, project, "alpha", []string{"FIXTURE"}, time.Minute, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := svc.ValidateToken(raw)
	if err != nil || claims.TokenType != "agent" || claims.SessionID != "" || claims.ID != jti {
		t.Fatalf("agent kind was not preserved: claims=%v err=%v", claims, err)
	}
	if validator.calls != 0 {
		t.Fatal("agent token was treated as a human browser session")
	}
	dl.revokedLeases[lease] = true
	if _, err := svc.ValidateToken(raw); err == nil {
		t.Fatal("agent revocation was bypassed with human sessions enabled")
	}
}
