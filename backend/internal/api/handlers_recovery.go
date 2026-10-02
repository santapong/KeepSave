package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

// RecoveryHandler exposes encrypted bundles and metadata-only verification.
// Recovery material is server-side; no request accepts master keys.
type RecoveryHandler struct{ vault *vault.Service }

func NewRecoveryHandler(v *vault.Service) *RecoveryHandler { return &RecoveryHandler{vault: v} }
func (h *RecoveryHandler) project(c *gin.Context) (uuid.UUID, bool) {
	if h.vault == nil {
		WrapError(c, ErrServiceUnavailable)
		return uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		WrapError(c, ErrInvalidInput)
		return uuid.Nil, false
	}
	return id, true
}
func (h *RecoveryHandler) Backup(c *gin.Context) {
	project, ok := h.project(c)
	if !ok {
		return
	}
	b, err := h.vault.Backup(c.Request.Context(), PrincipalFromContext(c), project)
	if err != nil {
		WrapError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="keepsave-encrypted-vault.json"`)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, b)
}
func (h *RecoveryHandler) Verify(c *gin.Context) {
	project, ok := h.project(c)
	if !ok {
		return
	}
	var b vault.Bundle
	if err := c.ShouldBindJSON(&b); err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	metadata, err := h.vault.VerifyBundle(c.Request.Context(), PrincipalFromContext(c), project, b)
	if err != nil {
		WrapError(c, err)
		return
	}
	c.JSON(http.StatusOK, metadata)
}
func (h *RecoveryHandler) Preview(c *gin.Context) {
	project, ok := h.project(c)
	if !ok {
		return
	}
	var b vault.Bundle
	if err := c.ShouldBindJSON(&b); err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	preview, err := h.vault.PreviewRestore(c.Request.Context(), PrincipalFromContext(c), project, b)
	if err != nil {
		WrapError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}
func (h *RecoveryHandler) RestoreSelected(c *gin.Context) {
	project, ok := h.project(c)
	if !ok {
		return
	}
	var req struct {
		Bundle  vault.Bundle             `json:"bundle"`
		Records []vault.RestoreSelection `json:"records"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	records, err := h.vault.RestoreSelected(c.Request.Context(), PrincipalFromContext(c), project, req.Bundle, req.Records)
	if err != nil {
		WrapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"records": records})
}

func (h *RecoveryHandler) List(c *gin.Context) {
	project, ok := h.project(c)
	if !ok {
		return
	}
	records, err := h.vault.ListBackups(c.Request.Context(), PrincipalFromContext(c), project)
	if err != nil {
		WrapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"backups": records})
}
