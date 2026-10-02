package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type OrganizationHandler struct {
	orgService *service.OrganizationService
}

func NewOrganizationHandler(orgService *service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{orgService: orgService}
}

func (h *OrganizationHandler) Create(c *gin.Context) {
	var req CreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	org, err := h.orgService.CreateWorkspace(req.Name, userID, c.GetString("client_ip"), c.GetHeader("Idempotency-Key"))
	if err != nil {
		respondOrganizationError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"organization": org})
}

func (h *OrganizationHandler) List(c *gin.Context) {
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	orgs, err := h.orgService.List(userID)
	if err != nil {
		WrapError(c, err)
		return
	}
	if orgs == nil {
		orgs = []models.Organization{}
	}
	c.JSON(http.StatusOK, gin.H{"organizations": orgs})
}

func (h *OrganizationHandler) Get(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	org, err := h.orgService.GetByID(orgID, userID)
	if err != nil {
		WrapError(c, Wrap(ErrNotFound, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"organization": org})
}

func (h *OrganizationHandler) Update(c *gin.Context) {
	var req UpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	org, err := h.orgService.Update(orgID, userID, req.Name, c.ClientIP())
	if err != nil {
		respondOrganizationError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"organization": org})
}

func (h *OrganizationHandler) Delete(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	if err := h.orgService.Delete(orgID, userID, c.ClientIP()); err != nil {
		respondOrganizationError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *OrganizationHandler) AddMember(c *gin.Context) {
	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	targetUserID, err := uuid.Parse(req.UserID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid user id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	member, err := h.orgService.AddMember(orgID, userID, targetUserID, req.Role, c.ClientIP())
	if err != nil {
		respondOrganizationError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"member": member})
}

func (h *OrganizationHandler) ListMembers(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	members, err := h.orgService.ListMembers(orgID, userID)
	if err != nil {
		WrapError(c, Wrap(ErrForbidden, err))
		return
	}

	if members == nil {
		members = []models.OrgMember{}
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

func (h *OrganizationHandler) UpdateMemberRole(c *gin.Context) {
	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	memberUserID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid user id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	member, err := h.orgService.UpdateMemberRole(orgID, userID, memberUserID, req.Role, c.ClientIP())
	if err != nil {
		respondOrganizationError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"member": member})
}

func (h *OrganizationHandler) RemoveMember(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	memberUserID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid user id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	if err := h.orgService.RemoveMember(orgID, userID, memberUserID, c.ClientIP()); err != nil {
		respondOrganizationError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *OrganizationHandler) AssignProject(c *gin.Context) {
	var req AssignProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	projectID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	if err := h.orgService.AssignProjectWithAudit(orgID, userID, projectID, c.GetString("client_ip")); err != nil {
		respondOrganizationError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "project assigned to organization"})
}

func respondOrganizationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrWorkspaceInput):
		WrapError(c, Wrap(ErrInvalidInput, err))
	case errors.Is(err, service.ErrOrgAccessDenied):
		WrapError(c, Wrap(ErrForbidden, err))
	case errors.Is(err, service.ErrOrganizationConflict):
		WrapError(c, WrapMessage(ErrConflict, "workspace operation conflicts with current state", err))
	default:
		WrapError(c, err)
	}
}

func (h *OrganizationHandler) ListProjects(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	projects, err := h.orgService.ListProjects(orgID, userID)
	if err != nil {
		WrapError(c, Wrap(ErrForbidden, err))
		return
	}

	if projects == nil {
		projects = []models.Project{}
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}
