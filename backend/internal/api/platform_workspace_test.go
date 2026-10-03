package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

func workspaceRouter(t *testing.T) (*platformFixture, *repository.OrganizationRepository, *repository.AuditRepository) {
	t.Helper()
	f := newPlatformFixture(t)
	orgs := repository.NewOrganizationRepository(f.db, f.d)
	audit := repository.NewAuditRepository(f.db, f.d)
	audit.SetChainKey([]byte("synthetic-workspace-chain-key-32bytes"))
	h := NewOrganizationHandler(service.NewOrganizationService(orgs, audit))
	r := gin.New()
	g := r.Group("/api/v1/organizations", JWTAuthMiddleware(f.jwt))
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:orgId", h.Get)
	g.PUT("/:orgId", h.Update)
	g.DELETE("/:orgId", h.Delete)
	g.POST("/:orgId/members", h.AddMember)
	g.PUT("/:orgId/members/:userId", h.UpdateMemberRole)
	g.DELETE("/:orgId/members/:userId", h.RemoveMember)
	g.POST("/:orgId/projects", h.AssignProject)
	g.GET("/:orgId/projects", h.ListProjects)
	f.r = r
	return f, orgs, audit
}

func workspaceRequest(t *testing.T, f *platformFixture, user uuid.UUID, body, key string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := f.jwt.GenerateToken(user, "synthetic-workspace@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/v1/organizations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	f.r.ServeHTTP(w, req)
	return w
}

func workspaceResult(t *testing.T, w *httptest.ResponseRecorder) models.Organization {
	t.Helper()
	if w.Code != 201 {
		t.Fatalf("workspace creation status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Organization models.Organization `json:"organization"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Organization
}

func assertWorkspaceAudit(t *testing.T, f *platformFixture, action string, actor uuid.UUID, want int) {
	t.Helper()
	var count int
	if err := f.db.QueryRow(repository.Q(f.d, `SELECT COUNT(*) FROM audit_log WHERE action=$1 AND user_id=$2`), action, actor).Scan(&count); err != nil || count != want {
		t.Fatalf("audit %s count=%d want=%d err=%v", action, count, want, err)
	}
}

// Exercise anonymous SQL bindings as well as PostgreSQL's indexed parameters.
// Project provenance stops granting authority after organization assignment.
func TestPlatformWorkspaceAssignedProjectRequiresCurrentMembership(t *testing.T) {
	f, orgs, _ := workspaceRouter(t)
	org := workspaceResult(t, workspaceRequest(t, f, f.other, `{"name":"Membership authority"}`, uuid.NewString()))
	if _, err := orgs.AddMember(org.ID, f.owner, "admin"); err != nil {
		t.Fatal(err)
	}
	if w := f.call("POST", "/api/v1/organizations/"+org.ID.String()+"/projects", fmt.Sprintf(`{"project_id":"%s"}`, f.project), "", f.owner); w.Code != 200 {
		t.Fatal("assignment", w.Code)
	}
	projects := repository.NewProjectRepository(f.db, f.d)
	listed, err := projects.ListByOwnerID(f.owner)
	if err != nil || len(listed) != 1 || listed[0].ID != f.project {
		t.Fatal("assigned owner listing", err)
	}
	if w := f.call("DELETE", "/api/v1/organizations/"+org.ID.String()+"/members/"+f.owner.String(), "", "", f.other); w.Code != 204 {
		t.Fatal("removal", w.Code)
	}
	if listed, err = projects.ListByOwnerID(f.owner); err != nil || len(listed) != 0 {
		t.Fatal("removed owner listing", err)
	}
	if allowed, err := projects.UserHasAccess(f.owner, f.project); err != nil || allowed {
		t.Fatal("removed owner metadata access", err)
	}
	if allowed, err := projects.UserHasRole(f.owner, f.project, "admin"); err != nil || allowed {
		t.Fatal("removed owner administrator access", err)
	}
	ids, err := projects.ListAccessibleProjectIDs(f.owner)
	if err != nil || len(ids) != 0 {
		t.Fatal("removed owner accessible IDs", err)
	}
	decision, err := (policy.Evaluator{Store: repository.AuthorityStore{DB: f.db, Dialect: f.d}}).Authorize(context.Background(), policy.Principal{Kind: policy.Human, SubjectID: f.owner, ActorID: f.owner}, policy.ReadValue, policy.Resource{ProjectID: f.project})
	if err != nil || decision.Allowed {
		t.Fatal("removed owner policy authority", err)
	}
	if allowed, err := projects.UserHasRole(f.other, f.foreign, "admin"); err != nil || !allowed {
		t.Fatal("personal owner compatibility", err)
	}
}

func TestPlatformWorkspaceCreationIsAtomicAndIdempotent(t *testing.T) {
	f, orgs, audit := workspaceRouter(t)
	var projectsBefore, secretsBefore int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&projectsBefore); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM secrets`).Scan(&secretsBefore); err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	first := workspaceResult(t, workspaceRequest(t, f, f.owner, `{"name":"My workspace"}`, key))
	again := workspaceResult(t, workspaceRequest(t, f, f.owner, `{"name":"My workspace"}`, key))
	if first.ID != again.ID || first.OwnerID != f.owner || first.Slug == "my-workspace" {
		t.Fatal("workspace creation lost stable result/owner or omitted unique slug")
	}
	member, err := orgs.GetMember(first.ID, f.owner)
	if err != nil || member.Role != "admin" {
		t.Fatalf("owner membership: %+v %v", member, err)
	}
	if w := workspaceRequest(t, f, f.owner, `{"name":"Changed request"}`, key); w.Code != 409 {
		t.Fatalf("changed idempotency request status=%d", w.Code)
	}
	other := workspaceResult(t, workspaceRequest(t, f, f.other, `{"name":"My workspace"}`, key))
	if other.ID == first.ID || other.Slug == first.Slug {
		t.Fatal("creation crossed caller scope or same-name workspace collision")
	}
	for _, tc := range []struct{ body, key string }{
		{`{"name":" "}`, ""}, {`{"name":"ok"}`, "invalid key"},
	} {
		if w := workspaceRequest(t, f, f.owner, tc.body, tc.key); w.Code != 400 {
			t.Fatalf("invalid creation status=%d", w.Code)
		}
	}
	var projectsAfter, secretsAfter int
	if err = f.db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&projectsAfter); err != nil {
		t.Fatal(err)
	}
	if err = f.db.QueryRow(`SELECT COUNT(*) FROM secrets`).Scan(&secretsAfter); err != nil {
		t.Fatal(err)
	}
	if projectsAfter != projectsBefore || secretsAfter != secretsBefore {
		t.Fatal("workspace creation made projects or secrets")
	}
	assertWorkspaceAudit(t, f, "org.created", f.owner, 1)
	assertWorkspaceAudit(t, f, "org.created", f.other, 1)
	if f.d.DBType() == repository.DBTypePostgres {
		var jobs int
		if err = f.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='org.event'`).Scan(&jobs); err != nil || jobs != 2 {
			t.Fatalf("creation jobs=%d err=%v", jobs, err)
		}
	}
	if broken, err := audit.VerifyChain(); err != nil || broken != nil {
		t.Fatalf("creation audit chain=%v %v", broken, err)
	}
}

