package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"net/http/httptest"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

// The contract uses this deliberately small JSON Schema subset. Reject unknown
// keywords so additions cannot silently stop being checked by the test validator.
func validateContractValue(spec, schema map[string]interface{}, value interface{}, at string) error {
	if len(schema) == 0 {
		return nil
	}
	for keyword := range schema {
		switch keyword {
		case "oneOf", "$ref", "type", "format", "nullable", "properties", "required", "additionalProperties", "items", "enum", "minimum", "maximum", "minLength", "maxLength", "minItems", "maxItems", "pattern":
		default:
			return fmt.Errorf("%s: unsupported schema keyword %s", at, keyword)
		}
	}
	if choices, ok := schema["oneOf"].([]interface{}); ok {
		matches := 0
		for _, choice := range choices {
			branch, ok := choice.(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid oneOf")
			}
			if validateContractValue(spec, branch, value, at) == nil {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("%s: oneOf matched %d branches", at, matches)
		}
		return nil
	}
	if name, ok := schema["$ref"].(string); ok {
		components := spec["components"].(map[string]interface{})["schemas"].(map[string]interface{})
		target, ok := components[strings.TrimPrefix(name, "#/components/schemas/")].(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s: unresolved ref", at)
		}
		return validateContractValue(spec, target, value, at)
	}
	if value == nil {
		if schema["nullable"] == true {
			return nil
		}
		return fmt.Errorf("%s: unexpected null", at)
	}
	if values, ok := schema["enum"].([]interface{}); ok {
		matched := false
		for _, item := range values {
			if item == value {
				matched = true
			}
		}
		if !matched {
			return fmt.Errorf("%s: outside enum", at)
		}
	}
	switch schema["type"] {
	case "object":
		object, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s: expected object", at)
		}
		props, _ := schema["properties"].(map[string]interface{})
		if required, ok := schema["required"].([]interface{}); ok {
			for _, key := range required {
				if _, exists := object[key.(string)]; !exists {
					return fmt.Errorf("%s: missing %s", at, key)
				}
			}
		}
		for key, child := range object {
			shape, exists := props[key]
			if !exists {
				if schema["additionalProperties"] == false {
					return fmt.Errorf("%s: unexpected %s", at, key)
				}
				if shape, ok := schema["additionalProperties"].(map[string]interface{}); ok {
					if err := validateContractValue(spec, shape, child, at+"."+key); err != nil {
						return err
					}
				}
				continue
			}
			if err := validateContractValue(spec, shape.(map[string]interface{}), child, at+"."+key); err != nil {
				return err
			}
		}
	case "array":
		values, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("%s: expected array", at)
		}
		if min, ok := schema["minItems"].(float64); ok && len(values) < int(min) {
			return fmt.Errorf("%s: too few items", at)
		}
		if max, ok := schema["maxItems"].(float64); ok && len(values) > int(max) {
			return fmt.Errorf("%s: too many items", at)
		}
		for index, child := range values {
			if err := validateContractValue(spec, schema["items"].(map[string]interface{}), child, fmt.Sprintf("%s[%d]", at, index)); err != nil {
				return err
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s: expected string", at)
		}
		count := len([]rune(text))
		if pattern, ok := schema["pattern"].(string); ok {
			matched, err := regexp.MatchString(pattern, text)
			if err != nil || !matched {
				return fmt.Errorf("%s: does not match pattern", at)
			}
		}
		if min, ok := schema["minLength"].(float64); ok && count < int(min) {
			return fmt.Errorf("%s: too short", at)
		}
		if max, ok := schema["maxLength"].(float64); ok && count > int(max) {
			return fmt.Errorf("%s: too long", at)
		}
		switch schema["format"] {
		case "uuid":
			if _, err := uuid.Parse(text); err != nil {
				return fmt.Errorf("%s: invalid uuid", at)
			}
		case "date-time":
			if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
				return fmt.Errorf("%s: invalid timestamp", at)
			}
		case "uri":
			if u, err := url.Parse(text); err != nil || u.Scheme == "" {
				return fmt.Errorf("%s: invalid URI", at)
			}
		case "email":
			if _, err := mail.ParseAddress(text); err != nil {
				return fmt.Errorf("%s: invalid email", at)
			}
		case "byte":
			if _, err := base64.StdEncoding.DecodeString(text); err != nil {
				return fmt.Errorf("%s: invalid base64", at)
			}
		case nil:
		default:
			return fmt.Errorf("%s: unsupported string format", at)
		}
	case "integer", "number":
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s: expected number", at)
		}
		if schema["type"] == "integer" && math.Trunc(number) != number {
			return fmt.Errorf("%s: expected integer", at)
		}
		if min, ok := schema["minimum"].(float64); ok && number < min {
			return fmt.Errorf("%s: below minimum", at)
		}
		if max, ok := schema["maximum"].(float64); ok && number > max {
			return fmt.Errorf("%s: above maximum", at)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean", at)
		}
	default:
		return fmt.Errorf("%s: unsupported type", at)
	}
	return nil
}
func assertCoreResponse(t *testing.T, path, method string, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("%s %s: status %d want %d (body omitted)", method, path, w.Code, want)
	}
	spec := openAPISpec()
	op := spec["paths"].(map[string]interface{})[path].(map[string]interface{})[strings.ToLower(method)].(map[string]interface{})
	response := op["responses"].(map[string]interface{})[strconv.Itoa(w.Code)].(map[string]interface{})
	content, ok := response["content"].(map[string]interface{})
	if !ok {
		if w.Body.Len() != 0 {
			t.Fatal("body on no-content response")
		}
		return
	}
	media, _, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
	if err != nil {
		t.Fatal("response has no valid content type")
	}
	entry, ok := content[media].(map[string]interface{})
	if !ok {
		t.Fatalf("%s %s: undocumented content type %s", method, path, media)
	}
	var value interface{}
	if media == "application/json" {
		if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
	} else if media == "text/plain" {
		value = w.Body.String()
	} else {
		t.Fatal("unsupported response validation content type")
	}
	schema := entry["schema"].(map[string]interface{})
	if err := validateContractValue(spec, schema, value, "response"); err != nil {
		t.Fatalf("%s %s violates core contract: %v", method, path, err)
	}
}
func contractRouter(t *testing.T) *platformFixture {
	t.Helper()
	f := newPlatformFixture(t)
	audit := repository.NewAuditRepository(f.db, f.d)
	sessions := service.NewSessionService(f.db, f.d, f.jwt, audit)
	login := service.NewAuthService(repository.NewUserRepository(f.db, f.d), repository.NewAuthAttemptsRepository(f.db, f.d), audit, f.jwt)
	login.EnableSessions(sessions)
	h := NewAuthHandler(login)
	h.SetSocial(NewSocialAuthHandler(service.NewSocialAuthService(config.SocialAuth{}, repository.NewSocialAuthRepository(f.db, f.d), audit, f.jwt)))
	f.r = NewRouter(Dependencies{CoreRelease: true, DisableLocalMCP: true, CORSOrigins: "http://localhost", JWTService: f.jwt, APIKeyRepo: f.keys, ProjectRepo: repository.NewProjectRepository(f.db, f.d), AuthHandler: h, SessionHandler: NewSessionHandler(sessions), RecoveryHandler: NewRecoveryHandler(nil), OpenAPIHandler: NewOpenAPIHandler(), TeamVaultHandler: NewTeamVaultHandler(nil, nil, nil, false), IdentityPlatformHandler: NewIdentityPlatformHandler(nil), ToolPlatformHandler: NewToolPlatformHandler(nil), MCPPlatformHandler: NewMCPPlatformHandler(nil), DB: f.db})
	return f
}
func TestPlatformCoreOpenAPIRoutesAndIdentity(t *testing.T) {
	f := contractRouter(t)
	routes := map[string]bool{}
	for _, route := range f.r.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	private := RunnerRouter(NewToolPlatformHandler(nil)).(*gin.Engine)
	for _, route := range private.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	spec := openAPISpec()
	for path, raw := range spec["paths"].(map[string]interface{}) {
		for method, operation := range raw.(map[string]interface{}) {
			prefix := "/api/v1"
			op := operation.(map[string]interface{})
			if servers, ok := op["servers"].([]interface{}); ok && len(servers) == 1 {
				if servers[0].(map[string]interface{})["url"] == "/" {
					prefix = ""
				}
			}
			route := strings.NewReplacer("{", ":", "}", "").Replace(path)
			if !routes[strings.ToUpper(method)+" "+prefix+route] {
				t.Fatalf("contract route is not mounted: %s %s", method, path)
			}
		}
	}
	docs := f.call("GET", "/api/docs", "", "", uuid.Nil)
	if docs.Code != 200 {
		t.Fatal("docs unavailable")
	}
	var served interface{}
	if err := json.Unmarshal(docs.Body.Bytes(), &served); err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(spec)
	got, _ := json.Marshal(served)
	if string(got) != string(want) {
		t.Fatal("served specification drifted")
	}
	assertCoreResponse(t, "/auth/providers", "GET", f.call("GET", "/api/v1/auth/providers", "", "", uuid.Nil), 200)
	assertCoreResponse(t, "/auth/social/{provider}/start", "POST", f.call("POST", "/api/v1/auth/social/google/start", `{"code_challenge":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, "", uuid.Nil), 503)
	registered := f.call("POST", "/api/v1/auth/register", `{"email":"contract@example.invalid","password":"Contract-Test-42!"}`, "", uuid.Nil)
	assertCoreResponse(t, "/auth/register", "POST", registered, 201)
	assertCoreResponse(t, "/auth/login", "POST", f.call("POST", "/api/v1/auth/login", `{"email":"contract@example.invalid","password":"Contract-Test-42!"}`, "", uuid.Nil), 200)
	assertCoreResponse(t, "/auth/login", "POST", f.call("POST", "/api/v1/auth/login", `{"email":"contract@example.invalid","password":"wrong"}`, "", uuid.Nil), 401)
}
func TestPlatformCoreOpenAPIVaultJourney(t *testing.T) {
	f := newCoreFixture(t)
	base := "/api/v1/projects/" + f.project.String()
	check := func(path, method, url, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		w := f.request(method, url, body, f.owner, "")
		assertCoreResponse(t, path, method, w, want)
		return w
	}
	check("/capabilities", "GET", "/api/v1/capabilities", "", 200)
	check("/account/sessions", "GET", "/api/v1/account/sessions", "", 200)
	workspace := check("/organizations", "POST", "/api/v1/organizations", `{"name":"Contract workspace"}`, 201)
	var org struct {
		Organization struct {
			ID uuid.UUID `json:"id"`
		} `json:"organization"`
	}
	if err := json.Unmarshal(workspace.Body.Bytes(), &org); err != nil {
		t.Fatal(err)
	}
	orgPath := "/api/v1/organizations/" + org.Organization.ID.String()
	check("/organizations", "GET", "/api/v1/organizations", "", 200)
	check("/organizations/{orgId}/members", "GET", orgPath+"/members", "", 200)
	check("/organizations/{orgId}/members/{userId}", "PUT", orgPath+"/members/"+f.owner.String(), `{"role":"viewer"}`, 403)
	check("/projects", "GET", "/api/v1/projects", "", 200)
	check("/projects/{id}", "GET", base, "", 200)
	created := check("/projects/{id}/secrets", "POST", base+"/secrets", `{"key":"CONTRACT","value":"synthetic-v1","environment":"alpha"}`, 201)
	var secret struct {
		Secret struct {
			ID uuid.UUID `json:"id"`
		} `json:"secret"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &secret); err != nil {
		t.Fatal(err)
	}
	path := base + "/secrets/" + secret.Secret.ID.String()
	check("/projects/{id}/secrets", "GET", base+"/secrets?environment=alpha", "", 200)
	check("/projects/{id}/secrets/batch", "POST", base+"/secrets/batch", `{"environment":"alpha","keys":["CONTRACT","ABSENT"]}`, 200)
	check("/projects/{id}/secrets/{secretId}", "PUT", path, `{"value":"synthetic-v2","expected_revision":1}`, 200)
	check("/projects/{id}/secrets/{secretId}/versions", "GET", path+"/versions", "", 200)
	check("/projects/{id}/secrets/{secretId}/versions/{version}", "GET", path+"/versions/1", "", 200)
	check("/projects/{id}/secrets/{secretId}/versions/{version}/restore", "POST", path+"/versions/1/restore", `{"expected_current_revision":2}`, 200)
	check("/projects/{id}/secrets/{secretId}/versions/{version}/restore", "POST", path+"/versions/1/restore", `{"expected_current_revision":2}`, 409)
	backup := check("/projects/{id}/backups", "POST", base+"/backups", "", 201)
	check("/projects/{id}/backups", "GET", base+"/backups", "", 200)
	check("/projects/{id}/backups/verify", "POST", base+"/backups/verify", backup.Body.String(), 200)
	check("/projects/{id}/backups/preview", "POST", base+"/backups/preview", backup.Body.String(), 200)
	check("/projects/{id}/secrets/{secretId}", "PUT", path, `{"value":"synthetic-v4","expected_revision":3}`, 200)
	var bundle vault.Bundle
	if err := json.Unmarshal(backup.Body.Bytes(), &bundle); err != nil {
		t.Fatal(err)
	}
	selected, _ := json.Marshal(map[string]interface{}{"bundle": bundle, "records": []vault.RestoreSelection{{SecretID: secret.Secret.ID, BackupRevision: 3, ExpectedRevision: 4}}})
	check("/projects/{id}/backups/restore", "POST", base+"/backups/restore", string(selected), 200)
	check("/projects/{id}/secrets/{secretId}", "DELETE", path+"?expected_revision=5", "", 204)
	check("/projects/{id}/secrets/{secretId}", "GET", path, "", 404)
	check("/auth/logout", "POST", "/api/v1/auth/logout", "", 204)
	check("/account/sessions", "GET", "/api/v1/account/sessions", "", 401)
}

// Validate the journal's remaining public controls through the composed router,
// including non-JSON credential export and approval failure responses.
func TestPlatformCoreOpenAPIControls(t *testing.T) {
	f := newCoreFixture(t)
	base := "/api/v1/projects/" + f.project.String()
	check := func(path, method, url, body string, who uuid.UUID, want int) *httptest.ResponseRecorder {
		t.Helper()
		w := f.request(method, url, body, who, "")
		assertCoreResponse(t, path, method, w, want)
		return w
	}
	check("/projects/{id}/env-import", "POST", base+"/env-import", `{"environment":"alpha","content":"CONTRACT_CONTROL=synthetic-value\n","overwrite":true}`, f.owner, 200)
	check("/projects/{id}/env-export", "GET", base+"/env-export?environment=alpha", "", f.owner, 200)
	check("/projects/{id}/env-export", "GET", base+"/env-export?environment=alpha&format=file", "", f.owner, 200)
	check("/projects/{id}/promote/diff", "POST", base+"/promote/diff", `{"source_environment":"alpha","target_environment":"uat","keys":["CONTRACT_CONTROL"]}`, f.owner, 200)
	w := check("/projects/{id}/promote", "POST", base+"/promote", `{"source_environment":"alpha","target_environment":"uat","keys":["CONTRACT_CONTROL"],"override_policy":"overwrite"}`, f.owner, 200)
	var envelope struct {
		Promotion struct {
			ID uuid.UUID `json:"id"`
		} `json:"promotion"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	promoted := base + "/promotions/" + envelope.Promotion.ID.String()
	check("/projects/{id}/promotions", "GET", base+"/promotions", "", f.owner, 200)
	check("/projects/{id}/promotions/{promotionId}", "GET", promoted, "", f.owner, 200)
	check("/projects/{id}/rotate-keys", "POST", base+"/rotate-keys", "", f.owner, 200)
	check("/projects/{id}/verify-encryption", "GET", base+"/verify-encryption", "", f.owner, 200)
	check("/projects/{id}/promotions/{promotionId}/rollback", "POST", promoted+"/rollback", "", f.owner, 200)
	check("/projects/{id}/promotions/{promotionId}", "GET", promoted, "", f.owner, 200)
	check("/projects/{id}/promotions", "GET", base+"/promotions", "", f.owner, 200)
	check("/projects/{id}/audit-log", "GET", base+"/audit-log?limit=500", "", f.owner, 200)
	check("/rotate-keys", "POST", "/api/v1/rotate-keys", "", f.owner, 200)
	key := check("/api-keys", "POST", "/api/v1/api-keys", fmt.Sprintf(`{"name":"contract-reader","project_id":%q,"scopes":["read:CONTRACT_CONTROL"],"environment":"alpha"}`, f.project), f.owner, 201)
	var issued service.CreateAPIKeyResponse
	if err := json.Unmarshal(key.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	check("/api-keys", "GET", "/api/v1/api-keys", "", f.owner, 200)
	check("/api-keys/{id}", "DELETE", "/api/v1/api-keys/"+issued.APIKey.ID.String(), "", f.owner, 204)
	org, err := f.org.Create("Contract approvers", f.owner, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.org.AssignProjectWithAudit(org.ID, f.owner, f.project, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = f.org.AddMember(org.ID, f.owner, f.other, "promoter", ""); err != nil {
		t.Fatal(err)
	}
	check("/projects/{id}/promote", "POST", base+"/promote", `{"source_environment":"alpha","target_environment":"uat","keys":["CONTRACT_CONTROL"],"override_policy":"overwrite"}`, f.owner, 200)
	pending := func() string {
		w := check("/projects/{id}/promote", "POST", base+"/promote", `{"source_environment":"uat","target_environment":"prod","keys":["CONTRACT_CONTROL"],"override_policy":"overwrite"}`, f.owner, 202)
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return base + "/promotions/" + envelope.Promotion.ID.String()
	}
	path := pending()
	check("/projects/{id}/promotions/{promotionId}/approve", "POST", path+"/approve", "", f.owner, 403)
	check("/projects/{id}/promotions/{promotionId}/reject", "POST", path+"/reject", "", f.other, 200)
	path = pending()
	check("/projects/{id}/promotions/{promotionId}/approve", "POST", path+"/approve", "", f.other, 200)
}
