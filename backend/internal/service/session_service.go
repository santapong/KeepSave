package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type SessionService struct {
	db      *sql.DB
	repo    *repository.SessionRepository
	jwt     *auth.JWTService
	audit   *repository.AuditRepository
	dialect repository.Dialect
}

func NewSessionService(db *sql.DB, d repository.Dialect, jwt *auth.JWTService, audit *repository.AuditRepository) *SessionService {
	return &SessionService{db: db, repo: repository.NewSessionRepository(db, d), jwt: jwt, audit: audit, dialect: d}
}

// RevokeDelegationsForRecoveryTx is the identity module's narrow legacy grant
// port. Its caller holds the exclusive subject barrier and records the required
// proof-consumed audit/outbox after all identity and grant mutations.
func (s *SessionService) RevokeDelegationsForRecoveryTx(ctx context.Context, tx *sql.Tx, user uuid.UUID) error {
	if s == nil || s.repo == nil {
		return auth.ErrSessionUnavailable
	}
	return s.repo.RevokeRecoveryDelegationsTx(ctx, tx, user)
}

func (s *SessionService) CheckHumanSession(ctx context.Context, c *auth.Claims, raw string) error {
	id, err := uuid.Parse(c.SessionID)
	if err != nil || id == uuid.Nil || c.ID != c.SessionID || c.ExpiresAt == nil {
		return auth.ErrSessionInvalid
	}
	return s.repo.Check(ctx, id, c.UserID, auth.HashAPIKey(raw), c.ExpiresAt.Time)
}
func (s *SessionService) RequireActiveTx(ctx context.Context, tx *sql.Tx, user, id uuid.UUID) error {
	return s.repo.LockActiveTx(ctx, tx, user, id, false)
}
func (s *SessionService) RequireRecentTx(ctx context.Context, tx *sql.Tx, user, id uuid.UUID) error {
	return s.repo.LockActiveTx(ctx, tx, user, id, true)
}
func (s *SessionService) RequireRecent(ctx context.Context, user, id uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.ErrSessionUnavailable
	}
	defer tx.Rollback()
	return s.RequireRecentTx(ctx, tx, user, id)
}
func (s *SessionService) IssueTx(ctx context.Context, tx *sql.Tx, user *models.User, method, ip, ua string) (string, error) {
	if s.dialect.DBType() == repository.DBTypePostgres {
		if err := (authority.Guard{DB: s.db, Dialect: s.dialect}).LockSubjects(ctx, tx, []uuid.UUID{user.ID}, true); err != nil {
			return "", err
		}
	}
	id := uuid.New()
	expires := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	token, err := s.jwt.GenerateSessionToken(user.ID, user.Email, id, expires)
	if err != nil {
		return "", err
	}
	if err = s.repo.CreateTx(ctx, tx, id, user.ID, auth.HashAPIKey(token), expires, ip, ua); err != nil {
		return "", err
	}
	if s.dialect.DBType() == repository.DBTypePostgres {
		recorded := method
		if recorded != "password" && recorded != "google" && recorded != "github" {
			recorded = "unknown"
		}
		if _, err = tx.ExecContext(ctx, `UPDATE session_tokens SET auth_method=$1 WHERE id=$2`, recorded, id); err != nil {
			return "", err
		}
	}
	if s.audit == nil {
		return "", auth.ErrSessionUnavailable
	}
	if err = s.audit.CreateTx(tx, &user.ID, nil, "auth.session_created", "", models.JSONMap{"session_id": id.String(), "method": method}, ip); err != nil {
		return "", err
	}
	return token, nil
}
func (s *SessionService) List(ctx context.Context, user, current uuid.UUID) ([]models.SessionView, error) {
	rows, err := s.repo.List(ctx, user)
	if err != nil {
		return nil, err
	}
	result := []models.SessionView{}
	for _, row := range rows {
		status := "active"
		if row.Revoked {
			status = "revoked"
		} else if !row.ExpiresAt.After(time.Now()) {
			status = "expired"
		}
		result = append(result, models.SessionView{ID: row.ID, Current: row.ID == current, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Status: status, IPAddress: row.IPAddress, UserAgent: row.UserAgent})
	}
	return result, nil
}
func (s *SessionService) Revoke(ctx context.Context, user, current, target uuid.UUID, ip string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.ErrSessionUnavailable
	}
	defer tx.Rollback()
	if err = (authority.Guard{DB: s.db, Dialect: s.dialect}).LockSubjects(ctx, tx, []uuid.UUID{user}, true); err != nil {
		return err
	}
	if err = s.repo.LockRevocationTargetsTx(ctx, tx, user, current, target); err != nil {
		return err
	}
	if err = s.RequireActiveTx(ctx, tx, user, current); err != nil {
		return err
	}
	changed, err := s.repo.RevokeTx(ctx, tx, user, target)
	if err != nil {
		return err
	}
	if changed {
		if s.audit == nil {
			return auth.ErrSessionUnavailable
		}
		if err = s.audit.CreateTx(tx, &user, nil, "auth.session_revoked", "", models.JSONMap{"session_id": target.String()}, ip); err != nil {
			return err
		}
		if err = identityEventTx(ctx, tx, s.dialect, user, "auth.session_revoked", map[string]string{"session_id": target.String()}); err != nil {
			return err
		}
	}
	return tx.Commit()
}
