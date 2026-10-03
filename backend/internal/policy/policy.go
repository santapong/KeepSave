// Package policy contains transport-independent authorization contracts.
package policy

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Kind string

const (
	Human           Kind = "human"
	APIKey          Kind = "api_key"
	AgentToken      Kind = "agent_token"
	Workload        Kind = "workload"
	OAuthDelegation Kind = "oauth_delegation"
)

type Principal struct {
	Kind          Kind
	SubjectID     uuid.UUID
	ActorID       uuid.UUID
	TenantID      uuid.UUID
	ParentGrantID uuid.UUID
	ExpiresAt     time.Time
	SessionID     uuid.UUID
	LeaseID       uuid.UUID
	TokenID       string
}

type Action string

const (
	ReadMetadata     Action = "secret.read_metadata"
	ReadValue        Action = "secret.read_value"
	WriteSecret      Action = "secret.write"
	DeleteSecret     Action = "secret.delete"
	RestoreSecret    Action = "secret.restore"
	ApprovePromotion Action = "promotion.approve"
	InvokeTool       Action = "tool.invoke"
	IssueGrant       Action = "grant.issue"
	ManageProject    Action = "project.manage"
)

type Resource struct {
	TenantID    uuid.UUID
	ProjectID   uuid.UUID
	Environment string
	Type        string
	ID          uuid.UUID
	Revision    int64
	Key         string
}

type Decision struct {
	Allowed          bool
	ApprovalRequired bool
	Reason           string
	PolicyRevision   int64
	ValidUntil       time.Time
}

type Authorizer interface {
	Authorize(context.Context, Principal, Action, Resource) (Decision, error)
}

// Authority is resolved by a trusted store, never deserialized from a client.
type Authority struct {
	ActorID     uuid.UUID
	TenantID    uuid.UUID
	Role        string
	ProjectID   uuid.UUID
	Environment string
	Scopes      []string
	ExpiresAt   time.Time
}

type AuthorityStore interface {
	LoadAuthority(context.Context, Principal, Resource) (Authority, error)
}

type Evaluator struct{ Store AuthorityStore }

func (e Evaluator) Authorize(ctx context.Context, p Principal, action Action, r Resource) (Decision, error) {
	deny := Decision{Reason: "authority denied"}
	if e.Store == nil || p.SubjectID == uuid.Nil || r.ProjectID == uuid.Nil {
		return deny, nil
	}
	if (!p.ExpiresAt.IsZero() && !p.ExpiresAt.After(time.Now())) || (p.Kind != Human && p.Kind != APIKey && p.Kind != AgentToken && p.Kind != OAuthDelegation) {
		return deny, nil
	} // workloads need explicit run authority
	if p.Kind == OAuthDelegation && (p.ParentGrantID == uuid.Nil || r.Type != "tool_platform" || (action != ReadMetadata && action != ReadValue && action != IssueGrant)) {
		return deny, nil
	}
	a, err := e.Store.LoadAuthority(ctx, p, r)
	if err != nil {
		return deny, err
	}
	if a.ProjectID != r.ProjectID || (r.TenantID != uuid.Nil && a.TenantID != r.TenantID) ||
		(p.TenantID != uuid.Nil && p.TenantID != a.TenantID) ||
		(p.ActorID != uuid.Nil && p.ActorID != a.ActorID) ||
		(!a.ExpiresAt.IsZero() && !a.ExpiresAt.After(time.Now())) {
		return deny, nil
	}
	required := ""
	legacyAction := ""
	switch action {
	case ReadMetadata:
		required = "viewer"
		legacyAction = "read"
	case ReadValue, IssueGrant:
		required = "editor"
		legacyAction = "read"
	case DeleteSecret:
		required = "editor"
		legacyAction = "delete"
	case WriteSecret, RestoreSecret:
		required = "editor"
		legacyAction = "write"
	case ApprovePromotion:
		required = "promoter"
	case ManageProject:
		required = "admin"
	default:
		return deny, nil // membership never grants broker/tool use
	}
	if !RoleAllows(a.Role, required) {
		return deny, nil
	}
	if p.Kind == AgentToken && action != ReadValue && action != ReadMetadata {
		return deny, nil
	}
	if p.Kind == APIKey || p.Kind == AgentToken {
		allowed := LegacyAllows(a.Scopes, legacyAction, r.Key)
		if r.Type == "secret_collection" {
			allowed = false
			for _, scope := range a.Scopes {
				verb, _, _ := strings.Cut(scope, ":")
				if verb == legacyAction {
					allowed = true
					break
				}
			}
		}
		if legacyAction == "" || (a.Environment != "" && a.Environment != r.Environment) || !allowed {
			return deny, nil
		}
	}
	return Decision{Allowed: true, Reason: "authorized", ValidUntil: a.ExpiresAt}, nil
}

func RoleAllows(role, required string) bool {
	levels := map[string]int{"viewer": 1, "editor": 2, "promoter": 3, "admin": 4}
	a, ok := levels[role]
	b, valid := levels[required]
	return ok && valid && a >= b
}

// LegacyAllows preserves ADR-0022, including write implying delete. Patterns
// have only one metacharacter, '*'; filepath/glob semantics must not be used.
func LegacyAllows(scopes []string, action, key string) bool {
	for _, scope := range scopes {
		a, pattern, _ := strings.Cut(scope, ":")
		if (a == action || (a == "write" && action == "delete")) && MatchKey(pattern, key) {
			return true
		}
	}
	return false
}

func MatchKey(pattern, key string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return key == pattern
	}
	if !strings.HasPrefix(key, parts[0]) {
		return false
	}
	key = key[len(parts[0]):]
	last := parts[len(parts)-1]
	if !strings.HasSuffix(key, last) {
		return false
	}
	key = key[:len(key)-len(last)]
	for _, part := range parts[1 : len(parts)-1] {
		i := strings.Index(key, part)
		if i < 0 {
			return false
		}
		key = key[i+len(part):]
	}
	return true
}

// LeaseKeysAllowed permits an explicit list of literal names or an unbounded
// list only when the parent itself has unbounded read authority.
func LeaseKeysAllowed(scopes []string, keys []string) bool {
	all := false
	for _, s := range scopes {
		if s == "read" || s == "read:*" {
			all = true
		}
	}
	if len(keys) == 0 {
		return all
	}
	for _, key := range keys {
		if key == "" || len(key) > 255 {
			return false
		}
		if strings.Contains(key, "*") {
			if key != "*" || !all {
				return false
			}
			continue
		}
		if !LegacyAllows(scopes, "read", key) {
			return false
		}
	}
	return true
}
