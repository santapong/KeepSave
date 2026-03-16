package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.authService.Register(req.Email, req.Password)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) LookupUser(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		RespondError(c, http.StatusBadRequest, "email query parameter required")
		return
	}

	user, err := h.authService.LookupByEmail(email)
	if err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": gin.H{"id": user.ID, "email": user.Email}})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}
