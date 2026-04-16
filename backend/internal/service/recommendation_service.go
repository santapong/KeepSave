package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// RecommendationService generates AI-powered secret recommendations.
type RecommendationService struct {
	db          *sql.DB
	dialect     repository.Dialect
	secretRepo  *repository.SecretRepository
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	cryptoSvc   *crypto.Service
	aiMgr       *AIProviderManager
}

func NewRecommendationService(
	db *sql.DB, dialect repository.Dialect,
	secretRepo *repository.SecretRepository, projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository, cryptoSvc *crypto.Service,
	aiMgr *AIProviderManager,
) *RecommendationService {
	return &RecommendationService{db: db, dialect: dialect, secretRepo: secretRepo, projectRepo: projectRepo, envRepo: envRepo, cryptoSvc: cryptoSvc, aiMgr: aiMgr}
}

// GenerateRecommendations analyzes a project's secrets and returns recommendations.
func (s *RecommendationService) GenerateRecommendations(projectID, userID uuid.UUID) ([]models.SecretRecommendation, error) {
	project, err := s.projectRepo.GetByID(projectID, userID)
	if err != nil {
		return nil, err
	}

	envs, err := s.envRepo.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	// Collect all secret keys per environment
	envKeys := make(map[string][]string)
	for _, env := range envs {
		secrets, err := s.secretRepo.ListByEnvironment(projectID, env.ID)
		if err != nil {
			continue
		}
		for _, sec := range secrets {
			envKeys[env.Name] = append(envKeys[env.Name], sec.Key)
		}
	}

	var recommendations []models.SecretRecommendation

	// Rule: detect missing keys across environments
	allKeys := make(map[string]map[string]bool)
	for envName, keys := range envKeys {
		for _, key := range keys {
			if allKeys[key] == nil {
				allKeys[key] = make(map[string]bool)
			}
			allKeys[key][envName] = true
		}
	}
	for key, envs := range allKeys {
		if len(envs) < len(envKeys) {
			var missing []string
			for envName := range envKeys {
				if !envs[envName] {
					missing = append(missing, envName)
				}
			}
			rec := models.SecretRecommendation{
				ID: uuid.New(), ProjectID: projectID,
				RecommType: "missing_secret", Severity: "warning",
				Title:           fmt.Sprintf("Key '%s' missing in some environments", key),
				Description:     fmt.Sprintf("'%s' exists in %d of %d environments. Missing in: %s", key, len(envs), len(envKeys), strings.Join(missing, ", ")),
				AffectedKeys:    models.StringList{key},
				SuggestedAction: "Promote or add the key to the missing environments.",
				AutoFixable:     true,
				Status:          "pending",
				CreatedAt:       time.Now(),
			}
			recommendations = append(recommendations, rec)
		}
	}

	// Rule: detect potential duplicates (similar key names)
	allKeyList := make([]string, 0, len(allKeys))
	for k := range allKeys {
		allKeyList = append(allKeyList, k)
	}
	for i := 0; i < len(allKeyList); i++ {
		for j := i + 1; j < len(allKeyList); j++ {
			if isSimilarKey(allKeyList[i], allKeyList[j]) {
				rec := models.SecretRecommendation{
					ID: uuid.New(), ProjectID: projectID,
					RecommType: "duplicate", Severity: "info",
					Title:           fmt.Sprintf("Possible duplicate keys: '%s' and '%s'", allKeyList[i], allKeyList[j]),
					Description:     "These keys have very similar names and may be redundant.",
					AffectedKeys:    models.StringList{allKeyList[i], allKeyList[j]},
					SuggestedAction: "Review and consolidate if they serve the same purpose.",
					AutoFixable:     false,
					Status:          "pending",
					CreatedAt:       time.Now(),
				}
				recommendations = append(recommendations, rec)
			}
		}
	}

	// AI-powered recommendations if provider available
	if s.aiMgr != nil && s.aiMgr.HasProvider() {
		keysJSON, _ := json.Marshal(envKeys)
		prompt := fmt.Sprintf("Analyze the following secret keys for project '%s' across environments. Suggest missing secrets, security improvements, and rotation needs. Return JSON array with objects {\"type\": \"missing_secret|rotation_needed|type_mismatch\", \"severity\": \"info|warning|critical\", \"title\": \"...\", \"description\": \"...\", \"affected_keys\": [...], \"suggested_action\": \"...\"}\n\nKeys per environment:\n%s", project.Name, string(keysJSON))
		resp, _, _, err := s.aiMgr.Chat("You are a security expert analyzing secret configurations. Return ONLY a JSON array. Never include actual secret values.", prompt)
		if err == nil {
			var aiRecs []struct {
				Type            string   `json:"type"`
				Severity        string   `json:"severity"`
				Title           string   `json:"title"`
				Description     string   `json:"description"`
				AffectedKeys    []string `json:"affected_keys"`
				SuggestedAction string   `json:"suggested_action"`
			}
			// Try to extract JSON from response
			clean := extractJSON(resp)
			if json.Unmarshal([]byte(clean), &aiRecs) == nil {
				for _, ar := range aiRecs {
					rec := models.SecretRecommendation{
						ID: uuid.New(), ProjectID: projectID,
						RecommType: ar.Type, Severity: ar.Severity,
						Title: ar.Title, Description: ar.Description,
						AffectedKeys: ar.AffectedKeys, SuggestedAction: ar.SuggestedAction,
						Status: "pending", CreatedAt: time.Now(),
					}
					recommendations = append(recommendations, rec)
				}
			}
		}
	}

	// Store recommendations
	for _, rec := range recommendations {
		keys, _ := json.Marshal([]string(rec.AffectedKeys))
		s.db.Exec(`INSERT INTO secret_recommendations (id, project_id, recomm_type, severity, title, description, affected_keys, suggested_action, auto_fixable, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			rec.ID, rec.ProjectID, rec.RecommType, rec.Severity, rec.Title, rec.Description, string(keys), rec.SuggestedAction, rec.AutoFixable, rec.Status, rec.CreatedAt)
	}

	return recommendations, nil
}

func (s *RecommendationService) ListRecommendations(projectID uuid.UUID, status string) ([]models.SecretRecommendation, error) {
	query := `SELECT id, project_id, recomm_type, severity, title, description, affected_keys, suggested_action, auto_fixable, status, created_at FROM secret_recommendations WHERE project_id = $1`
	args := []interface{}{projectID}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []models.SecretRecommendation
	for rows.Next() {
		var r models.SecretRecommendation
		var keysStr string
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.RecommType, &r.Severity, &r.Title, &r.Description, &keysStr, &r.SuggestedAction, &r.AutoFixable, &r.Status, &r.CreatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(keysStr), &r.AffectedKeys)
		recs = append(recs, r)
	}
	return recs, nil
}

func (s *RecommendationService) DismissRecommendation(id uuid.UUID) error {
	_, err := s.db.Exec(`UPDATE secret_recommendations SET status = 'dismissed' WHERE id = $1`, id)
	return err
}

// isSimilarKey checks if two key names are suspiciously similar.
func isSimilarKey(a, b string) bool {
	normA := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(a, "_", ""), "-", ""))
	normB := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(b, "_", ""), "-", ""))
	if normA == normB && a != b {
		return true
	}
	// Check if one is a prefix/suffix variant of the other
	if len(normA) > 4 && len(normB) > 4 {
		if strings.HasPrefix(normA, normB) || strings.HasPrefix(normB, normA) {
			return true
		}
	}
	return false
}

// extractJSON extracts JSON array from a possibly markdown-wrapped response.
func extractJSON(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
