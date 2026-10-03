package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auditview"
	"github.com/santapong/KeepSave/backend/internal/diagnostics"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

type TeamVaultHandler struct {
	vault   *vault.Service
	audit   *auditview.Service
	doctor  *diagnostics.Service
	enabled bool
}

func NewTeamVaultHandler(v *vault.Service, a *auditview.Service, d *diagnostics.Service, enabled bool) *TeamVaultHandler {
	return &TeamVaultHandler{v, a, d, enabled}
}
func (h *TeamVaultHandler) available(c *gin.Context) bool {
	c.Header("Cache-Control", "no-store")
	if h == nil || !h.enabled || h.vault == nil || h.audit == nil {
		WrapError(c, ErrServiceUnavailable)
		return false
	}
	return true
}
func (h *TeamVaultHandler) Doctor(c *gin.Context) {
	if h == nil || h.doctor == nil {
		WrapError(c, ErrServiceUnavailable)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, h.doctor.Inspect(c.Request.Context()))
}
func teamIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	p, e := uuid.Parse(c.Param("id"))
	s, err := uuid.Parse(c.Param("secretId"))
	if e != nil || err != nil || p == uuid.Nil || s == uuid.Nil {
		WrapError(c, ErrInvalidInput)
		return p, s, false
	}
	return p, s, true
}
func (h *TeamVaultHandler) Lifecycle(c *gin.Context) {
	if !h.available(c) {
		return
	}
	p, s, ok := teamIDs(c)
	if !ok {
		return
	}
	r, e := h.vault.Lifecycle(c.Request.Context(), PrincipalFromContext(c), p, s)
	if e != nil {
		WrapError(c, e)
		return
	}
	c.JSON(200, r)
}
func (h *TeamVaultHandler) UpdateLifecycle(c *gin.Context) {
	if !h.available(c) {
		return
	}
	p, s, ok := teamIDs(c)
	if !ok {
		return
	}
	var input vault.LifecycleChange
	if c.ShouldBindJSON(&input) != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	r, e := h.vault.UpdateLifecycle(c.Request.Context(), PrincipalFromContext(c), p, s, input)
	if e != nil {
		WrapError(c, e)
		return
	}
	c.JSON(200, r)
}
func (h *TeamVaultHandler) Notifications(c *gin.Context) {
	if !h.available(c) {
		return
	}
	r, e := h.vault.Notifications(c.Request.Context(), PrincipalFromContext(c))
	if e != nil {
		WrapError(c, e)
		return
	}
	c.JSON(200, gin.H{"notifications": r})
}
func auditFilter(c *gin.Context) (auditview.Filter, error) {
	now := time.Now().UTC()
	f := auditview.Filter{Action: c.Query("action"), Environment: c.Query("environment"), Cursor: c.Query("cursor"), From: now.Add(-31 * 24 * time.Hour), To: now, Limit: 100}
	var err error
	if raw := c.Query("from"); raw != "" {
		f.From, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return f, err
		}
	}
	if raw := c.Query("to"); raw != "" {
		f.To, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return f, err
		}
	}
	if raw := c.Query("limit"); raw != "" {
		f.Limit, err = strconv.Atoi(raw)
		if err != nil {
			return f, err
		}
	}
	return f, nil
}
func auditError(c *gin.Context, e error) {
	if errors.Is(e, auditview.ErrInvalid) {
		WrapError(c, ErrInvalidInput)
	} else if errors.Is(e, auditview.ErrDenied) {
		WrapError(c, ErrForbidden)
	} else {
		WrapError(c, ErrServiceUnavailable)
	}
}
func (h *TeamVaultHandler) AuditSearch(c *gin.Context) {
	if !h.available(c) {
		return
	}
	p, e := uuid.Parse(c.Param("id"))
	f, err := auditFilter(c)
	if e != nil || err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	r, e := h.audit.Search(c.Request.Context(), PrincipalFromContext(c), p, f)
	if e != nil {
		auditError(c, e)
		return
	}
	c.JSON(200, r)
}
func (h *TeamVaultHandler) AuditExport(c *gin.Context) {
	if !h.available(c) {
		return
	}
	p, e := uuid.Parse(c.Param("id"))
	if e != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	var input struct {
		From        time.Time `json:"from"`
		To          time.Time `json:"to"`
		Action      string    `json:"action"`
		Environment string    `json:"environment"`
	}
	if c.ShouldBindJSON(&input) != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	r, e := h.audit.CreateExport(c.Request.Context(), PrincipalFromContext(c), p, auditview.Filter{From: input.From, To: input.To, Action: input.Action, Environment: input.Environment})
	if e != nil {
		auditError(c, e)
		return
	}
	c.JSON(http.StatusCreated, r)
}
func (h *TeamVaultHandler) AuditDownload(c *gin.Context) {
	if !h.available(c) {
		return
	}
	p, e := uuid.Parse(c.Param("id"))
	id, err := uuid.Parse(c.Param("exportId"))
	if e != nil || err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	r, e := h.audit.Download(c.Request.Context(), PrincipalFromContext(c), p, id)
	if e != nil {
		auditError(c, e)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="keepsave-audit.json"`)
	c.Data(200, "application/json", r)
}

func (h *TeamVaultHandler) AuditExportStatus(c *gin.Context) {
	if !h.available(c) {
		return
	}
	project, e := uuid.Parse(c.Param("id"))
	id, err := uuid.Parse(c.Param("exportId"))
	if e != nil || err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	out, e := h.audit.Status(c.Request.Context(), PrincipalFromContext(c), project, id)
	if e != nil {
		auditError(c, e)
		return
	}
	c.JSON(200, out)
}
func (h *TeamVaultHandler) LifecycleList(c *gin.Context) {
	if !h.available(c) {
		return
	}
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	rows, e := h.vault.ListLifecycle(c.Request.Context(), PrincipalFromContext(c), id)
	if e != nil {
		WrapError(c, e)
		return
	}
	c.JSON(200, gin.H{"records": rows})
}
