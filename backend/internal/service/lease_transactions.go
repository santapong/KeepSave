package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

func (s *LeaseService) EnableSessions(sessions *SessionService) { s.sessions = sessions }

// withGrantTx bounds local grant work and commits its authority check, mutation,
// required audit and PostgreSQL outbox together. Domain locks precede audit head.
func (s *LeaseService) withGrantTx(ctx context.Context, fn func(context.Context, *sql.Tx) error) error {
	if s.auditRepo == nil {
		return errors.New("grant audit unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if s.dialect.DBType() == repository.DBTypePostgres {
		// AuditRepository's legacy Tx methods lack per-statement contexts.
		// Database timeouts also bound those required writes and lock waits.
		if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout = '10s'`); err != nil {
			return err
		}
	}
	if err = fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *LeaseService) rowLock() string {
	if s.dialect.DBType() != repository.DBTypeSQLite {
		return " FOR UPDATE"
	}
	return ""
}

// lockGrantActorTx follows subject -> organization/project -> membership ->
// session -> parent -> lease. The policy decision still occurs under these locks.
func (s *LeaseService) lockGrantActorTx(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID) error {
	if p.Kind != policy.Human && p.Kind != policy.APIKey || p.SubjectID == uuid.Nil || p.ActorID == uuid.Nil {
		return ErrLeaseAuthority
	}
	if p.Kind == policy.Human && p.SubjectID != p.ActorID {
		return ErrLeaseAuthority
	}
	if s.dialect.DBType() != repository.DBTypePostgres {
		if p.Kind == policy.Human && s.sessions != nil {
			if err := s.sessions.RequireActiveTx(ctx, tx, p.SubjectID, p.SessionID); err != nil {
				return err
			}
		}
		var tenant uuid.NullUUID
		if err := tx.QueryRowContext(ctx, repository.Q(s.dialect, `SELECT organization_id FROM projects WHERE id=$1 AND deleted_at IS NULL`+s.rowLock()), project).Scan(&tenant); err != nil {
			return ErrLeaseAuthority
		}
		if tenant.Valid {
			var role string
			if err := tx.QueryRowContext(ctx, repository.Q(s.dialect, `SELECT role FROM organization_members WHERE organization_id=$1 AND user_id=$2`+s.rowLock()), tenant.UUID, p.ActorID).Scan(&role); err != nil {
				return ErrLeaseAuthority
			}
		}
		return nil
	}
	g := authority.Guard{DB: s.db, Dialect: s.dialect}
	admission := p
	admission.SessionID = uuid.Nil
	if err := g.LockProject(ctx, tx, admission, project, true); err != nil {
		return ErrLeaseAuthority
	}
	if p.Kind == policy.Human && s.sessions != nil {
		if e := g.RequireSession(ctx, tx, p, false); e != nil {
			return auth.ErrSessionInvalid
		}
		return nil
	}
	return nil
}

func (s *LeaseService) parentTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, lock bool) (*models.APIKey, error) {
	parent := &models.APIKey{ID: id}
	var env sql.NullString
	query := `SELECT user_id,project_id,scopes,environment,expires_at FROM api_keys WHERE id=$1`
	if lock {
		query += s.rowLock()
	}
	err := tx.QueryRowContext(ctx, repository.Q(s.dialect, query), id).Scan(&parent.UserID, &parent.ProjectID, &parent.Scopes, &env, repository.ScanTime(&parent.ExpiresAt))
	if env.Valid {
		parent.Environment = &env.String
	}
	return parent, err
}

func (s *LeaseService) leaseTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, lock bool) (*models.SecretLease, error) {
	lease := &models.SecretLease{}
	query := `SELECT id,api_key_id,project_id,environment,secret_keys,granted_at,expires_at,revoked,revoked_at FROM secret_leases WHERE id=$1`
	if lock {
		query += s.rowLock()
	}
	err := tx.QueryRowContext(ctx, repository.Q(s.dialect, query), id).Scan(&lease.ID, &lease.APIKeyID, &lease.ProjectID, &lease.Environment, &lease.SecretKeys, repository.ScanTime(&lease.GrantedAt), repository.ScanTime(&lease.ExpiresAt), &lease.Revoked, repository.ScanTime(&lease.RevokedAt))
	return lease, err
}

func (s *LeaseService) authorizeGrantTx(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, environment string, keys []string, collection bool) error {
	key := ""
	if len(keys) > 0 {
		key = keys[0]
	}
	resource := policy.Resource{ProjectID: project, Environment: environment, Key: key}
	if collection {
		resource.Type = "secret_collection"
	}
	decision, err := (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: s.dialect, RequireHumanSession: s.sessions != nil}}).Authorize(ctx, p, policy.IssueGrant, resource)
	if err != nil || !decision.Allowed {
		return ErrLeaseAuthority
	}
	return nil
}

func validLeaseParent(parent *models.APIKey, project uuid.UUID, environment string, keys []string, expires time.Time) bool {
	return parent.ProjectID == project && (parent.Environment == nil || *parent.Environment == environment) && policy.LeaseKeysAllowed(parent.Scopes, keys) &&
		(parent.ExpiresAt == nil || parent.ExpiresAt.After(time.Now()) && !expires.After(*parent.ExpiresAt))
}

func (s *LeaseService) CreateLeaseAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, environment string, keys []string, duration time.Duration, ip string) (*models.SecretLease, error) {
	if p.Kind != policy.APIKey || duration <= 0 || duration > 24*time.Hour || (environment != "alpha" && environment != "uat" && environment != "prod") {
		return nil, ErrLeaseAuthority
	}
	var lease *models.SecretLease
	err := s.withGrantTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		parent, err := s.parentTx(ctx, tx, p.SubjectID, false)
		if err != nil || p.ActorID != uuid.Nil && p.ActorID != parent.UserID {
			return ErrLeaseAuthority
		}
		p.ActorID = parent.UserID
		if err = s.lockGrantActorTx(ctx, tx, p, project); err != nil {
			return err
		}
		parent, err = s.parentTx(ctx, tx, p.SubjectID, true)
		expires := time.Now().UTC().Add(duration)
		if err != nil || parent.UserID != p.ActorID || !validLeaseParent(parent, project, environment, keys, expires) {
			return ErrLeaseAuthority
		}
		if err = s.authorizeGrantTx(ctx, tx, p, project, environment, keys, false); err != nil {
			return err
		}
		if len(keys) == 0 {
			keys = []string{"*"}
		}
		value, err := s.dialect.ArrayParam(keys)
		if err != nil {
			return err
		}
		id := uuid.New()
		query := `INSERT INTO secret_leases(id,api_key_id,project_id,environment,secret_keys,expires_at) VALUES($1,$2,$3,$4,$5,$6)`
		if _, err = tx.ExecContext(ctx, repository.Q(s.dialect, query), id, parent.ID, project, environment, value, expires); err != nil {
			return fmt.Errorf("creating lease: %w", err)
		}
		lease, err = s.leaseTx(ctx, tx, id, false)
		if err != nil {
			return err
		}
		if err = s.auditRepo.CreateTx(tx, &parent.UserID, &project, "lease.created", environment, models.JSONMap{"project_id": project.String(), "lease_id": id.String(), "secret_keys": keys}, ip); err != nil {
			return err
		}
		return identityEventTx(ctx, tx, s.dialect, parent.UserID, "lease.created", map[string]string{"project_id": project.String(), "lease_id": id.String(), "api_key_id": parent.ID.String()})
	})
	if err != nil {
		return nil, err
	}
	return lease, nil
}

func (s *LeaseService) RevokeLeaseAuthorized(ctx context.Context, p policy.Principal, id, project uuid.UUID, ip string) error {
	return s.withGrantTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		lease, err := s.leaseTx(ctx, tx, id, false)
		if err != nil || lease.ProjectID != project {
			return ErrLeaseNotFound
		}
		if p.Kind == policy.APIKey {
			parent, err := s.parentTx(ctx, tx, p.SubjectID, false)
			if err != nil || p.SubjectID != lease.APIKeyID || p.ActorID != uuid.Nil && p.ActorID != parent.UserID {
				return ErrLeaseNotFound
			}
			p.ActorID = parent.UserID
		}
		if err = s.lockGrantActorTx(ctx, tx, p, project); err != nil {
			return err
		}
		parent, err := s.parentTx(ctx, tx, lease.APIKeyID, true)
		if err != nil || parent.ProjectID != project || p.Kind == policy.APIKey && parent.UserID != p.ActorID {
			return ErrLeaseNotFound
		}
		current, err := s.leaseTx(ctx, tx, id, true)
		if err != nil || current.ProjectID != project || current.APIKeyID != parent.ID {
			return ErrLeaseNotFound
		}
		// Revocation remains available to an authorized human when a parent or
		// lease expires. API keys must retain current authority and exact lineage.
		if err = s.authorizeGrantTx(ctx, tx, p, project, current.Environment, nil, true); err != nil {
			return err
		}
		if current.Revoked {
			return nil
		}
		query := `UPDATE secret_leases SET revoked=` + s.dialect.BoolLiteral(true) + `,revoked_at=` + s.dialect.Now() + ` WHERE id=$1 AND project_id=$2`
		if _, err = tx.ExecContext(ctx, repository.Q(s.dialect, query), id, project); err != nil {
			return err
		}
		if err = s.auditRepo.CreateTx(tx, &p.ActorID, &project, "lease.revoked", current.Environment, models.JSONMap{"lease_id": id.String()}, ip); err != nil {
			return err
		}
		return identityEventTx(ctx, tx, s.dialect, p.ActorID, "lease.revoked", map[string]string{"project_id": project.String(), "lease_id": id.String()})
	})
}
