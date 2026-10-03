package service

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/migrations"
)

const testVerifier = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123"

func testChallenge() string {
	h := sha256.Sum256([]byte(testVerifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
func socialFixture(t *testing.T) (*SocialAuthService, *sql.DB) {
	t.Helper()
	db, d, err := repository.NewDB("sqlite://" + filepath.Join(t.TempDir(), "social.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = repository.RunMigrationsFS(db, d, migrations.FS); err != nil {
		t.Fatal(err)
	}
	cfg := config.SocialAuth{Origin: "http://127.0.0.1:4651", GitHub: config.SocialProvider{ClientID: "github-client", ClientSecret: "test-secret"}, Google: config.SocialProvider{ClientID: "google-client", ClientSecret: "test-secret"}}
	audit := repository.NewAuditRepository(db, d)
	audit.SetChainKey([]byte("test-audit-chain-key"))
	return NewSocialAuthService(cfg, repository.NewSocialAuthRepository(db, d), audit, auth.NewJWTService("test-signing-key")), db
}

type rewriteTransport struct{ target *url.URL }

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.URL.Scheme = r.target.Scheme
	copy.URL.Host = r.target.Host
	return http.DefaultTransport.RoundTrip(copy)
}
func providerServer(t *testing.T, s *SocialAuthService, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL)
	s.client.Transport = rewriteTransport{target}
}
func githubMock(t *testing.T, s *SocialAuthService, verified bool) *atomic.Int32 {
	t.Helper()
	calls := &atomic.Int32{}
	providerServer(t, s, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/oauth/access_token":
			if r.Method != "POST" {
				t.Error("token must be POST")
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("code_verifier") != testVerifier || r.Form.Get("redirect_uri") != "http://127.0.0.1:4651/auth/callback/github" || r.Form.Get("client_secret") != "test-secret" {
				t.Error("token exchange binding missing")
			}
			json.NewEncoder(w).Encode(map[string]string{"access_token": "provider-token-not-for-storage", "token_type": "bearer"})
		case "/user":
			if r.Header.Get("Authorization") != "Bearer provider-token-not-for-storage" {
				t.Error("missing provider auth")
			}
			w.Write([]byte(`{"id":1234567}`))
		case "/user/emails":
			json.NewEncoder(w).Encode([]map[string]any{{"email": "social@example.com", "primary": true, "verified": verified}})
		default:
			t.Errorf("unexpected provider path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	return calls
}
func startTestFlow(t *testing.T, s *SocialAuthService, provider string, user *uuid.UUID) *SocialStart {
	t.Helper()
	start, err := s.Start(context.Background(), provider, testChallenge(), "127.0.0.1", user)
	if err != nil {
		t.Fatal(err)
	}
	return start
}
func completeTestFlow(s *SocialAuthService, provider, state string, user *uuid.UUID) (*AuthResponse, error) {
	return s.Complete(context.Background(), provider, "test-code", state, testVerifier, "127.0.0.1", user)
}

func TestSocialGitHubEndToEndAndReplay(t *testing.T) {
	s, db := socialFixture(t)
	calls := githubMock(t, s, true)
	start := startTestFlow(t, s, "github", nil)
	u, _ := url.Parse(start.URL)
	q := u.Query()
	if q.Get("code_challenge") != testChallenge() || q.Get("code_challenge_method") != "S256" || q.Get("scope") != "read:user user:email" {
		t.Fatal("incorrect authorization parameters")
	}
	result, err := completeTestFlow(s, "github", start.State, nil)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := s.jwt.ValidateToken(result.Token)
	if err != nil || claims.UserID != result.User.ID {
		t.Fatal("no valid local JWT")
	}
	if result.User.PasswordHash != "!" {
		t.Fatal("social user has usable password")
	}
	if _, err = completeTestFlow(s, "github", start.State, nil); !errors.Is(err, repository.ErrSocialFlow) {
		t.Fatalf("replay accepted: %v", err)
	}
	if calls.Load() != 3 {
		t.Fatal("replay reached provider")
	}
	next := startTestFlow(t, s, "github", nil)
	again, err := completeTestFlow(s, "github", next.State, nil)
	if err != nil || again.User.ID != result.User.ID {
		t.Fatal("identity not stable")
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count != 1 {
		t.Fatal("repeat login created a user")
	}
	db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='auth.login'`).Scan(&count)
	if count != 2 {
		t.Fatal("missing login audit")
	}
	var details string
	rows, _ := db.Query(`SELECT details FROM audit_log`)
	defer rows.Close()
	for rows.Next() {
		rows.Scan(&details)
		if strings.Contains(details, "token") || strings.Contains(details, "test-code") || strings.Contains(details, "test-secret") {
			t.Fatal("audit contains credentials")
		}
	}
}
func TestSocialFlowBindingAndExpiry(t *testing.T) {
	s, db := socialFixture(t)
	calls := githubMock(t, s, true)
	for _, mode := range []string{"verifier", "state", "provider", "expired"} {
		t.Run(mode, func(t *testing.T) {
			start := startTestFlow(t, s, "github", nil)
			provider, state, verifier := "github", start.State, testVerifier
			switch mode {
			case "verifier":
				verifier = strings.Repeat("x", 64)
			case "state":
				state = strings.Repeat("a", 64)
			case "provider":
				provider = "google"
			case "expired":
				db.Exec(`UPDATE social_auth_flows SET expires_at=0`)
			}
			_, err := s.Complete(context.Background(), provider, "test-code", state, verifier, "127.0.0.1", nil)
			if !errors.Is(err, repository.ErrSocialFlow) {
				t.Fatalf("binding failed: %v", err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("invalid flow reached provider")
	}
}
func TestSocialEmailCollisionAndAuthenticatedLink(t *testing.T) {
	s, db := socialFixture(t)
	githubMock(t, s, true)
	users := repository.NewUserRepository(db, repository.NewDialect(repository.DBTypeSQLite))
	existing, err := users.Create("Social@Example.com", "password-hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := users.Create("other@example.com", "password-hash")
	if err != nil {
		t.Fatal(err)
	}
	start := startTestFlow(t, s, "github", nil)
	if _, err = completeTestFlow(s, "github", start.State, nil); !errors.Is(err, repository.ErrSocialConflict) {
		t.Fatal("email automatically linked")
	}
	start = startTestFlow(t, s, "github", &existing.ID)
	for _, wrong := range []*uuid.UUID{nil, &other.ID} {
		if _, err = completeTestFlow(s, "github", start.State, wrong); !errors.Is(err, repository.ErrSocialFlow) {
			t.Fatal("link user not bound")
		}
	}
	linked, err := completeTestFlow(s, "github", start.State, &existing.ID)
	if err != nil || linked.User.ID != existing.ID || linked.Token != "" {
		t.Fatalf("link failed: %v", err)
	}
	connected, err := s.Connections(context.Background(), existing.ID)
	if err != nil || len(connected) != 1 || connected[0] != "github" {
		t.Fatal("connection not saved")
	}
	start = startTestFlow(t, s, "github", &other.ID)
	if _, err = completeTestFlow(s, "github", start.State, &other.ID); !errors.Is(err, repository.ErrSocialConflict) {
		t.Fatal("identity was moved")
	}
	start = startTestFlow(t, s, "github", nil)
	login, err := completeTestFlow(s, "github", start.State, nil)
	if err != nil || login.User.ID != existing.ID {
		t.Fatal("linked login lost existing account")
	}
}
func TestSocialUnverifiedAndDisabled(t *testing.T) {
	s, db := socialFixture(t)
	githubMock(t, s, false)
	start := startTestFlow(t, s, "github", nil)
	if _, err := completeTestFlow(s, "github", start.State, nil); !errors.Is(err, ErrSocialIdentity) {
		t.Fatal("unverified email accepted")
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count != 0 {
		t.Fatal("unverified account persisted")
	}
	s.config.GitHub.ClientSecret = ""
	if s.Providers()["github"] {
		t.Fatal("incomplete provider enabled")
	}
	if _, err := s.Start(context.Background(), "github", testChallenge(), "", nil); !errors.Is(err, ErrSocialUnavailable) {
		t.Fatal("disabled start accepted")
	}
}
func TestSocialConcurrentConsume(t *testing.T) {
	s, _ := socialFixture(t)
	start := startTestFlow(t, s, "github", nil)
	var successful atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.repo.ConsumeFlow(context.Background(), socialHash(start.State), "github", testChallenge(), nil) == nil {
				successful.Add(1)
			}
		}()
	}
	wg.Wait()
	if successful.Load() != 1 {
		t.Fatalf("consumed %d times", successful.Load())
	}
}
func TestSocialAuditFailureDoesNotIssueToken(t *testing.T) {
	s, db := socialFixture(t)
	githubMock(t, s, true)
	start := startTestFlow(t, s, "github", nil)
	if _, err := db.Exec(`CREATE TRIGGER fail_login_audit BEFORE INSERT ON audit_log WHEN NEW.action='auth.login' BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	result, err := completeTestFlow(s, "github", start.State, nil)
	if err == nil || result != nil {
		t.Fatal("issued token without outcome audit")
	}
}
func TestSocialGoogleSignedClaims(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	badKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"valid", "issuer", "audience", "expiry", "nonce", "signature", "unverified", "subject", "authorized-party"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := socialFixture(t)
			s.googleVerifier = oidc.NewVerifier("https://accounts.google.com", &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{ClientID: "google-client"})
			start := startTestFlow(t, s, "google", nil)
			u, _ := url.Parse(start.URL)
			if u.Query().Get("nonce") != start.State || u.Query().Get("scope") != "openid email" {
				t.Fatal("missing Google binding")
			}
			claims := jwt.MapClaims{"iss": "https://accounts.google.com", "aud": "google-client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "sub": "google-stable-id", "nonce": start.State, "email": "google@example.com", "email_verified": true}
			signingKey := key
			switch mode {
			case "issuer":
				claims["iss"] = "https://attacker.example"
			case "audience":
				claims["aud"] = "wrong-client"
			case "expiry":
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			case "nonce":
				claims["nonce"] = "wrong"
			case "signature":
				signingKey = badKey
			case "unverified":
				claims["email_verified"] = false
			case "subject":
				claims["sub"] = ""
			case "authorized-party":
				claims["azp"] = "wrong-client"
			}
			signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(signingKey)
			if err != nil {
				t.Fatal(err)
			}
			providerServer(t, s, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/token" {
					t.Error("unexpected Google request")
				}
				r.ParseForm()
				if r.Form.Get("code_verifier") != testVerifier {
					t.Error("missing PKCE")
				}
				json.NewEncoder(w).Encode(map[string]string{"id_token": signed})
			})
			result, err := completeTestFlow(s, "google", start.State, nil)
			if mode == "valid" {
				if err != nil || result.Token == "" {
					t.Fatalf("valid Google identity rejected: %v", err)
				}
			} else if !errors.Is(err, ErrSocialIdentity) || result != nil {
				t.Fatalf("invalid %s accepted: %v", mode, err)
			}
		})
	}
}