func TestPlatformWorkspaceCreateRollsBackRequiredAudit(t *testing.T) {
	f, _, _ := workspaceRouter(t)
	// Rename the required audit table to force a failure after the domain writes.
	if _, err := f.db.Exec(`ALTER TABLE audit_log RENAME TO unavailable_audit_log`); err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	if w := workspaceRequest(t, f, f.owner, `{"name":"Rollback fixture"}`, key); w.Code != 500 {
		t.Fatalf("audit failure status=%d", w.Code)
	}
	for _, table := range []string{"organizations", "organization_members", "workspace_creation_requests"} {
		var count int
		if err := f.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial %s count=%d err=%v", table, count, err)
		}
	}
	if f.d.DBType() == repository.DBTypePostgres {
		var count int
		if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='org.event'`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("misleading outbox count=%d err=%v", count, err)
		}
	}
	if _, err := f.db.Exec(`ALTER TABLE unavailable_audit_log RENAME TO audit_log`); err != nil {
		t.Fatal(err)
	}
	workspaceResult(t, workspaceRequest(t, f, f.owner, `{"name":"Rollback fixture"}`, key))
	assertWorkspaceAudit(t, f, "org.created", f.owner, 1)
}

func TestPlatformWorkspaceMembershipAndAssignment(t *testing.T) {
	f, orgs, audit := workspaceRouter(t)
	first := workspaceResult(t, workspaceRequest(t, f, f.owner, `{"name":"Workspace A"}`, uuid.NewString()))
	second := workspaceResult(t, workspaceRequest(t, f, f.owner, `{"name":"Workspace B"}`, uuid.NewString()))
	if _, err := orgs.AddMember(first.ID, f.other, "admin"); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"POST", "PUT"} {
		path := "/api/v1/organizations/" + first.ID.String() + "/members"
		body := fmt.Sprintf(`{"user_id":"%s","role":"viewer"}`, f.owner)
		if method == "PUT" {
			path += "/" + f.owner.String()
			body = `{"role":"viewer"}`
		}
		if w := f.call(method, path, body, "", f.other); w.Code != 403 {
			t.Fatalf("owner demotion %s status=%d", method, w.Code)
		}
	}
	if w := f.call("DELETE", "/api/v1/organizations/"+first.ID.String()+"/members/"+f.owner.String(), "", "", f.other); w.Code != 403 {
		t.Fatalf("owner removal status=%d", w.Code)
	}
	member, err := orgs.GetMember(first.ID, f.owner)
	if err != nil || member.Role != "admin" {
		t.Fatal("owner authority changed", err)
	}
	assign := "/api/v1/organizations/" + first.ID.String() + "/projects"
	if w := f.call("POST", assign, fmt.Sprintf(`{"project_id":"%s"}`, f.foreign), "", f.owner); w.Code != 403 {
		t.Fatalf("foreign project attach status=%d", w.Code)
	}
	var foreignOrg sql.NullString
	if err = f.db.QueryRow(repository.Q(f.d, `SELECT organization_id FROM projects WHERE id=$1`), f.foreign).Scan(&foreignOrg); err != nil || foreignOrg.Valid {
		t.Fatal("foreign project was moved", err)
	}
	_, keyID := f.key([]string{"read"}, nil, nil)
	body := fmt.Sprintf(`{"project_id":"%s"}`, f.project)
	if w := f.call("POST", assign, body, "", f.owner); w.Code != 200 {
		t.Fatalf("personal assignment status=%d %s", w.Code, w.Body.String())
	}
	if w := f.call("POST", assign, body, "", f.owner); w.Code != 200 {
		t.Fatalf("same workspace retry status=%d", w.Code)
	}
	if w := f.call("POST", "/api/v1/organizations/"+second.ID.String()+"/projects", body, "", f.owner); w.Code != 409 {
		t.Fatalf("cross-workspace transfer status=%d", w.Code)
	}
	if _, err = f.keys.GetByID(keyID); err == nil {
		t.Fatal("source-scope API key survived assignment")
	}
	if w := f.call("DELETE", "/api/v1/organizations/"+first.ID.String(), "", "", f.owner); w.Code != 409 {
		t.Fatalf("nonempty workspace delete status=%d", w.Code)
	}
	assertWorkspaceAudit(t, f, "org.project_assigned", f.owner, 1)
	assertWorkspaceAudit(t, f, "org.member_added", f.other, 0)
	assertWorkspaceAudit(t, f, "org.deleted", f.owner, 0)
	// Assignment and lists refuse an archived project as well as an active foreign one.
	if _, err = f.db.Exec(repository.Q(f.d, `UPDATE projects SET deleted_at=`+f.d.Now()+` WHERE id=$1`), f.project); err != nil {
		t.Fatal(err)
	}
	if w := f.call("POST", assign, body, "", f.owner); w.Code != 403 {
		t.Fatalf("archived assignment status=%d", w.Code)
	}
	listed, err := orgs.ListProjectsByOrg(first.ID)
	if err != nil || len(listed) != 0 {
		t.Fatalf("archived project listed: %d %v", len(listed), err)
	}
	if w := f.call("DELETE", "/api/v1/organizations/"+first.ID.String(), "", "", f.owner); w.Code != 409 {
		t.Fatalf("workspace detached retained project status=%d", w.Code)
	}
	if broken, err := audit.VerifyChain(); err != nil || broken != nil {
		t.Fatalf("assignment audit chain=%v %v", broken, err)
	}
}

func TestPlatformWorkspaceEmptyDeleteDoesNotResurrect(t *testing.T) {
	f, _, audit := workspaceRouter(t)
	key := uuid.NewString()
	org := workspaceResult(t, workspaceRequest(t, f, f.owner, `{"name":"Empty workspace"}`, key))
	if w := f.call("DELETE", "/api/v1/organizations/"+org.ID.String(), "", "", f.owner); w.Code != 204 {
		t.Fatalf("empty delete status=%d", w.Code)
	}
	if w := workspaceRequest(t, f, f.owner, `{"name":"Empty workspace"}`, key); w.Code != 409 {
		t.Fatalf("deleted workspace replay status=%d", w.Code)
	}
	assertWorkspaceAudit(t, f, "org.deleted", f.owner, 1)
	if broken, err := audit.VerifyChain(); err != nil || broken != nil {
		t.Fatalf("deletion audit chain=%v %v", broken, err)
	}
}

func TestPlatformWorkspaceAssignmentRollsBackRequiredAudit(t *testing.T) {
	f, _, audit := workspaceRouter(t)
	org := workspaceResult(t, workspaceRequest(t, f, f.owner, `{"name":"Assignment rollback"}`, uuid.NewString()))
	_, keyID := f.key([]string{"read"}, nil, nil)
	if _, err := f.db.Exec(`ALTER TABLE audit_log RENAME TO unavailable_audit_log`); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"project_id":"%s"}`, f.project)
	if w := f.call("POST", "/api/v1/organizations/"+org.ID.String()+"/projects", body, "", f.owner); w.Code != 500 {
		t.Fatalf("assignment audit failure status=%d", w.Code)
	}
	var organization sql.NullString
	if err := f.db.QueryRow(repository.Q(f.d, `SELECT organization_id FROM projects WHERE id=$1`), f.project).Scan(&organization); err != nil || organization.Valid {
		t.Fatal("failed assignment moved project", err)
	}
	if _, err := f.keys.GetByID(keyID); err != nil {
		t.Fatal("failed assignment revoked parent key", err)
	}
	if _, err := f.db.Exec(`ALTER TABLE unavailable_audit_log RENAME TO audit_log`); err != nil {
		t.Fatal(err)
	}
	assertWorkspaceAudit(t, f, "org.project_assigned", f.owner, 0)
	if broken, err := audit.VerifyChain(); err != nil || broken != nil {
		t.Fatalf("rolled-back assignment audit chain=%v %v", broken, err)
	}
}

