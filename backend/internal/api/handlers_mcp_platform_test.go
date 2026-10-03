package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/mcpauth"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestMCPPlatformTrackedHumanConsentAndPublicOAuthRouter(t *testing.T) {
	if os.Getenv("KEEPSAVE_PLATFORM_POSTGRES_TEST") != "1" {
		t.Skip("requires fixed disposable PostgreSQL target")
	}
	f := newPlatformFixture(t)
	a := repository.NewAuditRepository(f.db, f.d)
	j := auth.NewJWTService("synthetic-mcp-handler-signing-key")
	sessions := service.NewSessionService(f.db, f.d, j, a)
	j.EnableHumanSessions(sessions)
	tx, e := f.db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	human, e := sessions.IssueTx(context.Background(), tx, &models.User{ID: f.owner, Email: "fixture@example.invalid"}, "synthetic", "", "")
	if e != nil {
		tx.Rollback()
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	cfg := mcpauth.Config{Issuer: "https://app.keepsave.example", AppURL: "https://app.keepsave.example", Resource: "https://app.keepsave.example/mcp"}
	s, e := mcpauth.New(f.db, f.d, a, authority.Postgres(f.db), cfg)
	if e != nil {
		t.Fatal(e)
	}
	h := NewMCPPlatformHandler(s)
	r := NewRouter(Dependencies{CoreRelease: true, CORSOrigins: cfg.AppURL, JWTService: j, APIKeyRepo: f.keys, ProjectRepo: repository.NewProjectRepository(f.db, f.d), AuthHandler: &AuthHandler{}, MCPPlatformHandler: h, DB: f.db})
	call := func(method, path, body, content, token, origin string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if content != "" {
			req.Header.Set("Content-Type", content)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	w := call("GET", "/.well-known/oauth-protected-resource/mcp", "", "", "", "https://evil.example")
	if w.Code != 403 {
		t.Fatal("public metadata origin bypass")
	}
	w = call("OPTIONS", "/oauth/mcp/token", "", "", "", "https://evil.example")
	if w.Code != 403 {
		t.Fatal("CORS preflight bypassed metadata origin boundary")
	}
	w = call("GET", "/.well-known/oauth-authorization-server", "", "", "", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "registration_endpoint") || !strings.Contains(w.Body.String(), "authorization_response_iss_parameter_supported") {
		t.Fatal("incorrect public authorization metadata")
	}
	verifier := strings.Repeat("a", 64)
	challenge := sha256.Sum256([]byte(verifier))
	v := url.Values{"response_type": {"code"}, "client_id": {"keepsave-hermes-linux-v1"}, "redirect_uri": {"http://127.0.0.1:17702/callback"}, "scope": {mcpauth.Scope + " offline_access"}, "state": {strings.Repeat("s", 32)}, "resource": {cfg.Resource}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}}
	w = call("GET", "/oauth/mcp/authorize?"+v.Encode(), "", "", "", "")
	if w.Code != 303 {
		t.Fatalf("authorize %d", w.Code)
	}
	location, _ := url.Parse(w.Header().Get("Location"))
	id := location.Query().Get("request_id")
	if _, e = uuid.Parse(id); e != nil {
		t.Fatal("missing bounded consent request")
	}
	w = call("GET", "/api/v1/mcp/consent?request_id="+id, "", "", "", "")
	if w.Code != 401 {
		t.Fatal("consent lacks human boundary")
	}
	w = call("GET", "/api/v1/mcp/consent?request_id="+id, "", "", human, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "code_challenge") || strings.Contains(w.Body.String(), `"state"`) {
		t.Fatal("preview schema or human session failed")
	}
	w = call("POST", "/api/v1/mcp/consent", `{"request_id":"`+id+`","approve":true}`, "application/json", human, "")
	if w.Code != 200 {
		t.Fatalf("decision %d %s", w.Code, w.Body)
	}
	var decision struct {
		Redirect string `json:"redirect_url"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &decision); e != nil {
		t.Fatal(e)
	}
	callback, _ := url.Parse(decision.Redirect)
	if callback.Query().Get("iss") != cfg.Issuer || callback.Query().Get("state") != v.Get("state") || strings.Contains(w.Body.String(), "access_token") {
		t.Fatal("browser received invalid callback or token")
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {v.Get("client_id")}, "resource": {cfg.Resource}, "redirect_uri": {v.Get("redirect_uri")}, "code": {callback.Query().Get("code")}, "code_verifier": {verifier}}
	bad := form.Encode() + "&resource=" + url.QueryEscape(cfg.Resource)
	w = call("POST", "/oauth/mcp/token", bad, "application/x-www-form-urlencoded", "", "")
	if w.Code != 400 {
		t.Fatal("ambiguous token request accepted")
	}
	w = call("POST", "/oauth/mcp/token", form.Encode()+"&client_secret=", "application/x-www-form-urlencoded", "", "")
	if w.Code != 400 {
		t.Fatal("confidential-client shortcut accepted")
	}
	w = call("POST", "/oauth/mcp/token", form.Encode(), "application/x-www-form-urlencoded", "", "")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("token exchange %d", w.Code)
	}
	var token mcpauth.TokenResponse
	if e = json.Unmarshal(w.Body.Bytes(), &token); e != nil {
		t.Fatal(e)
	}
	principal, e := s.Validate(context.Background(), token.AccessToken)
	if e != nil {
		t.Fatal(e)
	}
	w = call("GET", "/api/v1/account/delegations", "", "", human, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), principal.FamilyID.String()) || strings.Contains(w.Body.String(), token.AccessToken) || strings.Contains(w.Body.String(), principal.TokenID.String()) {
		t.Fatal("owned delegation discovery leaks or omits metadata")
	}
	w = call("DELETE", "/api/v1/account/delegations/"+principal.FamilyID.String(), "", "", human, "")
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatalf("owned family revoke failed: %d %s", w.Code, w.Body)
	}
	w = call("POST", "/oauth/mcp/token", form.Encode(), "application/x-www-form-urlencoded", "", "")
	if w.Code != 400 {
		t.Fatal("router code replay accepted")
	}
	revoke := url.Values{"client_id": {v.Get("client_id")}, "token": {token.RefreshToken}}
	w = call("POST", "/oauth/mcp/revoke", revoke.Encode(), "application/x-www-form-urlencoded", "", "")
	if w.Code != 200 {
		t.Fatal("revoke failed")
	}
	if _, e = s.Validate(context.Background(), token.AccessToken); e == nil {
		t.Fatal("revoked router token survived")
	}
}
func TestMCPDecisionRequiresOneExactHumanChoice(t *testing.T) {
	id := uuid.NewString()
	for _, body := range []string{`{"request_id":"` + id + `","approve":false,"approve":true}`, `{"request_id":"` + id + `","approve":true,"unexpected":1}`, `{"request_id":"` + id + `","approve":true} {}`, `{"request_id":"` + id + `","approve":null}`, `{"request_id":"` + uuid.Nil.String() + `","approve":true}`} {
		if _, _, e := mcpDecision(strings.NewReader(body)); e == nil {
			t.Fatalf("ambiguous decision accepted: %s", body)
		}
	}
	got, approve, e := mcpDecision(strings.NewReader(`{"request_id":"` + id + `","approve":false}`))
	if e != nil || got.String() != id || approve {
		t.Fatal("explicit denial rejected")
	}
}
