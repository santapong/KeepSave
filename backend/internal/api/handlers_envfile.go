package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type EnvFileHandler struct {
	envFileService *service.EnvFileService
}

func NewEnvFileHandler(envFileService *service.EnvFileService) *EnvFileHandler {
	return &EnvFileHandler{envFileService: envFileService}
}

func (h *EnvFileHandler) Export(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	envName := c.Query("environment")
	if envName == "" {
		envName = "alpha"
	}

	content, err := h.envFileService.ExportAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, envName)
	if err != nil {
		WrapError(c, err)
		return
	}

	format := c.Query("format")
	if format == "file" {
		c.Header("Content-Disposition", "attachment; filename=.env")
		c.Data(http.StatusOK, "text/plain", []byte(content))
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": content, "environment": envName})
}

func (h *EnvFileHandler) Import(c *gin.Context) {
	var req ImportEnvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	result, err := h.envFileService.ImportAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, req.Environment, req.Content, req.Overwrite, c.ClientIP())
	if err != nil {
		WrapError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result})
}
