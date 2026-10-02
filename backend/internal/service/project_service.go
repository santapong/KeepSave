package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/vault"
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
	sessions    *SessionService
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	auditRepo   *repository.AuditRepository
	cryptoSvc   *crypto.Service
	vault       *vault.Service
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

func (s *ProjectService) EnableVault(v *vault.Service) { s.vault = v }
func (s *ProjectService) Create(name, description string, owner uuid.UUID, ip string) (*models.Project, error) {
	return s.CreateAuthorized(context.Background(), policy.Principal{Kind: policy.Human, SubjectID: owner, ActorID: owner}, name, description, ip)
}
func (s *ProjectService) CreateAuthorized(ctx context.Context, p policy.Principal, name, description, ip string) (*models.Project, error) {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil {
		return nil, vault.ErrDenied
	}
	dek, err := s.cryptoSvc.GenerateDEK()
	if err != nil {
		return nil, err
	}
	defer crypto.SecureZero(dek)
	cipher, nonce, err := s.cryptoSvc.EncryptDEK(dek)
	if err != nil {
		return nil, err
	}
	var project *models.Project
	err = s.projectRepo.WithTx(func(tx *sql.Tx) error {
		var err error
		if s.sessions != nil {
			if e := s.sessions.RequireActiveTx(ctx, tx, p.SubjectID, p.SessionID); e != nil {
				return e
			}
		}
		project, err = s.projectRepo.CreateTx(tx, name, description, p.SubjectID, cipher, nonce)
		if err != nil {
			return err
		}
		if err = s.auditRepo.CreateTx(tx, &p.SubjectID, &project.ID, "project.created", "", models.JSONMap{"name": name}, ip); err != nil {
			return err
		}
		if s.projectRepo.Dialect().DBType() == repository.DBTypePostgres {
			payload, _ := json.Marshal(map[string]any{"project_id": project.ID, "action": "project.created"})
			if err = jobs.EnqueueTx(ctx, tx, uuid.New(), "project.event", payload, "local", 5); err != nil {
				return err
			}
		}
		if s.vault != nil {
			return s.vault.EnrollTx(ctx, tx, p, project.ID)
		}
		return nil
	})
	return project, err
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
func (s *ProjectService) UpdateEmbedConfig(id, ownerID uuid.UUID, allowedOrigins []string, embedPolicyEnabled bool, ipAddr string) error {
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
	if err := s.projectRepo.UpdateEmbedConfig(id, allowedOrigins, embedPolicyEnabled); err != nil {
		return err
	}

	// Compute the added/removed diff versus the prior allow-list so the audit
	// row records exactly what changed (docs/AUDIT_LOG_COVERAGE.md:42).
	oldSet := make(map[string]bool, len(project.AllowedOrigins))
	for _, o := range project.AllowedOrigins {
		oldSet[o] = true
	}
	newSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		newSet[o] = true
	}
	added := make([]string, 0)
	for _, o := range allowedOrigins {
		if !oldSet[o] {
			added = append(added, o)
		}
	}
	removed := make([]string, 0)
	for _, o := range project.AllowedOrigins {
		if !newSet[o] {
			removed = append(removed, o)
		}
	}

	emitAudit(s.auditRepo, &ownerID, &id, "embed.origins_updated", "",
		models.JSONMap{"allowed_origins": allowedOrigins, "added": added, "removed": removed}, ipAddr)
	return nil
}

func (s *ProjectService) Delete(id, owner uuid.UUID, ip string) error {
	return s.DeleteAuthorized(context.Background(), policy.Principal{Kind: policy.Human, SubjectID: owner, ActorID: owner}, id, ip)
}
func (s *ProjectService) DeleteAuthorized(ctx context.Context, p policy.Principal, id uuid.UUID, ip string) error {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil {
		return vault.ErrDenied
	}
	if deleted, err := s.projectRepo.IsDeletedOwned(id, p.SubjectID); err != nil {
		return err
	} else if deleted {
		return nil
	}
	if s.vault != nil {
		return s.vault.ArchiveProject(ctx, p, id)
	}
	return s.projectRepo.WithTx(func(tx *sql.Tx) error {
		if s.sessions != nil {
			if e := s.sessions.RequireActiveTx(ctx, tx, p.SubjectID, p.SessionID); e != nil {
				return e
			}
		}
		if err := s.projectRepo.TombstoneTx(tx, id, p.SubjectID); err != nil {
			return err
		}
		return s.auditRepo.CreateTx(tx, &p.SubjectID, &id, "project.deleted", "", models.JSONMap{"retained_encrypted_history": true}, ip)
	})
}
