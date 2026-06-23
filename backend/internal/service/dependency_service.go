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
	regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`),   // ${VAR_NAME}
	regexp.MustCompile(`\$([A-Z_][A-Z0-9_]*)`),       // $VAR_NAME
	regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*)\}\}`), // {{VAR_NAME}}
	regexp.MustCompile(`%([A-Z_][A-Z0-9_]*)%`),       // %VAR_NAME%
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

// maxResolveDepth bounds transitive secret-reference resolution so a deep or
// cyclic chain can never loop unbounded (ADR-0020).
const maxResolveDepth = 16

// ResolveEnvReferences resolves ${VAR}/$VAR/{{VAR}}/%VAR% references in each
// value against the other keys in the same environment, transitively. Cycles,
// unknown keys, and chains deeper than maxResolveDepth are left as the literal
// token rather than erroring — resolution must never fail a read.
func ResolveEnvReferences(raw map[string]string) map[string]string {
	out := make(map[string]string, len(raw))
	for key, val := range raw {
		out[key] = resolveOne(val, raw, map[string]bool{key: true}, 0)
	}
	return out
}

func resolveOne(value string, raw map[string]string, stack map[string]bool, depth int) string {
	if depth >= maxResolveDepth {
		return value
	}
	return substituteRefs(value, func(refKey string) (string, bool) {
		refVal, known := raw[refKey]
		if !known || stack[refKey] {
			return "", false // unknown key or cycle: keep the literal token
		}
		stack[refKey] = true
		resolved := resolveOne(refVal, raw, stack, depth+1)
		delete(stack, refKey)
		return resolved, true
	})
}

// substituteRefs replaces each reference token via lookup, leaving the literal
// token when lookup reports the key is unresolvable (unknown or cyclic).
func substituteRefs(value string, lookup func(key string) (string, bool)) string {
	for _, re := range referencePatterns {
		value = re.ReplaceAllStringFunc(value, func(match string) string {
			sub := re.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			if v, ok := lookup(sub[1]); ok {
				return v
			}
			return match
		})
	}
	return value
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
