package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"time"
)

// AuthorityStore implements the policy port. Only stored identity, project and
// membership records contribute authority. Callers cannot supply a trusted role.
type AuthorityStore struct {
	DB interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}
	Dialect             Dialect
	RequireHumanSession bool
}

func (s AuthorityStore) LoadAuthority(ctx context.Context, p policy.Principal, r policy.Resource) (policy.Authority, error) {
	a := policy.Authority{ActorID: p.SubjectID, ProjectID: r.ProjectID}
	if p.Kind == policy.APIKey {
		var scopes models.StringList
		var env sql.NullString
		var expiry sql.NullTime
		err := s.DB.QueryRowContext(ctx, Q(s.Dialect, `SELECT user_id,project_id,scopes,environment,expires_at FROM api_keys WHERE id=$1`), p.SubjectID).Scan(&a.ActorID, &a.ProjectID, &scopes, &env, dbTime(&expiry))
		if err != nil {
			return a, err
		}
		a.Scopes = scopes
		a.Environment = env.String
		if expiry.Valid {
			a.ExpiresAt = expiry.Time
		}
	}
	if p.Kind == policy.Human && s.RequireHumanSession {
		if p.SessionID == uuid.Nil {
			return a, errors.New("human session missing")
		}
		var count int
		err := s.DB.QueryRowContext(ctx, Q(s.Dialect, `SELECT COUNT(*) FROM session_tokens WHERE id=$1 AND user_id=$2 AND revoked=`+s.Dialect.BoolLiteral(false)+` AND expires_at>`+s.Dialect.Now()), p.SessionID, p.SubjectID).Scan(&count)
		if err != nil {
			return a, err
		}
		if count != 1 {
			return a, errors.New("human session revoked")
		}
	}
	if p.Kind == policy.AgentToken {
		if p.LeaseID == uuid.Nil || p.TokenID == "" {
			return a, errors.New("agent authority missing")
		}
		var parent uuid.UUID
		var keys, scopes models.StringList
		var parentEnvironment sql.NullString
		var leaseExpiry, tokenExpiry time.Time
		var parentExpiry sql.NullTime
		err := s.DB.QueryRowContext(ctx, Q(s.Dialect, `SELECT k.id,k.user_id,k.project_id,k.scopes,k.environment,k.expires_at,l.environment,l.secret_keys,l.expires_at,i.expires_at
	 FROM agent_token_issuance i JOIN secret_leases l ON l.id=i.lease_id AND l.project_id=i.project_id JOIN api_keys k ON k.id=l.api_key_id AND k.project_id=l.project_id
	 WHERE i.jti=$1 AND i.lease_id=$2 AND i.user_id=$3 AND l.revoked=`+s.Dialect.BoolLiteral(false)+` AND NOT EXISTS(SELECT 1 FROM token_denylist d WHERE d.jti=i.jti AND d.expires_at>`+s.Dialect.Now()+`)`), p.TokenID, p.LeaseID, p.ActorID).Scan(&parent, &a.ActorID, &a.ProjectID, &scopes, &parentEnvironment, dbTime(&parentExpiry), &a.Environment, &keys, dbTime(&leaseExpiry), dbTime(&tokenExpiry))
		if err != nil {
			return a, err
		}
		if !leaseExpiry.After(time.Now()) || !tokenExpiry.After(time.Now()) || (parentExpiry.Valid && (!parentExpiry.Time.After(time.Now()) || leaseExpiry.After(parentExpiry.Time))) || (parentEnvironment.Valid && parentEnvironment.String != a.Environment) || !policy.LeaseKeysAllowed(scopes, keys) {
			return a, errors.New("agent authority denied")
		}
		a.ExpiresAt = tokenExpiry
		if leaseExpiry.Before(a.ExpiresAt) {
			a.ExpiresAt = leaseExpiry
		}
		if parentExpiry.Valid && parentExpiry.Time.Before(a.ExpiresAt) {
			a.ExpiresAt = parentExpiry.Time
		}
		if len(keys) == 0 {
			a.Scopes = []string{"read"}
		}
		for _, key := range keys {
			a.Scopes = append(a.Scopes, "read:"+key)
		}
	}
	var tenant uuid.NullUUID
	err := s.DB.QueryRowContext(ctx, Q(s.Dialect, `SELECT organization_id,CASE WHEN organization_id IS NULL AND owner_id=$1 THEN 'admin' ELSE
 COALESCE((SELECT role FROM organization_members WHERE organization_id=projects.organization_id AND user_id=$2),'') END
 FROM projects WHERE id=$3 AND deleted_at IS NULL`), a.ActorID, a.ActorID, a.ProjectID).Scan(&tenant, &a.Role)
	if tenant.Valid {
		a.TenantID = tenant.UUID
	}
	return a, err
}
