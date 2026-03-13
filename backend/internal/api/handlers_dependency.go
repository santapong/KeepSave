package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type DependencyHandler struct {
	depService *service.DependencyService
}

func NewDependencyHandler(depService *service.DependencyService) *DependencyHandler {
	return &DependencyHandler{depService: depService}
}

func (h *DependencyHandler) Analyze(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	envName := c.Query("environment")
	if envName == "" {
		envName = "alpha"
	}

	deps, err := h.depService.AnalyzeDependencies(projectID, envName)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if deps == nil {
		deps = []models.SecretDependency{}
	}
	c.JSON(http.StatusOK, gin.H{"dependencies": deps})
}

func (h *DependencyHandler) Graph(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	envName := c.Query("environment")
	if envName == "" {
		envName = "alpha"
	}

	graph, err := h.depService.GetDependencyGraph(projectID, envName)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if graph == nil {
		graph = []models.DependencyNode{}
	}
	c.JSON(http.StatusOK, gin.H{"graph": graph})
}
