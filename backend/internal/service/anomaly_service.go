package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// ErrAnomalyNotFound / ErrRuleNotFound are returned when a mutation targets a
// row that does not exist OR belongs to a project the caller cannot access —
// the two are deliberately indistinguishable so a caller cannot enumerate
// another tenant's anomaly/rule IDs (handlers map both to 404).
var (
	ErrAnomalyNotFound = errors.New("anomaly not found")
	ErrRuleNotFound    = errors.New("rule not found")
)

// projectInClause builds a "$start,$start+1,..." placeholder list for an IN
// (...) over the given project ids, plus the matching args. Callers gate on
// len(ids)==0 first (an empty IN would match nothing and is clearer handled as
// "no access"). Placeholders use $N (rebound per dialect by repository.Q).
func projectInClause(start int, ids []uuid.UUID) (string, []interface{}) {
	ph := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		ph[i] = fmt.Sprintf("$%d", start+i)
		args[i] = id
	}
	return strings.Join(ph, ","), args
}

// AnomalyService detects anomalies in secret access patterns.
type AnomalyService struct {
	db        *sql.DB
	dialect   repository.Dialect
	aiMgr     *AIProviderManager
	auditRepo *repository.AuditRepository
}

func NewAnomalyService(db *sql.DB, dialect repository.Dialect, aiMgr *AIProviderManager, auditRepo *repository.AuditRepository) *AnomalyService {
	return &AnomalyService{db: db, dialect: dialect, aiMgr: aiMgr, auditRepo: auditRepo}
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
				Status:      "open", DetectedAt: time.Now(),
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
				Status:      "open", DetectedAt: time.Now(),
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
				Status:      "open", DetectedAt: time.Now(),
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
				Status:      "open", DetectedAt: time.Now(),
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

// ListAnomalies returns anomalies for the given projects only. projectIDs is the
// caller's accessible-project set (a single requested project the caller can
// reach, or all projects they can access) — never unbounded — so this can no
// longer leak other tenants' anomalies (a NULL-project anomaly never matches an
// IN list and is therefore never returned to a tenant).
func (s *AnomalyService) ListAnomalies(projectIDs []uuid.UUID, status string) ([]models.Anomaly, error) {
	if len(projectIDs) == 0 {
		return []models.Anomaly{}, nil
	}
	inSQL, args := projectInClause(1, projectIDs)
	query := `SELECT id, project_id, api_key_id, anomaly_type, severity, description, details, status, detected_at, acknowledged_at, resolved_at FROM anomalies WHERE project_id IN (` + inSQL + `)`
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, status)
	}
	query += " ORDER BY detected_at DESC LIMIT 100"

	rows, err := s.db.Query(repository.Q(s.dialect, query), args...)
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

// AcknowledgeAnomaly marks an anomaly acknowledged. The UPDATE is bound to the
// caller's accessible projects (accessibleProjectIDs), so a caller cannot
// acknowledge — and thereby silence — another tenant's anomaly by its id. Zero
// rows affected (wrong id, or an anomaly in an inaccessible project) is reported
// as ErrAnomalyNotFound (anti-enumeration).
func (s *AnomalyService) AcknowledgeAnomaly(id uuid.UUID, accessibleProjectIDs []uuid.UUID, actorID uuid.UUID, ipAddr string) error {
	if len(accessibleProjectIDs) == 0 {
		return ErrAnomalyNotFound
	}
	args := []interface{}{time.Now(), id}
	inSQL, inArgs := projectInClause(3, accessibleProjectIDs)
	args = append(args, inArgs...)
	res, err := s.db.Exec(repository.Q(s.dialect, `UPDATE anomalies SET status = 'acknowledged', acknowledged_at = $1 WHERE id = $2 AND project_id IN (`+inSQL+`)`), args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAnomalyNotFound
	}
	emitAudit(s.auditRepo, &actorID, nil, "anomaly.acknowledged", "",
		models.JSONMap{"anomaly_id": id.String()}, ipAddr)
	return nil
}

