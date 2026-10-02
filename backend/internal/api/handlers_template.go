package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
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
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	var orgID *uuid.UUID
	if req.OrganizationID != "" {
		id, err := uuid.Parse(req.OrganizationID)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "invalid organization id")
			return
		}
		orgID = &id
	}

	tmpl, err := h.templateService.CreateAuthorized(c.Request.Context(), PrincipalFromContext(c), req.Name, req.Description, req.Stack, req.Keys, orgID, req.IsGlobal, c.ClientIP())
	if err != nil {
		wrapTemplateError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"template": tmpl})
}

func (h *TemplateHandler) List(c *gin.Context) {
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
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
		wrapTemplateError(c, err)
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

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	tmpl, err := h.templateService.GetByID(templateID, userID)
	if err != nil {
		wrapTemplateError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"template": tmpl})
}

func (h *TemplateHandler) Update(c *gin.Context) {
	var req UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid template id")
		return
	}

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	tmpl, err := h.templateService.UpdateAuthorized(c.Request.Context(), PrincipalFromContext(c), templateID, req.Name, req.Description, req.Stack, req.Keys, c.ClientIP())
	if err != nil {
		wrapTemplateError(c, err)
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

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	if err := h.templateService.DeleteAuthorized(c.Request.Context(), PrincipalFromContext(c), templateID, c.ClientIP()); err != nil {
		wrapTemplateError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TemplateHandler) Apply(c *gin.Context) {
	var req ApplyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
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

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	secrets, err := h.templateService.ApplyTemplateAuthorized(c.Request.Context(), PrincipalFromContext(c), templateID, projectID, req.Environment)
	if err != nil {
		wrapTemplateError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"secrets": secrets})
}

func wrapTemplateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrSessionInvalid):
		RespondError(c, http.StatusUnauthorized, "invalid or expired session")
	case errors.Is(err, auth.ErrSessionUnavailable):
		RespondError(c, http.StatusServiceUnavailable, "session authority unavailable")
	case errors.Is(err, service.ErrTemplateProjectAccess):
		WrapError(c, ErrForbidden)
	case errors.Is(err, service.ErrTemplateNotFound):
		WrapError(c, ErrNotFound)
	default:
		WrapError(c, err)
	}
}

func (h *TemplateHandler) ListBuiltin(c *gin.Context) {
	templates := service.GetBuiltinTemplates()
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}
