package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// IntelligenceHandler handles Phase 15 AI intelligence endpoints.
type IntelligenceHandler struct {
	driftSvc   *service.DriftService
	anomalySvc *service.AnomalyService
	analytics  *service.UsageAnalyticsService
	recommSvc  *service.RecommendationService
	nlpSvc     *service.NLPQueryService
	aiMgr      *service.AIProviderManager
}

func NewIntelligenceHandler(
	driftSvc *service.DriftService,
	anomalySvc *service.AnomalyService,
	analytics *service.UsageAnalyticsService,
	recommSvc *service.RecommendationService,
	nlpSvc *service.NLPQueryService,
	aiMgr *service.AIProviderManager,
) *IntelligenceHandler {
	return &IntelligenceHandler{
		driftSvc: driftSvc, anomalySvc: anomalySvc, analytics: analytics,
		recommSvc: recommSvc, nlpSvc: nlpSvc, aiMgr: aiMgr,
	}
}

// --- AI Provider Status ---

func (h *IntelligenceHandler) ListProviders(c *gin.Context) {
	providers := h.aiMgr.ListProviders()
	c.JSON(http.StatusOK, gin.H{"providers": providers, "has_provider": h.aiMgr.HasProvider()})
}

// --- Drift Detection ---

func (h *IntelligenceHandler) DetectDrift(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}
	userID := c.MustGet("user_id").(uuid.UUID)

	var req struct {
		SourceEnv string `json:"source_env" binding:"required"`
		TargetEnv string `json:"target_env" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	check, err := h.driftSvc.DetectDrift(projectID, userID, req.SourceEnv, req.TargetEnv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"drift_check": check})
}

func (h *IntelligenceHandler) ListDriftChecks(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}
	checks, err := h.driftSvc.ListDriftChecks(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"drift_checks": checks})
}

// --- Anomaly Detection ---

func (h *IntelligenceHandler) RunAnomalyDetection(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}
	anomalies, err := h.anomalySvc.RunDetection(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"anomalies": anomalies, "count": len(anomalies)})
}

func (h *IntelligenceHandler) ListAnomalies(c *gin.Context) {
	var projectID *uuid.UUID
	if pid := c.Query("project_id"); pid != "" {
		if parsed, err := uuid.Parse(pid); err == nil {
			projectID = &parsed
		}
	}
	status := c.DefaultQuery("status", "")
	anomalies, err := h.anomalySvc.ListAnomalies(projectID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"anomalies": anomalies})
}

func (h *IntelligenceHandler) AcknowledgeAnomaly(c *gin.Context) {
	id, err := uuid.Parse(c.Param("anomalyId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid anomaly ID"})
		return
	}
	if err := h.anomalySvc.AcknowledgeAnomaly(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}

func (h *IntelligenceHandler) ResolveAnomaly(c *gin.Context) {
	id, err := uuid.Parse(c.Param("anomalyId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid anomaly ID"})
		return
	}
	if err := h.anomalySvc.ResolveAnomaly(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "resolved"})
}

// --- Usage Analytics & Forecasting ---

func (h *IntelligenceHandler) GetUsageTrends(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}
	period := c.DefaultQuery("period", "daily")
	days := 30
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	trends, err := h.analytics.GetTrends(projectID, period, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"trends": trends, "period": period})
}

func (h *IntelligenceHandler) GetUsageForecast(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}
	days := 14
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	forecasts, err := h.analytics.Forecast(projectID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"forecasts": forecasts})
}

// --- Smart Recommendations ---

func (h *IntelligenceHandler) GenerateRecommendations(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}
	userID := c.MustGet("user_id").(uuid.UUID)
	recs, err := h.recommSvc.GenerateRecommendations(projectID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recommendations": recs})
}

func (h *IntelligenceHandler) ListRecommendations(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}
	status := c.DefaultQuery("status", "")
	recs, err := h.recommSvc.ListRecommendations(projectID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recommendations": recs})
}

func (h *IntelligenceHandler) DismissRecommendation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("recId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recommendation ID"})
		return
	}
	if err := h.recommSvc.DismissRecommendation(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "dismissed"})
}

// --- NLP Query ---

func (h *IntelligenceHandler) NLPQuery(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	var req struct {
		Query string `json:"query" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.nlpSvc.Query(userID, req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result})
}
