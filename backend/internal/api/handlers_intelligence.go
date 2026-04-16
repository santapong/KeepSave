package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type IntelligenceHandler struct {
	driftSvc   *service.DriftService
	anomalySvc *service.AnomalyService
	analytics  *service.UsageAnalyticsService
	recommSvc  *service.RecommendationService
	nlpSvc     *service.NLPQueryService
	aiMgr      *service.AIProviderManager
}

func NewIntelligenceHandler(driftSvc *service.DriftService, anomalySvc *service.AnomalyService, analytics *service.UsageAnalyticsService, recommSvc *service.RecommendationService, nlpSvc *service.NLPQueryService, aiMgr *service.AIProviderManager) *IntelligenceHandler {
	return &IntelligenceHandler{driftSvc: driftSvc, anomalySvc: anomalySvc, analytics: analytics, recommSvc: recommSvc, nlpSvc: nlpSvc, aiMgr: aiMgr}
}

// --- Providers ---
func (h *IntelligenceHandler) ListProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"providers": h.aiMgr.ListProviders(), "has_provider": h.aiMgr.HasProvider()})
}

// --- Drift ---
func (h *IntelligenceHandler) DetectDrift(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	uid := c.MustGet("user_id").(uuid.UUID)
	var req struct { SourceEnv string `json:"source_env" binding:"required"`; TargetEnv string `json:"target_env" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
	check, err := h.driftSvc.DetectDrift(pid, uid, req.SourceEnv, req.TargetEnv)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"drift_check": check})
}

func (h *IntelligenceHandler) ListDriftChecks(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	checks, err := h.driftSvc.ListDriftChecks(pid)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"drift_checks": checks})
}

// --- Drift Schedules ---
func (h *IntelligenceHandler) CreateDriftSchedule(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	var req struct { SourceEnv string `json:"source_env" binding:"required"`; TargetEnv string `json:"target_env" binding:"required"`; CronExpr string `json:"cron_expr"` }
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
	if req.CronExpr == "" { req.CronExpr = "0 */6 * * *" }
	sch, err := h.driftSvc.CreateSchedule(pid, req.SourceEnv, req.TargetEnv, req.CronExpr)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(201, gin.H{"schedule": sch})
}

func (h *IntelligenceHandler) ListDriftSchedules(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	schedules, err := h.driftSvc.ListSchedules(pid)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"schedules": schedules})
}

func (h *IntelligenceHandler) UpdateDriftSchedule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("scheduleId"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid schedule ID"}); return }
	var req struct { Enabled bool `json:"enabled"`; CronExpr string `json:"cron_expr"` }
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
	if err := h.driftSvc.UpdateSchedule(id, req.Enabled, req.CronExpr); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"status": "updated"})
}

func (h *IntelligenceHandler) DeleteDriftSchedule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("scheduleId"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid schedule ID"}); return }
	if err := h.driftSvc.DeleteSchedule(id); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"status": "deleted"})
}

func (h *IntelligenceHandler) RunScheduledDriftChecks(c *gin.Context) {
	count, err := h.driftSvc.RunScheduledChecks()
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"ran": count})
}

// --- Anomalies ---
func (h *IntelligenceHandler) RunAnomalyDetection(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	anomalies, err := h.anomalySvc.RunDetection(pid)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"anomalies": anomalies, "count": len(anomalies)})
}

func (h *IntelligenceHandler) ListAnomalies(c *gin.Context) {
	var pid *uuid.UUID
	if p := c.Query("project_id"); p != "" { if parsed, err := uuid.Parse(p); err == nil { pid = &parsed } }
	anomalies, err := h.anomalySvc.ListAnomalies(pid, c.DefaultQuery("status", ""))
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"anomalies": anomalies})
}

func (h *IntelligenceHandler) AcknowledgeAnomaly(c *gin.Context) {
	id, err := uuid.Parse(c.Param("anomalyId"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid ID"}); return }
	if err := h.anomalySvc.AcknowledgeAnomaly(id); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"status": "acknowledged"})
}

func (h *IntelligenceHandler) ResolveAnomaly(c *gin.Context) {
	id, err := uuid.Parse(c.Param("anomalyId"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid ID"}); return }
	if err := h.anomalySvc.ResolveAnomaly(id); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"status": "resolved"})
}

// --- Alert Rules ---
func (h *IntelligenceHandler) CreateAlertRule(c *gin.Context) {
	uid := c.MustGet("user_id").(uuid.UUID)
	var req struct {
		ProjectID string     `json:"project_id"`
		APIKeyID  string     `json:"api_key_id"`
		RuleType  string     `json:"rule_type" binding:"required"`
		Config    models.JSONMap `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
	var pid, kid *uuid.UUID
	if req.ProjectID != "" { p, _ := uuid.Parse(req.ProjectID); pid = &p }
	if req.APIKeyID != "" { k, _ := uuid.Parse(req.APIKeyID); kid = &k }
	rule, err := h.anomalySvc.CreateRule(pid, kid, req.RuleType, req.Config, uid)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(201, gin.H{"rule": rule})
}

func (h *IntelligenceHandler) ListAlertRules(c *gin.Context) {
	var pid *uuid.UUID
	if p := c.Query("project_id"); p != "" { if parsed, err := uuid.Parse(p); err == nil { pid = &parsed } }
	rules, err := h.anomalySvc.ListRules(pid)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"rules": rules})
}

