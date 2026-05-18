package api

import (
	"errors"
	"net/http"
	"time"

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
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	projectID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project_id")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsed, perr := time.Parse(time.RFC3339, *req.ExpiresAt)
		if perr != nil {
			WrapError(c, Wrap(ErrInvalidInput, perr))
			return
		}
		expiresAt = &parsed
	}

	resp, err := h.apikeyService.Create(req.Name, userID, projectID, req.Scopes, req.Environment, expiresAt, c.GetString("client_ip"))
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			RespondError(c, http.StatusNotFound, "project not found")
			return
		}
		if errors.Is(err, service.ErrNotAuthorized) {
			RespondError(c, http.StatusForbidden, "not authorized for this project")
			return
		}
		if errors.Is(err, service.ErrAPIKeyExpiryOutOfRange) {
			WrapError(c, Wrap(ErrInvalidInput, err))
			return
		}
		WrapError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *APIKeyHandler) List(c *gin.Context) {
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	keys, err := h.apikeyService.List(userID)
	if err != nil {
		WrapError(c, err)
		return
	}

	if keys == nil {
		keys = []models.APIKey{}
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

func (h *APIKeyHandler) Delete(c *gin.Context) {
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid api key id")
		return
	}

	if err := h.apikeyService.Delete(keyID, userID, c.GetString("client_ip")); err != nil {
		RespondError(c, http.StatusNotFound, "api key not found")
		return
	}

	c.Status(http.StatusNoContent)
}
