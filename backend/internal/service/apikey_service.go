package service

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrNotAuthorized   = errors.New("not authorized for this project")
	ErrAPIKeyNotFound  = errors.New("api key not found")
)

type APIKeyService struct {
	apikeyRepo  *repository.APIKeyRepository
	projectRepo *repository.ProjectRepository
}

func NewAPIKeyService(apikeyRepo *repository.APIKeyRepository, projectRepo *repository.ProjectRepository) *APIKeyService {
	return &APIKeyService{apikeyRepo: apikeyRepo, projectRepo: projectRepo}
}

type CreateAPIKeyResponse struct {
	APIKey *models.APIKey `json:"api_key"`
	RawKey string         `json:"raw_key"`
}

func (s *APIKeyService) Create(name string, userID, projectID uuid.UUID, scopes []string, environment *string) (*CreateAPIKeyResponse, error) {
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

	rawKey, hashedKey, err := auth.GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("generating api key: %w", err)
	}

	apiKey, err := s.apikeyRepo.Create(name, hashedKey, userID, projectID, scopes, environment)
	if err != nil {
		return nil, fmt.Errorf("storing api key: %w", err)
	}

	return &CreateAPIKeyResponse{
		APIKey: apiKey,
		RawKey: rawKey,
	}, nil
}

func (s *APIKeyService) List(userID uuid.UUID) ([]models.APIKey, error) {
	return s.apikeyRepo.ListByUserID(userID)
}

func (s *APIKeyService) Delete(id, userID uuid.UUID) error {
	return s.apikeyRepo.Delete(id, userID)
}
