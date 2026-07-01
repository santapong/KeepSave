package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type PromotionHandler struct {
	promotionService *service.PromotionService
}

func NewPromotionHandler(promotionService *service.PromotionService) *PromotionHandler {
	return &PromotionHandler{promotionService: promotionService}
}

// Promote handles POST /api/v1/projects/:id/promote
func (h *PromotionHandler) Promote(c *gin.Context) {
	var req PromoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	promotion, err := h.promotionService.Promote(
		projectID,
		req.SourceEnvironment,
		req.TargetEnvironment,
		req.Keys,
		req.OverridePolicy,
		req.Notes,
		userID,
		c.ClientIP(),
	)
	if err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	status := http.StatusOK
	if promotion.Status == "pending" {
		status = http.StatusAccepted
	}

	c.JSON(status, gin.H{"promotion": promotion})
}

// Diff handles POST /api/v1/projects/:id/promote/diff
func (h *PromotionHandler) Diff(c *gin.Context) {
	var req DiffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	diffs, err := h.promotionService.Diff(projectID, req.SourceEnvironment, req.TargetEnvironment, req.Keys)
	if err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	if diffs == nil {
		diffs = []models.DiffEntry{}
	}

	c.JSON(http.StatusOK, gin.H{"diff": diffs})
}

// ListPromotions handles GET /api/v1/projects/:id/promotions
func (h *PromotionHandler) ListPromotions(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	promotions, err := h.promotionService.ListPromotions(projectID)
	if err != nil {
		WrapError(c, err)
		return
	}

	if promotions == nil {
		promotions = []models.PromotionRequest{}
	}

	c.JSON(http.StatusOK, gin.H{"promotions": promotions})
}

// resolveScopedPromotion parses the :id (project) and :promotionId path params,
// loads the promotion, and verifies it belongs to that project. The
// /projects/:id route group already proves the caller may access project :id
// (RequireProjectAccess); this additionally prevents acting on another
// project's promotion by its UUID. On any failure it writes a 404
// (anti-enumeration: a promotion the caller may not see is indistinguishable
// from one that does not exist) and returns ok=false.
func (h *PromotionHandler) resolveScopedPromotion(c *gin.Context) (*models.PromotionRequest, bool) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return nil, false
	}
	promotionID, err := uuid.Parse(c.Param("promotionId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid promotion id")
		return nil, false
	}
	promotion, err := h.promotionService.GetPromotion(promotionID)
	if err != nil || promotion.ProjectID != projectID {
		WrapError(c, ErrNotFound)
		return nil, false
	}
	return promotion, true
}

// GetPromotion handles GET /api/v1/projects/:id/promotions/:promotionId
func (h *PromotionHandler) GetPromotion(c *gin.Context) {
	promotion, ok := h.resolveScopedPromotion(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"promotion": promotion})
}

// ApprovePromotion handles POST /api/v1/projects/:id/promotions/:promotionId/approve
func (h *PromotionHandler) ApprovePromotion(c *gin.Context) {
	promotion, ok := h.resolveScopedPromotion(c)
	if !ok {
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	updated, err := h.promotionService.ApprovePromotion(promotion.ID, userID, c.ClientIP())
	if err != nil {
		// Four-eyes invariant (ADR-0003 / NEGATIVE_AUTH_PLAN A10): the requester
		// approving their own promotion is an authorization failure, not a
		// malformed request — surface it as 403 FORBIDDEN.
		if errors.Is(err, service.ErrSelfApproval) {
			WrapError(c, Wrap(ErrForbidden, err))
			return
		}
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"promotion": updated})
}

// RejectPromotion handles POST /api/v1/projects/:id/promotions/:promotionId/reject
func (h *PromotionHandler) RejectPromotion(c *gin.Context) {
	promotion, ok := h.resolveScopedPromotion(c)
	if !ok {
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	updated, err := h.promotionService.RejectPromotion(promotion.ID, userID, c.ClientIP())
	if err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"promotion": updated})
}

// Rollback handles POST /api/v1/projects/:id/promotions/:promotionId/rollback
func (h *PromotionHandler) Rollback(c *gin.Context) {
	promotion, ok := h.resolveScopedPromotion(c)
	if !ok {
		return
	}

	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	if err := h.promotionService.Rollback(promotion.ID, userID, c.ClientIP()); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "rollback completed successfully"})
}

// AuditLog handles GET /api/v1/projects/:id/audit-log
func (h *PromotionHandler) AuditLog(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	// Cap to bound the DB scan / response size (AUTH-10).
	const maxAuditLimit = 500
	if limit > maxAuditLimit {
		limit = maxAuditLimit
	}

	entries, err := h.promotionService.ListAuditLog(projectID, limit)
	if err != nil {
		WrapError(c, err)
		return
	}

	if entries == nil {
		entries = []models.AuditEntry{}
	}

	c.JSON(http.StatusOK, gin.H{"audit_log": entries})
}