func (h *IntelligenceHandler) UpdateAlertRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("ruleId"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid ID"}); return }
	var req struct { Enabled bool `json:"enabled"`; Config models.JSONMap `json:"config"` }
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
	if err := h.anomalySvc.UpdateRule(id, req.Enabled, req.Config); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"status": "updated"})
}

func (h *IntelligenceHandler) DeleteAlertRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("ruleId"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid ID"}); return }
	if err := h.anomalySvc.DeleteRule(id); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"status": "deleted"})
}

// --- Analytics ---
func (h *IntelligenceHandler) GetUsageTrends(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	period := c.DefaultQuery("period", "daily")
	days := 30
	if d := c.Query("days"); d != "" { fmt.Sscanf(d, "%d", &days) }
	trends, err := h.analytics.GetTrends(pid, period, days)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"trends": trends, "period": period})
}

func (h *IntelligenceHandler) GetUsageForecast(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	days := 14
	if d := c.Query("days"); d != "" { fmt.Sscanf(d, "%d", &days) }
	forecasts, err := h.analytics.Forecast(pid, days)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"forecasts": forecasts})
}

func (h *IntelligenceHandler) ExportAnalyticsCSV(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	period := c.DefaultQuery("period", "daily")
	days := 30
	if d := c.Query("days"); d != "" { fmt.Sscanf(d, "%d", &days) }
	format := c.DefaultQuery("format", "csv")
	if format == "json" {
		trends, err := h.analytics.GetTrends(pid, period, days)
		if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
		c.JSON(200, gin.H{"trends": trends})
		return
	}
	csv, err := h.analytics.ExportCSV(pid, period, days)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=analytics.csv")
	c.String(200, csv)
}

// --- Quota ---
func (h *IntelligenceHandler) GetQuota(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid org ID"}); return }
	q, err := h.analytics.GetQuota(orgID)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"quota": q})
}

func (h *IntelligenceHandler) SetQuota(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid org ID"}); return }
	var req struct { MaxSecrets int `json:"max_secrets"`; MaxProjects int `json:"max_projects"`; MaxAPIKeys int `json:"max_api_keys"`; MaxRequestsPerDay int `json:"max_requests_per_day"` }
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
	q, err := h.analytics.SetQuota(orgID, req.MaxSecrets, req.MaxProjects, req.MaxAPIKeys, req.MaxRequestsPerDay)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"quota": q})
}

// --- Recommendations ---
func (h *IntelligenceHandler) GenerateRecommendations(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	uid := c.MustGet("user_id").(uuid.UUID)
	recs, err := h.recommSvc.GenerateRecommendations(pid, uid)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"recommendations": recs})
}

func (h *IntelligenceHandler) ListRecommendations(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid project ID"}); return }
	recs, err := h.recommSvc.ListRecommendations(pid, c.DefaultQuery("status", ""))
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"recommendations": recs})
}

func (h *IntelligenceHandler) DismissRecommendation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("recId"))
	if err != nil { c.JSON(400, gin.H{"error": "invalid ID"}); return }
	if err := h.recommSvc.DismissRecommendation(id); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"status": "dismissed"})
}

// --- NLP ---
func (h *IntelligenceHandler) NLPQuery(c *gin.Context) {
	uid := c.MustGet("user_id").(uuid.UUID)
	var req struct { Query string `json:"query" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
	result, err := h.nlpSvc.Query(uid, req.Query)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"result": result})
}

func (h *IntelligenceHandler) NLPConverse(c *gin.Context) {
	uid := c.MustGet("user_id").(uuid.UUID)
	var req struct { Messages []models.ConversationMessage `json:"messages" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
	result, err := h.nlpSvc.Converse(uid, req.Messages)
	if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
	c.JSON(200, gin.H{"result": result})
}
