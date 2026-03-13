package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type APIKeyHandler struct {
	apikeyService *service.APIKeyService
}

func NewAPIKeyHandler(apikeyService *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{apikeyService: apikeyService}
}

func (h *APIKeyHandler) Create(c *gin.Context) {
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	projectID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project_id")
		return
	}

	resp, err := h.apikeyService.Create(req.Name, userID, projectID, req.Scopes, req.Environment)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *APIKeyHandler) List(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	keys, err := h.apikeyService.List(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if keys == nil {
		keys = []models.APIKey{}
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

func (h *APIKeyHandler) Delete(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid api key id")
		return
	}

	if err := h.apikeyService.Delete(keyID, userID); err != nil {
		RespondError(c, http.StatusNotFound, "api key not found")
		return
	}

	c.Status(http.StatusNoContent)
}
