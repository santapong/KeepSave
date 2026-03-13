package api

import (
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
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

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
		RespondError(c, http.StatusBadRequest, err.Error())
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
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	diffs, err := h.promotionService.Diff(projectID, req.SourceEnvironment, req.TargetEnvironment, req.Keys)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
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
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if promotions == nil {
		promotions = []models.PromotionRequest{}
	}

	c.JSON(http.StatusOK, gin.H{"promotions": promotions})
}

// GetPromotion handles GET /api/v1/projects/:id/promotions/:promotionId
func (h *PromotionHandler) GetPromotion(c *gin.Context) {
	promotionID, err := uuid.Parse(c.Param("promotionId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid promotion id")
		return
	}

	promotion, err := h.promotionService.GetPromotion(promotionID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "promotion not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"promotion": promotion})
}

// ApprovePromotion handles POST /api/v1/projects/:id/promotions/:promotionId/approve
func (h *PromotionHandler) ApprovePromotion(c *gin.Context) {
	promotionID, err := uuid.Parse(c.Param("promotionId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid promotion id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	promotion, err := h.promotionService.ApprovePromotion(promotionID, userID, c.ClientIP())
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"promotion": promotion})
}

// RejectPromotion handles POST /api/v1/projects/:id/promotions/:promotionId/reject
func (h *PromotionHandler) RejectPromotion(c *gin.Context) {
	promotionID, err := uuid.Parse(c.Param("promotionId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid promotion id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	promotion, err := h.promotionService.RejectPromotion(promotionID, userID, c.ClientIP())
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"promotion": promotion})
}

// Rollback handles POST /api/v1/projects/:id/promotions/:promotionId/rollback
func (h *PromotionHandler) Rollback(c *gin.Context) {
	promotionID, err := uuid.Parse(c.Param("promotionId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid promotion id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	if err := h.promotionService.Rollback(promotionID, userID, c.ClientIP()); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
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

	entries, err := h.promotionService.ListAuditLog(projectID, limit)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if entries == nil {
		entries = []models.AuditEntry{}
	}

	c.JSON(http.StatusOK, gin.H{"audit_log": entries})
}