func TestPlatformWorkspaceViewerCannotMutate(t *testing.T) {
	f, orgs, _ := workspaceRouter(t)
	org := workspaceResult(t, workspaceRequest(t, f, f.owner, `{"name":"Viewer fixture"}`, uuid.NewString()))
	if _, err := orgs.AddMember(org.ID, f.other, "viewer"); err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/organizations/" + org.ID.String()
	for _, tc := range []struct{ method, path, body string }{
		{"PUT", base, `{"name":"Denied"}`},
		{"DELETE", base, ""},
		{"POST", base + "/projects", fmt.Sprintf(`{"project_id":"%s"}`, f.foreign)},
		{"POST", base + "/members", fmt.Sprintf(`{"user_id":"%s","role":"admin"}`, f.other)},
		{"PUT", base + "/members/" + f.other.String(), `{"role":"admin"}`},
		{"DELETE", base + "/members/" + f.other.String(), ""},
	} {
		if w := f.call(tc.method, tc.path, tc.body, "", f.other); w.Code != 403 {
			t.Fatalf("viewer %s %s status=%d", tc.method, tc.path, w.Code)
		}
	}
	member, err := orgs.GetMember(org.ID, f.other)
	if err != nil || member.Role != "viewer" {
		t.Fatal("viewer escalated or removed membership", err)
	}
	assertWorkspaceAudit(t, f, "org.updated", f.other, 0)
	assertWorkspaceAudit(t, f, "org.project_assigned", f.other, 0)
	assertWorkspaceAudit(t, f, "org.member_added", f.other, 0)
	assertWorkspaceAudit(t, f, "org.member_role_updated", f.other, 0)
	assertWorkspaceAudit(t, f, "org.member_removed", f.other, 0)
	assertWorkspaceAudit(t, f, "org.deleted", f.other, 0)
}

func TestPlatformWorkspaceConcurrentCreate(t *testing.T) {
	f, _, _ := workspaceRouter(t)
	if f.d.DBType() != repository.DBTypePostgres {
		t.Skip("PostgreSQL concurrency contract")
	}
	key := uuid.NewString()
	token, err := f.jwt.GenerateToken(f.owner, "concurrency@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	responses := make([]*httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for i := range responses {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/api/v1/organizations", strings.NewReader(`{"name":"Concurrent workspace"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Idempotency-Key", key)
			responses[index] = httptest.NewRecorder()
			f.r.ServeHTTP(responses[index], req)
		}(i)
	}
	wg.Wait()
	first, second := workspaceResult(t, responses[0]), workspaceResult(t, responses[1])
	if first.ID != second.ID {
		t.Fatal("concurrent retry created duplicate workspaces")
	}
	assertWorkspaceAudit(t, f, "org.created", f.owner, 1)
}
