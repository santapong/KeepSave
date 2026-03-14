package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/events"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/plugins"
)

// PlatformHandler handles platform ecosystem endpoints.
type PlatformHandler struct {
	eventBus       *events.Bus
	pluginRegistry *plugins.Registry
	policyDB       policyStore
}

type policyStore interface {
	GetPolicies(projectID uuid.UUID) ([]models.AccessPolicy, error)
	CreatePolicy(policy *models.AccessPolicy) (*models.AccessPolicy, error)
	DeletePolicy(policyID uuid.UUID) error
}

// NewPlatformHandler creates a new platform handler.
func NewPlatformHandler(bus *events.Bus, registry *plugins.Registry, ps policyStore) *PlatformHandler {
	return &PlatformHandler{eventBus: bus, pluginRegistry: registry, policyDB: ps}
}

// ListEvents returns recent events from the event log.
func (h *PlatformHandler) ListEvents(c *gin.Context) {
	evts, err := h.eventBus.GetRecentEvents(100)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": evts})
}

// ReplayEvents replays events of a given type.
func (h *PlatformHandler) ReplayEvents(c *gin.Context) {
	var req struct {
		EventType string `json:"event_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.eventBus.Replay(req.EventType, time.Now().Add(-24*time.Hour)); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "replay initiated"})
}

// ListPlugins returns registered plugins.
func (h *PlatformHandler) ListPlugins(c *gin.Context) {
	pluginList, err := h.pluginRegistry.ListPlugins()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"plugins": pluginList})
}

// RegisterPlugin adds a new plugin to the registry.
func (h *PlatformHandler) RegisterPlugin(c *gin.Context) {
	var req struct {
		Name       string         `json:"name" binding:"required"`
		PluginType string         `json:"plugin_type" binding:"required,oneof=secret_provider notification validation"`
		Version    string         `json:"version" binding:"required"`
		Config     models.JSONMap `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	plugin, err := h.pluginRegistry.RegisterPlugin(req.Name, req.PluginType, req.Version, req.Config)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"plugin": plugin})
}

// TogglePlugin enables or disables a plugin.
func (h *PlatformHandler) TogglePlugin(c *gin.Context) {
	pluginID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid plugin ID")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.pluginRegistry.TogglePlugin(pluginID, req.Enabled); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "plugin updated"})
}

// ListAccessPolicies returns access policies for a project.
func (h *PlatformHandler) ListAccessPolicies(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	policies, err := h.policyDB.GetPolicies(projectID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// CreateAccessPolicy creates a new access policy.
func (h *PlatformHandler) CreateAccessPolicy(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	var req struct {
		PolicyType string         `json:"policy_type" binding:"required,oneof=time_window ip_restriction geo_restriction"`
		Config     models.JSONMap `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	policy := &models.AccessPolicy{
		ProjectID:  projectID,
		PolicyType: req.PolicyType,
		Config:     req.Config,
		Enabled:    true,
		CreatedBy:  userID,
	}

	created, err := h.policyDB.CreatePolicy(policy)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"policy": created})
}

// DeleteAccessPolicy removes an access policy.
func (h *PlatformHandler) DeleteAccessPolicy(c *gin.Context) {
	policyID, err := uuid.Parse(c.Param("policyId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid policy ID")
		return
	}

	if err := h.policyDB.DeletePolicy(policyID); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}
