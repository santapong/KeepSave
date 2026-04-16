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

type NLPQueryService struct {
	db          *sql.DB
	dialect     repository.Dialect
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	secretRepo  *repository.SecretRepository
	aiMgr       *AIProviderManager
}

func NewNLPQueryService(db *sql.DB, dialect repository.Dialect, projectRepo *repository.ProjectRepository, envRepo *repository.EnvironmentRepository, secretRepo *repository.SecretRepository, aiMgr *AIProviderManager) *NLPQueryService {
	return &NLPQueryService{db: db, dialect: dialect, projectRepo: projectRepo, envRepo: envRepo, secretRepo: secretRepo, aiMgr: aiMgr}
}

func (s *NLPQueryService) buildContext(userID uuid.UUID) string {
	projects, err := s.projectRepo.ListByOwnerID(userID)
	if err != nil { return "" }
	var b strings.Builder
	b.WriteString("Available projects and their secret keys:\n")
	for _, p := range projects {
		b.WriteString(fmt.Sprintf("\nProject: %s (ID: %s)\n", p.Name, p.ID))
		envs, _ := s.envRepo.ListByProjectID(p.ID)
		for _, env := range envs {
			secrets, _ := s.secretRepo.ListByProjectAndEnv(p.ID, env.ID)
			var keys []string
			for _, sec := range secrets { keys = append(keys, sec.Key) }
			if len(keys) > 0 { b.WriteString(fmt.Sprintf("  Environment %s: %s\n", env.Name, strings.Join(keys, ", "))) }
		}
	}
	return b.String()
}

func (s *NLPQueryService) parseAIResponse(resp, query, provider, model string) *models.NLPQueryResult {
	result := &models.NLPQueryResult{Query: query, Provider: provider, Model: model}
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
			result.MatchedSecrets = append(result.MatchedSecrets, models.NLPSecretMatch{ProjectID: pid, ProjectName: m.ProjectName, Environment: m.Environment, Key: m.Key, Score: m.Score, Reason: m.Reason})
		}
	} else {
		result.Intent = "suggest"
		result.Explanation = resp
	}
	return result
}

const nlpSystemPrompt = `You are KeepSave's AI assistant for natural language secret queries.
Return ONLY a JSON object with: "intent" (find_secret|describe_project|list_env|suggest), "matched_secrets" (array of {project_name, project_id, environment, key, score, reason}), "explanation", "suggestions".
Never include actual secret values.`

func (s *NLPQueryService) Query(userID uuid.UUID, query string) (*models.NLPQueryResult, error) {
	if s.aiMgr == nil || !s.aiMgr.HasProvider() { return s.fuzzyQuery(userID, query) }
	ctx := s.buildContext(userID)
	resp, provider, model, err := s.aiMgr.Chat(nlpSystemPrompt, fmt.Sprintf("Context:\n%s\n\nUser query: %s", ctx, query))
	if err != nil { return s.fuzzyQuery(userID, query) }
	result := s.parseAIResponse(resp, query, provider, model)
	s.db.Exec(`INSERT INTO nlp_query_log (id, user_id, query, intent, provider, model, matched_count, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.New(), userID, query, result.Intent, provider, model, len(result.MatchedSecrets), time.Now())
	return result, nil
}

func (s *NLPQueryService) Converse(userID uuid.UUID, messages []models.ConversationMessage) (*models.NLPQueryResult, error) {
	lastMsg := ""
	if len(messages) > 0 { lastMsg = messages[len(messages)-1].Content }
	if s.aiMgr == nil || !s.aiMgr.HasProvider() { return s.fuzzyQuery(userID, lastMsg) }
	ctx := s.buildContext(userID)
	var history strings.Builder
	for _, msg := range messages {
		if msg.Role == "user" { history.WriteString(fmt.Sprintf("User: %s\n", msg.Content)) } else { history.WriteString(fmt.Sprintf("Assistant: %s\n", msg.Content)) }
	}
	resp, provider, model, err := s.aiMgr.Chat("You are KeepSave's AI assistant for multi-turn secret management conversations. Return JSON with: intent, explanation, matched_secrets, suggestions. Never include secret values.", fmt.Sprintf("Context:\n%s\n\nConversation:\n%s", ctx, history.String()))
	if err != nil { return s.fuzzyQuery(userID, lastMsg) }
	return s.parseAIResponse(resp, lastMsg, provider, model), nil
}

func (s *NLPQueryService) fuzzyQuery(userID uuid.UUID, query string) (*models.NLPQueryResult, error) {
	projects, err := s.projectRepo.ListByOwnerID(userID)
	if err != nil { return nil, err }
	terms := strings.Fields(strings.ToLower(query))
	result := &models.NLPQueryResult{Query: query, Intent: "find_secret"}
	for _, p := range projects {
		envs, _ := s.envRepo.ListByProjectID(p.ID)
		for _, env := range envs {
			secrets, _ := s.secretRepo.ListByProjectAndEnv(p.ID, env.ID)
			for _, sec := range secrets {
				for _, term := range terms {
					if strings.Contains(strings.ToLower(sec.Key), term) || strings.Contains(strings.ToLower(p.Name), term) {
						result.MatchedSecrets = append(result.MatchedSecrets, models.NLPSecretMatch{ProjectID: p.ID, ProjectName: p.Name, Environment: env.Name, Key: sec.Key, Score: 0.7, Reason: fmt.Sprintf("Matched term '%s'", term)})
						break
					}
				}
			}
		}
	}
	result.Explanation = fmt.Sprintf("Found %d matching secrets via keyword search", len(result.MatchedSecrets))
	result.Provider = "builtin"
	result.Model = "keyword-fuzzy"
	return result, nil
}

func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start { return s[start : end+1] }
	return s
}
