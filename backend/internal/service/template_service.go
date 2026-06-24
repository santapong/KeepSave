package service

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// ErrTemplateNotFound is returned when a template does not exist or the caller
// may not read/modify it (deliberately indistinguishable — handlers map it to
// 404). ErrTemplateProjectAccess is returned by ApplyTemplate when the caller
// lacks access to the target project (handlers map it to 403).
var (
	ErrTemplateNotFound      = errors.New("template not found")
	ErrTemplateProjectAccess = errors.New("project access denied")
)

type TemplateService struct {
	templateRepo *repository.TemplateRepository
	secretRepo   *repository.SecretRepository
	projectRepo  *repository.ProjectRepository
	envRepo      *repository.EnvironmentRepository
	auditRepo    *repository.AuditRepository
	cryptoSvc    *crypto.Service
}

func NewTemplateService(
	templateRepo *repository.TemplateRepository,
	secretRepo *repository.SecretRepository,
	projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository,
	auditRepo *repository.AuditRepository,
	cryptoSvc *crypto.Service,
) *TemplateService {
	return &TemplateService{
		templateRepo: templateRepo,
		secretRepo:   secretRepo,
		projectRepo:  projectRepo,
		envRepo:      envRepo,
		auditRepo:    auditRepo,
		cryptoSvc:    cryptoSvc,
	}
}

func (s *TemplateService) Create(name, description, stack string, keys models.JSONMap, createdBy uuid.UUID, orgID *uuid.UUID, isGlobal bool, ipAddr string) (*models.SecretTemplate, error) {
	if name == "" {
		return nil, fmt.Errorf("template name is required")
	}
	tmpl, err := s.templateRepo.Create(name, description, stack, keys, createdBy, orgID, isGlobal)
	if err != nil {
		return nil, err
	}
	emitAudit(s.auditRepo, &createdBy, nil, "template.created", "",
		models.JSONMap{"template_id": tmpl.ID.String(), "name": name}, ipAddr)
	return tmpl, nil
}

// GetByID returns a template only if userID may read it (global, owned, or in
// the user's org). Not-readable and not-existing both surface as
// ErrTemplateNotFound.
func (s *TemplateService) GetByID(id, userID uuid.UUID) (*models.SecretTemplate, error) {
	tmpl, err := s.templateRepo.GetByIDForUser(id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	return tmpl, nil
}

func (s *TemplateService) List(userID uuid.UUID, orgID *uuid.UUID) ([]models.SecretTemplate, error) {
	if orgID != nil {
		return s.templateRepo.ListByOrganization(*orgID)
	}
	return s.templateRepo.ListByUser(userID)
}

// Update mutates a template only when actorID owns it (created it). A non-owner
// gets ErrTemplateNotFound and no mutation occurs.
func (s *TemplateService) Update(id uuid.UUID, name, description, stack string, keys models.JSONMap, actorID uuid.UUID, ipAddr string) (*models.SecretTemplate, error) {
	tmpl, err := s.templateRepo.Update(id, name, description, stack, keys, actorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	emitAudit(s.auditRepo, &actorID, nil, "template.updated", "",
		models.JSONMap{"template_id": id.String(), "name": name}, ipAddr)
	return tmpl, nil
}

// Delete removes a template only when actorID owns it.
func (s *TemplateService) Delete(id uuid.UUID, actorID uuid.UUID, ipAddr string) error {
	if err := s.templateRepo.Delete(id, actorID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTemplateNotFound
		}
		return err
	}
	emitAudit(s.auditRepo, &actorID, nil, "template.deleted", "",
		models.JSONMap{"template_id": id.String()}, ipAddr)
	return nil
}

// ApplyTemplate creates secrets in a project environment based on a template.
// userID must have access to the target project (else ErrTemplateProjectAccess)
// and be able to read the template (else ErrTemplateNotFound) — without these
// checks any authenticated user could write CHANGEME secrets into another
// tenant's project, or apply a template they cannot see.
func (s *TemplateService) ApplyTemplate(templateID, projectID uuid.UUID, envName string, userID uuid.UUID) ([]models.Secret, error) {
	allowed, err := s.projectRepo.UserHasAccess(userID, projectID)
	if err != nil || !allowed {
		return nil, ErrTemplateProjectAccess
	}

	tmpl, err := s.templateRepo.GetByIDForUser(templateID, userID)
	if err != nil {
		return nil, ErrTemplateNotFound
	}

	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	env, err := s.envRepo.GetByProjectAndName(projectID, envName)
	if err != nil {
		return nil, fmt.Errorf("getting environment: %w", err)
	}

	dek, err := s.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting project DEK: %w", err)
	}

	// Parse template keys - expected format: [{"key": "...", "default_value": "...", ...}]
	keysRaw, ok := tmpl.Keys["keys"]
	if !ok {
		return nil, fmt.Errorf("template has no keys defined")
	}

	keysList, ok := keysRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid template keys format")
	}

	var created []models.Secret
	for _, item := range keysList {
		keyMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		keyName, _ := keyMap["key"].(string)
		defaultValue, _ := keyMap["default_value"].(string)
		if keyName == "" {
			continue
		}
		if defaultValue == "" {
			defaultValue = "CHANGEME"
		}

		encryptedValue, nonce, err := crypto.Encrypt(dek, []byte(defaultValue))
		if err != nil {
			return nil, fmt.Errorf("encrypting template value for %s: %w", keyName, err)
		}

		secret, err := s.secretRepo.Upsert(projectID, env.ID, keyName, encryptedValue, nonce)
		if err != nil {
			return nil, fmt.Errorf("creating secret %s from template: %w", keyName, err)
		}
		secret.Value = defaultValue
		secret.EncryptedValue = nil
		secret.ValueNonce = nil
		created = append(created, *secret)
	}

	return created, nil
}

