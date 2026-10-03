package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

func (s *AgentTokenService) MintTokenAuthorized(ctx context.Context, p policy.Principal, project, id uuid.UUID, requestedTTL time.Duration, ip string) (*MintedAgentToken, error) {
	if s.auditRepo == nil || s.jwt == nil {
		return nil, errors.New("agent token issuer unavailable")
	}
	if p.Kind != policy.Human && p.Kind != policy.APIKey {
		return nil, ErrLeaseAuthority
	}
	var result *MintedAgentToken
	err := s.leases.withGrantTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		lease, err := s.leases.leaseTx(ctx, tx, id, false)
		if err != nil || lease.Revoked || !lease.ExpiresAt.After(time.Now()) {
			return ErrLeaseNotActive
		}
		if lease.ProjectID != project {
			return ErrLeaseProjectMismatch
		}
		parent, err := s.leases.parentTx(ctx, tx, lease.APIKeyID, false)
		if err != nil || p.ActorID != uuid.Nil && p.ActorID != parent.UserID || p.Kind == policy.Human && p.SubjectID != parent.UserID || p.Kind == policy.APIKey && p.SubjectID != parent.ID {
			return ErrLeaseAuthority
		}
		p.ActorID = parent.UserID
		if err = s.leases.lockGrantActorTx(ctx, tx, p, project); err != nil {
			return err
		}
		parent, err = s.leases.parentTx(ctx, tx, parent.ID, true)
		if err != nil || parent.UserID != p.ActorID {
			return ErrLeaseAuthority
		}
		lease, err = s.leases.leaseTx(ctx, tx, id, true)
		if err != nil || lease.Revoked || !lease.ExpiresAt.After(time.Now()) {
			return ErrLeaseNotActive
		}
		if lease.ProjectID != project || lease.APIKeyID != parent.ID || !validLeaseParent(parent, project, lease.Environment, lease.SecretKeys, lease.ExpiresAt) {
			return ErrLeaseAuthority
		}
		if err = s.leases.authorizeGrantTx(ctx, tx, p, project, lease.Environment, lease.SecretKeys, false); err != nil {
			return err
		}
		// A human cannot mint broader authority than the stored parent key.
		parentPrincipal := policy.Principal{Kind: policy.APIKey, SubjectID: parent.ID, ActorID: parent.UserID}
		if err = s.leases.authorizeGrantTx(ctx, tx, parentPrincipal, project, lease.Environment, lease.SecretKeys, false); err != nil {
			return err
		}
		token, jti, expires, err := s.jwt.GenerateAgentToken(parent.UserID, "", id, project, lease.Environment, []string(lease.SecretKeys), requestedTTL, time.Until(lease.ExpiresAt))
		if err != nil {
			return err
		}
		// The persisted authoritative expiry also removes subsecond drift
		// between calculating remaining lifetime and signing the JWT.
		if expires.After(lease.ExpiresAt) {
			expires = lease.ExpiresAt
		}
		query := `INSERT INTO agent_token_issuance(jti,lease_id,project_id,user_id,expires_at) VALUES($1,$2,$3,$4,$5)`
		if _, err = tx.ExecContext(ctx, repository.Q(s.leases.dialect, query), jti, id, project, parent.UserID, expires); err != nil {
			return err
		}
		if err = s.auditRepo.CreateTx(tx, &parent.UserID, &project, "agent.token.minted", lease.Environment, models.JSONMap{"project_id": project.String(), "lease_id": id.String(), "jti": jti, "expires_at": expires.UTC().Format(time.RFC3339)}, ip); err != nil {
			return err
		}
		if err = identityEventTx(ctx, tx, s.leases.dialect, parent.UserID, "agent.token.minted", map[string]string{"project_id": project.String(), "lease_id": id.String(), "jti": jti}); err != nil {
			return err
		}
		result = &MintedAgentToken{Token: token, TokenType: "agent", ExpiresAt: expires, LeaseID: id}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *AgentTokenService) RevokeTokenAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, jti, ip string) error {
	if s.auditRepo == nil {
		return errors.New("agent token audit unavailable")
	}
	if jti == "" || p.ActorID == uuid.Nil || p.Kind != policy.Human && p.Kind != policy.APIKey {
		return ErrAgentTokenNotFound
	}
	return s.leases.withGrantTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var leaseID uuid.UUID
		query := `SELECT lease_id FROM agent_token_issuance WHERE jti=$1 AND project_id=$2 AND user_id=$3`
		if err := tx.QueryRowContext(ctx, repository.Q(s.leases.dialect, query), jti, project, p.ActorID).Scan(&leaseID); err != nil {
			return ErrAgentTokenNotFound
		}
		lease, err := s.leases.leaseTx(ctx, tx, leaseID, false)
		if err != nil || lease.ProjectID != project || p.Kind == policy.APIKey && p.SubjectID != lease.APIKeyID {
			return ErrAgentTokenNotFound
		}
		if err = s.leases.lockGrantActorTx(ctx, tx, p, project); err != nil {
			return err
		}
		parent, err := s.leases.parentTx(ctx, tx, lease.APIKeyID, true)
		if err != nil || parent.ProjectID != project || parent.UserID != p.ActorID {
			return ErrAgentTokenNotFound
		}
		lease, err = s.leases.leaseTx(ctx, tx, leaseID, true)
		if err != nil || lease.ProjectID != project || lease.APIKeyID != parent.ID {
			return ErrAgentTokenNotFound
		}
		if err = s.leases.authorizeGrantTx(ctx, tx, p, project, lease.Environment, nil, true); err != nil {
			return err
		}
		var expiry time.Time
		query = `SELECT expires_at FROM agent_token_issuance WHERE jti=$1 AND lease_id=$2 AND project_id=$3 AND user_id=$4` + s.leases.rowLock()
		if err = tx.QueryRowContext(ctx, repository.Q(s.leases.dialect, query), jti, leaseID, project, p.ActorID).Scan(repository.ScanTime(&expiry)); err != nil {
			return ErrAgentTokenNotFound
		}
		var count int
		if err = tx.QueryRowContext(ctx, repository.Q(s.leases.dialect, `SELECT COUNT(*) FROM token_denylist WHERE jti=$1`), jti).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return nil
		}
		query = `INSERT INTO token_denylist(jti,expires_at,reason) VALUES($1,$2,$3) ` + s.leases.dialect.FormatUpsert("jti", "jti = EXCLUDED.jti")
		if _, err = tx.ExecContext(ctx, repository.Q(s.leases.dialect, query), jti, expiry.UTC(), "revoked via scoped api"); err != nil {
			return err
		}
		if err = s.auditRepo.CreateTx(tx, &p.ActorID, &project, "agent.token.revoked", lease.Environment, models.JSONMap{"project_id": project.String(), "jti": jti}, ip); err != nil {
			return err
		}
		return identityEventTx(ctx, tx, s.leases.dialect, p.ActorID, "agent.token.revoked", map[string]string{"project_id": project.String(), "jti": jti})
	})
}
