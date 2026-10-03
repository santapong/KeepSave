package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// ErrLeaseNotActive is returned when a mint request references a lease that is
// missing, revoked, or expired. ErrLeaseProjectMismatch is returned when the
// lease exists but belongs to a different project than the caller's scope.
var (
	ErrLeaseNotActive       = errors.New("lease not active")
	ErrLeaseProjectMismatch = errors.New("lease does not belong to project")
)

// AgentTokenService exchanges a JIT lease for a short-lived, lease-bound agent
// token and revokes such tokens (ADR-0021).
type AgentTokenService struct {
	jwt       *auth.JWTService
	leases    *LeaseService
	denylist  *repository.TokenDenylistRepository
	auditRepo *repository.AuditRepository
}

func NewAgentTokenService(jwt *auth.JWTService, leases *LeaseService, denylist *repository.TokenDenylistRepository, auditRepo *repository.AuditRepository) *AgentTokenService {
	return &AgentTokenService{jwt: jwt, leases: leases, denylist: denylist, auditRepo: auditRepo}
}

// MintedAgentToken is the mint endpoint's response payload.
type MintedAgentToken struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	LeaseID   uuid.UUID `json:"lease_id"`
}

// MintToken exchanges an active lease (owned in projectID) for a short-lived
// token bound to it. The token's subject is the lease's principal and its TTL
// can never exceed the lease's remaining lifetime (enforced in auth).
func (s *AgentTokenService) MintToken(actorID, projectID, leaseID uuid.UUID, requestedTTL time.Duration, ip string) (*MintedAgentToken, error) {
	return s.MintTokenAuthorized(context.Background(), policy.Principal{Kind: policy.Human, SubjectID: actorID, ActorID: actorID}, projectID, leaseID, requestedTTL, ip)
}

// RevokeToken scopes the persisted issuance to its project and human owner.
// API-key callers additionally supply their exact key ID; sibling keys cannot
// revoke each other's tokens. Pre-migration tokens can be revoked by lease.
func (s *AgentTokenService) RevokeToken(actorID, projectID uuid.UUID, jti, ip string, callerKey ...uuid.UUID) error {
	p := policy.Principal{Kind: policy.Human, SubjectID: actorID, ActorID: actorID}
	if len(callerKey) > 0 {
		p.Kind, p.SubjectID = policy.APIKey, callerKey[0]
	}
	return s.RevokeTokenAuthorized(context.Background(), p, projectID, jti, ip)
}

var ErrAgentTokenNotFound = errors.New("agent token not found")