// GetBuiltinTemplates returns predefined templates for common stacks.
func GetBuiltinTemplates() []models.SecretTemplate {
	return []models.SecretTemplate{
		{
			Name:        "Node.js Web App",
			Description: "Common environment variables for a Node.js web application",
			Stack:       "nodejs",
			IsGlobal:    true,
			Keys: models.JSONMap{
				"keys": []map[string]interface{}{
					{"key": "NODE_ENV", "description": "Runtime environment", "default_value": "development", "required": true},
					{"key": "PORT", "description": "Server port", "default_value": "3000", "required": true},
					{"key": "DATABASE_URL", "description": "Database connection string", "default_value": "", "required": true},
					{"key": "REDIS_URL", "description": "Redis connection string", "default_value": "", "required": false},
					{"key": "JWT_SECRET", "description": "JWT signing secret", "default_value": "", "required": true},
					{"key": "LOG_LEVEL", "description": "Logging level", "default_value": "info", "required": false},
				},
			},
		},
		{
			Name:        "Python Django",
			Description: "Common environment variables for a Django application",
			Stack:       "python",
			IsGlobal:    true,
			Keys: models.JSONMap{
				"keys": []map[string]interface{}{
					{"key": "DJANGO_SETTINGS_MODULE", "description": "Django settings module", "default_value": "config.settings", "required": true},
					{"key": "SECRET_KEY", "description": "Django secret key", "default_value": "", "required": true},
					{"key": "DEBUG", "description": "Debug mode", "default_value": "False", "required": true},
					{"key": "DATABASE_URL", "description": "Database connection string", "default_value": "", "required": true},
					{"key": "ALLOWED_HOSTS", "description": "Allowed hosts", "default_value": "*", "required": true},
					{"key": "STATIC_URL", "description": "Static files URL", "default_value": "/static/", "required": false},
				},
			},
		},
		{
			Name:        "Go Microservice",
			Description: "Common environment variables for a Go microservice",
			Stack:       "go",
			IsGlobal:    true,
			Keys: models.JSONMap{
				"keys": []map[string]interface{}{
					{"key": "PORT", "description": "Server port", "default_value": "8080", "required": true},
					{"key": "DATABASE_URL", "description": "Database connection string", "default_value": "", "required": true},
					{"key": "LOG_LEVEL", "description": "Logging level", "default_value": "info", "required": false},
					{"key": "GRPC_PORT", "description": "gRPC port", "default_value": "9090", "required": false},
					{"key": "METRICS_PORT", "description": "Prometheus metrics port", "default_value": "2112", "required": false},
				},
			},
		},
		{
			Name:        "AWS Services",
			Description: "AWS credentials and common service configuration",
			Stack:       "aws",
			IsGlobal:    true,
			Keys: models.JSONMap{
				"keys": []map[string]interface{}{
					{"key": "AWS_ACCESS_KEY_ID", "description": "AWS access key", "default_value": "", "required": true},
					{"key": "AWS_SECRET_ACCESS_KEY", "description": "AWS secret key", "default_value": "", "required": true},
					{"key": "AWS_REGION", "description": "AWS region", "default_value": "us-east-1", "required": true},
					{"key": "AWS_S3_BUCKET", "description": "S3 bucket name", "default_value": "", "required": false},
					{"key": "AWS_SQS_QUEUE_URL", "description": "SQS queue URL", "default_value": "", "required": false},
				},
			},
		},
	}
}
