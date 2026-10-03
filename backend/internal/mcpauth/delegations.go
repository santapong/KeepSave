package mcpauth

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

type Delegation struct {
	FamilyID        uuid.UUID `json:"family_id"`
	ClientID        string    `json:"client_id"`
	Harness         string    `json:"harness"`
	Resource        string    `json:"resource"`
	Scope           string    `json:"scope"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	RequiresRefresh bool      `json:"requires_refresh"`
}

func humanPrincipal(p policy.Principal) bool {
	return p.Kind == policy.Human && p.SubjectID != uuid.Nil && p.SessionID != uuid.Nil && (p.ActorID == uuid.Nil || p.ActorID == p.SubjectID) && (p.ExpiresAt.IsZero() || p.ExpiresAt.After(time.Now()))
}
func (s *Service) ListDelegations(ctx context.Context, p policy.Principal) ([]Delegation, error) {
	if !humanPrincipal(p) {
		return nil, ErrDenied
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = s.parent(ctx, tx, p.SubjectID, p.SessionID, false); e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT f.id,c.client_id,client.harness,c.resource,c.scope,f.created_at,LEAST(f.expires_at,c.expires_at,parent.expires_at),NOT EXISTS(SELECT 1 FROM mcp_oauth_tokens t WHERE t.family_id=f.id AND t.rotated_at IS NULL AND t.expires_at>NOW()) FROM mcp_oauth_families f JOIN mcp_oauth_consents c ON c.id=f.consent_id JOIN mcp_oauth_clients client ON client.client_id=c.client_id JOIN session_tokens parent ON parent.id=c.session_id AND parent.user_id=c.user_id WHERE c.user_id=$1 AND c.resource=$2 AND f.revoked_at IS NULL AND c.revoked_at IS NULL AND client.enabled AND parent.revoked=FALSE AND parent.expires_at>NOW() AND f.expires_at>NOW() AND c.expires_at>NOW() ORDER BY f.created_at DESC,f.id LIMIT 100`, p.SubjectID, s.cfg.Resource)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	out := []Delegation{}
	for rows.Next() {
		var d Delegation
		if e = rows.Scan(&d.FamilyID, &d.ClientID, &d.Harness, &d.Resource, &d.Scope, &d.CreatedAt, &d.ExpiresAt, &d.RequiresRefresh); e != nil {
			return nil, ErrUnavailable
		}
		out = append(out, d)
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	return out, nil
}
func (s *Service) RevokeDelegation(ctx context.Context, p policy.Principal, family uuid.UUID) error {
	if !humanPrincipal(p) || family == uuid.Nil {
		return ErrDenied
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = s.parent(ctx, tx, p.SubjectID, p.SessionID, false); e != nil {
		return e
	}
	var consent uuid.UUID
	var client string
	if e = tx.QueryRowContext(ctx, `SELECT c.id,c.client_id FROM mcp_oauth_families f JOIN mcp_oauth_consents c ON c.id=f.consent_id WHERE f.id=$1 AND c.user_id=$2 AND c.resource=$3`, family, p.SubjectID, s.cfg.Resource).Scan(&consent, &client); e != nil {
		return ErrDenied
	}
	var id uuid.UUID
	if e = tx.QueryRowContext(ctx, `SELECT id FROM mcp_oauth_consents WHERE id=$1 FOR UPDATE`, consent).Scan(&id); e != nil {
		return ErrUnavailable
	}
	var revoked sql.NullTime
	if e = tx.QueryRowContext(ctx, `SELECT revoked_at FROM mcp_oauth_families WHERE id=$1 FOR UPDATE`, family).Scan(&revoked); e != nil {
		return ErrUnavailable
	}
	if revoked.Valid {
		return nil
	}
	if _, e = tx.ExecContext(ctx, `UPDATE mcp_oauth_families SET revoked_at=NOW() WHERE id=$1`, family); e != nil {
		return ErrUnavailable
	}
	if e = s.auditTx(tx, p.SubjectID, "mcp.delegation_revoked", models.JSONMap{"family_id": family.String(), "client_id": client}); e != nil {
		return ErrUnavailable
	}
	if e = tx.Commit(); e != nil {
		return ErrUnavailable
	}
	return nil
}

// FamilyPrincipalTx narrows a human management request to a stored current
// client delegation. The caller holds its full authority barrier and evaluates
// project capability before this method, then admits the returned delegation.
// No client secret or bearer credential is returned or reconstructed.
func (s *Service) FamilyPrincipalTx(ctx context.Context, tx *sql.Tx, human policy.Principal, client string, family uuid.UUID) (policy.Principal, error) {
	if tx == nil || !humanPrincipal(human) || client == "" || family == uuid.Nil {
		return policy.Principal{}, ErrDenied
	}
	var user, sid uuid.UUID
	if e := tx.QueryRowContext(ctx, `SELECT c.user_id,c.session_id FROM mcp_oauth_families f JOIN mcp_oauth_consents c ON c.id=f.consent_id WHERE f.id=$1 AND c.client_id=$2 AND c.resource=$3`, family, client, s.cfg.Resource).Scan(&user, &sid); e != nil || user != human.SubjectID {
		return policy.Principal{}, ErrDenied
	}
	if _, e := s.parent(ctx, tx, user, human.SessionID, false); e != nil {
		return policy.Principal{}, e
	}
	parentExpiry, e := s.parent(ctx, tx, user, sid, false)
	if e != nil {
		return policy.Principal{}, e
	}
	var tokenID uuid.UUID
	var expiry time.Time
	if e = tx.QueryRowContext(ctx, `SELECT t.id,LEAST(t.expires_at,f.expires_at,c.expires_at) FROM mcp_oauth_tokens t JOIN mcp_oauth_families f ON f.id=t.family_id JOIN mcp_oauth_consents c ON c.id=f.consent_id WHERE t.family_id=$1 AND t.rotated_at IS NULL AND t.expires_at>NOW() ORDER BY t.created_at DESC LIMIT 1`, family).Scan(&tokenID, &expiry); e != nil {
		return policy.Principal{}, ErrDenied
	}
	if parentExpiry.Before(expiry) {
		expiry = parentExpiry
	}
	p := policy.Principal{Kind: policy.OAuthDelegation, SubjectID: user, ActorID: user, SessionID: sid, ParentGrantID: family, TokenID: tokenID.String(), ExpiresAt: expiry}
	if e = s.RequireGrantTx(ctx, tx, p, client); e != nil {
		return policy.Principal{}, e
	}
	return p, nil
}
