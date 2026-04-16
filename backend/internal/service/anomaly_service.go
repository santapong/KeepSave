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

// RunDetection scans recent agent_activities and flags anomalies using Z-score.
func (s *AnomalyService) RunDetection(projectID uuid.UUID) ([]models.Anomaly, error) {
	var detected []models.Anomaly

	// Frequency spike detection: compare last hour vs rolling average
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

		// Get rolling average (last 7 days, per hour)
		var avgCount float64
		var stddev float64
		err := s.db.QueryRow(`SELECT COALESCE(AVG(hourly_count),0), COALESCE(MAX(hourly_count)-MIN(hourly_count),1) FROM (SELECT COUNT(*) as hourly_count FROM agent_activities WHERE project_id = $1 AND api_key_id = $2 AND created_at > $3 GROUP BY strftime('%Y-%m-%d %H', created_at)) sub`, projectID, keyID, time.Now().Add(-7*24*time.Hour)).Scan(&avgCount, &stddev)
		if err != nil || stddev == 0 {
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
			anomaly := models.Anomaly{
				ID: uuid.New(), ProjectID: &projectID, APIKeyID: &keyID,
				AnomalyType: "frequency_spike", Severity: severity,
				Description: fmt.Sprintf("Access frequency spike: %d accesses in last hour (avg %.1f, z-score %.2f)", cnt, avgCount, zScore),
				Details:     models.JSONMap{"count": cnt, "average": avgCount, "z_score": zScore},
				Status: "open", DetectedAt: time.Now(),
			}
			s.storeAnomaly(&anomaly)
			detected = append(detected, anomaly)
		}
	}

	// Unusual time detection: access outside business hours (22:00-06:00)
	hour := time.Now().Hour()
	if hour >= 22 || hour < 6 {
		var offHourCount int
		s.db.QueryRow(`SELECT COUNT(*) FROM agent_activities WHERE project_id = $1 AND created_at > $2`, projectID, time.Now().Add(-1*time.Hour)).Scan(&offHourCount)
		if offHourCount > 5 {
			anomaly := models.Anomaly{
				ID: uuid.New(), ProjectID: &projectID,
				AnomalyType: "unusual_time", Severity: "medium",
				Description: fmt.Sprintf("%d secret accesses during off-hours (hour %d)", offHourCount, hour),
				Details:     models.JSONMap{"count": offHourCount, "hour": hour},
				Status: "open", DetectedAt: time.Now(),
			}
			s.storeAnomaly(&anomaly)
			detected = append(detected, anomaly)
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
	now := time.Now()
	_, err := s.db.Exec(`UPDATE anomalies SET status = 'acknowledged', acknowledged_at = $1 WHERE id = $2`, now, id)
	return err
}

func (s *AnomalyService) ResolveAnomaly(id uuid.UUID) error {
	now := time.Now()
	_, err := s.db.Exec(`UPDATE anomalies SET status = 'resolved', resolved_at = $1 WHERE id = $2`, now, id)
	return err
}
