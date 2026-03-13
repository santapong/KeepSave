package service

import (
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
	cryptoSvc   *crypto.Service
}

// NewKeyRotationService creates a new key rotation service.
func NewKeyRotationService(
	projectRepo *repository.ProjectRepository,
	secretRepo *repository.SecretRepository,
	envRepo *repository.EnvironmentRepository,
	cryptoSvc *crypto.Service,
) *KeyRotationService {
	return &KeyRotationService{
		projectRepo: projectRepo,
		secretRepo:  secretRepo,
		envRepo:     envRepo,
		cryptoSvc:   cryptoSvc,
	}
}

// RotateProjectKey generates a new DEK for a project and re-encrypts all secrets.
func (s *KeyRotationService) RotateProjectKey(projectID uuid.UUID) (*RotationResult, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	// Decrypt old DEK
	oldDEK, err := s.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting old DEK: %w", err)
	}

	// Generate new DEK
	newDEK, err := s.cryptoSvc.GenerateDEK()
	if err != nil {
		return nil, fmt.Errorf("generating new DEK: %w", err)
	}

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

	reEncryptedCount := 0

	// Re-encrypt all secrets in every environment
	for _, env := range envs {
		secrets, err := s.secretRepo.ListByProjectAndEnv(projectID, env.ID)
		if err != nil {
			return nil, fmt.Errorf("listing secrets for env %s: %w", env.Name, err)
		}

		for _, secret := range secrets {
			// Decrypt with old DEK
			plaintext, err := crypto.Decrypt(oldDEK, secret.EncryptedValue, secret.ValueNonce)
			if err != nil {
				return nil, fmt.Errorf("decrypting secret %s: %w", secret.Key, err)
			}

			// Re-encrypt with new DEK
			newCiphertext, newNonce, err := crypto.Encrypt(newDEK, plaintext)
			if err != nil {
				return nil, fmt.Errorf("re-encrypting secret %s: %w", secret.Key, err)
			}

			// Update secret
			_, err = s.secretRepo.Update(secret.ID, newCiphertext, newNonce)
			if err != nil {
				return nil, fmt.Errorf("updating secret %s: %w", secret.Key, err)
			}

			reEncryptedCount++
		}
	}

	// Update project DEK
	if err := s.projectRepo.UpdateDEK(projectID, encryptedDEK, dekNonce); err != nil {
		return nil, fmt.Errorf("updating project DEK: %w", err)
	}

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
func (s *KeyRotationService) RotateAllProjects(ownerID uuid.UUID) ([]RotationResult, error) {
	projects, err := s.projectRepo.ListByOwner(ownerID)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}

	var results []RotationResult
	for _, p := range projects {
		result, err := s.RotateProjectKey(p.ID)
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
