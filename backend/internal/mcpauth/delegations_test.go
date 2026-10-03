package mcpauth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

func TestOwnedDelegationDiscoveryMappingAndRevocation(t *testing.T) {
	f := pgFixture(t)
	ctx := context.Background()
	_, r := f.code(t, "keepsave-hermes-linux-v1")
	issued, e := f.s.Exchange(ctx, r)
	if e != nil {
		t.Fatal(e)
	}
	delegated, e := f.s.Validate(ctx, issued.AccessToken)
	if e != nil {
		t.Fatal(e)
	}
	human := policy.Principal{Kind: policy.Human, SubjectID: f.user, ActorID: f.user, SessionID: f.sid}
	list, e := f.s2.ListDelegations(ctx, human)
	if e != nil || len(list) != 1 || list[0].FamilyID != delegated.FamilyID || list[0].RequiresRefresh {
		t.Fatalf("discovery %#v %v", list, e)
	}
	wire, e := json.Marshal(list)
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{issued.AccessToken, issued.RefreshToken, delegated.TokenID.String(), "token_hash"} {
		if strings.Contains(string(wire), secret) {
			t.Fatal("delegation metadata disclosed token material")
		}
	}
	mapFamily := func(p policy.Principal, client string, want bool) {
		t.Helper()
		tx, e := f.db.BeginTx(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		got, e := f.s.FamilyPrincipalTx(ctx, tx, p, client, delegated.FamilyID)
		if (e == nil) != want {
			t.Fatalf("mapped family=%#v error=%v", got, e)
		}
		if want && (got.Kind != policy.OAuthDelegation || got.SessionID != f.sid || got.ParentGrantID != delegated.FamilyID || got.TokenID != delegated.TokenID.String()) {
			t.Fatal("human was not narrowed to exact stored family")
		}
	}
	mapFamily(human, r.ClientID, true)
	mapFamily(human, "keepsave-codex-linux-v1", false)
	foreign := human
	foreign.SubjectID = uuid.New()
	foreign.ActorID = foreign.SubjectID
	mapFamily(foreign, r.ClientID, false)
	foreign = human
	foreign.Kind = policy.APIKey
	mapFamily(foreign, r.ClientID, false)
	if e = f.s.RevokeDelegation(ctx, foreign, delegated.FamilyID); !errors.Is(e, ErrDenied) {
		t.Fatal("non-human revoked family")
	}
	if _, e = f.db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT synthetic_delegation_audit_failure CHECK(action!='mcp.delegation_revoked') NOT VALID`); e != nil {
		t.Fatal(e)
	}
	if e = f.s.RevokeDelegation(ctx, human, delegated.FamilyID); !errors.Is(e, ErrUnavailable) {
		t.Fatal("revocation ignored audit failure")
	}
	if _, e = f.s.Validate(ctx, issued.AccessToken); e != nil {
		t.Fatal("failed audit still revoked family")
	}
	if _, e = f.db.Exec(`ALTER TABLE audit_log DROP CONSTRAINT synthetic_delegation_audit_failure`); e != nil {
		t.Fatal(e)
	}
	if e = f.s2.RevokeDelegation(ctx, human, delegated.FamilyID); e != nil {
		t.Fatal(e)
	}
	mapFamily(human, r.ClientID, false)
	if _, e = f.s.Validate(ctx, issued.AccessToken); !errors.Is(e, ErrDenied) {
		t.Fatal("owned revocation not current across services")
	}
	list, e = f.s.ListDelegations(ctx, human)
	if e != nil || len(list) != 0 {
		t.Fatal("revoked family remains discoverable")
	}
}
