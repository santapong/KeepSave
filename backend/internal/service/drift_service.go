package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// DriftService detects secret differences between environments.
type DriftService struct {
	db          *sql.DB
	dialect     repository.Dialect
	secretRepo  *repository.SecretRepository
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
	cryptoSvc   *crypto.Service
	aiMgr       *AIProviderManager
}

func NewDriftService(
	db *sql.DB, dialect repository.Dialect,
	secretRepo *repository.SecretRepository, projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository, cryptoSvc *crypto.Service,
	aiMgr *AIProviderManager,
) *DriftService {
	return &DriftService{db: db, dialect: dialect, secretRepo: secretRepo, projectRepo: projectRepo, envRepo: envRepo, cryptoSvc: cryptoSvc, aiMgr: aiMgr}
}

func (s *DriftService) DetectDrift(projectID, userID uuid.UUID, sourceEnv, targetEnv string) (*models.DriftCheck, error) {
	project, err := s.projectRepo.GetByID(projectID, userID)
	if err != nil {
		return nil, err
	}

	envs, err := s.envRepo.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	var srcID, tgtID uuid.UUID
	for _, e := range envs {
		if e.Name == sourceEnv {
			srcID = e.ID
		}
		if e.Name == targetEnv {
			tgtID = e.ID
		}
	}
	if srcID == uuid.Nil || tgtID == uuid.Nil {
		return nil, fmt.Errorf("invalid environment names")
	}

	srcSecrets, err := s.secretRepo.ListByEnvironment(projectID, srcID)
	if err != nil {
		return nil, err
	}
	tgtSecrets, err := s.secretRepo.ListByEnvironment(projectID, tgtID)
	if err != nil {
		return nil, err
	}

	dek, err := s.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		return nil, err
	}

	srcMap := make(map[string]string)
	for _, sec := range srcSecrets {
		if val, err := s.cryptoSvc.Decrypt(sec.EncryptedValue, sec.ValueNonce, dek); err == nil {
			srcMap[sec.Key] = string(val)
		}
	}
	tgtMap := make(map[string]string)
	for _, sec := range tgtSecrets {
		if val, err := s.cryptoSvc.Decrypt(sec.EncryptedValue, sec.ValueNonce, dek); err == nil {
			tgtMap[sec.Key] = string(val)
		}
	}

	allKeys := make(map[string]bool)
	for k := range srcMap {
		allKeys[k] = true
	}
	for k := range tgtMap {
		allKeys[k] = true
	}

	var entries []models.DriftDetailEntry
	missingSrc, missingTgt, drifted := 0, 0, 0

	for key := range allKeys {
		_, inSrc := srcMap[key]
		_, inTgt := tgtMap[key]
		if !inSrc {
			entries = append(entries, models.DriftDetailEntry{Key: key, DriftType: "missing_source", SourceExists: false, TargetExists: true, Recommendation: fmt.Sprintf("Key '%s' exists in %s but not %s", key, targetEnv, sourceEnv)})
			missingSrc++
			drifted++
		} else if !inTgt {
			entries = append(entries, models.DriftDetailEntry{Key: key, DriftType: "missing_target", SourceExists: true, TargetExists: false, Recommendation: fmt.Sprintf("Key '%s' exists in %s but not %s. Consider promoting.", key, sourceEnv, targetEnv)})
			missingTgt++
			drifted++
		} else if srcMap[key] != tgtMap[key] {
			entries = append(entries, models.DriftDetailEntry{Key: key, DriftType: "value_mismatch", SourceExists: true, TargetExists: true, ValuesDiffer: true, Recommendation: fmt.Sprintf("Key '%s' has different values between %s and %s", key, sourceEnv, targetEnv)})
			drifted++
		}
	}

	remediation := ""
	if drifted > 0 && s.aiMgr != nil && s.aiMgr.HasProvider() {
		ej, _ := json.Marshal(entries)
		prompt := fmt.Sprintf("Analyze drift between '%s' and '%s' for project '%s'. Provide a concise remediation plan.\n\nDrift entries:\n%s", sourceEnv, targetEnv, project.Name, string(ej))
		if resp, _, _, err := s.aiMgr.Chat("You are a DevOps expert analyzing environment configuration drift. Provide actionable remediation steps. Never include secret values. Be concise.", prompt); err == nil {
			remediation = resp
		}
	}

	entriesJSON, _ := json.Marshal(entries)
	var entriesMap models.JSONMap
	json.Unmarshal(entriesJSON, &entriesMap)

	now := time.Now()
	check := &models.DriftCheck{
		ID: uuid.New(), ProjectID: projectID, SourceEnv: sourceEnv, TargetEnv: targetEnv,
		Status: "completed", TotalKeys: len(allKeys), DriftedKeys: drifted,
		MissingInSource: missingSrc, MissingInTarget: missingTgt,
		DriftEntries: entriesMap, Remediation: remediation,
		CreatedAt: now, CompletedAt: &now,
	}

	_, err = s.db.Exec(`INSERT INTO drift_checks (id, project_id, source_env, target_env, status, total_keys, drifted_keys, missing_in_source, missing_in_target, drift_entries, remediation, created_at, completed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		check.ID, check.ProjectID, check.SourceEnv, check.TargetEnv, check.Status, check.TotalKeys, check.DriftedKeys, check.MissingInSource, check.MissingInTarget, string(entriesJSON), check.Remediation, check.CreatedAt, check.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("storing drift check: %w", err)
	}
	return check, nil
}

func (s *DriftService) ListDriftChecks(projectID uuid.UUID) ([]models.DriftCheck, error) {
	rows, err := s.db.Query(`SELECT id, project_id, source_env, target_env, status, total_keys, drifted_keys, missing_in_source, missing_in_target, drift_entries, remediation, created_at, completed_at FROM drift_checks WHERE project_id = $1 ORDER BY created_at DESC LIMIT 50`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.DriftCheck
	for rows.Next() {
		var c models.DriftCheck
		var entriesStr string
		var completedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.SourceEnv, &c.TargetEnv, &c.Status, &c.TotalKeys, &c.DriftedKeys, &c.MissingInSource, &c.MissingInTarget, &entriesStr, &c.Remediation, &c.CreatedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			c.CompletedAt = &completedAt.Time
		}
		json.Unmarshal([]byte(entriesStr), &c.DriftEntries)
		checks = append(checks, c)
	}
	return checks, nil
}
