package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// ErrUserExists is re-exported so handlers can detect duplicate-email registration without
// importing the repository package.
var ErrUserExists = repository.ErrUserExists

// ErrAccountLocked is returned when the email is currently in a soft-lock
// window from too many recent failures (audit S-H7).
var ErrAccountLocked = errors.New("account locked due to too many failed login attempts")

// lockoutThreshold and lockoutWindow define the soft-lock policy. 10
// consecutive failures within the window flip the account locked for the
// window's duration; the counter resets on the next successful login.
const (
	lockoutThreshold = 10
	lockoutWindow    = 15 * time.Minute
)

type AuthService struct {
	userRepo     *repository.UserRepository
	attemptsRepo *repository.AuthAttemptsRepository
	auditRepo    *repository.AuditRepository
	jwtService   *auth.JWTService
	sessions     *SessionService
}

func (s *AuthService) EnableSessions(v *SessionService) { s.sessions = v }

func NewAuthService(userRepo *repository.UserRepository, attemptsRepo *repository.AuthAttemptsRepository, auditRepo *repository.AuditRepository, jwtService *auth.JWTService) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		attemptsRepo: attemptsRepo,
		auditRepo:    auditRepo,
		jwtService:   jwtService,
	}
}

type AuthResponse struct {
	User  *models.User `json:"user"`
	Token string       `json:"token"`
}

func (s *AuthService) Register(email, password string) (*AuthResponse, error) {
	return s.RegisterContext(context.Background(), email, password, "", "")
}
func (s *AuthService) RegisterContext(ctx context.Context, email, password, ip, ua string) (*AuthResponse, error) {
	if err := auth.DefaultPasswordPolicy().Validate(password); err != nil {
		return nil, err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("%w: hashing password: %v", auth.ErrSessionUnavailable, err)
	}

	tx, err := s.userRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: opening identity transaction", auth.ErrSessionUnavailable)
	}
	defer tx.Rollback()
	user, err := s.userRepo.CreateTx(ctx, tx, email, hash)
	if err != nil {
		if errors.Is(err, repository.ErrUserExists) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: creating user: %v", auth.ErrSessionUnavailable, err)
	}

	var token string
	if s.sessions != nil {
		token, err = s.sessions.IssueTx(ctx, tx, user, "password", ip, ua)
	} else {
		token, err = s.jwtService.GenerateToken(user.ID, user.Email)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: issuing session: %v", auth.ErrSessionUnavailable, err)
	}

	if s.auditRepo == nil {
		return nil, auth.ErrSessionUnavailable
	}
	if err = s.auditRepo.CreateTx(tx, &user.ID, nil, "auth.register", "", models.JSONMap{"method": "password"}, ip); err != nil {
		return nil, fmt.Errorf("%w: identity audit unavailable", auth.ErrSessionUnavailable)
	}
	if err = identityEventTx(ctx, tx, s.userRepo.Dialect(), user.ID, "auth.register", map[string]string{"method": "password"}); err != nil {
		return nil, fmt.Errorf("%w: identity outbox unavailable", auth.ErrSessionUnavailable)
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("%w: committing identity transaction", auth.ErrSessionUnavailable)
	}
	return &AuthResponse{User: user, Token: token}, nil
}

func (s *AuthService) LookupByEmail(email string) (*models.User, error) {
	return s.userRepo.GetByEmail(email)
}

func (s *AuthService) Login(email, password, ipAddr string) (*AuthResponse, error) {
	return s.LoginContext(context.Background(), email, password, ipAddr, "")
}
func (s *AuthService) LoginContext(ctx context.Context, email, password, ipAddr, ua string) (*AuthResponse, error) {
	email = repository.CanonicalEmail(email)
	// Soft-lock check fires before any DB lookup so the attacker doesn't
	// even get a timing oracle for "does this email exist".
	if s.attemptsRepo != nil {
		la, err := s.attemptsRepo.Get(email)
		if err != nil {
			return nil, fmt.Errorf("%w: loading authentication attempts", auth.ErrSessionUnavailable)
		}
		if err == nil && la.LockedUntil != nil && la.LockedUntil.After(time.Now()) {
			emitAudit(s.auditRepo, nil, nil, "auth.login_failed", "",
				models.JSONMap{"email_attempted": email, "reason": "locked"}, ipAddr)
			return nil, ErrAccountLocked
		}
	}

	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.recordFailure(email, ipAddr)
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("%w: finding user: %v", auth.ErrSessionUnavailable, err)
	}

	if err := auth.CheckPassword(password, user.PasswordHash); err != nil {
		s.recordFailure(email, ipAddr)
		return nil, fmt.Errorf("invalid credentials")
	}

	tx, err := s.userRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: opening identity transaction", auth.ErrSessionUnavailable)
	}
	defer tx.Rollback()
	var token string
	if s.sessions != nil {
		if err = (authority.Guard{Dialect: s.userRepo.Dialect()}).LockSubjects(ctx, tx, []uuid.UUID{user.ID}, true); err != nil {
			return nil, auth.ErrSessionUnavailable
		}
		// A password validated before a recovery or method-removal commit may
		// not mint a session after it. Compare the verified hash under the barrier.
		var currentHash string
		if err = tx.QueryRowContext(ctx, repository.Q(s.userRepo.Dialect(), `SELECT password_hash FROM users WHERE id=$1`), user.ID).Scan(&currentHash); err != nil {
			return nil, auth.ErrSessionUnavailable
		}
		if currentHash != user.PasswordHash {
			return nil, fmt.Errorf("invalid credentials")
		}
		token, err = s.sessions.IssueTx(ctx, tx, user, "password", ipAddr, ua)
	} else {
		token, err = s.jwtService.GenerateToken(user.ID, user.Email)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: issuing session: %v", auth.ErrSessionUnavailable, err)
	}

	if s.auditRepo == nil {
		return nil, auth.ErrSessionUnavailable
	}
	if err = s.auditRepo.CreateTx(tx, &user.ID, nil, "auth.login", "", models.JSONMap{"success": true, "method": "password"}, ipAddr); err != nil {
		return nil, fmt.Errorf("%w: identity audit unavailable", auth.ErrSessionUnavailable)
	}
	if err = identityEventTx(ctx, tx, s.userRepo.Dialect(), user.ID, "auth.login", map[string]string{"method": "password"}); err != nil {
		return nil, fmt.Errorf("%w: identity outbox unavailable", auth.ErrSessionUnavailable)
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("%w: committing identity transaction", auth.ErrSessionUnavailable)
	}
	if s.attemptsRepo != nil {
		_ = s.attemptsRepo.Reset(email)
	}

	return &AuthResponse{User: user, Token: token}, nil
}

func (s *AuthService) recordFailure(email, ipAddr string) {
	if s.attemptsRepo != nil {
		_ = s.attemptsRepo.RegisterFailure(email, lockoutThreshold, lockoutWindow)
	}
	emitAudit(s.auditRepo, nil, nil, "auth.login_failed", "",
		models.JSONMap{"email_attempted": email}, ipAddr)
}
