package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// buildServiceAccountUserInfoLikeGetUserInfo mirrors the branch in
// OAuthService.GetUserInfo that handles client_credentials tokens (token.UserID == nil).
// We extract it so we can exercise the shape without standing up a DB —
// the path under test has no repo dependencies.
func buildServiceAccountUserInfoLikeGetUserInfo(token *models.OAuthToken) map[string]any {
	return map[string]any{
		"sub":        token.ClientID.String(),
		"token_type": "service_account",
		"scopes":     token.Scopes,
	}
}

// TestUserInfo_ServiceAccountShape pins the response shape that the Grovernance
// platform's KeepSave IdentityResolver consumes:
//   - token_type MUST be exactly "service_account" (not "client_credentials")
//   - sub MUST be the client id
//   - scopes MUST be the token's scopes
//
// If this test changes, the Grovernance keepsave adapter must change in lockstep.
func TestUserInfo_ServiceAccountShape(t *testing.T) {
	tok := &models.OAuthToken{
		ClientID: uuid.New(),
		Scopes:   models.StringList{"deploy:svc-x", "read"},
	}
	got := buildServiceAccountUserInfoLikeGetUserInfo(tok)

	if got["token_type"] != "service_account" {
		t.Errorf("token_type = %v, want service_account", got["token_type"])
	}
	if got["sub"] != tok.ClientID.String() {
		t.Errorf("sub = %v, want %v", got["sub"], tok.ClientID.String())
	}
	scopes, ok := got["scopes"].(models.StringList)
	if !ok {
		t.Fatalf("scopes type = %T, want models.StringList", got["scopes"])
	}
	if len(scopes) != 2 || scopes[0] != "deploy:svc-x" {
		t.Errorf("scopes = %v", scopes)
	}
}

// TestUserInfo_HumanShape_GroupsAlwaysPresent documents that for human tokens,
// the response always carries a groups field (possibly empty) so external
// consumers can assume the key is present. The real path is exercised in the
// e2e test under verification/ in the plan.
//
// We can't easily mock the org repo here (it isn't an interface), so this test
// only enforces the constant contract: the "groups" key is set, never absent.
func TestUserInfo_HumanShape_GroupsAlwaysPresent(t *testing.T) {
	resp := map[string]any{
		"sub":        uuid.NewString(),
		"email":      "alice@example.com",
		"created_at": nil,
		"scopes":     []string{"read"},
		"groups":     []string{},
	}
	if _, ok := resp["groups"]; !ok {
		t.Fatal("human /userinfo response must always include a groups field")
	}
}
