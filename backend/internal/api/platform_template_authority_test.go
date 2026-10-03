package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

const templateFixtureBody = `{"name":"Synthetic review","description":"Fixture","stack":"nodejs","keys":{"keys":[{"key":"TEMPLATE_FIXTURE","default_value":"synthetic"}]}}`

func templateResponseID(t *testing.T, body []byte) uuid.UUID {
	t.Helper()
	var result struct {
		Template models.SecretTemplate `json:"template"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.Template.ID == uuid.Nil {
		t.Fatal("missing template ID")
	}
	return result.Template.ID
}

func TestPlatformCoreTemplateCurrentMembershipAndManagement(t *testing.T) {
	f := newCoreFixture(t)
	created := f.request("POST", "/api/v1/organizations", `{"name":"Template workspace"}`, f.other, "")
	var org struct {
		Organization models.Organization `json:"organization"`
	}
	if created.Code != 201 || json.Unmarshal(created.Body.Bytes(), &org) != nil {
		t.Fatal("workspace setup", created.Code)
	}
	orgPath := "/api/v1/organizations/" + org.Organization.ID.String()
	memberPath := orgPath + "/members/" + f.owner.String()
	if w := f.request("POST", orgPath+"/members", `{"user_id":"`+f.owner.String()+`","role":"admin"}`, f.other, ""); w.Code != 201 {
		t.Fatal("membership setup", w.Code)
	}
	orgBody := strings.TrimSuffix(templateFixtureBody, "}") + `,"organization_id":"` + org.Organization.ID.String() + `"}`
	w := f.request("POST", "/api/v1/templates", orgBody, f.owner, "")
	assertCoreResponse(t, "/templates", "POST", w, 201)
	id := templateResponseID(t, w.Body.Bytes())
	path := "/api/v1/templates/" + id.String()
	assertCoreResponse(t, "/templates/{templateId}", "PUT", f.request("PUT", path, templateFixtureBody, f.other, ""), 200)
	assertCoreResponse(t, "/templates/builtin", "GET", f.request("GET", "/api/v1/templates/builtin", "", f.owner, ""), 200)
	if w := f.request("PUT", memberPath, `{"role":"viewer"}`, f.other, ""); w.Code != 200 {
		t.Fatal("demotion", w.Code)
	}
	assertCoreResponse(t, "/templates/{templateId}", "GET", f.request("GET", path, "", f.owner, ""), 200)
	assertCoreResponse(t, "/templates", "GET", f.request("GET", "/api/v1/templates?organization_id="+org.Organization.ID.String(), "", f.owner, ""), 200)
	assertCoreResponse(t, "/templates/{templateId}", "PUT", f.request("PUT", path, templateFixtureBody, f.owner, ""), 404)
	assertCoreResponse(t, "/templates/{templateId}", "DELETE", f.request("DELETE", path, "", f.owner, ""), 404)
	assertCoreResponse(t, "/templates", "POST", f.request("POST", "/api/v1/templates", orgBody, f.owner, ""), 403)
	applyBody := `{"project_id":"` + f.project.String() + `","environment":"alpha"}`
	assertCoreResponse(t, "/templates/{templateId}/apply", "POST", f.request("POST", path+"/apply", applyBody, f.owner, ""), 201)
	if w := f.request("DELETE", memberPath, "", f.other, ""); w.Code != 204 {
		t.Fatal("removal", w.Code)
	}
	assertCoreResponse(t, "/templates", "GET", f.request("GET", "/api/v1/templates?organization_id="+org.Organization.ID.String(), "", f.owner, ""), 403)
	assertCoreResponse(t, "/templates/{templateId}", "GET", f.request("GET", path, "", f.owner, ""), 404)
	assertCoreResponse(t, "/templates/{templateId}/apply", "POST", f.request("POST", path+"/apply", applyBody, f.owner, ""), 404)
	assertCoreResponse(t, "/templates/{templateId}", "PUT", f.request("PUT", path, templateFixtureBody, f.owner, ""), 404)
	assertCoreResponse(t, "/templates/{templateId}", "DELETE", f.request("DELETE", path, "", f.owner, ""), 404)
	w = f.request("GET", "/api/v1/templates", "", f.owner, "")
	assertCoreResponse(t, "/templates", "GET", w, 200)
	if strings.Contains(w.Body.String(), id.String()) {
		t.Fatal("removed creator retained template listing")
	}
	assertCoreResponse(t, "/templates", "POST", f.request("POST", "/api/v1/templates", orgBody, f.owner, ""), 403)
	globalBody := strings.TrimSuffix(templateFixtureBody, "}") + `,"is_global":true}`
	assertCoreResponse(t, "/templates", "POST", f.request("POST", "/api/v1/templates", globalBody, f.owner, ""), 403)
	w = f.request("POST", "/api/v1/templates", templateFixtureBody, f.owner, "")
	assertCoreResponse(t, "/templates", "POST", w, 201)
	personal := "/api/v1/templates/" + templateResponseID(t, w.Body.Bytes()).String()
	assertCoreResponse(t, "/templates/{templateId}", "GET", f.request("GET", personal, "", f.other, ""), 404)
	assertCoreResponse(t, "/templates/{templateId}", "PUT", f.request("PUT", personal, templateFixtureBody, f.other, ""), 404)
	assertCoreResponse(t, "/templates/{templateId}", "DELETE", f.request("DELETE", personal, "", f.other, ""), 404)
	assertCoreResponse(t, "/templates/{templateId}", "DELETE", f.request("DELETE", path, "", f.other, ""), 204)
}

func TestPlatformCoreTemplateAuditFailureRollsBackEveryMutation(t *testing.T) {
	f := newCoreFixture(t)
	w := f.request("POST", "/api/v1/templates", templateFixtureBody, f.owner, "")
	assertCoreResponse(t, "/templates", "POST", w, 201)
	id := templateResponseID(t, w.Body.Bytes())
	path := "/api/v1/templates/" + id.String()
	var before int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='template.event'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT fail_template_mutation CHECK(action NOT LIKE 'template.%') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	assertCoreResponse(t, "/templates", "POST", f.request("POST", "/api/v1/templates", templateFixtureBody, f.owner, ""), 500)
	updatedBody := strings.Replace(templateFixtureBody, "Synthetic review", "Must not commit", 1)
	assertCoreResponse(t, "/templates/{templateId}", "PUT", f.request("PUT", path, updatedBody, f.owner, ""), 500)
	assertCoreResponse(t, "/templates/{templateId}", "DELETE", f.request("DELETE", path, "", f.owner, ""), 500)
	var count, after int
	var name string
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM secret_templates`).Scan(&count); err != nil || count != 1 {
		t.Fatal("creation/deletion escaped audit transaction", err)
	}
	if err := f.db.QueryRow(`SELECT name FROM secret_templates WHERE id=$1`, id).Scan(&name); err != nil || name != "Synthetic review" {
		t.Fatal("update escaped audit transaction", err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='template.event'`).Scan(&after); err != nil || after != before {
		t.Fatal("failed mutation emitted outbox event", err)
	}
	if _, err := f.db.Exec(`ALTER TABLE audit_log DROP CONSTRAINT fail_template_mutation`); err != nil {
		t.Fatal(err)
	}
	assertCoreResponse(t, "/templates/{templateId}", "PUT", f.request("PUT", path, templateFixtureBody, f.owner, ""), 200)
	assertCoreResponse(t, "/templates/{templateId}", "DELETE", f.request("DELETE", path, "", f.owner, ""), 204)
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action IN ('template.created','template.updated','template.deleted')`).Scan(&count); err != nil || count != 3 {
		t.Fatal("required mutation audit missing", err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='template.event'`).Scan(&after); err != nil || after != before+2 {
		t.Fatal("required outbox missing", err)
	}
}

func TestPlatformCoreTemplateSessionRevokedAfterAdmission(t *testing.T) {
	f := newCoreFixture(t)
	claims, err := f.jwt.ValidateToken(f.tokens[f.owner])
	if err != nil {
		t.Fatal(err)
	}
	sid, err := uuid.Parse(claims.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	p := policy.Principal{Kind: policy.Human, SubjectID: f.owner, ActorID: f.owner, SessionID: sid}
	ar := repository.NewAuditRepository(f.db, f.d)
	svc := service.NewTemplateService(repository.NewTemplateRepository(f.db, f.d), nil, nil, nil, ar, nil)
	svc.EnableSessions(service.NewSessionService(f.db, f.d, f.jwt, ar))
	if w := f.request("POST", "/api/v1/auth/logout", "", f.owner, ""); w.Code != 204 {
		t.Fatal("logout", w.Code)
	}
	_, err = svc.CreateAuthorized(context.Background(), p, "Denied", "", "nodejs", models.JSONMap{"keys": []any{}}, nil, false, "")
	if !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatal("stale admitted identity authorized template write", err)
	}
	var count int
	if err = f.db.QueryRow(`SELECT COUNT(*) FROM secret_templates`).Scan(&count); err != nil || count != 0 {
		t.Fatal("revoked session mutated templates", err)
	}
}
