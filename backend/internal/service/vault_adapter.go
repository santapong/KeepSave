package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

func humanPrincipal(actor uuid.UUID) policy.Principal {
	return policy.Principal{Kind: policy.Human, SubjectID: actor, ActorID: actor}
}
func secretFromRecord(r vault.Record) models.Secret {
	return models.Secret{ID: r.ID, ProjectID: r.ProjectID, EnvironmentID: r.EnvironmentID, Key: r.Key, Revision: r.Revision, Value: r.Value, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}
func (s *SecretService) EnableVault(v *vault.Service) { s.vault = v }
func (s *SecretService) VaultEnabled() bool           { return s.vault != nil }
func (s *SecretService) CreateAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, environment, key, value, ip string) (*models.Secret, error) {
	if s.vault == nil {
		if err := s.legacyPermit(ctx, p, policy.WriteSecret, project, uuid.Nil, environment, key); err != nil {
			return nil, err
		}
		return s.Create(project, environment, key, value, p.ActorID, ip)
	}
	r, err := s.vault.Put(ctx, p, vault.Record{ProjectID: project, Environment: environment, Key: key, Value: value}, 0)
	if err != nil {
		return nil, err
	}
	r.Value = value
	secret := secretFromRecord(r)
	return &secret, nil
}
func (s *SecretService) GetByIDAuthorized(ctx context.Context, p policy.Principal, project, id uuid.UUID) (*models.Secret, error) {
	if s.vault == nil {
		return s.legacyReadAuthorized(ctx, p, project, id)
	}
	r, err := s.vault.Read(ctx, p, project, id, 0)
	if err != nil {
		return nil, err
	}
	secret := secretFromRecord(r)
	return &secret, nil
}
func (s *SecretService) GetMetadataAuthorized(ctx context.Context, p policy.Principal, project, id uuid.UUID, action policy.Action) (*models.Secret, error) {
	if s.vault == nil {
		key, env, err := s.projectRepo.SecretAccessMetadata(ctx, project, id)
		if err != nil {
			return nil, vault.ErrNotFound
		}
		if err = s.legacyPermit(ctx, p, action, project, id, env, key); err != nil {
			return nil, err
		}
		return s.secretRepo.GetByID(id)
	}
	r, err := s.vault.Metadata(ctx, p, project, id, action)
	if err != nil {
		return nil, err
	}
	secret := secretFromRecord(r)
	return &secret, nil
}
func (s *SecretService) ListAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, environment string) ([]models.Secret, error) {
	if s.vault == nil {
		return s.legacyListAuthorized(ctx, p, project, environment)
	}
	records, err := s.vault.List(ctx, p, project, environment)
	if err != nil {
		return nil, err
	}
	result := []models.Secret{}
	for _, r := range records {
		result = append(result, secretFromRecord(r))
	}
	return result, nil
}
func (s *SecretService) BatchAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, environment string, keys []string) ([]models.Secret, []string, error) {
	if s.vault == nil {
		if len(keys) == 0 || len(keys) > vault.MaxBatchKeys {
			return nil, nil, vault.ErrInvalid
		}
		seen := map[string]bool{}
		ordered := []string{}
		for _, key := range keys {
			if !vault.ValidKey(key) {
				return nil, nil, vault.ErrInvalid
			}
			if !seen[key] {
				seen[key] = true
				ordered = append(ordered, key)
			}
		}
		d, err := s.projectRepo.AuthorizeLegacy(ctx, p, policy.ReadValue, policy.Resource{ProjectID: project, Environment: environment, Type: "secret_collection"})
		if err != nil || !d.Allowed {
			return nil, nil, vault.ErrDenied
		}
		env, err := s.envRepo.GetByProjectAndName(project, environment)
		if err != nil {
			return nil, nil, vault.ErrNotFound
		}
		secrets := []models.Secret{}
		missing := []string{}
		for _, key := range ordered {
			metadata, e := s.secretRepo.GetByEnvAndKey(env.ID, key)
			if errors.Is(e, sql.ErrNoRows) {
				missing = append(missing, key)
				continue
			}
			if e != nil {
				return nil, nil, e
			}
			d, e = s.projectRepo.AuthorizeLegacy(ctx, p, policy.ReadValue, policy.Resource{ProjectID: project, Environment: environment, ID: metadata.ID, Key: key, Type: "secret"})
			if e != nil {
				return nil, nil, vault.ErrDenied
			}
			if !d.Allowed {
				missing = append(missing, key)
				continue
			}
			secret, e := s.legacyReadAuthorized(ctx, p, project, metadata.ID)
			if e != nil {
				return nil, nil, e
			}
			secrets = append(secrets, *secret)
		}
		encoded, err := json.Marshal(struct {
			Secrets []models.Secret `json:"secrets"`
			Missing []string        `json:"missing_keys"`
		}{secrets, missing})
		if err != nil {
			return nil, nil, err
		}
		if len(encoded) > vault.MaxReadBytes {
			return nil, nil, vault.ErrInvalid
		}
		return secrets, missing, nil
	}
	result, err := s.vault.BatchRead(ctx, p, project, environment, keys)
	if err != nil {
		return nil, nil, err
	}
	secrets := []models.Secret{}
	for _, r := range result.Secrets {
		secrets = append(secrets, secretFromRecord(r))
	}
	return secrets, result.MissingKeys, nil
}
func (s *SecretService) UpdateAuthorized(ctx context.Context, p policy.Principal, project, id uuid.UUID, value, ip string, expected *int64) (*models.Secret, error) {
	if s.vault == nil {
		if _, err := s.GetMetadataAuthorized(ctx, p, project, id, policy.WriteSecret); err != nil {
			return nil, err
		}
		return s.Update(project, id, value, p.ActorID, ip)
	}
	r, err := s.vault.UpdateCurrent(ctx, p, project, id, value, expected)
	if err != nil {
		return nil, err
	}
	r.Value = value
	secret := secretFromRecord(r)
	return &secret, nil
}
func (s *SecretService) DeleteAuthorized(ctx context.Context, p policy.Principal, project, id uuid.UUID, ip string, expected *int64) error {
	if s.vault == nil {
		if _, err := s.GetMetadataAuthorized(ctx, p, project, id, policy.DeleteSecret); err != nil {
			return err
		}
		return s.Delete(project, id, p.ActorID, ip)
	}
	return s.vault.DeleteCurrent(ctx, p, project, id, expected)
}

