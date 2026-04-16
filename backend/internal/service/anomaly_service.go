package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// AnomalyService detects anomalies in secret access patterns.
type AnomalyService struct {
	db      *sql.DB
	dialect repository.Dialect
	aiMgr   *AIProviderManager
}

func NewAnomalyService(db *sql.DB, dialect repository.Dialect, aiMgr *AIProviderManager) *AnomalyService {
	return &AnomalyService{db: db, dialect: dialect, aiMgr: aiMgr}
}

// RunDetection scans recent agent_activities and flags anomalies.
func (s *AnomalyService) RunDetection(projectID uuid.UUID) ([]models.Anomaly, error) {
	var detected []models.Anomaly

	// 1. Frequency spike detection (Z-score)
	rows, err := s.db.Query(`SELECT api_key_id, COUNT(*) as cnt FROM agent_activities WHERE project_id = $1 AND created_at > $2 GROUP BY api_key_id`, projectID, time.Now().Add(-1*time.Hour))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var keyID uuid.UUID
		var cnt int
		if err := rows.Scan(&keyID, &cnt); err != nil {
			continue
		}
		var avgCount, stddev float64
		s.db.QueryRow(`SELECT COALESCE(AVG(hourly_count),0), COALESCE(MAX(hourly_count)-MIN(hourly_count),1) FROM (SELECT COUNT(*) as hourly_count FROM agent_activities WHERE project_id = $1 AND api_key_id = $2 AND created_at > $3 GROUP BY strftime('%Y-%m-%d %H', created_at)) sub`, projectID, keyID, time.Now().Add(-7*24*time.Hour)).Scan(&avgCount, &stddev)
		if stddev == 0 {
			continue
		}
		zScore := (float64(cnt) - avgCount) / math.Max(stddev, 1)
		if zScore > 2.5 {
			severity := "medium"
			if zScore > 4 {
				severity = "critical"
			} else if zScore > 3 {
				severity = "high"
			}
			a := models.Anomaly{
				ID: uuid.New(), ProjectID: &projectID, APIKeyID: &keyID,
				AnomalyType: "frequency_spike", Severity: severity,
				Description: fmt.Sprintf("Access frequency spike: %d accesses in last hour (avg %.1f, z-score %.2f)", cnt, avgCount, zScore),
				Details:     models.JSONMap{"count": cnt, "average": avgCount, "z_score": zScore},
				Status: "open", DetectedAt: time.Now(),
			}
			s.storeAnomaly(&a)
			detected = append(detected, a)
		}
	}

	// 2. Unusual time detection (off-hours: 22:00-06:00)
	hour := time.Now().Hour()
	if hour >= 22 || hour < 6 {
		var offHourCount int
		s.db.QueryRow(`SELECT COUNT(*) FROM agent_activities WHERE project_id = $1 AND created_at > $2`, projectID, time.Now().Add(-1*time.Hour)).Scan(&offHourCount)
		if offHourCount > 5 {
			a := models.Anomaly{
				ID: uuid.New(), ProjectID: &projectID,
				AnomalyType: "unusual_time", Severity: "medium",
				Description: fmt.Sprintf("%d secret accesses during off-hours (hour %d)", offHourCount, hour),
				Details:     models.JSONMap{"count": offHourCount, "hour": hour},
				Status: "open", DetectedAt: time.Now(),
			}
			s.storeAnomaly(&a)
			detected = append(detected, a)
		}
	}

	// 3. New IP detection: IPs seen in last hour that never appeared before
	ipRows, err := s.db.Query(`SELECT DISTINCT ip_address FROM agent_activities WHERE project_id = $1 AND created_at > $2 AND ip_address NOT IN (SELECT DISTINCT ip_address FROM agent_activities WHERE project_id = $1 AND created_at <= $2)`, projectID, time.Now().Add(-1*time.Hour))
	if err == nil {
		defer ipRows.Close()
		for ipRows.Next() {
			var ip string
			if err := ipRows.Scan(&ip); err != nil {
				continue
			}
			a := models.Anomaly{
				ID: uuid.New(), ProjectID: &projectID,
				AnomalyType: "new_ip", Severity: "high",
				Description: fmt.Sprintf("New IP address '%s' accessing secrets for the first time", ip),
				Details:     models.JSONMap{"ip_address": ip},
				Status: "open", DetectedAt: time.Now(),
			}
			s.storeAnomaly(&a)
			detected = append(detected, a)
		}
	}

	// 4. Unusual key access: keys accessed in last hour that were never accessed before by this project's agents
	keyRows, err := s.db.Query(`SELECT DISTINCT secret_key FROM agent_activities WHERE project_id = $1 AND created_at > $2 AND secret_key != '' AND secret_key NOT IN (SELECT DISTINCT secret_key FROM agent_activities WHERE project_id = $1 AND created_at <= $2 AND secret_key != '')`, projectID, time.Now().Add(-1*time.Hour))
	if err == nil {
		defer keyRows.Close()
		for keyRows.Next() {
			var key string
			if err := keyRows.Scan(&key); err != nil {
				continue
			}
			a := models.Anomaly{
				ID: uuid.New(), ProjectID: &projectID,
				AnomalyType: "unusual_key", Severity: "medium",
				Description: fmt.Sprintf("Secret key '%s' accessed for the first time by an agent", key),
				Details:     models.JSONMap{"secret_key": key},
				Status: "open", DetectedAt: time.Now(),
			}
			s.storeAnomaly(&a)
			detected = append(detected, a)
		}
	}

	return detected, nil
}

