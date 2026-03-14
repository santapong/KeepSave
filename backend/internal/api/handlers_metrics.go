package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/metrics"
	"github.com/santapong/KeepSave/backend/internal/tracing"
)

// MetricsHandler handles observability endpoints.
type MetricsHandler struct {
	appMetrics *metrics.AppMetrics
	tracer     *tracing.Tracer
}

// NewMetricsHandler creates a new metrics handler.
func NewMetricsHandler(m *metrics.AppMetrics, t *tracing.Tracer) *MetricsHandler {
	return &MetricsHandler{appMetrics: m, tracer: t}
}

// Metrics serves the Prometheus metrics endpoint.
func (h *MetricsHandler) Metrics(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(h.appMetrics.Collector.Render()))
}

// Traces returns recent trace spans.
func (h *MetricsHandler) Traces(c *gin.Context) {
	spans := h.tracer.GetRecentSpans(100)
	c.JSON(http.StatusOK, gin.H{"spans": spans})
}

// AdminDashboard returns a system health overview.
func (h *MetricsHandler) AdminDashboard(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"metrics_url": "/metrics",
		"traces_url":  "/api/v1/admin/traces",
		"health_url":  "/healthz",
		"ready_url":   "/readyz",
		"status":      "operational",
	})
}
