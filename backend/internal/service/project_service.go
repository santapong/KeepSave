package service

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// ErrEmbedConfigNotFound is returned when a project does not exist OR has not
// opted into the embed widget. Callers MUST surface the same error shape for
// both cases to prevent project-ID enumeration via the unauthenticated
// embed-config endpoint (ADR-0006 §Open-Q #1, threat model §4 row S).
var ErrEmbedConfigNotFound = errors.New("embed config not found")

// ErrWildcardOriginNotPermitted is returned when an operator attempts to set
// allowed_origins=["*"] on a project. Operators must use embed_policy_enabled
// to disable the widget; '*' is rejected at the API boundary per ADR-0006
// §Open-Q #2 (sentinel handling).
var ErrWildcardOriginNotPermitted = errors.New("wildcard origin not permitted")

// EmbedConfig is the minimal, non-secret-bearing payload returned to the
// embed widget at boot. Crucially this struct never contains the DEK,
// secrets, owner identity, or any other privileged data.
type EmbedConfig struct {
	ProjectID          uuid.UUID `json:"project_id"`
	AllowedOrigins     []string  `json:"allowed_origins"`
	EmbedPolicyEnabled bool      `json:"embed_policy_enabled"`
}

type ProjectService struct {
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	auditRepo   *repository.AuditRepository
	cryptoSvc   *crypto.Service
}

func NewProjectService(
	projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository,
	auditRepo *repository.AuditRepository,
	cryptoSvc *crypto.Service,
) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		envRepo:     envRepo,
		auditRepo:   auditRepo,
		cryptoSvc:   cryptoSvc,
	}
}

func (s *ProjectService) Create(name, description string, ownerID uuid.UUID, ipAddr string) (*models.Project, error) {
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

	emitAudit(s.auditRepo, &ownerID, &project.ID, "project.created", "",
		models.JSONMap{"name": name}, ipAddr)

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

func (s *ProjectService) Update(id, ownerID uuid.UUID, name, description, ipAddr string) (*models.Project, error) {
	project, err := s.projectRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	if project.OwnerID != ownerID {
		return nil, fmt.Errorf("project not found")
	}
	updated, err := s.projectRepo.Update(id, name, description)
	if err != nil {
		return nil, err
	}
	emitAudit(s.auditRepo, &ownerID, &id, "project.updated", "",
		models.JSONMap{"name": name}, ipAddr)
	return updated, nil
}

// GetEmbedConfig returns the public embed-widget configuration for a project.
//
// This method is the ONLY path the unauthenticated /embed-config endpoint
// goes through. It MUST:
//   - Return ErrEmbedConfigNotFound for both "project missing" and
//     "embed policy disabled" so the response shape cannot be used to
//     enumerate valid project IDs.
//   - Refuse to leak any secret-bearing data (no DEK, no owner, no
//     secret keys). The returned EmbedConfig struct is exhaustive by design.
//   - Treat a stored "*" sentinel as an empty list (defence in depth in
//     case the API-level sentinel reject is ever bypassed).
func (s *ProjectService) GetEmbedConfig(projectID uuid.UUID) (EmbedConfig, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		// Repository wraps sql.ErrNoRows; we deliberately do not differentiate.
		return EmbedConfig{}, ErrEmbedConfigNotFound
	}
	if !project.EmbedPolicyEnabled {
		// Same error shape as not-found to mitigate enumeration.
		return EmbedConfig{}, ErrEmbedConfigNotFound
	}

	// Defence in depth: strip any wildcard sentinel that may have slipped past
	// the API-level validator. The frontend ALSO treats "*" as a refusal, but
	// belt-and-suspenders is cheap here.
	origins := make([]string, 0, len(project.AllowedOrigins))
	for _, o := range project.AllowedOrigins {
		if o == "*" {
			continue
		}
		origins = append(origins, o)
	}

	return EmbedConfig{
		ProjectID:          project.ID,
		AllowedOrigins:     origins,
		EmbedPolicyEnabled: true,
	}, nil
}

// UpdateEmbedConfig sets a project's embed allow-list. Rejects the wildcard
// sentinel per ADR-0006 §Open-Q #2. Callers (handlers) MUST verify the
// requester owns the project.
func (s *ProjectService) UpdateEmbedConfig(id, ownerID uuid.UUID, allowedOrigins []string, embedPolicyEnabled bool) error {
	project, err := s.projectRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("getting project: %w", err)
	}
	if project.OwnerID != ownerID {
		return fmt.Errorf("project not found")
	}
	for _, o := range allowedOrigins {
		if o == "*" {
			return ErrWildcardOriginNotPermitted
		}
	}
	return s.projectRepo.UpdateEmbedConfig(id, allowedOrigins, embedPolicyEnabled)
}

func (s *ProjectService) Delete(id, ownerID uuid.UUID, ipAddr string) error {
	project, err := s.projectRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("getting project: %w", err)
	}
	if project.OwnerID != ownerID {
		return fmt.Errorf("project not found")
	}
	if err := s.projectRepo.Delete(id); err != nil {
		return err
	}
	emitAudit(s.auditRepo, &ownerID, &id, "project.deleted", "",
		models.JSONMap{"name": project.Name}, ipAddr)
	return nil
}