func (s *AnomalyService) storeAnomaly(a *models.Anomaly) {
	details, _ := json.Marshal(a.Details)
	s.db.Exec(`INSERT INTO anomalies (id, project_id, api_key_id, anomaly_type, severity, description, details, status, detected_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		a.ID, a.ProjectID, a.APIKeyID, a.AnomalyType, a.Severity, a.Description, string(details), a.Status, a.DetectedAt)
}

func (s *AnomalyService) ListAnomalies(projectID *uuid.UUID, status string) ([]models.Anomaly, error) {
	query := `SELECT id, project_id, api_key_id, anomaly_type, severity, description, details, status, detected_at, acknowledged_at, resolved_at FROM anomalies WHERE 1=1`
	var args []interface{}
	argIdx := 1
	if projectID != nil {
		query += fmt.Sprintf(" AND project_id = $%d", argIdx)
		args = append(args, *projectID)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	query += " ORDER BY detected_at DESC LIMIT 100"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []models.Anomaly
	for rows.Next() {
		var a models.Anomaly
		var detailsStr string
		var projectIDNullable, apiKeyIDNullable sql.NullString
		var ackAt, resAt sql.NullTime
		if err := rows.Scan(&a.ID, &projectIDNullable, &apiKeyIDNullable, &a.AnomalyType, &a.Severity, &a.Description, &detailsStr, &a.Status, &a.DetectedAt, &ackAt, &resAt); err != nil {
			continue
		}
		if projectIDNullable.Valid {
			pid, _ := uuid.Parse(projectIDNullable.String)
			a.ProjectID = &pid
		}
		if apiKeyIDNullable.Valid {
			kid, _ := uuid.Parse(apiKeyIDNullable.String)
			a.APIKeyID = &kid
		}
		if ackAt.Valid {
			a.AcknowledgedAt = &ackAt.Time
		}
		if resAt.Valid {
			a.ResolvedAt = &resAt.Time
		}
		json.Unmarshal([]byte(detailsStr), &a.Details)
		anomalies = append(anomalies, a)
	}
	return anomalies, nil
}

func (s *AnomalyService) AcknowledgeAnomaly(id uuid.UUID) error {
	_, err := s.db.Exec(`UPDATE anomalies SET status = 'acknowledged', acknowledged_at = $1 WHERE id = $2`, time.Now(), id)
	return err
}

func (s *AnomalyService) ResolveAnomaly(id uuid.UUID) error {
	_, err := s.db.Exec(`UPDATE anomalies SET status = 'resolved', resolved_at = $1 WHERE id = $2`, time.Now(), id)
	return err
}

// --- Alert Rules CRUD ---

func (s *AnomalyService) CreateRule(projectID *uuid.UUID, apiKeyID *uuid.UUID, ruleType string, config models.JSONMap, createdBy uuid.UUID) (*models.AnomalyRule, error) {
	now := time.Now()
	rule := &models.AnomalyRule{
		ID: uuid.New(), ProjectID: projectID, APIKeyID: apiKeyID,
		RuleType: ruleType, Config: config, Enabled: true,
		CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
	configJSON, _ := json.Marshal(config)
	_, err := s.db.Exec(`INSERT INTO anomaly_rules (id, project_id, api_key_id, rule_type, config, enabled, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		rule.ID, rule.ProjectID, rule.APIKeyID, rule.RuleType, string(configJSON), rule.Enabled, rule.CreatedBy, rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *AnomalyService) ListRules(projectID *uuid.UUID) ([]models.AnomalyRule, error) {
	query := `SELECT id, project_id, api_key_id, rule_type, config, enabled, created_by, created_at, updated_at FROM anomaly_rules`
	var args []interface{}
	if projectID != nil {
		query += ` WHERE project_id = $1`
		args = append(args, *projectID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.AnomalyRule
	for rows.Next() {
		var r models.AnomalyRule
		var projID, keyID sql.NullString
		var configStr string
		if err := rows.Scan(&r.ID, &projID, &keyID, &r.RuleType, &configStr, &r.Enabled, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt); err != nil {
			continue
		}
		if projID.Valid {
			pid, _ := uuid.Parse(projID.String)
			r.ProjectID = &pid
		}
		if keyID.Valid {
			kid, _ := uuid.Parse(keyID.String)
			r.APIKeyID = &kid
		}
		json.Unmarshal([]byte(configStr), &r.Config)
		rules = append(rules, r)
	}
	return rules, nil
}

func (s *AnomalyService) UpdateRule(id uuid.UUID, enabled bool, config models.JSONMap) error {
	configJSON, _ := json.Marshal(config)
	_, err := s.db.Exec(`UPDATE anomaly_rules SET enabled = $1, config = $2, updated_at = $3 WHERE id = $4`, enabled, string(configJSON), time.Now(), id)
	return err
}

func (s *AnomalyService) DeleteRule(id uuid.UUID) error {
	_, err := s.db.Exec(`DELETE FROM anomaly_rules WHERE id = $1`, id)
	return err
}
