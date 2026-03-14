package metrics

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCounterInc(t *testing.T) {
	c := NewCollector()
	counter := c.NewCounter("test_counter", "A test counter")

	counter.Inc("method_GET")
	counter.Inc("method_GET")
	counter.Inc("method_POST")

	output := c.Render()
	if !strings.Contains(output, `test_counter{label="method_GET"} 2`) {
		t.Errorf("expected GET count 2, got:\n%s", output)
	}
	if !strings.Contains(output, `test_counter{label="method_POST"} 1`) {
		t.Errorf("expected POST count 1, got:\n%s", output)
	}
}

func TestGaugeSetIncDec(t *testing.T) {
	c := NewCollector()
	gauge := c.NewGauge("test_gauge", "A test gauge")

	gauge.Set("conn", 10)
	gauge.Inc("conn")
	gauge.Dec("conn")

	output := c.Render()
	if !strings.Contains(output, `test_gauge{label="conn"} 10`) {
		t.Errorf("expected gauge 10, got:\n%s", output)
	}
}

func TestHistogramObserve(t *testing.T) {
	c := NewCollector()
	hist := c.NewHistogram("test_hist", "A test histogram", []float64{0.1, 0.5, 1.0})

	hist.Observe("api", 0.05)
	hist.Observe("api", 0.3)
	hist.Observe("api", 0.8)

	output := c.Render()
	if !strings.Contains(output, "test_hist_count 3") {
		t.Errorf("expected count 3, got:\n%s", output)
	}
	if !strings.Contains(output, `le="0.100"`) {
		t.Errorf("expected bucket le=0.100, got:\n%s", output)
	}
}

func TestHistogramObserveDuration(t *testing.T) {
	c := NewCollector()
	hist := c.NewHistogram("duration", "Request duration", DefaultBuckets)

	hist.ObserveDuration("test", 50*time.Millisecond)
	hist.ObserveDuration("test", 200*time.Millisecond)

	output := c.Render()
	if !strings.Contains(output, "duration_count 2") {
		t.Errorf("expected count 2, got:\n%s", output)
	}
}

func TestConcurrentCounterInc(t *testing.T) {
	c := NewCollector()
	counter := c.NewCounter("concurrent", "Concurrent counter")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				counter.Inc("total")
			}
		}()
	}
	wg.Wait()

	output := c.Render()
	if !strings.Contains(output, `concurrent{label="total"} 10000`) {
		t.Errorf("expected 10000, got:\n%s", output)
	}
}

func TestAppMetricsInit(t *testing.T) {
	m := NewAppMetrics()
	if m.Collector == nil {
		t.Fatal("collector should not be nil")
	}
	if m.RequestsTotal == nil {
		t.Fatal("RequestsTotal should not be nil")
	}
	if m.RequestDuration == nil {
		t.Fatal("RequestDuration should not be nil")
	}

	m.RequestsTotal.Inc("GET_/api/v1/projects")
	m.AuthAttemptsTotal.Inc("login")
	m.SecretsEncrypted.Inc("")

	output := m.Collector.Render()
	if !strings.Contains(output, "keepsave_http_requests_total") {
		t.Errorf("expected keepsave_http_requests_total in output")
	}
	if !strings.Contains(output, "keepsave_auth_attempts_total") {
		t.Errorf("expected keepsave_auth_attempts_total in output")
	}
}

func TestRenderFormat(t *testing.T) {
	c := NewCollector()
	c.NewCounter("my_counter", "Help text")

	output := c.Render()
	if !strings.Contains(output, "# HELP my_counter Help text") {
		t.Errorf("expected HELP line, got:\n%s", output)
	}
	if !strings.Contains(output, "# TYPE my_counter counter") {
		t.Errorf("expected TYPE line, got:\n%s", output)
	}
}

func TestEmptyLabelKey(t *testing.T) {
	c := NewCollector()
	counter := c.NewCounter("no_label", "No label counter")

	counter.Inc("")
	counter.Inc("")

	output := c.Render()
	if !strings.Contains(output, "no_label 2") {
		t.Errorf("expected 'no_label 2', got:\n%s", output)
	}
}
