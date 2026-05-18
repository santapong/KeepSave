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

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrNotAuthorized   = errors.New("not authorized for this project")
	ErrAPIKeyNotFound  = errors.New("api key not found")
	// ErrAPIKeyExpiryOutOfRange is returned when a caller submits
	// expires_at that is in the past or further than 365 days out.
	// Per ADR-0009 / audit S-M4.
	ErrAPIKeyExpiryOutOfRange = errors.New("expires_at must be in the future and at most 365 days from now")
)

const (
	// apiKeyDefaultTTL is applied when CreateAPIKeyRequest omits
	// expires_at. Matches GitHub fine-grained-PAT default.
	apiKeyDefaultTTL = 90 * 24 * time.Hour
	// apiKeyMaxTTL is the hard upper bound (ADR-0009 §Decision). 365d
	// matches the GitHub PAT ceiling and is high enough to satisfy
	// long-running CI integrations without an explicit service-token
	// feature (Phase B).
	apiKeyMaxTTL = 365 * 24 * time.Hour
)

// computeEffectiveAPIKeyExpiry returns the expires_at value the service
// will persist. Defaults to now+90d when caller omits, refuses past
// times and anything beyond now+365d. Extracted from Create so the
// branch is unit-testable without the surrounding repo dependencies.
func computeEffectiveAPIKeyExpiry(now time.Time, requested *time.Time) (time.Time, error) {
	if requested == nil {
		return now.Add(apiKeyDefaultTTL), nil
	}
	maxExpiry := now.Add(apiKeyMaxTTL)
	if requested.Before(now.Add(1*time.Minute)) || requested.After(maxExpiry) {
		return time.Time{}, ErrAPIKeyExpiryOutOfRange
	}
	return requested.UTC(), nil
}

type APIKeyService struct {
	apikeyRepo  *repository.APIKeyRepository
	projectRepo *repository.ProjectRepository
	auditRepo   *repository.AuditRepository
}

func NewAPIKeyService(apikeyRepo *repository.APIKeyRepository, projectRepo *repository.ProjectRepository, auditRepo *repository.AuditRepository) *APIKeyService {
	return &APIKeyService{apikeyRepo: apikeyRepo, projectRepo: projectRepo, auditRepo: auditRepo}
}

type CreateAPIKeyResponse struct {
	APIKey *models.APIKey `json:"api_key"`
	RawKey string         `json:"raw_key"`
}

func (s *APIKeyService) Create(name string, userID, projectID uuid.UUID, scopes []string, environment *string, expiresAt *time.Time, ipAddr string) (*CreateAPIKeyResponse, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, ErrProjectNotFound
	}
	if project.OwnerID != userID {
		return nil, ErrNotAuthorized
	}

	if len(scopes) == 0 {
		scopes = []string{"read"}
	}

	effectiveExpiry, err := computeEffectiveAPIKeyExpiry(time.Now().UTC(), expiresAt)
	if err != nil {
		return nil, err
	}

	rawKey, hashedKey, err := auth.GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("generating api key: %w", err)
	}

	apiKey, err := s.apikeyRepo.Create(name, hashedKey, userID, projectID, scopes, environment, &effectiveExpiry)
	if err != nil {
		return nil, fmt.Errorf("storing api key: %w", err)
	}

	envStr := ""
	if environment != nil {
		envStr = *environment
	}
	emitAudit(s.auditRepo, &userID, &projectID, "apikey.created", envStr,
		models.JSONMap{"key_id": apiKey.ID.String(), "name": name, "scopes": scopes}, ipAddr)

	return &CreateAPIKeyResponse{
		APIKey: apiKey,
		RawKey: rawKey,
	}, nil
}

func (s *APIKeyService) List(userID uuid.UUID) ([]models.APIKey, error) {
	return s.apikeyRepo.ListByUserID(userID)
}

func (s *APIKeyService) Delete(id, userID uuid.UUID, ipAddr string) error {
	if err := s.apikeyRepo.Delete(id, userID); err != nil {
		return err
	}
	emitAudit(s.auditRepo, &userID, nil, "apikey.deleted", "",
		models.JSONMap{"key_id": id.String()}, ipAddr)
	return nil
}
