package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// NLPQueryService handles natural language secret queries via AI providers.
type NLPQueryService struct {
	db          *sql.DB
	dialect     repository.Dialect
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	secretRepo  *repository.SecretRepository
	aiMgr       *AIProviderManager
}

func NewNLPQueryService(
	db *sql.DB, dialect repository.Dialect,
	projectRepo *repository.ProjectRepository, envRepo *repository.EnvironmentRepository,
	secretRepo *repository.SecretRepository, aiMgr *AIProviderManager,
) *NLPQueryService {
	return &NLPQueryService{db: db, dialect: dialect, projectRepo: projectRepo, envRepo: envRepo, secretRepo: secretRepo, aiMgr: aiMgr}
}

// Query processes a natural language query about secrets.
func (s *NLPQueryService) Query(userID uuid.UUID, query string) (*models.NLPQueryResult, error) {
	if s.aiMgr == nil || !s.aiMgr.HasProvider() {
		// Fallback to fuzzy matching without AI
		return s.fuzzyQuery(userID, query)
	}

	// Gather context: list user's projects and their secret keys
	projects, err := s.projectRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	var context strings.Builder
	context.WriteString("Available projects and their secret keys:\n")
	for _, p := range projects {
		context.WriteString(fmt.Sprintf("\nProject: %s (ID: %s)\n", p.Name, p.ID))
		envs, err := s.envRepo.ListByProject(p.ID)
		if err != nil {
			continue
		}
		for _, env := range envs {
			secrets, err := s.secretRepo.ListByEnvironment(p.ID, env.ID)
			if err != nil {
				continue
			}
			var keys []string
			for _, sec := range secrets {
				keys = append(keys, sec.Key)
			}
			if len(keys) > 0 {
				context.WriteString(fmt.Sprintf("  Environment %s: %s\n", env.Name, strings.Join(keys, ", ")))
			}
		}
	}

	systemPrompt := `You are KeepSave's AI assistant for natural language secret queries. 
Given the user's projects and secret keys, answer their query.
Return ONLY a JSON object with these fields:
- "intent": one of "find_secret", "describe_project", "list_env", "suggest"
- "matched_secrets": array of {"project_name": "...", "project_id": "...", "environment": "...", "key": "...", "score": 0.0-1.0, "reason": "..."}
- "explanation": brief human-readable answer
- "suggestions": array of follow-up suggestion strings
Never include actual secret values. Only match by key names.`

	userPrompt := fmt.Sprintf("Context:\n%s\n\nUser query: %s", context.String(), query)

	resp, provider, model, err := s.aiMgr.Chat(systemPrompt, userPrompt)
	if err != nil {
		// Fallback to fuzzy
		return s.fuzzyQuery(userID, query)
	}

	// Parse AI response
	result := &models.NLPQueryResult{
		Query:    query,
		Provider: provider,
		Model:    model,
	}

	clean := extractJSONObject(resp)
	var parsed struct {
		Intent         string `json:"intent"`
		MatchedSecrets []struct {
			ProjectName string  `json:"project_name"`
			ProjectID   string  `json:"project_id"`
			Environment string  `json:"environment"`
			Key         string  `json:"key"`
			Score       float64 `json:"score"`
			Reason      string  `json:"reason"`
		} `json:"matched_secrets"`
		Explanation string   `json:"explanation"`
		Suggestions []string `json:"suggestions"`
	}

	if json.Unmarshal([]byte(clean), &parsed) == nil {
		result.Intent = parsed.Intent
		result.Explanation = parsed.Explanation
		result.Suggestions = parsed.Suggestions
		for _, m := range parsed.MatchedSecrets {
			pid, _ := uuid.Parse(m.ProjectID)
			result.MatchedSecrets = append(result.MatchedSecrets, models.NLPSecretMatch{
				ProjectID: pid, ProjectName: m.ProjectName,
				Environment: m.Environment, Key: m.Key,
				Score: m.Score, Reason: m.Reason,
			})
		}
	} else {
		result.Intent = "suggest"
		result.Explanation = resp
	}

	// Log query
	s.db.Exec(`INSERT INTO nlp_query_log (id, user_id, query, intent, provider, model, matched_count, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		uuid.New(), userID, query, result.Intent, provider, model, len(result.MatchedSecrets), time.Now())

	return result, nil
}

// fuzzyQuery does keyword-based matching without AI.
func (s *NLPQueryService) fuzzyQuery(userID uuid.UUID, query string) (*models.NLPQueryResult, error) {
	projects, err := s.projectRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	terms := strings.Fields(strings.ToLower(query))
	result := &models.NLPQueryResult{
		Query:  query,
		Intent: "find_secret",
	}

	for _, p := range projects {
		envs, _ := s.envRepo.ListByProject(p.ID)
		for _, env := range envs {
			secrets, _ := s.secretRepo.ListByEnvironment(p.ID, env.ID)
			for _, sec := range secrets {
				keyLower := strings.ToLower(sec.Key)
				for _, term := range terms {
					if strings.Contains(keyLower, term) || strings.Contains(strings.ToLower(p.Name), term) {
						result.MatchedSecrets = append(result.MatchedSecrets, models.NLPSecretMatch{
							ProjectID: p.ID, ProjectName: p.Name,
							Environment: env.Name, Key: sec.Key,
							Score: 0.7, Reason: fmt.Sprintf("Matched term '%s'", term),
						})
						break
					}
				}
			}
		}
	}

	result.Explanation = fmt.Sprintf("Found %d matching secrets using keyword search (no AI provider configured)", len(result.MatchedSecrets))
	result.Provider = "builtin"
	result.Model = "keyword-fuzzy"
	return result, nil
}

func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
