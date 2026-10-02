package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"net/http"
)

type SocialAuthHandler struct{ service *service.SocialAuthService }

func NewSocialAuthHandler(s *service.SocialAuthService) *SocialAuthHandler {
	return &SocialAuthHandler{s}
}
func (h *AuthHandler) SetSocial(s *SocialAuthHandler) { h.social = s }

func socialResponseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrRecentAuthentication):
		WrapError(c, WrapMessage(ErrForbidden, "Sign in again before connecting a provider. Recent authentication is required.", nil))
	case errors.Is(err, auth.ErrSessionUnavailable):
		WrapError(c, ErrServiceUnavailable)
	case errors.Is(err, auth.ErrSessionInvalid):
		WrapError(c, ErrUnauthorized)
	case errors.Is(err, service.ErrSocialUnavailable):
		WrapError(c, WrapMessage(ErrServiceUnavailable, "This sign-in provider has not been configured.", nil))
	case errors.Is(err, repository.ErrSocialConflict):
		WrapError(c, WrapMessage(ErrConflict, "Sign in with your existing method, then connect this provider from Account.", nil))
	case errors.Is(err, repository.ErrSocialFlow):
		WrapError(c, WrapMessage(ErrInvalidInput, "This sign-in attempt expired or could not be verified. Please start again.", nil))
	case errors.Is(err, service.ErrSocialIdentity):
		WrapError(c, WrapMessage(ErrUnauthorized, "We could not verify your provider account. A verified email is required. Please try again.", nil))
	default:
		WrapError(c, ErrInternal) // Do not log provider responses, codes or token-bearing errors.
	}
}
func (h *SocialAuthHandler) Providers(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, h.service.Providers())
}
func socialUser(c *gin.Context, link bool) (*uuid.UUID, bool) {
	if !link {
		return nil, true
	}
	value, ok := c.Get("user_id")
	id, valid := value.(uuid.UUID)
	if !ok || !valid || id == uuid.Nil {
		WrapError(c, ErrUnauthorized)
		return nil, false
	}
	return &id, true
}
func (h *SocialAuthHandler) Start(link bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		user, ok := socialUser(c, link)
		if !ok {
			return
		}
		var req struct {
			Challenge string `json:"code_challenge" binding:"required,max=43"`
		}
		if c.ShouldBindJSON(&req) != nil {
			WrapError(c, ErrInvalidInput)
			return
		}
		result, err := h.service.StartWithSession(c.Request.Context(), c.Param("provider"), req.Challenge, c.ClientIP(), user, humanSessionReference(c))
		if err != nil {
			socialResponseError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
func (h *SocialAuthHandler) Complete(link bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		user, ok := socialUser(c, link)
		if !ok {
			return
		}
		var req struct {
			Code     string `json:"code" binding:"required,max=4096"`
			State    string `json:"state" binding:"required,max=64"`
			Verifier string `json:"code_verifier" binding:"required,max=128"`
		}
		if c.ShouldBindJSON(&req) != nil {
			WrapError(c, ErrInvalidInput)
			return
		}
		result, err := h.service.CompleteWithSession(c.Request.Context(), c.Param("provider"), req.Code, req.State, req.Verifier, c.ClientIP(), user, humanSessionReference(c), c.Request.UserAgent())
		if err != nil {
			socialResponseError(c, err)
			return
		}
		if link {
			c.JSON(http.StatusOK, gin.H{"linked": true})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
func (h *SocialAuthHandler) Connections(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	user, ok := socialUser(c, true)
	if !ok {
		return
	}
	result, err := h.service.Connections(c.Request.Context(), *user)
	if err != nil {
		socialResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"connected": result, "available": h.service.Providers()})
}
