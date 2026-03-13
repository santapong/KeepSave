package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type TemplateHandler struct {
	templateService *service.TemplateService
}

func NewTemplateHandler(templateService *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{templateService: templateService}
}

func (h *TemplateHandler) Create(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	var orgID *uuid.UUID
	if req.OrganizationID != "" {
		id, err := uuid.Parse(req.OrganizationID)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "invalid organization id")
			return
		}
		orgID = &id
	}

	tmpl, err := h.templateService.Create(req.Name, req.Description, req.Stack, req.Keys, userID, orgID, req.IsGlobal)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"template": tmpl})
}

func (h *TemplateHandler) List(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	var orgID *uuid.UUID
	if orgIDStr := c.Query("organization_id"); orgIDStr != "" {
		id, err := uuid.Parse(orgIDStr)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "invalid organization id")
			return
		}
		orgID = &id
	}

	templates, err := h.templateService.List(userID, orgID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if templates == nil {
		templates = []models.SecretTemplate{}
	}

	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (h *TemplateHandler) Get(c *gin.Context) {
	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid template id")
		return
	}

	tmpl, err := h.templateService.GetByID(templateID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "template not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"template": tmpl})
}

func (h *TemplateHandler) Update(c *gin.Context) {
	var req UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid template id")
		return
	}

	tmpl, err := h.templateService.Update(templateID, req.Name, req.Description, req.Stack, req.Keys)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"template": tmpl})
}

func (h *TemplateHandler) Delete(c *gin.Context) {
	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid template id")
		return
	}

	if err := h.templateService.Delete(templateID); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TemplateHandler) Apply(c *gin.Context) {
	var req ApplyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid template id")
		return
	}

	projectID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	secrets, err := h.templateService.ApplyTemplate(templateID, projectID, req.Environment)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"secrets": secrets})
}

func (h *TemplateHandler) ListBuiltin(c *gin.Context) {
	templates := service.GetBuiltinTemplates()
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}
