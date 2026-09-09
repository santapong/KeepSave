package tracing

import (
	"fmt"
	"sync"
	"testing"
)

func TestRetentionWrapKeepsRecentSpansInOrder(t *testing.T) {
	tracer := NewTracer("test")
	tracer.maxSpans = 3
	for i := 0; i < 11; i++ {
		tracer.record(Span{Operation: fmt.Sprint(i)})
	}
	for _, limit := range []int{0, 1, 2, 3, 100} {
		spans := tracer.GetRecentSpans(limit)
		want := limit
		if want <= 0 || want > 3 {
			want = 3
		}
		if len(spans) != want {
			t.Fatalf("limit %d: got %d spans", limit, len(spans))
		}
		for i, span := range spans {
			if span.Operation != fmt.Sprint(11-want+i) {
				t.Fatal("recent spans are out of order after wrap")
			}
		}
	}
}

func TestConcurrentRetention(t *testing.T) {
	tracer := NewTracer("test")
	tracer.maxSpans = 100
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 1000; j++ {
				tracer.record(Span{Operation: "read"})
				tracer.GetRecentSpans(2)
			}
		}()
	}
	workers.Wait()
	if len(tracer.GetRecentSpans(0)) != 100 {
		t.Fatal("retention bound changed")
	}
}

// Compare retention only, with an identical warm full buffer and Span.
// Excludes request, UUID, and attribute allocation costs in both variants.
func BenchmarkTraceRetention(b *testing.B) {
	for _, legacy := range []bool{true, false} {
		name := "ring"
		if legacy {
			name = "baseline_append"
		}
		b.Run(name, func(b *testing.B) {
			tracer := NewTracer("bench")
			span := Span{Operation: "GET /api/v1/projects/:id/secrets"}
			for i := 0; i < tracer.maxSpans; i++ {
				tracer.record(span)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if legacy {
					tracer.mu.Lock()
					tracer.spans = append(tracer.spans, span)
					if len(tracer.spans) > tracer.maxSpans {
						tracer.spans = tracer.spans[len(tracer.spans)-tracer.maxSpans:]
					}
					tracer.mu.Unlock()
				} else {
					tracer.record(span)
				}
			}
		})
	}
}
