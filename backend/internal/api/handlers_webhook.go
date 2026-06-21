package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// WebhookHandler handles webhook management endpoints.
type WebhookHandler struct {
	webhookService *service.WebhookService
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(webhookService *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookService: webhookService}
}

type registerWebhookRequest struct {
	URL    string   `json:"url" binding:"required"`
	Secret string   `json:"secret"`
	Events []string `json:"events"`
}

// Register adds a webhook for a project.
func (h *WebhookHandler) Register(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	var req registerWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	config := service.WebhookConfig{
		URL:    req.URL,
		Secret: req.Secret,
		Events: req.Events,
	}

	if err := h.webhookService.RegisterWebhook(projectID, config, userID, c.ClientIP()); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "webhook registered successfully",
		"webhook": gin.H{
			"url":    req.URL,
			"events": req.Events,
		},
	})
}

// List returns all webhooks for a project.
func (h *WebhookHandler) List(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	configs := h.webhookService.ListWebhooks(projectID)
	c.JSON(http.StatusOK, configs)
}

// Remove removes all webhooks for a project.
func (h *WebhookHandler) Remove(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	h.webhookService.RemoveWebhooks(projectID, userID, c.ClientIP())
	c.Status(http.StatusNoContent)
}

// Deliveries returns recent webhook delivery records.
func (h *WebhookHandler) Deliveries(c *gin.Context) {
	deliveries := h.webhookService.GetDeliveries()
	c.JSON(http.StatusOK, deliveries)
}
