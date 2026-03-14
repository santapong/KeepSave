package tracing

import (
	"testing"
	"time"
)

func TestStartAndEndSpan(t *testing.T) {
	tracer := NewTracer("keepsave-api")

	span := tracer.StartSpan("", "", "GET /api/v1/projects")
	span.SetAttribute("http.method", "GET")
	span.SetAttribute("http.status_code", "200")
	time.Sleep(time.Millisecond)
	span.End()

	spans := tracer.GetRecentSpans(10)
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Operation != "GET /api/v1/projects" {
		t.Errorf("unexpected operation: %s", spans[0].Operation)
	}
	if spans[0].TraceID == "" {
		t.Error("trace ID should not be empty")
	}
	if spans[0].SpanID == "" {
		t.Error("span ID should not be empty")
	}
	if spans[0].Status != "ok" {
		t.Errorf("expected status ok, got %s", spans[0].Status)
	}
	if spans[0].Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestSpanPropagation(t *testing.T) {
	tracer := NewTracer("keepsave-api")

	parent := tracer.StartSpan("trace-123", "", "parent-op")
	child := tracer.StartSpan(parent.TraceID(), parent.SpanID(), "child-op")
	child.End()
	parent.End()

	spans := tracer.GetRecentSpans(10)
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	if spans[0].TraceID != "trace-123" {
		t.Errorf("expected trace-123, got %s", spans[0].TraceID)
	}
	if spans[0].ParentID != parent.SpanID() {
		t.Errorf("child parent ID should be parent span ID")
	}
}

func TestSpanErrorStatus(t *testing.T) {
	tracer := NewTracer("test-service")

	span := tracer.StartSpan("", "", "failing-op")
	span.SetStatus("error")
	span.End()

	spans := tracer.GetRecentSpans(1)
	if spans[0].Status != "error" {
		t.Errorf("expected error status, got %s", spans[0].Status)
	}
}

func TestGetRecentSpansLimit(t *testing.T) {
	tracer := NewTracer("test")

	for i := 0; i < 20; i++ {
		span := tracer.StartSpan("", "", "op")
		span.End()
	}

	spans := tracer.GetRecentSpans(5)
	if len(spans) != 5 {
		t.Fatalf("expected 5 spans, got %d", len(spans))
	}
}

func TestMaxSpansEviction(t *testing.T) {
	tracer := NewTracer("test")
	tracer.maxSpans = 100

	for i := 0; i < 150; i++ {
		span := tracer.StartSpan("", "", "op")
		span.End()
	}

	spans := tracer.GetRecentSpans(0)
	if len(spans) != 100 {
		t.Fatalf("expected 100 spans after eviction, got %d", len(spans))
	}
}

func TestSpanAttributes(t *testing.T) {
	tracer := NewTracer("test")

	span := tracer.StartSpan("", "", "test-op")
	span.SetAttribute("key1", "value1")
	span.SetAttribute("key2", "value2")
	span.End()

	spans := tracer.GetRecentSpans(1)
	if spans[0].Attributes["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %s", spans[0].Attributes["key1"])
	}
	if spans[0].Attributes["key2"] != "value2" {
		t.Errorf("expected key2=value2, got %s", spans[0].Attributes["key2"])
	}
}
