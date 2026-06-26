package api

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/version"
)

// HealthHandler provides health check and readiness endpoints.
type HealthHandler struct {
	db        *sql.DB
	startTime time.Time
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{
		db:        db,
		startTime: time.Now(),
	}
}

// Liveness returns 200 if the service is alive.
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"uptime":  time.Since(h.startTime).String(),
		"version": version.Version,
	})
}

// Readiness returns 200 if the service is ready to accept traffic (DB connected).
func (h *HealthHandler) Readiness(c *gin.Context) {
	if err := h.db.Ping(); err != nil {
		// Don't leak the raw driver/DSN error to (often unauthenticated)
		// callers; log it server-side and report only that the dependency is
		// unavailable. docs/ERROR_HANDLING_STANDARD.md.
		log.Printf("readiness: database ping failed: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "unavailable",
			"database": "disconnected",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ready",
		"database": "connected",
		"uptime":   time.Since(h.startTime).String(),
		"version":  version.Version,
	})
}
