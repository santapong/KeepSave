package api

import (
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
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	org, err := h.orgService.Create(req.Name, userID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"organization": org})
}

func (h *OrganizationHandler) List(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	orgs, err := h.orgService.List(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
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

	userID := c.MustGet("user_id").(uuid.UUID)
	org, err := h.orgService.GetByID(orgID, userID)
	if err != nil {
		RespondError(c, http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"organization": org})
}

func (h *OrganizationHandler) Update(c *gin.Context) {
	var req UpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	org, err := h.orgService.Update(orgID, userID, req.Name)
	if err != nil {
		RespondError(c, http.StatusForbidden, err.Error())
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

	userID := c.MustGet("user_id").(uuid.UUID)
	if err := h.orgService.Delete(orgID, userID); err != nil {
		RespondError(c, http.StatusForbidden, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *OrganizationHandler) AddMember(c *gin.Context) {
	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
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

	userID := c.MustGet("user_id").(uuid.UUID)
	member, err := h.orgService.AddMember(orgID, userID, targetUserID, req.Role)
	if err != nil {
		RespondError(c, http.StatusForbidden, err.Error())
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

	userID := c.MustGet("user_id").(uuid.UUID)
	members, err := h.orgService.ListMembers(orgID, userID)
	if err != nil {
		RespondError(c, http.StatusForbidden, err.Error())
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
		RespondError(c, http.StatusBadRequest, err.Error())
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

	userID := c.MustGet("user_id").(uuid.UUID)
	member, err := h.orgService.UpdateMemberRole(orgID, userID, memberUserID, req.Role)
	if err != nil {
		RespondError(c, http.StatusForbidden, err.Error())
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

	userID := c.MustGet("user_id").(uuid.UUID)
	if err := h.orgService.RemoveMember(orgID, userID, memberUserID); err != nil {
		RespondError(c, http.StatusForbidden, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *OrganizationHandler) AssignProject(c *gin.Context) {
	var req AssignProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
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

	userID := c.MustGet("user_id").(uuid.UUID)
	if err := h.orgService.AssignProject(orgID, userID, projectID); err != nil {
		RespondError(c, http.StatusForbidden, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "project assigned to organization"})
}

func (h *OrganizationHandler) ListProjects(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	projects, err := h.orgService.ListProjects(orgID, userID)
	if err != nil {
		RespondError(c, http.StatusForbidden, err.Error())
		return
	}

	if projects == nil {
		projects = []models.Project{}
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}
