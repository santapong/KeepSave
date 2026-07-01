package service

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/santapong/KeepSave/backend/internal/auth"
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
}

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
	if err := auth.DefaultPasswordPolicy().Validate(password); err != nil {
		return nil, err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user, err := s.userRepo.Create(email, hash)
	if err != nil {
		if errors.Is(err, repository.ErrUserExists) {
			return nil, err
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}

	token, err := s.jwtService.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}

	return &AuthResponse{User: user, Token: token}, nil
}

func (s *AuthService) LookupByEmail(email string) (*models.User, error) {
	return s.userRepo.GetByEmail(email)
}

func (s *AuthService) Login(email, password, ipAddr string) (*AuthResponse, error) {
	// Soft-lock check fires before any DB lookup so the attacker doesn't
	// even get a timing oracle for "does this email exist".
	if s.attemptsRepo != nil {
		la, err := s.attemptsRepo.Get(email)
		if err == nil && la.LockedUntil != nil && la.LockedUntil.After(time.Now()) {
			emitAudit(s.auditRepo, nil, nil, "auth.login_failed", "",
				models.JSONMap{"email_attempted": email, "reason": "locked"}, ipAddr)
			return nil, ErrAccountLocked
		}
	}

	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		if err == sql.ErrNoRows {
			s.recordFailure(email, ipAddr)
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("finding user: %w", err)
	}

	if err := auth.CheckPassword(password, user.PasswordHash); err != nil {
		s.recordFailure(email, ipAddr)
		return nil, fmt.Errorf("invalid credentials")
	}

	token, err := s.jwtService.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}

	if s.attemptsRepo != nil {
		_ = s.attemptsRepo.Reset(email)
	}
	emitAudit(s.auditRepo, &user.ID, nil, "auth.login", "",
		models.JSONMap{"success": true}, ipAddr)

	return &AuthResponse{User: user, Token: token}, nil
}

func (s *AuthService) recordFailure(email, ipAddr string) {
	if s.attemptsRepo != nil {
		_ = s.attemptsRepo.RegisterFailure(email, lockoutThreshold, lockoutWindow)
	}
	emitAudit(s.auditRepo, nil, nil, "auth.login_failed", "",
		models.JSONMap{"email_attempted": email}, ipAddr)
}
