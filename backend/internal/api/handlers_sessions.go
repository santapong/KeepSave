package api

import (
	"database/sql"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/service"
	"net/http"
)

type SessionHandler struct{ service *service.SessionService }

func NewSessionHandler(s *service.SessionService) *SessionHandler { return &SessionHandler{s} }
func humanSessionReference(c *gin.Context) *uuid.UUID {
	value, ok := c.Get("auth_claims")
	if !ok {
		return nil
	}
	claims, ok := value.(*auth.Claims)
	if !ok || claims.TokenType != "human" {
		return nil
	}
	id, err := uuid.Parse(claims.SessionID)
	if err != nil || id == uuid.Nil {
		return nil
	}
	return &id
}
func sessionPrincipal(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	user, ok := socialUser(c, true)
	session := humanSessionReference(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	if session == nil {
		WrapError(c, ErrUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}
	return *user, *session, true
}
func sessionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		WrapError(c, ErrNotFound)
	case errors.Is(err, auth.ErrSessionInvalid):
		WrapError(c, ErrUnauthorized)
	case errors.Is(err, auth.ErrSessionUnavailable):
		WrapError(c, ErrServiceUnavailable)
	default:
		WrapError(c, ErrInternal)
	}
}
func (h *SessionHandler) List(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	user, current, ok := sessionPrincipal(c)
	if !ok {
		return
	}
	rows, err := h.service.List(c.Request.Context(), user, current)
	if err != nil {
		sessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": rows})
}
func (h *SessionHandler) Logout(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	user, current, ok := sessionPrincipal(c)
	if !ok {
		return
	}
	if err := h.service.Revoke(c.Request.Context(), user, current, current, c.ClientIP()); err != nil {
		sessionError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *SessionHandler) Revoke(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	user, current, ok := sessionPrincipal(c)
	if !ok {
		return
	}
	target, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		WrapError(c, ErrNotFound)
		return
	}
	if err = h.service.Revoke(c.Request.Context(), user, current, target, c.ClientIP()); err != nil {
		sessionError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
