package service

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type EnvFileService struct {
	secretRepo  *repository.SecretRepository
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	cryptoSvc   *crypto.Service
}

func NewEnvFileService(
	secretRepo *repository.SecretRepository,
	projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository,
	cryptoSvc *crypto.Service,
) *EnvFileService {
	return &EnvFileService{
		secretRepo:  secretRepo,
		projectRepo: projectRepo,
		envRepo:     envRepo,
		cryptoSvc:   cryptoSvc,
	}
}

// Export generates a .env file content from secrets in a project environment.
func (s *EnvFileService) Export(projectID uuid.UUID, envName string) (string, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return "", fmt.Errorf("getting project: %w", err)
	}

	env, err := s.envRepo.GetByProjectAndName(projectID, envName)
	if err != nil {
		return "", fmt.Errorf("getting environment: %w", err)
	}

	secrets, err := s.secretRepo.ListByProjectAndEnv(projectID, env.ID)
	if err != nil {
		return "", fmt.Errorf("listing secrets: %w", err)
	}

	dek, err := s.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		return "", fmt.Errorf("decrypting project DEK: %w", err)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("# KeepSave export - Project: %s, Environment: %s", project.Name, envName))
	lines = append(lines, "")

	for _, sec := range secrets {
		plaintext, err := crypto.Decrypt(dek, sec.EncryptedValue, sec.ValueNonce)
		if err != nil {
			return "", fmt.Errorf("decrypting secret %s: %w", sec.Key, err)
		}
		value := string(plaintext)
		// Quote values that contain spaces, #, or newlines
		if strings.ContainsAny(value, " #\n\r\t\"'") {
			value = "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
		}
		lines = append(lines, fmt.Sprintf("%s=%s", sec.Key, value))
	}

	return strings.Join(lines, "\n") + "\n", nil
}

// Import parses .env file content and creates/updates secrets in a project environment.
func (s *EnvFileService) Import(projectID uuid.UUID, envName, content string, overwrite bool) (*ImportResult, error) {
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

	vars := parseEnvContent(content)
	result := &ImportResult{}

	for key, value := range vars {
		// Check if key already exists
		existing, err := s.secretRepo.GetByEnvAndKey(env.ID, key)
		if err == nil && existing != nil {
			if !overwrite {
				result.Skipped = append(result.Skipped, key)
				continue
			}
			// Update existing
			encryptedValue, nonce, err := crypto.Encrypt(dek, []byte(value))
			if err != nil {
				return nil, fmt.Errorf("encrypting value for %s: %w", key, err)
			}
			if _, err := s.secretRepo.Update(existing.ID, encryptedValue, nonce); err != nil {
				return nil, fmt.Errorf("updating secret %s: %w", key, err)
			}
			result.Updated = append(result.Updated, key)
		} else {
			// Create new
			encryptedValue, nonce, err := crypto.Encrypt(dek, []byte(value))
			if err != nil {
				return nil, fmt.Errorf("encrypting value for %s: %w", key, err)
			}
			if _, err := s.secretRepo.Create(projectID, env.ID, key, encryptedValue, nonce); err != nil {
				return nil, fmt.Errorf("creating secret %s: %w", key, err)
			}
			result.Created = append(result.Created, key)
		}
	}

	return result, nil
}

type ImportResult struct {
	Created []string `json:"created"`
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
}

func parseEnvContent(content string) map[string]string {
	vars := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Skip export prefix
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimPrefix(line, "export ")
			line = strings.TrimSpace(line)
		}

		// Split on first =
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Remove surrounding quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Unescape quotes
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\'`, `'`)

		if key != "" {
			vars[key] = value
		}
	}

	return vars
}
