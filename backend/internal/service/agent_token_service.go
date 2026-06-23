package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/models"
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
	lease, err := s.leases.GetActiveLease(leaseID)
	if err != nil {
		// GetActiveLease only returns non-revoked, non-expired rows; anything
		// else (missing/revoked/expired) is "not active" — do not leak which.
		return nil, ErrLeaseNotActive
	}
	if lease.ProjectID != projectID {
		return nil, ErrLeaseProjectMismatch
	}
	remaining := time.Until(lease.ExpiresAt)
	if remaining <= 0 {
		return nil, ErrLeaseNotActive
	}

	token, jti, expiresAt, err := s.jwt.GenerateAgentToken(lease.APIKeyID, "", leaseID, requestedTTL, remaining)
	if err != nil {
		return nil, fmt.Errorf("minting agent token: %w", err)
	}

	emitAudit(s.auditRepo, &actorID, &projectID, "agent.token.minted", lease.Environment,
		models.JSONMap{
			"project_id": projectID.String(),
			"lease_id":   leaseID.String(),
			"jti":        jti,
			"expires_at": expiresAt.UTC().Format(time.RFC3339),
		}, ip)

	return &MintedAgentToken{Token: token, TokenType: "agent", ExpiresAt: expiresAt, LeaseID: leaseID}, nil
}

// RevokeToken adds an agent token's jti to the denylist (explicit single-token
// revocation). expires_at on the denylist row is set conservatively to the max
// agent-token lifetime so the row prunes shortly after the token would expire.
func (s *AgentTokenService) RevokeToken(actorID, projectID uuid.UUID, jti, ip string) error {
	if jti == "" {
		return errors.New("jti required")
	}
	// Upper bound: a token cannot outlive maxAgentTokenTTL, so a row added now
	// is safe to prune after that window even if the original expiry was sooner.
	pruneAfter := time.Now().Add(auth.MaxAgentTokenTTL())
	if err := s.denylist.Revoke(jti, nil, pruneAfter, "revoked via api"); err != nil {
		return fmt.Errorf("revoking agent token: %w", err)
	}
	emitAudit(s.auditRepo, &actorID, &projectID, "agent.token.revoked", "",
		models.JSONMap{"project_id": projectID.String(), "jti": jti}, ip)
	return nil
}