// ResolveAnomaly marks an anomaly resolved, scoped to the caller's accessible
// projects exactly like AcknowledgeAnomaly.
func (s *AnomalyService) ResolveAnomaly(id uuid.UUID, accessibleProjectIDs []uuid.UUID, actorID uuid.UUID, ipAddr string) error {
	if len(accessibleProjectIDs) == 0 {
		return ErrAnomalyNotFound
	}
	args := []interface{}{time.Now(), id}
	inSQL, inArgs := projectInClause(3, accessibleProjectIDs)
	args = append(args, inArgs...)
	res, err := s.db.Exec(repository.Q(s.dialect, `UPDATE anomalies SET status = 'resolved', resolved_at = $1 WHERE id = $2 AND project_id IN (`+inSQL+`)`), args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAnomalyNotFound
	}
	emitAudit(s.auditRepo, &actorID, nil, "anomaly.resolved", "",
		models.JSONMap{"anomaly_id": id.String()}, ipAddr)
	return nil
}

// --- Alert Rules CRUD ---

func (s *AnomalyService) CreateRule(projectID *uuid.UUID, apiKeyID *uuid.UUID, ruleType string, config models.JSONMap, createdBy uuid.UUID, ipAddr string) (*models.AnomalyRule, error) {
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
	emitAudit(s.auditRepo, &createdBy, projectID, "anomaly.rule_created", "",
		models.JSONMap{"rule_id": rule.ID.String(), "rule_type": ruleType}, ipAddr)
	return rule, nil
}

// ListRules returns alert rules for the caller's accessible projects only
// (projectIDs is the requested-and-allowed project, or all accessible). A
// global/NULL-project rule never matches the IN list, so it is not leaked to a
// tenant.
func (s *AnomalyService) ListRules(projectIDs []uuid.UUID) ([]models.AnomalyRule, error) {
	if len(projectIDs) == 0 {
		return []models.AnomalyRule{}, nil
	}
	inSQL, args := projectInClause(1, projectIDs)
	query := `SELECT id, project_id, api_key_id, rule_type, config, enabled, created_by, created_at, updated_at FROM anomaly_rules WHERE project_id IN (` + inSQL + `) ORDER BY created_at DESC`

	rows, err := s.db.Query(repository.Q(s.dialect, query), args...)
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

// UpdateRule toggles/reconfigures a rule, bound to the caller's accessible
// projects so another tenant's alert rule cannot be disabled or rewritten by
// its id. Zero rows affected → ErrRuleNotFound.
func (s *AnomalyService) UpdateRule(id uuid.UUID, enabled bool, config models.JSONMap, accessibleProjectIDs []uuid.UUID, actorID uuid.UUID, ipAddr string) error {
	if len(accessibleProjectIDs) == 0 {
		return ErrRuleNotFound
	}
	configJSON, _ := json.Marshal(config)
	args := []interface{}{enabled, string(configJSON), time.Now(), id}
	inSQL, inArgs := projectInClause(5, accessibleProjectIDs)
	args = append(args, inArgs...)
	res, err := s.db.Exec(repository.Q(s.dialect, `UPDATE anomaly_rules SET enabled = $1, config = $2, updated_at = $3 WHERE id = $4 AND project_id IN (`+inSQL+`)`), args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRuleNotFound
	}
	emitAudit(s.auditRepo, &actorID, nil, "anomaly.rule_updated", "",
		models.JSONMap{"rule_id": id.String(), "enabled": enabled}, ipAddr)
	return nil
}

// DeleteRule removes a rule, bound to the caller's accessible projects.
func (s *AnomalyService) DeleteRule(id uuid.UUID, accessibleProjectIDs []uuid.UUID, actorID uuid.UUID, ipAddr string) error {
	if len(accessibleProjectIDs) == 0 {
		return ErrRuleNotFound
	}
	args := []interface{}{id}
	inSQL, inArgs := projectInClause(2, accessibleProjectIDs)
	args = append(args, inArgs...)
	res, err := s.db.Exec(repository.Q(s.dialect, `DELETE FROM anomaly_rules WHERE id = $1 AND project_id IN (`+inSQL+`)`), args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRuleNotFound
	}
	emitAudit(s.auditRepo, &actorID, nil, "anomaly.rule_deleted", "",
		models.JSONMap{"rule_id": id.String()}, ipAddr)
	return nil
}
