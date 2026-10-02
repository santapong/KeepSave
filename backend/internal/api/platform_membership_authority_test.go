package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// Project provenance is not continuing authority after joining a workspace.
// The original personal owner must obey the workspace's current membership.
func TestPlatformCoreAssignedOwnerUsesCurrentMembership(t *testing.T) {
	f := newCoreFixture(t)
	pr := repository.NewProjectRepository(f.db, f.d)
	before, err := pr.UserHasRole(f.owner, f.project, "admin")
	if err != nil || !before {
		t.Fatal("personal owner lost administrator compatibility", err)
	}
	created := f.request("POST", "/api/v1/organizations", `{"name":"Other owner workspace"}`, f.other, "")
	var response struct {
		Organization struct {
			ID uuid.UUID `json:"id"`
		} `json:"organization"`
	}
	if created.Code != 201 || json.Unmarshal(created.Body.Bytes(), &response) != nil {
		t.Fatal("workspace setup failed", created.Code)
	}
	orgPath := "/api/v1/organizations/" + response.Organization.ID.String()
	memberPath := orgPath + "/members/" + f.owner.String()
	if w := f.request("POST", orgPath+"/members", `{"user_id":"`+f.owner.String()+`","role":"admin"}`, f.other, ""); w.Code != 201 {
		t.Fatal("admin membership setup", w.Code)
	}
	if w := f.request("POST", orgPath+"/projects", `{"project_id":"`+f.project.String()+`"}`, f.owner, ""); w.Code != 200 {
		t.Fatal("assignment", w.Code)
	}
	base := "/api/v1/projects/" + f.project.String()
	if w := f.request("GET", base+"/secrets?environment=alpha", "", f.owner, ""); w.Code != 200 {
		t.Fatal("active admin cannot read", w.Code)
	}
	// A key created in the assigned scope must narrow immediately with membership.
	alpha := "alpha"
	key, _ := f.key([]string{"read"}, &alpha, nil)
	if w := f.request("PUT", memberPath, `{"role":"viewer"}`, f.other, ""); w.Code != 200 {
		t.Fatal("demotion", w.Code)
	}
	if allowed, err := pr.UserHasRole(f.owner, f.project, "viewer"); err != nil || !allowed {
		t.Fatal("viewer metadata authority", err)
	}
	if w := f.request("GET", base, "", f.owner, ""); w.Code != 200 {
		t.Fatal("viewer project metadata", w.Code)
	}
	if allowed, err := pr.UserHasRole(f.owner, f.project, "editor"); err != nil || allowed {
		t.Fatal("stored owner bypassed viewer role", err)
	}
	for _, item := range []struct {
		user uuid.UUID
		key  string
	}{{f.owner, ""}, {uuid.Nil, key}} {
		if w := f.request("GET", base+"/secrets?environment=alpha", "", item.user, item.key); w.Code != 403 {
			t.Fatal("demoted owner credential read", w.Code)
		}
	}
	claims, err := f.jwt.ValidateToken(f.tokens[f.owner])
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	principal := policy.Principal{Kind: policy.Human, SubjectID: f.owner, ActorID: f.owner, SessionID: sessionID}
	decision, err := (policy.Evaluator{Store: repository.AuthorityStore{DB: f.db, Dialect: f.d, RequireHumanSession: true}}).Authorize(context.Background(), principal, policy.ReadValue, policy.Resource{ProjectID: f.project})
	if err != nil || decision.Allowed {
		t.Fatal("service authority bypassed workspace viewer", err)
	}
	if w := f.request("DELETE", memberPath, "", f.other, ""); w.Code != 204 {
		t.Fatal("membership removal", w.Code)
	}
	if allowed, err := pr.UserHasAccess(f.owner, f.project); err != nil || allowed {
		t.Fatal("removed owner retained metadata access", err)
	}
	if w := f.request("GET", base, "", f.owner, ""); w.Code != 403 {
		t.Fatal("removed owner project metadata", w.Code)
	}
	ids, err := pr.ListAccessibleProjectIDs(f.owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if id == f.project {
			t.Fatal("removed owner retained accessible project")
		}
	}
	for _, item := range []struct {
		user uuid.UUID
		key  string
	}{{f.owner, ""}, {uuid.Nil, key}} {
		if w := f.request("GET", base+"/secrets?environment=alpha", "", item.user, item.key); w.Code != 403 {
			t.Fatal("removed owner credential read", w.Code)
		}
	}
	if w := f.request("POST", orgPath+"/projects", `{"project_id":"`+f.project.String()+`"}`, f.owner, ""); w.Code != 403 {
		t.Fatal("removed member assignment retry", w.Code)
	}
	if allowed, err := pr.UserHasRole(f.other, f.foreign, "admin"); err != nil || !allowed {
		t.Fatal("unassigned personal project changed", err)
	}
}
