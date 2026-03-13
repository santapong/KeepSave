package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// referencePatterns matches common patterns for secret references within values.
var referencePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`),          // ${VAR_NAME}
	regexp.MustCompile(`\$([A-Z_][A-Z0-9_]*)`),              // $VAR_NAME
	regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*)\}\}`),        // {{VAR_NAME}}
	regexp.MustCompile(`%([A-Z_][A-Z0-9_]*)%`),              // %VAR_NAME%
}

type DependencyService struct {
	depRepo     *repository.DependencyRepository
	secretRepo  *repository.SecretRepository
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	cryptoSvc   *crypto.Service
}

func NewDependencyService(
	depRepo *repository.DependencyRepository,
	secretRepo *repository.SecretRepository,
	projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository,
	cryptoSvc *crypto.Service,
) *DependencyService {
	return &DependencyService{
		depRepo:     depRepo,
		secretRepo:  secretRepo,
		projectRepo: projectRepo,
		envRepo:     envRepo,
		cryptoSvc:   cryptoSvc,
	}
}

// AnalyzeDependencies scans all secrets in an environment and detects references.
func (s *DependencyService) AnalyzeDependencies(projectID uuid.UUID, envName string) ([]models.SecretDependency, error) {
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

	dek, err := s.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting project DEK: %w", err)
	}

	// Build set of known keys
	knownKeys := make(map[string]bool)
	for _, sec := range secrets {
		knownKeys[sec.Key] = true
	}

	// Clear existing dependencies for this environment
	if err := s.depRepo.DeleteByProjectAndEnv(projectID, env.ID); err != nil {
		return nil, fmt.Errorf("clearing existing dependencies: %w", err)
	}

	var deps []models.SecretDependency
	for _, sec := range secrets {
		plaintext, err := crypto.Decrypt(dek, sec.EncryptedValue, sec.ValueNonce)
		if err != nil {
			continue // skip secrets that can't be decrypted
		}

		value := string(plaintext)
		refs := findReferences(value)
		for _, ref := range refs {
			if ref.key == sec.Key {
				continue // skip self-references
			}
			if !knownKeys[ref.key] {
				continue // skip references to unknown keys
			}
			dep, err := s.depRepo.Create(projectID, env.ID, sec.Key, ref.key, ref.pattern)
			if err != nil {
				return nil, fmt.Errorf("creating dependency: %w", err)
			}
			deps = append(deps, *dep)
		}
	}

	return deps, nil
}

// GetDependencyGraph returns the full dependency graph for an environment.
func (s *DependencyService) GetDependencyGraph(projectID uuid.UUID, envName string) ([]models.DependencyNode, error) {
	env, err := s.envRepo.GetByProjectAndName(projectID, envName)
	if err != nil {
		return nil, fmt.Errorf("getting environment: %w", err)
	}

	deps, err := s.depRepo.ListByProjectAndEnv(projectID, env.ID)
	if err != nil {
		return nil, fmt.Errorf("listing dependencies: %w", err)
	}

	// Build adjacency lists
	dependsOn := make(map[string][]string)
	referencedBy := make(map[string][]string)
	allKeys := make(map[string]bool)

	for _, dep := range deps {
		allKeys[dep.SecretKey] = true
		allKeys[dep.DependsOnKey] = true
		dependsOn[dep.SecretKey] = append(dependsOn[dep.SecretKey], dep.DependsOnKey)
		referencedBy[dep.DependsOnKey] = append(referencedBy[dep.DependsOnKey], dep.SecretKey)
	}

	// Also include secrets with no dependencies
	secrets, err := s.secretRepo.ListByProjectAndEnv(projectID, env.ID)
	if err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}
	for _, sec := range secrets {
		allKeys[sec.Key] = true
	}

	var nodes []models.DependencyNode
	for key := range allKeys {
		node := models.DependencyNode{
			Key:          key,
			DependsOn:    dependsOn[key],
			ReferencedBy: referencedBy[key],
		}
		if node.DependsOn == nil {
			node.DependsOn = []string{}
		}
		if node.ReferencedBy == nil {
			node.ReferencedBy = []string{}
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

type reference struct {
	key     string
	pattern string
}

func findReferences(value string) []reference {
	seen := make(map[string]bool)
	var refs []reference

	for _, pattern := range referencePatterns {
		matches := pattern.FindAllStringSubmatch(value, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			key := strings.TrimSpace(match[1])
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			refs = append(refs, reference{key: key, pattern: match[0]})
		}
	}

	return refs
}
