package service

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// KeyRotationService handles master key rotation and re-encryption of secrets.
type KeyRotationService struct {
	projectRepo *repository.ProjectRepository
	secretRepo  *repository.SecretRepository
	envRepo     *repository.EnvironmentRepository
	auditRepo   *repository.AuditRepository
	cryptoSvc   *crypto.Service
}

// NewKeyRotationService creates a new key rotation service.
func NewKeyRotationService(
	projectRepo *repository.ProjectRepository,
	secretRepo *repository.SecretRepository,
	envRepo *repository.EnvironmentRepository,
	auditRepo *repository.AuditRepository,
	cryptoSvc *crypto.Service,
) *KeyRotationService {
	return &KeyRotationService{
		projectRepo: projectRepo,
		secretRepo:  secretRepo,
		envRepo:     envRepo,
		auditRepo:   auditRepo,
		cryptoSvc:   cryptoSvc,
	}
}

// RotateProjectKey generates a new DEK for a project and re-encrypts all secrets.
// Emits one key.dek_rotated audit row (docs/AUDIT_LOG_COVERAGE.md) on success.
func (s *KeyRotationService) RotateProjectKey(projectID, actorID uuid.UUID, ipAddr string) (*RotationResult, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	// Decrypt old DEK
	oldDEK, err := s.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting old DEK: %w", err)
	}
	defer crypto.SecureZero(oldDEK)

	// Generate new DEK
	newDEK, err := s.cryptoSvc.GenerateDEK()
	if err != nil {
		return nil, fmt.Errorf("generating new DEK: %w", err)
	}
	defer crypto.SecureZero(newDEK)

	// Encrypt new DEK with master key
	encryptedDEK, dekNonce, err := s.cryptoSvc.EncryptDEK(newDEK)
	if err != nil {
		return nil, fmt.Errorf("encrypting new DEK: %w", err)
	}

	// Get all environments for project
	envs, err := s.envRepo.ListByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("listing environments: %w", err)
	}

	// Read every secret BEFORE opening the transaction. Doing a pool query
	// inside the tx would need a second connection and deadlock under a
	// single-connection pool (and reads stale data on a larger pool).
	var toRotate []models.Secret
	for _, env := range envs {
		secrets, err := s.secretRepo.ListByProjectAndEnv(projectID, env.ID)
		if err != nil {
			return nil, fmt.Errorf("listing secrets for env %s: %w", env.Name, err)
		}
		toRotate = append(toRotate, secrets...)
	}

	reEncryptedCount := 0

	// Re-encrypt every secret AND swap the project DEK in one transaction so a
	// failure cannot leave the project split across two keys (ADR-0018, C-01).
	txErr := s.projectRepo.WithTx(func(tx *sql.Tx) error {
		for _, secret := range toRotate {
			plaintext, err := crypto.Decrypt(oldDEK, secret.EncryptedValue, secret.ValueNonce)
			if err != nil {
				return fmt.Errorf("decrypting secret %s: %w", secret.Key, err)
			}
			newCiphertext, newNonce, err := crypto.Encrypt(newDEK, plaintext)
			crypto.SecureZero(plaintext)
			if err != nil {
				return fmt.Errorf("re-encrypting secret %s: %w", secret.Key, err)
			}
			if err := s.secretRepo.UpdateValueTx(tx, secret.ID, newCiphertext, newNonce); err != nil {
				return fmt.Errorf("updating secret %s: %w", secret.Key, err)
			}
			reEncryptedCount++
		}
		if err := s.projectRepo.UpdateDEKTx(tx, projectID, encryptedDEK, dekNonce); err != nil {
			return fmt.Errorf("updating project DEK: %w", err)
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	// Environment column is empty: rotation spans every environment in the
	// project (same convention as project-scoped secret events).
	emitAudit(s.auditRepo, &actorID, &projectID, "key.dek_rotated", "",
		models.JSONMap{"secrets_rotated": reEncryptedCount, "environments": len(envs)}, ipAddr)

	return &RotationResult{
		ProjectID:        projectID,
		SecretsRotated:   reEncryptedCount,
		EnvironmentsUsed: len(envs),
	}, nil
}

// RotationResult contains the result of a key rotation.
type RotationResult struct {
	ProjectID        uuid.UUID `json:"project_id"`
	SecretsRotated   int       `json:"secrets_rotated"`
	EnvironmentsUsed int       `json:"environments_rotated"`
}

// RotateAllProjects rotates keys for all projects owned by a user.
// Audit shape: one key.dek_rotated row per project (no summary event —
// the rows share actor/IP/timestamp, which reconstructs the bulk run).
func (s *KeyRotationService) RotateAllProjects(ownerID uuid.UUID, ipAddr string) ([]RotationResult, error) {
	projects, err := s.projectRepo.ListByOwner(ownerID)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}

	var results []RotationResult
	for _, p := range projects {
		result, err := s.RotateProjectKey(p.ID, ownerID, ipAddr)
		if err != nil {
			return results, fmt.Errorf("rotating project %s (%s): %w", p.Name, p.ID, err)
		}
		results = append(results, *result)
	}

	return results, nil
}

// VerifyProjectEncryption checks that all secrets can be decrypted with the current DEK.
func (s *KeyRotationService) VerifyProjectEncryption(projectID uuid.UUID) ([]models.Secret, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	dek, err := s.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting DEK: %w", err)
	}

	envs, err := s.envRepo.ListByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("listing environments: %w", err)
	}

	var failedSecrets []models.Secret

	for _, env := range envs {
		secrets, err := s.secretRepo.ListByProjectAndEnv(projectID, env.ID)
		if err != nil {
			return nil, fmt.Errorf("listing secrets: %w", err)
		}

		for _, secret := range secrets {
			_, err := crypto.Decrypt(dek, secret.EncryptedValue, secret.ValueNonce)
			if err != nil {
				failedSecrets = append(failedSecrets, secret)
			}
		}
	}

	return failedSecrets, nil
}
