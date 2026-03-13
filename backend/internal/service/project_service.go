package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type ProjectService struct {
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	cryptoSvc   *crypto.Service
}

func NewProjectService(
	projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository,
	cryptoSvc *crypto.Service,
) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		envRepo:     envRepo,
		cryptoSvc:   cryptoSvc,
	}
}

func (s *ProjectService) Create(name, description string, ownerID uuid.UUID) (*models.Project, error) {
	dek, err := s.cryptoSvc.GenerateDEK()
	if err != nil {
		return nil, fmt.Errorf("generating DEK: %w", err)
	}

	encryptedDEK, dekNonce, err := s.cryptoSvc.EncryptDEK(dek)
	if err != nil {
		return nil, fmt.Errorf("encrypting DEK: %w", err)
	}

	project, err := s.projectRepo.Create(name, description, ownerID, encryptedDEK, dekNonce)
	if err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	if _, err := s.envRepo.CreateDefaultsForProject(project.ID); err != nil {
		return nil, fmt.Errorf("creating default environments: %w", err)
	}

	return project, nil
}

func (s *ProjectService) GetByID(id, ownerID uuid.UUID) (*models.Project, error) {
	project, err := s.projectRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	if project.OwnerID != ownerID {
		return nil, fmt.Errorf("project not found")
	}
	return project, nil
}

func (s *ProjectService) List(ownerID uuid.UUID) ([]models.Project, error) {
	return s.projectRepo.ListByOwnerID(ownerID)
}

func (s *ProjectService) Update(id, ownerID uuid.UUID, name, description string) (*models.Project, error) {
	project, err := s.projectRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	if project.OwnerID != ownerID {
		return nil, fmt.Errorf("project not found")
	}
	return s.projectRepo.Update(id, name, description)
}

func (s *ProjectService) Delete(id, ownerID uuid.UUID) error {
	project, err := s.projectRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("getting project: %w", err)
	}
	if project.OwnerID != ownerID {
		return fmt.Errorf("project not found")
	}
	return s.projectRepo.Delete(id)
}
