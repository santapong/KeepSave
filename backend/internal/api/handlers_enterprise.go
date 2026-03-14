package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// EnterpriseHandler handles enterprise feature endpoints.
type EnterpriseHandler struct {
	ssoService        *service.SSOService
	complianceService *service.ComplianceService
	backupService     *service.BackupService
	policyService     *service.SecretPolicyService
}

// NewEnterpriseHandler creates a new enterprise handler.
func NewEnterpriseHandler(sso *service.SSOService, compliance *service.ComplianceService, backup *service.BackupService, policy *service.SecretPolicyService) *EnterpriseHandler {
	return &EnterpriseHandler{
		ssoService:        sso,
		complianceService: compliance,
		backupService:     backup,
		policyService:     policy,
	}
}

// ConfigureSSO sets up an SSO provider for an organization.
func (h *EnterpriseHandler) ConfigureSSO(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization ID")
		return
	}

	var req struct {
		Provider     string         `json:"provider" binding:"required,oneof=oidc saml"`
		IssuerURL    string         `json:"issuer_url" binding:"required"`
		ClientID     string         `json:"client_id" binding:"required"`
		ClientSecret string         `json:"client_secret" binding:"required"`
		Metadata     models.JSONMap `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	config, err := h.ssoService.ConfigureSSO(orgID, req.Provider, req.IssuerURL, req.ClientID, req.ClientSecret, req.Metadata)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"sso_config": config})
}

// ListSSOConfigs returns SSO configurations for an organization.
func (h *EnterpriseHandler) ListSSOConfigs(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization ID")
		return
	}

	configs, err := h.ssoService.ListSSOConfigs(orgID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"sso_configs": configs})
}

// DeleteSSOConfig removes an SSO configuration.
func (h *EnterpriseHandler) DeleteSSOConfig(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization ID")
		return
	}
	provider := c.Param("provider")

	if err := h.ssoService.DeleteSSOConfig(orgID, provider); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// GenerateComplianceReport creates a compliance report.
func (h *EnterpriseHandler) GenerateComplianceReport(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization ID")
		return
	}

	var req struct {
		ReportType string `json:"report_type" binding:"required,oneof=soc2 gdpr pci"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	report, err := h.complianceService.GenerateReport(orgID, userID, req.ReportType)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"report": report})
}

// ListComplianceReports returns compliance reports for an organization.
func (h *EnterpriseHandler) ListComplianceReports(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("orgId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid organization ID")
		return
	}

	reports, err := h.complianceService.ListReports(orgID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports})
}

// CreateBackup creates an encrypted backup of a project.
func (h *EnterpriseHandler) CreateBackup(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	var req struct {
		Type string `json:"type" binding:"omitempty,oneof=full incremental"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Type = "full"
	}
	if req.Type == "" {
		req.Type = "full"
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	snapshot, err := h.backupService.CreateBackup(projectID, userID, req.Type)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"backup": snapshot})
}

// ListBackups returns backups for a project.
func (h *EnterpriseHandler) ListBackups(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	backups, err := h.backupService.ListBackups(projectID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"backups": backups})
}

// GetSecretPolicy returns the secret policy for a project.
func (h *EnterpriseHandler) GetSecretPolicy(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	policy, err := h.policyService.GetPolicy(projectID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "no policy configured")
		return
	}

	c.JSON(http.StatusOK, gin.H{"policy": policy})
}

// SetSecretPolicy creates or updates a secret policy.
func (h *EnterpriseHandler) SetSecretPolicy(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	var req struct {
		MaxAgeDays      int  `json:"max_age_days" binding:"required,min=1"`
		ReminderDays    int  `json:"rotation_reminder_days" binding:"required,min=1"`
		RequireRotation bool `json:"require_rotation"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	policy, err := h.policyService.SetPolicy(projectID, req.MaxAgeDays, req.ReminderDays, req.RequireRotation)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"policy": policy})
}

// ListSecurityEvents returns recent security events.
func (h *EnterpriseHandler) ListSecurityEvents(c *gin.Context) {
	// This is handled via the security event repo directly
	c.JSON(http.StatusOK, gin.H{"message": "use /api/v1/admin/security-events"})
}
