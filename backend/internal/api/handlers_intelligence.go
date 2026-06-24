package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type IntelligenceHandler struct {
	driftSvc    *service.DriftService
	anomalySvc  *service.AnomalyService
	analytics   *service.UsageAnalyticsService
	recommSvc   *service.RecommendationService
	nlpSvc      *service.NLPQueryService
	aiMgr       *service.AIProviderManager
	projectRepo *repository.ProjectRepository
	orgService  *service.OrganizationService
}

func NewIntelligenceHandler(driftSvc *service.DriftService, anomalySvc *service.AnomalyService, analytics *service.UsageAnalyticsService, recommSvc *service.RecommendationService, nlpSvc *service.NLPQueryService, aiMgr *service.AIProviderManager, projectRepo *repository.ProjectRepository, orgService *service.OrganizationService) *IntelligenceHandler {
	return &IntelligenceHandler{driftSvc: driftSvc, anomalySvc: anomalySvc, analytics: analytics, recommSvc: recommSvc, nlpSvc: nlpSvc, aiMgr: aiMgr, projectRepo: projectRepo, orgService: orgService}
}

// accessibleProjectIDs returns every project the caller can act on (owned or via
// org membership). The global /ai/* endpoints have no :id and JWTAuth only, so
// this is how they are scoped to the caller's tenant instead of all tenants.
func (h *IntelligenceHandler) accessibleProjectIDs(c *gin.Context, uid uuid.UUID) ([]uuid.UUID, bool) {
	ids, err := h.projectRepo.ListAccessibleProjectIDs(uid)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to resolve project access")
		return nil, false
	}
	return ids, true
}

// listScope resolves which projects a global AI listing should cover: a single
// ?project_id the caller can access, else all projects they can access. Aborts
// (and returns ok=false) on an invalid id, denied access, or lookup error.
func (h *IntelligenceHandler) listScope(c *gin.Context, uid uuid.UUID) ([]uuid.UUID, bool) {
	if p := c.Query("project_id"); p != "" {
		pid, err := uuid.Parse(p)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "invalid project_id")
			return nil, false
		}
		allowed, err := h.projectRepo.UserHasAccess(uid, pid)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				WrapError(c, ErrNotFound)
				return nil, false
			}
			RespondError(c, http.StatusInternalServerError, "project access check failed")
			return nil, false
		}
		if !allowed {
			WrapError(c, ErrForbidden)
			return nil, false
		}
		return []uuid.UUID{pid}, true
	}
	return h.accessibleProjectIDs(c, uid)
}

// --- Providers ---
func (h *IntelligenceHandler) ListProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"providers": h.aiMgr.ListProviders(), "has_provider": h.aiMgr.HasProvider()})
}

// --- Drift ---
func (h *IntelligenceHandler) DetectDrift(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	var req struct {
		SourceEnv string `json:"source_env" binding:"required"`
		TargetEnv string `json:"target_env" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	check, err := h.driftSvc.DetectDrift(pid, uid, req.SourceEnv, req.TargetEnv)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"drift_check": check})
}

func (h *IntelligenceHandler) ListDriftChecks(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	checks, err := h.driftSvc.ListDriftChecks(pid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"drift_checks": checks})
}

// --- Drift Schedules ---
func (h *IntelligenceHandler) CreateDriftSchedule(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	var req struct {
		SourceEnv string `json:"source_env" binding:"required"`
		TargetEnv string `json:"target_env" binding:"required"`
		CronExpr  string `json:"cron_expr"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.CronExpr == "" {
		req.CronExpr = "0 */6 * * *"
	}
	sch, err := h.driftSvc.CreateSchedule(pid, req.SourceEnv, req.TargetEnv, req.CronExpr)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"schedule": sch})
}

func (h *IntelligenceHandler) ListDriftSchedules(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	schedules, err := h.driftSvc.ListSchedules(pid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"schedules": schedules})
}

