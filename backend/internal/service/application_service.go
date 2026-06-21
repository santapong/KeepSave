package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type ApplicationService struct {
	appRepo   *repository.ApplicationRepository
	auditRepo *repository.AuditRepository
}

func NewApplicationService(appRepo *repository.ApplicationRepository, auditRepo *repository.AuditRepository) *ApplicationService {
	return &ApplicationService{appRepo: appRepo, auditRepo: auditRepo}
}

func (s *ApplicationService) Create(name, url, description, icon, category string, ownerID uuid.UUID, ipAddr string) (*models.Application, error) {
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
	emitAudit(s.auditRepo, &ownerID, nil, "application.created", "",
		models.JSONMap{"application_id": app.ID.String(), "name": name}, ipAddr)
	return app, nil
}

func (s *ApplicationService) Get(id uuid.UUID) (*models.Application, error) {
	return s.appRepo.GetByID(id)
}

func (s *ApplicationService) List(ownerID uuid.UUID, search, category string, limit, offset int) ([]models.Application, int, error) {
	apps, total, err := s.appRepo.ListByOwner(ownerID, search, category, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	// Annotate favorites
	favs, err := s.appRepo.GetFavoriteAppIDs(ownerID)
	if err != nil {
		return nil, 0, err
	}
	for i := range apps {
		if favs[apps[i].ID] {
			apps[i].IsFavorite = true
		}
	}

	return apps, total, nil
}

func (s *ApplicationService) Update(id uuid.UUID, name, url, description, icon, category string, ownerID uuid.UUID, ipAddr string) (*models.Application, error) {
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
	emitAudit(s.auditRepo, &ownerID, nil, "application.updated", "",
		models.JSONMap{"application_id": id.String(), "name": name}, ipAddr)
	return s.appRepo.GetByID(id)
}

func (s *ApplicationService) Delete(id, ownerID uuid.UUID, ipAddr string) error {
	if err := s.appRepo.Delete(id, ownerID); err != nil {
		return err
	}
	emitAudit(s.auditRepo, &ownerID, nil, "application.deleted", "",
		models.JSONMap{"application_id": id.String()}, ipAddr)
	return nil
}

func (s *ApplicationService) ToggleFavorite(userID, appID uuid.UUID) (bool, error) {
	return s.appRepo.ToggleFavorite(userID, appID)
}

func (s *ApplicationService) GetCategories(ownerID uuid.UUID) ([]string, error) {
	return s.appRepo.GetCategories(ownerID)
}
