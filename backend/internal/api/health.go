package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
		"version": "0.5.0",
	})
}

// Readiness returns 200 if the service is ready to accept traffic (DB connected).
func (h *HealthHandler) Readiness(c *gin.Context) {
	if err := h.db.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "unavailable",
			"database": "disconnected",
			"error":    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ready",
		"database": "connected",
		"uptime":   time.Since(h.startTime).String(),
		"version":  "0.5.0",
	})
}
