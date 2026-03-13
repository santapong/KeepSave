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

	content, err := h.envFileService.Export(projectID, envName)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
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
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	result, err := h.envFileService.Import(projectID, req.Environment, req.Content, req.Overwrite)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result})
}
