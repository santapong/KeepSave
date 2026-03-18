package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type ApplicationHandler struct {
	appService *service.ApplicationService
}

func NewApplicationHandler(appService *service.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{appService: appService}
}

type CreateApplicationRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=255"`
	URL         string `json:"url" binding:"required"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
}

type UpdateApplicationRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=255"`
	URL         string `json:"url" binding:"required"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
}

func (h *ApplicationHandler) Create(c *gin.Context) {
	var req CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	app, err := h.appService.Create(req.Name, req.URL, req.Description, req.Icon, req.Category, userID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"application": app})
}

func (h *ApplicationHandler) List(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	search := c.Query("search")
	category := c.Query("category")

	// Pagination
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > 100 {
				v = 100
			}
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	apps, total, err := h.appService.List(userID, search, category, limit, offset)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if apps == nil {
		apps = make([]models.Application, 0)
	}

	categories, _ := h.appService.GetCategories(userID)
	if categories == nil {
		categories = []string{}
	}

	c.JSON(http.StatusOK, gin.H{
		"applications": apps,
		"categories":   categories,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	})
}

func (h *ApplicationHandler) Get(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("appId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid application id")
		return
	}

	app, err := h.appService.Get(appID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "application not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"application": app})
}

func (h *ApplicationHandler) Update(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("appId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid application id")
		return
	}

	var req UpdateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	app, err := h.appService.Update(appID, req.Name, req.URL, req.Description, req.Icon, req.Category, userID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"application": app})
}

func (h *ApplicationHandler) Delete(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("appId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid application id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	if err := h.appService.Delete(appID, userID); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ApplicationHandler) ToggleFavorite(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("appId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid application id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	isFavorite, err := h.appService.ToggleFavorite(userID, appID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"is_favorite": isFavorite})
}
