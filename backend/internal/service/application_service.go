package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type ApplicationService struct {
	appRepo *repository.ApplicationRepository
}

func NewApplicationService(appRepo *repository.ApplicationRepository) *ApplicationService {
	return &ApplicationService{appRepo: appRepo}
}

func (s *ApplicationService) Create(name, url, description, icon, category string, ownerID uuid.UUID) (*models.Application, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}
	if icon == "" {
		icon = "🚀"
	}
	if category == "" {
		category = "General"
	}

	app := &models.Application{
		Name:        name,
		URL:         url,
		Description: description,
		Icon:        icon,
		Category:    category,
		OwnerID:     ownerID,
	}

	if err := s.appRepo.Create(app); err != nil {
		return nil, fmt.Errorf("creating application: %w", err)
	}
	return app, nil
}

func (s *ApplicationService) Get(id uuid.UUID) (*models.Application, error) {
	return s.appRepo.GetByID(id)
}

func (s *ApplicationService) List(ownerID uuid.UUID, search, category string) ([]models.Application, error) {
	apps, err := s.appRepo.ListByOwner(ownerID, search, category)
	if err != nil {
		return nil, err
	}

	// Annotate favorites
	favs, err := s.appRepo.GetFavoriteAppIDs(ownerID)
	if err != nil {
		return nil, err
	}
	for i := range apps {
		if favs[apps[i].ID] {
			apps[i].IsFavorite = true
		}
	}

	return apps, nil
}

func (s *ApplicationService) Update(id uuid.UUID, name, url, description, icon, category string, ownerID uuid.UUID) (*models.Application, error) {
	app := &models.Application{
		ID:          id,
		Name:        name,
		URL:         url,
		Description: description,
		Icon:        icon,
		Category:    category,
		OwnerID:     ownerID,
	}
	if err := s.appRepo.Update(app); err != nil {
		return nil, err
	}
	return s.appRepo.GetByID(id)
}

func (s *ApplicationService) Delete(id, ownerID uuid.UUID) error {
	return s.appRepo.Delete(id, ownerID)
}

func (s *ApplicationService) ToggleFavorite(userID, appID uuid.UUID) (bool, error) {
	return s.appRepo.ToggleFavorite(userID, appID)
}

func (s *ApplicationService) GetCategories(ownerID uuid.UUID) ([]string, error) {
	return s.appRepo.GetCategories(ownerID)
}
