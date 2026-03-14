package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// AgentHandler handles AI agent experience endpoints.
type AgentHandler struct {
	leaseService     *service.LeaseService
	analyticsService *service.AgentAnalyticsService
}

// NewAgentHandler creates a new agent handler.
func NewAgentHandler(leaseService *service.LeaseService, analyticsService *service.AgentAnalyticsService) *AgentHandler {
	return &AgentHandler{leaseService: leaseService, analyticsService: analyticsService}
}

// CreateLease grants time-limited access to specific secrets.
func (h *AgentHandler) CreateLease(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	var req struct {
		Environment string   `json:"environment" binding:"required,oneof=alpha uat prod"`
		SecretKeys  []string `json:"secret_keys" binding:"required"`
		DurationMin int      `json:"duration_minutes" binding:"required,min=1,max=1440"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Get API key ID from context (agent must use API key auth)
	apiKeyProjectID, exists := c.Get("api_key_project_id")
	if !exists {
		RespondError(c, http.StatusForbidden, "lease creation requires API key authentication")
		return
	}

	// Verify the API key is for this project
	if apiKeyProjectID.(uuid.UUID) != projectID {
		RespondError(c, http.StatusForbidden, "API key not authorized for this project")
		return
	}

	// Use a synthetic API key ID from context
	userID := c.MustGet("user_id").(uuid.UUID)
	duration := time.Duration(req.DurationMin) * time.Minute

	lease, err := h.leaseService.CreateLease(userID, projectID, req.Environment, req.SecretKeys, duration)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"lease": lease})
}

// ListLeases returns active leases for the current agent.
func (h *AgentHandler) ListLeases(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	leases, err := h.leaseService.ListActiveLeases(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"leases": leases})
}

// RevokeLease revokes an active lease.
func (h *AgentHandler) RevokeLease(c *gin.Context) {
	leaseID, err := uuid.Parse(c.Param("leaseId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid lease ID")
		return
	}

	if err := h.leaseService.RevokeLease(leaseID); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// GetActivitySummary returns agent activity summary.
func (h *AgentHandler) GetActivitySummary(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	summary, err := h.analyticsService.GetActivitySummary(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// GetRecentActivity returns recent agent activities for a project.
func (h *AgentHandler) GetRecentActivity(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	activities, err := h.analyticsService.GetRecentActivity(projectID, 100)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"activities": activities})
}

// GetAccessHeatmap returns secret access frequency data.
func (h *AgentHandler) GetAccessHeatmap(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	heatmap, err := h.analyticsService.GetAccessHeatmap(projectID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"heatmap": heatmap})
}
