package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/santapong/KeepSave/backend/internal/vault"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type SecretService struct {
	vault       *vault.Service
	secretRepo  *repository.SecretRepository
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	auditRepo   *repository.AuditRepository
	cryptoSvc   *crypto.Service
}

func NewSecretService(
	secretRepo *repository.SecretRepository,
	projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository,
	auditRepo *repository.AuditRepository,
	cryptoSvc *crypto.Service,
) *SecretService {
	return &SecretService{
		secretRepo:  secretRepo,
		projectRepo: projectRepo,
		envRepo:     envRepo,
		auditRepo:   auditRepo,
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

func (s *SecretService) Create(projectID uuid.UUID, envName, key, value string, actorID uuid.UUID, ipAddr string) (*models.Secret, error) {
	if s.vault != nil {
		return s.CreateAuthorized(context.Background(), humanPrincipal(actorID), projectID, envName, key, value, ipAddr)
	}
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

	emitAudit(s.auditRepo, &actorID, &projectID, "secret.created", envName,
		models.JSONMap{"secret_key": key, "secret_id": secret.ID.String()}, ipAddr)

	// Return without encrypted data, with the original value
	secret.Value = value
	secret.EncryptedValue = nil
	secret.ValueNonce = nil
	return secret, nil
}

func (s *SecretService) GetByID(projectID, secretID uuid.UUID) (*models.Secret, error) {
	if s.vault != nil {
		return nil, vault.ErrDenied
	}
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
	if s.vault != nil {
		return nil, vault.ErrDenied
	}
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

// ListResolved is List with secret-reference interpolation applied (ADR-0020):
// ${VAR} and the other supported tokens are replaced with the referenced key's
// value within the same environment, transitively. It is opt-in (the raw List
// is unchanged) and is deliberately NOT used by the promotion/diff path, which
// must operate on the stored (raw) ciphertext so resolution can't leak a
// referenced value across an environment boundary.
func (s *SecretService) ListResolved(projectID uuid.UUID, envName string) ([]models.Secret, error) {
	secrets, err := s.List(projectID, envName)
	if err != nil {
		return nil, err
	}
	raw := make(map[string]string, len(secrets))
	for _, sec := range secrets {
		raw[sec.Key] = sec.Value
	}
	resolved := ResolveEnvReferences(raw)
	for i := range secrets {
		secrets[i].Value = resolved[secrets[i].Key]
	}
	return secrets, nil
}

func (s *SecretService) Update(projectID, secretID uuid.UUID, value string, actorID uuid.UUID, ipAddr string) (*models.Secret, error) {
	if s.vault != nil {
		return s.UpdateAuthorized(context.Background(), humanPrincipal(actorID), projectID, secretID, value, ipAddr, nil)
	}
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

	emitAudit(s.auditRepo, &actorID, &projectID, "secret.updated", "",
		models.JSONMap{"secret_key": existing.Key, "secret_id": secretID.String()}, ipAddr)

	secret.Value = value
	secret.EncryptedValue = nil
	secret.ValueNonce = nil
	return secret, nil
}

func (s *SecretService) Delete(projectID, secretID uuid.UUID, actorID uuid.UUID, ipAddr string) error {
	if s.vault != nil {
		return s.DeleteAuthorized(context.Background(), humanPrincipal(actorID), projectID, secretID, ipAddr, nil)
	}
	existing, err := s.secretRepo.GetByID(secretID)
	if err != nil {
		return fmt.Errorf("getting secret: %w", err)
	}
	if existing.ProjectID != projectID {
		return fmt.Errorf("secret not found")
	}
	if err := s.secretRepo.Delete(secretID); err != nil {
		return err
	}
	emitAudit(s.auditRepo, &actorID, &projectID, "secret.deleted", "",
		models.JSONMap{"secret_key": existing.Key, "secret_id": secretID.String()}, ipAddr)
	return nil
}

// legacyPermit loads current identity, project, environment and key scope before
// a SQLite/MySQL compatibility adapter obtains any plaintext.
func (s *SecretService) legacyPermit(ctx context.Context, p policy.Principal, action policy.Action, project, id uuid.UUID, environment, key string) error {
	decision, err := s.projectRepo.AuthorizeLegacy(ctx, p, action, policy.Resource{ProjectID: project, ID: id, Environment: environment, Key: key, Type: "secret"})
	if err != nil || !decision.Allowed {
		return vault.ErrDenied
	}
	return nil
}
func (s *SecretService) legacyReadAuthorized(ctx context.Context, p policy.Principal, project, id uuid.UUID) (*models.Secret, error) {
	key, environment, err := s.projectRepo.SecretAccessMetadata(ctx, project, id)
	if err != nil {
		return nil, vault.ErrNotFound
	}
	if err = s.legacyPermit(ctx, p, policy.ReadValue, project, id, environment, key); err != nil {
		return nil, err
	}
	return s.GetByID(project, id)
}
func (s *SecretService) legacyListAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, environment string) ([]models.Secret, error) {
	decision, err := s.projectRepo.AuthorizeLegacy(ctx, p, policy.ReadValue, policy.Resource{ProjectID: project, Environment: environment, Type: "secret_collection"})
	if err != nil || !decision.Allowed {
		return nil, vault.ErrDenied
	}
	env, err := s.envRepo.GetByProjectAndName(project, environment)
	if err != nil {
		return nil, err
	}
	records, err := s.secretRepo.ListByProjectAndEnv(project, env.ID)
	if err != nil {
		return nil, err
	}
	meta, err := s.projectRepo.GetByID(project)
	if err != nil {
		return nil, err
	}
	allowed := []models.Secret{}
	for _, record := range records {
		decision, err := s.projectRepo.AuthorizeLegacy(ctx, p, policy.ReadValue, policy.Resource{ProjectID: project, ID: record.ID, Environment: environment, Key: record.Key, Type: "secret"})
		if err != nil {
			return nil, vault.ErrDenied
		}
		if decision.Allowed {
			allowed = append(allowed, record)
		}
	}
	if len(allowed) == 0 {
		return allowed, nil
	}
	dek, err := s.decryptProjectDEK(meta)
	if err != nil {
		return nil, err
	}
	defer crypto.SecureZero(dek)
	for i := range allowed {
		plain, err := crypto.Decrypt(dek, allowed[i].EncryptedValue, allowed[i].ValueNonce)
		if err != nil {
			return nil, err
		}
		allowed[i].Value = string(plain)
		crypto.SecureZero(plain)
		allowed[i].EncryptedValue = nil
		allowed[i].ValueNonce = nil
	}
	encoded, err := json.Marshal(allowed)
	if err != nil {
		return nil, err
	}
	if len(encoded) > vault.MaxReadBytes {
		return nil, vault.ErrInvalid
	}
	return allowed, nil
}
