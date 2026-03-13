package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type SecretHandler struct {
	secretService *service.SecretService
}

func NewSecretHandler(secretService *service.SecretService) *SecretHandler {
	return &SecretHandler{secretService: secretService}
}

func (h *SecretHandler) Create(c *gin.Context) {
	var req CreateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	secret, err := h.secretService.Create(projectID, req.Environment, req.Key, req.Value)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"secret": secret})
}

func (h *SecretHandler) List(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	envName := c.Query("environment")
	if envName == "" {
		RespondError(c, http.StatusBadRequest, "environment query parameter is required")
		return
	}

	secrets, err := h.secretService.List(projectID, envName)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if secrets == nil {
		secrets = []models.Secret{}
	}

	c.JSON(http.StatusOK, gin.H{"secrets": secrets})
}

func (h *SecretHandler) Get(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid secret id")
		return
	}

	secret, err := h.secretService.GetByID(projectID, secretID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"secret": secret})
}

func (h *SecretHandler) Update(c *gin.Context) {
	var req UpdateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid secret id")
		return
	}

	secret, err := h.secretService.Update(projectID, secretID, req.Value)
	if err != nil {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"secret": secret})
}

func (h *SecretHandler) Delete(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid secret id")
		return
	}

	if err := h.secretService.Delete(projectID, secretID); err != nil {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}

	c.Status(http.StatusNoContent)
}