func (h *IntelligenceHandler) UpdateDriftSchedule(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	id, err := uuid.Parse(c.Param("scheduleId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid schedule ID"})
		return
	}
	var req struct {
		Enabled  bool   `json:"enabled"`
		CronExpr string `json:"cron_expr"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Bind to :id (RequireProjectAccess already authorized it) so a schedule
	// from another project cannot be updated by its id.
	if err := h.driftSvc.UpdateSchedule(id, pid, req.Enabled, req.CronExpr); err != nil {
		if errors.Is(err, service.ErrDriftScheduleNotFound) {
			WrapError(c, ErrNotFound)
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "updated"})
}

func (h *IntelligenceHandler) DeleteDriftSchedule(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	id, err := uuid.Parse(c.Param("scheduleId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid schedule ID"})
		return
	}
	if err := h.driftSvc.DeleteSchedule(id, pid); err != nil {
		if errors.Is(err, service.ErrDriftScheduleNotFound) {
			WrapError(c, ErrNotFound)
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "deleted"})
}

func (h *IntelligenceHandler) RunScheduledDriftChecks(c *gin.Context) {
	count, err := h.driftSvc.RunScheduledChecks()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ran": count})
}

// --- Anomalies ---
func (h *IntelligenceHandler) RunAnomalyDetection(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	anomalies, err := h.anomalySvc.RunDetection(pid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"anomalies": anomalies, "count": len(anomalies)})
}

func (h *IntelligenceHandler) ListAnomalies(c *gin.Context) {
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	ids, ok := h.listScope(c, uid)
	if !ok {
		return
	}
	anomalies, err := h.anomalySvc.ListAnomalies(ids, c.DefaultQuery("status", ""))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"anomalies": anomalies})
}

func (h *IntelligenceHandler) AcknowledgeAnomaly(c *gin.Context) {
	id, err := uuid.Parse(c.Param("anomalyId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ID"})
		return
	}
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	ids, ok := h.accessibleProjectIDs(c, uid)
	if !ok {
		return
	}
	if err := h.anomalySvc.AcknowledgeAnomaly(id, ids, uid, c.ClientIP()); err != nil {
		if errors.Is(err, service.ErrAnomalyNotFound) {
			WrapError(c, ErrNotFound)
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "acknowledged"})
}

func (h *IntelligenceHandler) ResolveAnomaly(c *gin.Context) {
	id, err := uuid.Parse(c.Param("anomalyId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ID"})
		return
	}
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	ids, ok := h.accessibleProjectIDs(c, uid)
	if !ok {
		return
	}
	if err := h.anomalySvc.ResolveAnomaly(id, ids, uid, c.ClientIP()); err != nil {
		if errors.Is(err, service.ErrAnomalyNotFound) {
			WrapError(c, ErrNotFound)
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "resolved"})
}

// --- Alert Rules ---
func (h *IntelligenceHandler) CreateAlertRule(c *gin.Context) {
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	var req struct {
		ProjectID string         `json:"project_id"`
		APIKeyID  string         `json:"api_key_id"`
		RuleType  string         `json:"rule_type" binding:"required"`
		Config    models.JSONMap `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// A rule must belong to a project the caller can access. project_id is now
	// required (a global, cross-tenant rule is not a normal-user capability) and
	// access is enforced — closing the inject-rule-into-another-tenant hole.
	if req.ProjectID == "" {
		RespondError(c, http.StatusBadRequest, "project_id is required")
		return
	}
	projID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project_id")
		return
	}
	allowed, err := h.projectRepo.UserHasAccess(uid, projID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			WrapError(c, ErrNotFound)
			return
		}
		RespondError(c, http.StatusInternalServerError, "project access check failed")
		return
	}
	if !allowed {
		WrapError(c, ErrForbidden)
		return
	}
	pid := &projID
	var kid *uuid.UUID
	if req.APIKeyID != "" {
		k, _ := uuid.Parse(req.APIKeyID)
		kid = &k
	}
	rule, err := h.anomalySvc.CreateRule(pid, kid, req.RuleType, req.Config, uid, c.ClientIP())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"rule": rule})
}

func (h *IntelligenceHandler) ListAlertRules(c *gin.Context) {
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	ids, ok := h.listScope(c, uid)
	if !ok {
		return
	}
	rules, err := h.anomalySvc.ListRules(ids)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"rules": rules})
}

