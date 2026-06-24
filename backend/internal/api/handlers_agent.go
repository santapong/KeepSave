package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// AgentHandler handles AI agent experience endpoints.
type AgentHandler struct {
	leaseService      *service.LeaseService
	analyticsService  *service.AgentAnalyticsService
	agentTokenService *service.AgentTokenService
}

// NewAgentHandler creates a new agent handler.
func NewAgentHandler(leaseService *service.LeaseService, analyticsService *service.AgentAnalyticsService, agentTokenService *service.AgentTokenService) *AgentHandler {
	return &AgentHandler{leaseService: leaseService, analyticsService: analyticsService, agentTokenService: agentTokenService}
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
		WrapError(c, Wrap(ErrInvalidInput, err))
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
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	duration := time.Duration(req.DurationMin) * time.Minute

	lease, err := h.leaseService.CreateLease(userID, projectID, req.Environment, req.SecretKeys, duration, c.ClientIP())
	if err != nil {
		WrapError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"lease": lease})
}

// MintAgentToken exchanges an active lease for a short-lived, lease-bound agent
// token (ADR-0021). The :id project is already authorized by
// RequireProjectAccess; the lease must belong to that project.
func (h *AgentHandler) MintAgentToken(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}
	var req struct {
		LeaseID     string `json:"lease_id" binding:"required"`
		DurationMin int    `json:"duration_minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}
	leaseID, err := uuid.Parse(req.LeaseID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid lease ID")
		return
	}
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	ttl := time.Duration(req.DurationMin) * time.Minute

	minted, err := h.agentTokenService.MintToken(userID, projectID, leaseID, ttl, c.ClientIP())
	if err != nil {
		if errors.Is(err, service.ErrLeaseNotActive) {
			WrapError(c, ErrNotFound)
			return
		}
		if errors.Is(err, service.ErrLeaseProjectMismatch) {
			RespondError(c, http.StatusForbidden, "lease not authorized for this project")
			return
		}
		WrapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, minted)
}

// RevokeAgentToken adds an agent token's jti to the denylist (ADR-0021),
// invalidating it before its natural expiry.
func (h *AgentHandler) RevokeAgentToken(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}
	var req struct {
		JTI string `json:"jti" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	if err := h.agentTokenService.RevokeToken(userID, projectID, req.JTI, c.ClientIP()); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ListLeases returns active leases for the current agent.
func (h *AgentHandler) ListLeases(c *gin.Context) {
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	leases, err := h.leaseService.ListActiveLeases(userID)
	if err != nil {
		WrapError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"leases": leases})
}

// RevokeLease revokes an active lease. The lease is scoped to the :id project
// (already authorized by RequireProjectAccess) so a caller cannot revoke a
// lease belonging to another project by its ID.
func (h *AgentHandler) RevokeLease(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}
	leaseID, err := uuid.Parse(c.Param("leaseId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid lease ID")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	if err := h.leaseService.RevokeLease(leaseID, projectID, userID, c.ClientIP()); err != nil {
		if errors.Is(err, service.ErrLeaseNotFound) {
			WrapError(c, ErrNotFound)
			return
		}
		WrapError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetActivitySummary returns agent activity summary.
func (h *AgentHandler) GetActivitySummary(c *gin.Context) {
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	summary, err := h.analyticsService.GetActivitySummary(userID)
	if err != nil {
		WrapError(c, err)
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
		WrapError(c, err)
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
		WrapError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"heatmap": heatmap})
}