// ListResolvedAuthorized never expands a reference to a value the principal
// cannot read. Missing or denied dependencies fail without revealing values.
func (s *SecretService) ListResolvedAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, environment string) ([]models.Secret, error) {
	secrets, err := s.ListAuthorized(ctx, p, project, environment)
	if err != nil {
		return nil, err
	}
	raw := map[string]string{}
	for _, secret := range secrets {
		raw[secret.Key] = secret.Value
	}
	for _, secret := range secrets {
		for _, ref := range findReferences(secret.Value) {
			if _, allowed := raw[ref.key]; !allowed {
				return nil, vault.ErrDenied
			}
		}
	}
	resolved := ResolveEnvReferences(raw)
	for i := range secrets {
		secrets[i].Value = resolved[secrets[i].Key]
	}
	return secrets, nil
}

func (s *EnvFileService) EnableVault(v *vault.Service) { s.vault = v }
func (s *EnvFileService) ExportAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, environment string) (string, error) {
	if s.vault == nil {
		ss := NewSecretService(s.secretRepo, s.projectRepo, s.envRepo, s.auditRepo, s.cryptoSvc)
		secrets, err := ss.ListAuthorized(ctx, p, project, environment)
		if err != nil {
			return "", err
		}
		records := []vault.Record{}
		for _, secret := range secrets {
			records = append(records, vault.Record{Key: secret.Key, Value: secret.Value})
		}
		return formatEnvRecords(records), nil
	}
	records, err := s.vault.List(ctx, p, project, environment)
	if err != nil {
		return "", err
	}
	return formatEnvRecords(records), nil
}
func (s *EnvFileService) ImportAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, environment, content string, overwrite bool, ip string) (*ImportResult, error) {
	if s.vault == nil {
		values, err := parseEnvStrict(content)
		if err != nil {
			return nil, err
		}
		ss := NewSecretService(s.secretRepo, s.projectRepo, s.envRepo, s.auditRepo, s.cryptoSvc)
		for key := range values {
			if err = ss.legacyPermit(ctx, p, policy.WriteSecret, project, uuid.Nil, environment, key); err != nil {
				return nil, err
			}
		}
		return s.Import(project, environment, content, overwrite, p.ActorID, ip)
	}
	values, err := parseEnvStrict(content)
	if err != nil {
		return nil, err
	}
	result, err := s.vault.PutMany(ctx, p, project, environment, values, overwrite, "import")
	if err != nil {
		return nil, err
	}
	return &ImportResult{Created: result.Created, Updated: result.Updated, Skipped: result.Skipped}, nil
}
func (s *TemplateService) EnableVault(v *vault.Service) { s.vault = v }
func (s *TemplateService) ApplyTemplateAuthorized(ctx context.Context, p policy.Principal, template, project uuid.UUID, environment string) ([]models.Secret, error) {
	if s.vault == nil {
		return s.ApplyTemplate(template, project, environment, p.ActorID)
	}
	tmpl, err := s.templateRepo.GetByIDForUser(template, p.ActorID)
	if err != nil {
		return nil, ErrTemplateNotFound
	}
	items, ok := tmpl.Keys["keys"].([]interface{})
	if !ok {
		return nil, vault.ErrInvalid
	}
	values := map[string]string{}
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			return nil, vault.ErrInvalid
		}
		key, ok := entry["key"].(string)
		if !ok || strings.TrimSpace(key) == "" {
			return nil, vault.ErrInvalid
		}
		if _, exists := values[key]; exists {
			return nil, vault.ErrInvalid
		}
		value, _ := entry["default_value"].(string)
		if value == "" {
			value = "CHANGEME"
		}
		values[key] = value
	}
	result, err := s.vault.PutMany(ctx, p, project, environment, values, true, "template")
	if err != nil {
		return nil, err
	}
	secrets := []models.Secret{}
	for _, r := range result.Records {
		r.Value = values[r.Key]
		secrets = append(secrets, secretFromRecord(r))
	}
	return secrets, nil
}
func (s *KeyRotationService) EnableVault(v *vault.Service) { s.vault = v }
func (s *KeyRotationService) RotateProjectKeyAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, ip string) (*RotationResult, error) {
	if s.vault == nil {
		d, err := s.projectRepo.AuthorizeLegacy(ctx, p, policy.ManageProject, policy.Resource{ProjectID: project, Type: "project"})
		if err != nil || !d.Allowed {
			return nil, vault.ErrDenied
		}
		return s.RotateProjectKey(project, p.ActorID, ip)
	}
	envs, err := s.envRepo.ListByProjectID(project)
	if err != nil {
		return nil, err
	}
	count, err := s.vault.Rotate(ctx, p, project)
	if err != nil {
		return nil, err
	}
	return &RotationResult{ProjectID: project, SecretsRotated: count, EnvironmentsUsed: len(envs)}, nil
}
func (s *KeyRotationService) VerifyProjectEncryptionAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID) ([]models.Secret, error) {
	if s.vault == nil {
		d, err := s.projectRepo.AuthorizeLegacy(ctx, p, policy.ManageProject, policy.Resource{ProjectID: project, Type: "project"})
		if err != nil || !d.Allowed {
			return nil, vault.ErrDenied
		}
		return s.VerifyProjectEncryption(project)
	}
	envs, err := s.envRepo.ListByProjectID(project)
	if err != nil {
		return nil, err
	}
	for _, env := range envs {
		if _, err = s.vault.List(ctx, p, project, env.Name); err != nil {
			return nil, err
		}
	}
	return []models.Secret{}, nil
}

// RotateAllProjectsAuthorized reports successfully committed projects even if
// a later project fails. Callers must preserve the returned partial result.
func (s *KeyRotationService) RotateAllProjectsAuthorized(ctx context.Context, p policy.Principal, ip string) ([]RotationResult, error) {
	if p.Kind != policy.Human || p.ActorID == uuid.Nil {
		return nil, vault.ErrDenied
	}
	projects, err := s.projectRepo.ListByOwner(p.ActorID)
	if err != nil {
		return nil, err
	}
	results := []RotationResult{}
	for _, project := range projects {
		result, e := s.RotateProjectKeyAuthorized(ctx, p, project.ID, ip)
		if e != nil {
			return results, e
		}
		results = append(results, *result)
	}
	return results, nil
}
