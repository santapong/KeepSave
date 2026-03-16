package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type MCPHubHandler struct {
	mcpService     *service.MCPService
	builderService *service.MCPBuilderService
}

func NewMCPHubHandler(mcpService *service.MCPService, builderService *service.MCPBuilderService) *MCPHubHandler {
	return &MCPHubHandler{mcpService: mcpService, builderService: builderService}
}

// Server Management

func (h *MCPHubHandler) RegisterServer(c *gin.Context) {
	var req RegisterMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	var envMappings models.JSONMap
	if req.EnvMappings != nil {
		envMappings = req.EnvMappings
	}

	server, err := h.mcpService.RegisterServer(
		req.Name, req.Description, userID, req.GitHubURL, req.GitHubBranch,
		req.EntryCommand, req.Transport, req.IconURL, req.Version, envMappings, req.IsPublic,
	)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Trigger build asynchronously
	go h.builderService.BuildServer(server.ID)

	c.JSON(http.StatusCreated, gin.H{"server": server})
}

func (h *MCPHubHandler) GetServer(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid server id")
		return
	}

	server, err := h.mcpService.GetServer(serverID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "server not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"server": server})
}

func (h *MCPHubHandler) ListPublicServers(c *gin.Context) {
	servers, err := h.mcpService.ListPublicServers()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if servers == nil {
		servers = []models.MCPServer{}
	}

	c.JSON(http.StatusOK, gin.H{"servers": servers})
}

func (h *MCPHubHandler) ListMyServers(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	servers, err := h.mcpService.ListMyServers(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if servers == nil {
		servers = []models.MCPServer{}
	}

	c.JSON(http.StatusOK, gin.H{"servers": servers})
}

func (h *MCPHubHandler) UpdateServer(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid server id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	var req UpdateMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	server := &models.MCPServer{
		ID:           serverID,
		OwnerID:      userID,
		Name:         req.Name,
		Description:  req.Description,
		GitHubBranch: req.GitHubBranch,
		EntryCommand: req.EntryCommand,
		Transport:    req.Transport,
		IconURL:      req.IconURL,
		Version:      req.Version,
		IsPublic:     req.IsPublic,
	}
	if req.EnvMappings != nil {
		server.EnvMappings = req.EnvMappings
	}

	if err := h.mcpService.UpdateServer(server); err != nil {
		RespondError(c, http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"server": server})
}

func (h *MCPHubHandler) DeleteServer(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid server id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	if err := h.mcpService.DeleteServer(serverID, userID); err != nil {
		RespondError(c, http.StatusNotFound, err.Error())
		return
	}

	h.builderService.CleanupBuild(serverID)
	c.Status(http.StatusNoContent)
}

func (h *MCPHubHandler) RebuildServer(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid server id")
		return
	}

	// Trigger rebuild asynchronously
	go h.builderService.RebuildServer(serverID)

	c.JSON(http.StatusAccepted, gin.H{"message": "rebuild started"})
}

// Installation Management

func (h *MCPHubHandler) InstallServer(c *gin.Context) {
	var req InstallMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	mcpServerID, err := uuid.Parse(req.MCPServerID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid server id")
		return
	}

	var projectID *uuid.UUID
	if req.ProjectID != "" {
		pid, err := uuid.Parse(req.ProjectID)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "invalid project id")
			return
		}
		projectID = &pid
	}

	var config models.JSONMap
	if req.Config != nil {
		config = req.Config
	}

	inst, err := h.mcpService.InstallServer(userID, mcpServerID, projectID, config)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"installation": inst})
}

func (h *MCPHubHandler) ListInstallations(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	installations, err := h.mcpService.ListInstallations(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if installations == nil {
		installations = []models.MCPInstallation{}
	}

	c.JSON(http.StatusOK, gin.H{"installations": installations})
}

func (h *MCPHubHandler) UpdateInstallation(c *gin.Context) {
	instID, err := uuid.Parse(c.Param("installId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid installation id")
		return
	}

	var req UpdateInstallationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.mcpService.UpdateInstallation(instID, req.Enabled, req.Config); err != nil {
		RespondError(c, http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": true})
}

func (h *MCPHubHandler) UninstallServer(c *gin.Context) {
	instID, err := uuid.Parse(c.Param("installId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid installation id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	if err := h.mcpService.UninstallServer(instID, userID); err != nil {
		RespondError(c, http.StatusNotFound, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// Gateway

func (h *MCPHubHandler) ListUserTools(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	serversWithTools, err := h.mcpService.ListUserTools(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if serversWithTools == nil {
		serversWithTools = []models.MCPServerWithTools{}
	}

	c.JSON(http.StatusOK, gin.H{"servers": serversWithTools})
}

func (h *MCPHubHandler) GetGatewayStats(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	stats, err := h.mcpService.GetGatewayStats(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if stats == nil {
		stats = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}
