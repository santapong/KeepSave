package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type SecretService struct {
	secretRepo  *repository.SecretRepository
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	cryptoSvc   *crypto.Service
}

func NewSecretService(
	secretRepo *repository.SecretRepository,
	projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository,
	cryptoSvc *crypto.Service,
) *SecretService {
	return &SecretService{
		secretRepo:  secretRepo,
		projectRepo: projectRepo,
		envRepo:     envRepo,
		cryptoSvc:   cryptoSvc,
	}
}

func (s *SecretService) decryptProjectDEK(project *models.Project) ([]byte, error) {
	dek, err := s.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting project DEK: %w", err)
	}
	return dek, nil
}

func (s *SecretService) Create(projectID uuid.UUID, envName, key, value string) (*models.Secret, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	env, err := s.envRepo.GetByProjectAndName(projectID, envName)
	if err != nil {
		return nil, fmt.Errorf("getting environment: %w", err)
	}

	dek, err := s.decryptProjectDEK(project)
	if err != nil {
		return nil, err
	}

	encryptedValue, nonce, err := crypto.Encrypt(dek, []byte(value))
	if err != nil {
		return nil, fmt.Errorf("encrypting secret value: %w", err)
	}

	secret, err := s.secretRepo.Create(projectID, env.ID, key, encryptedValue, nonce)
	if err != nil {
		return nil, fmt.Errorf("creating secret: %w", err)
	}

	// Return without encrypted data, with the original value
	secret.Value = value
	secret.EncryptedValue = nil
	secret.ValueNonce = nil
	return secret, nil
}

func (s *SecretService) GetByID(projectID, secretID uuid.UUID) (*models.Secret, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	secret, err := s.secretRepo.GetByID(secretID)
	if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}

	if secret.ProjectID != projectID {
		return nil, fmt.Errorf("secret not found")
	}

	dek, err := s.decryptProjectDEK(project)
	if err != nil {
		return nil, err
	}

	plaintext, err := crypto.Decrypt(dek, secret.EncryptedValue, secret.ValueNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting secret value: %w", err)
	}

	secret.Value = string(plaintext)
	secret.EncryptedValue = nil
	secret.ValueNonce = nil
	return secret, nil
}

func (s *SecretService) List(projectID uuid.UUID, envName string) ([]models.Secret, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	env, err := s.envRepo.GetByProjectAndName(projectID, envName)
	if err != nil {
		return nil, fmt.Errorf("getting environment: %w", err)
	}

	secrets, err := s.secretRepo.ListByProjectAndEnv(projectID, env.ID)
	if err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}

	dek, err := s.decryptProjectDEK(project)
	if err != nil {
		return nil, err
	}

	for i := range secrets {
		plaintext, err := crypto.Decrypt(dek, secrets[i].EncryptedValue, secrets[i].ValueNonce)
		if err != nil {
			return nil, fmt.Errorf("decrypting secret %s: %w", secrets[i].Key, err)
		}
		secrets[i].Value = string(plaintext)
		secrets[i].EncryptedValue = nil
		secrets[i].ValueNonce = nil
	}

	return secrets, nil
}

func (s *SecretService) Update(projectID, secretID uuid.UUID, value string) (*models.Secret, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	existing, err := s.secretRepo.GetByID(secretID)
	if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}
	if existing.ProjectID != projectID {
		return nil, fmt.Errorf("secret not found")
	}

	dek, err := s.decryptProjectDEK(project)
	if err != nil {
		return nil, err
	}

	encryptedValue, nonce, err := crypto.Encrypt(dek, []byte(value))
	if err != nil {
		return nil, fmt.Errorf("encrypting secret value: %w", err)
	}

	secret, err := s.secretRepo.Update(secretID, encryptedValue, nonce)
	if err != nil {
		return nil, fmt.Errorf("updating secret: %w", err)
	}

	secret.Value = value
	secret.EncryptedValue = nil
	secret.ValueNonce = nil
	return secret, nil
}

func (s *SecretService) Delete(projectID, secretID uuid.UUID) error {
	existing, err := s.secretRepo.GetByID(secretID)
	if err != nil {
		return fmt.Errorf("getting secret: %w", err)
	}
	if existing.ProjectID != projectID {
		return fmt.Errorf("secret not found")
	}
	return s.secretRepo.Delete(secretID)
}