func (h *IntelligenceHandler) UpdateAlertRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("ruleId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ID"})
		return
	}
	var req struct {
		Enabled bool           `json:"enabled"`
		Config  models.JSONMap `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	ids, ok := h.accessibleProjectIDs(c, uid)
	if !ok {
		return
	}
	if err := h.anomalySvc.UpdateRule(id, req.Enabled, req.Config, ids, uid, c.ClientIP()); err != nil {
		if errors.Is(err, service.ErrRuleNotFound) {
			WrapError(c, ErrNotFound)
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "updated"})
}

func (h *IntelligenceHandler) DeleteAlertRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("ruleId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ID"})
		return
	}
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	ids, ok := h.accessibleProjectIDs(c, uid)
	if !ok {
		return
	}
	if err := h.anomalySvc.DeleteRule(id, ids, uid, c.ClientIP()); err != nil {
		if errors.Is(err, service.ErrRuleNotFound) {
			WrapError(c, ErrNotFound)
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "deleted"})
}

// --- Analytics ---
func (h *IntelligenceHandler) GetUsageTrends(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	period := c.DefaultQuery("period", "daily")
	days := 30
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	trends, err := h.analytics.GetTrends(pid, period, days)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"trends": trends, "period": period})
}

func (h *IntelligenceHandler) GetUsageForecast(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	days := 14
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	forecasts, err := h.analytics.Forecast(pid, days)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"forecasts": forecasts})
}

func (h *IntelligenceHandler) ExportAnalyticsCSV(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	period := c.DefaultQuery("period", "daily")
	days := 30
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	format := c.DefaultQuery("format", "csv")
	if format == "json" {
		trends, err := h.analytics.GetTrends(pid, period, days)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"trends": trends})
		return
	}
	csv, err := h.analytics.ExportCSV(pid, period, days)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=analytics.csv")
	c.String(200, csv)
}

// --- Quota ---
func (h *IntelligenceHandler) GetQuota(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid org ID"})
		return
	}
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	// Reading an org's quota requires org membership (any role). Without this
	// check any authenticated user could read any org's quota config.
	if err := h.orgService.CheckProjectAccess(orgID, uid, "viewer"); err != nil {
		WrapError(c, ErrForbidden)
		return
	}
	q, err := h.analytics.GetQuota(orgID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"quota": q})
}

func (h *IntelligenceHandler) SetQuota(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid org ID"})
		return
	}
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	// Writing quota is an admin action; without this check any authenticated
	// user could overwrite any org's resource limits.
	if err := h.orgService.CheckProjectAccess(orgID, uid, "admin"); err != nil {
		WrapError(c, ErrForbidden)
		return
	}
	var req struct {
		MaxSecrets        int `json:"max_secrets"`
		MaxProjects       int `json:"max_projects"`
		MaxAPIKeys        int `json:"max_api_keys"`
		MaxRequestsPerDay int `json:"max_requests_per_day"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	q, err := h.analytics.SetQuota(orgID, req.MaxSecrets, req.MaxProjects, req.MaxAPIKeys, req.MaxRequestsPerDay)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"quota": q})
}

// --- Recommendations ---
func (h *IntelligenceHandler) GenerateRecommendations(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	recs, err := h.recommSvc.GenerateRecommendations(pid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"recommendations": recs})
}

func (h *IntelligenceHandler) ListRecommendations(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	recs, err := h.recommSvc.ListRecommendations(pid, c.DefaultQuery("status", ""))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"recommendations": recs})
}

func (h *IntelligenceHandler) DismissRecommendation(c *gin.Context) {
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project ID"})
		return
	}
	id, err := uuid.Parse(c.Param("recId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ID"})
		return
	}
	// Bind to :id (already authorized by RequireProjectAccess) so another
	// project's recommendation cannot be dismissed by its id.
	if err := h.recommSvc.DismissRecommendation(id, pid); err != nil {
		if errors.Is(err, service.ErrRecommendationNotFound) {
			WrapError(c, ErrNotFound)
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "dismissed"})
}

// --- NLP ---
func (h *IntelligenceHandler) NLPQuery(c *gin.Context) {
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	var req struct {
		Query string `json:"query" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	result, err := h.nlpSvc.Query(uid, req.Query)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"result": result})
}

func (h *IntelligenceHandler) NLPConverse(c *gin.Context) {
	uid, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	var req struct {
		Messages []models.ConversationMessage `json:"messages" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	result, err := h.nlpSvc.Converse(uid, req.Messages)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"result": result})
}
