package policy

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"
	"time"
)

type authorityFixture struct {
	a   Authority
	err error
}

func (f authorityFixture) LoadAuthority(context.Context, Principal, Resource) (Authority, error) {
	return f.a, f.err
}
func TestAuthorityIntersection(t *testing.T) {
	actor, project, tenant := uuid.New(), uuid.New(), uuid.New()
	base := Authority{ActorID: actor, ProjectID: project, TenantID: tenant, Role: "editor", Scopes: []string{"read:DB_*"}, Environment: "alpha", ExpiresAt: time.Now().Add(time.Minute)}
	p := Principal{Kind: APIKey, SubjectID: uuid.New(), ActorID: actor, TenantID: tenant}
	r := Resource{ProjectID: project, TenantID: tenant, Environment: "alpha", Key: "DB_URL"}
	for _, tc := range []struct {
		name   string
		modify func(*Authority)
		action Action
		want   bool
	}{
		{"read allowed", func(*Authority) {}, ReadValue, true},
		{"read is not write", func(*Authority) {}, WriteSecret, false},
		{"membership is not broker access", func(*Authority) {}, InvokeTool, false},
		{"viewer", func(a *Authority) { a.Role = "viewer" }, ReadValue, false},
		{"unknown role", func(a *Authority) { a.Role = "owner" }, ReadValue, false},
		{"foreign project", func(a *Authority) { a.ProjectID = uuid.New() }, ReadValue, false},
		{"foreign tenant", func(a *Authority) { a.TenantID = uuid.New() }, ReadValue, false},
		{"different actor", func(a *Authority) { a.ActorID = uuid.New() }, ReadValue, false},
		{"wrong environment", func(a *Authority) { a.Environment = "prod" }, ReadValue, false},
		{"wrong key", func(a *Authority) { a.Scopes = []string{"read:OTHER"} }, ReadValue, false},
		{"expired", func(a *Authority) { a.ExpiresAt = time.Now().Add(-time.Second) }, ReadValue, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := base
			tc.modify(&a)
			d, err := (Evaluator{authorityFixture{a: a}}).Authorize(context.Background(), p, tc.action, r)
			if err != nil || d.Allowed != tc.want {
				t.Fatalf("decision %+v %v", d, err)
			}
		})
	}
	if d, err := (Evaluator{authorityFixture{err: errors.New("database unavailable")}}).Authorize(context.Background(), p, ReadValue, r); err == nil || d.Allowed {
		t.Fatal("unavailable authority accepted")
	}
}
func TestLeaseNarrowing(t *testing.T) {
	for _, tc := range []struct {
		scopes, keys []string
		want         bool
	}{
		{[]string{"read:DB_*"}, []string{"DB_URL"}, true},
		{[]string{"read:DB_*"}, []string{"DB_*"}, false},
		{[]string{"read:DB_*"}, nil, false},
		{[]string{"read"}, nil, true},
		{[]string{"write"}, []string{"DB_URL"}, false},
		{[]string{"read:DB_*"}, []string{"DB_URL", "SECRET"}, false},
	} {
		if got := LeaseKeysAllowed(tc.scopes, tc.keys); got != tc.want {
			t.Fatalf("%v %v: %v", tc.scopes, tc.keys, got)
		}
	}
}
func FuzzMatchKey(f *testing.F) {
	f.Add("DB_*_TOKEN", "DB_X_TOKEN")
	f.Add("aaa*aaa", "aaa")
	f.Fuzz(func(t *testing.T, p, k string) { _ = MatchKey(p, k) })
}
