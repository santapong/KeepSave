package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type ProjectHandler struct {
	projectService *service.ProjectService
}

func NewProjectHandler(projectService *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{projectService: projectService}
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	_ = userID
	project, err := h.projectService.CreateAuthorized(c.Request.Context(), PrincipalFromContext(c), req.Name, req.Description, c.GetString("client_ip"))
	if err != nil {
		WrapError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"project": project})
}

func (h *ProjectHandler) List(c *gin.Context) {
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	projects, err := h.projectService.List(userID)
	if err != nil {
		WrapError(c, err)
		return
	}

	if projects == nil {
		projects = []models.Project{}
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (h *ProjectHandler) Get(c *gin.Context) {
	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	project, err := h.projectService.GetByIDAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "project not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"project": project})
}

func (h *ProjectHandler) Update(c *gin.Context) {
	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	project, err := h.projectService.UpdateAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, req.Name, req.Description, c.ClientIP())
	if err != nil {
		RespondError(c, http.StatusNotFound, "project not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"project": project})
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	_ = userID
	if err := h.projectService.DeleteAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, c.GetString("client_ip")); err != nil {
		RespondError(c, http.StatusNotFound, "project not found")
		return
	}

	c.Status(http.StatusNoContent)
}
