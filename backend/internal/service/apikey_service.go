package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type APIKeyService struct {
	apikeyRepo *repository.APIKeyRepository
}

func NewAPIKeyService(apikeyRepo *repository.APIKeyRepository) *APIKeyService {
	return &APIKeyService{apikeyRepo: apikeyRepo}
}

type CreateAPIKeyResponse struct {
	APIKey *models.APIKey `json:"api_key"`
	RawKey string         `json:"raw_key"`
}

func (s *APIKeyService) Create(name string, userID, projectID uuid.UUID, scopes []string, environment *string) (*CreateAPIKeyResponse, error) {
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
	return s.apikeyRepo.Delete(id)
}
