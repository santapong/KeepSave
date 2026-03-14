package tracing

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Span represents a single unit of work within a trace.
type Span struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Operation  string            `json:"operation"`
	Service    string            `json:"service"`
	StartTime  time.Time         `json:"start_time"`
	Duration   time.Duration     `json:"duration"`
	Status     string            `json:"status"` // ok, error
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Tracer provides distributed tracing capabilities.
type Tracer struct {
	serviceName string
	mu          sync.RWMutex
	spans       []Span
	maxSpans    int
}

// NewTracer creates a new tracer for the given service.
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		serviceName: serviceName,
		spans:       make([]Span, 0, 1000),
		maxSpans:    10000,
	}
}

// StartSpan begins a new span.
func (t *Tracer) StartSpan(traceID, parentID, operation string) *SpanContext {
	spanID := uuid.New().String()[:16]
	if traceID == "" {
		traceID = uuid.New().String()
	}
	return &SpanContext{
		tracer:    t,
		traceID:   traceID,
		spanID:    spanID,
		parentID:  parentID,
		operation: operation,
		startTime: time.Now(),
		attrs:     make(map[string]string),
	}
}

// GetRecentSpans returns the most recent spans.
func (t *Tracer) GetRecentSpans(limit int) []Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.spans) {
		limit = len(t.spans)
	}

	start := len(t.spans) - limit
	if start < 0 {
		start = 0
	}

	result := make([]Span, limit)
	copy(result, t.spans[start:])
	return result
}

func (t *Tracer) record(s Span) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.spans = append(t.spans, s)
	if len(t.spans) > t.maxSpans {
		t.spans = t.spans[len(t.spans)-t.maxSpans:]
	}
}

// SpanContext is an active span that can be finished.
type SpanContext struct {
	tracer    *Tracer
	traceID   string
	spanID    string
	parentID  string
	operation string
	startTime time.Time
	attrs     map[string]string
	status    string
}

// SetAttribute adds an attribute to the span.
func (sc *SpanContext) SetAttribute(key, value string) {
	sc.attrs[key] = value
}

// SetStatus sets the span status.
func (sc *SpanContext) SetStatus(status string) {
	sc.status = status
}

// End finishes the span and records it.
func (sc *SpanContext) End() {
	if sc.status == "" {
		sc.status = "ok"
	}
	span := Span{
		TraceID:    sc.traceID,
		SpanID:     sc.spanID,
		ParentID:   sc.parentID,
		Operation:  sc.operation,
		Service:    sc.tracer.serviceName,
		StartTime:  sc.startTime,
		Duration:   time.Since(sc.startTime),
		Status:     sc.status,
		Attributes: sc.attrs,
	}
	sc.tracer.record(span)
}

// TraceID returns the trace ID for propagation.
func (sc *SpanContext) TraceID() string {
	return sc.traceID
}

// SpanID returns the span ID for propagation.
func (sc *SpanContext) SpanID() string {
	return sc.spanID
}

// GinMiddleware returns a Gin middleware that creates traces for each request.
func GinMiddleware(tracer *Tracer) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		parentID := c.GetHeader("X-Span-ID")

		operation := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
		if c.FullPath() == "" {
			operation = fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
		}

		span := tracer.StartSpan(traceID, parentID, operation)
		span.SetAttribute("http.method", c.Request.Method)
		span.SetAttribute("http.url", c.Request.URL.Path)
		span.SetAttribute("http.client_ip", c.ClientIP())

		c.Set("trace_id", span.TraceID())
		c.Set("span_id", span.SpanID())
		c.Header("X-Trace-ID", span.TraceID())
		c.Header("X-Span-ID", span.SpanID())

		c.Next()

		status := c.Writer.Status()
		span.SetAttribute("http.status_code", fmt.Sprintf("%d", status))
		if status >= 400 {
			span.SetStatus("error")
		}
		span.End()
	}
}
