package metrics

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Collector gathers application metrics in Prometheus exposition format.
type Collector struct {
	mu       sync.RWMutex
	counters map[string]*Counter
	gauges   map[string]*Gauge
	histos   map[string]*Histogram
}

// Counter is a monotonically increasing counter.
type Counter struct {
	name   string
	help   string
	labels map[string]*atomic.Int64
	mu     sync.RWMutex
}

// Gauge is a value that can go up or down.
type Gauge struct {
	name   string
	help   string
	labels map[string]*atomic.Int64
	mu     sync.RWMutex
}

// Histogram tracks value distributions with buckets.
type Histogram struct {
	name    string
	help    string
	buckets []float64
	entries map[string]*histEntry
	mu      sync.RWMutex
}

type histEntry struct {
	bucketCounts []atomic.Int64
	sum          atomic.Int64 // stored as microseconds
	count        atomic.Int64
}

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	return &Collector{
		counters: make(map[string]*Counter),
		gauges:   make(map[string]*Gauge),
		histos:   make(map[string]*Histogram),
	}
}

// NewCounter registers a counter metric.
func (c *Collector) NewCounter(name, help string) *Counter {
	c.mu.Lock()
	defer c.mu.Unlock()
	counter := &Counter{name: name, help: help, labels: make(map[string]*atomic.Int64)}
	c.counters[name] = counter
	return counter
}

// NewGauge registers a gauge metric.
func (c *Collector) NewGauge(name, help string) *Gauge {
	c.mu.Lock()
	defer c.mu.Unlock()
	gauge := &Gauge{name: name, help: help, labels: make(map[string]*atomic.Int64)}
	c.gauges[name] = gauge
	return gauge
}

// NewHistogram registers a histogram metric.
func (c *Collector) NewHistogram(name, help string, buckets []float64) *Histogram {
	c.mu.Lock()
	defer c.mu.Unlock()
	h := &Histogram{
		name:    name,
		help:    help,
		buckets: buckets,
		entries: make(map[string]*histEntry),
	}
	c.histos[name] = h
	return h
}

// Inc increments a counter by 1.
func (ct *Counter) Inc(labelKey string) {
	ct.mu.RLock()
	v, ok := ct.labels[labelKey]
	ct.mu.RUnlock()
	if ok {
		v.Add(1)
		return
	}
	ct.mu.Lock()
	if v, ok = ct.labels[labelKey]; ok {
		ct.mu.Unlock()
		v.Add(1)
		return
	}
	v = &atomic.Int64{}
	v.Store(1)
	ct.labels[labelKey] = v
	ct.mu.Unlock()
}

// Set sets a gauge to a value.
func (g *Gauge) Set(labelKey string, value int64) {
	g.mu.RLock()
	v, ok := g.labels[labelKey]
	g.mu.RUnlock()
	if ok {
		v.Store(value)
		return
	}
	g.mu.Lock()
	if v, ok = g.labels[labelKey]; ok {
		g.mu.Unlock()
		v.Store(value)
		return
	}
	v = &atomic.Int64{}
	v.Store(value)
	g.labels[labelKey] = v
	g.mu.Unlock()
}

// Inc increments a gauge by 1.
func (g *Gauge) Inc(labelKey string) {
	g.mu.RLock()
	v, ok := g.labels[labelKey]
	g.mu.RUnlock()
	if ok {
		v.Add(1)
		return
	}
	g.mu.Lock()
	if v, ok = g.labels[labelKey]; ok {
		g.mu.Unlock()
		v.Add(1)
		return
	}
	v = &atomic.Int64{}
	v.Store(1)
	g.labels[labelKey] = v
	g.mu.Unlock()
}

// Dec decrements a gauge by 1.
func (g *Gauge) Dec(labelKey string) {
	g.mu.RLock()
	v, ok := g.labels[labelKey]
	g.mu.RUnlock()
	if ok {
		v.Add(-1)
		return
	}
	g.mu.Lock()
	if v, ok = g.labels[labelKey]; ok {
		g.mu.Unlock()
		v.Add(-1)
		return
	}
	v = &atomic.Int64{}
	v.Store(-1)
	g.labels[labelKey] = v
	g.mu.Unlock()
}

// Observe records a value in the histogram.
func (h *Histogram) Observe(labelKey string, value float64) {
	h.mu.RLock()
	e, ok := h.entries[labelKey]
	h.mu.RUnlock()

	if !ok {
		h.mu.Lock()
		if e, ok = h.entries[labelKey]; !ok {
			e = &histEntry{
				bucketCounts: make([]atomic.Int64, len(h.buckets)+1),
			}
			h.entries[labelKey] = e
		}
		h.mu.Unlock()
	}

	e.count.Add(1)
	e.sum.Add(int64(value * 1e6))
	for i, b := range h.buckets {
		if value <= b {
			e.bucketCounts[i].Add(1)
		}
	}
	e.bucketCounts[len(h.buckets)].Add(1) // +Inf
}

// ObserveDuration records a duration value.
func (h *Histogram) ObserveDuration(labelKey string, d time.Duration) {
	h.Observe(labelKey, d.Seconds())
}

// Render outputs all metrics in Prometheus exposition format.
func (c *Collector) Render() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var out string

	// Counters
	for _, ct := range c.counters {
		out += fmt.Sprintf("# HELP %s %s\n# TYPE %s counter\n", ct.name, ct.help, ct.name)
		ct.mu.RLock()
		keys := sortedKeys(ct.labels)
		for _, k := range keys {
			v := ct.labels[k]
			if k == "" {
				out += fmt.Sprintf("%s %d\n", ct.name, v.Load())
			} else {
				out += fmt.Sprintf("%s{label=\"%s\"} %d\n", ct.name, k, v.Load())
			}
		}
		ct.mu.RUnlock()
	}

	// Gauges
	for _, g := range c.gauges {
		out += fmt.Sprintf("# HELP %s %s\n# TYPE %s gauge\n", g.name, g.help, g.name)
		g.mu.RLock()
		keys := sortedKeys(g.labels)
		for _, k := range keys {
			v := g.labels[k]
			if k == "" {
				out += fmt.Sprintf("%s %d\n", g.name, v.Load())
			} else {
				out += fmt.Sprintf("%s{label=\"%s\"} %d\n", g.name, k, v.Load())
			}
		}
		g.mu.RUnlock()
	}

	// Histograms
	for _, h := range c.histos {
		out += fmt.Sprintf("# HELP %s %s\n# TYPE %s histogram\n", h.name, h.help, h.name)
		h.mu.RLock()
		for labelKey, e := range h.entries {
			prefix := h.name
			if labelKey != "" {
				prefix = fmt.Sprintf("%s{label=\"%s\"}", h.name, labelKey)
			}
			var cumulative int64
			for i, b := range h.buckets {
				cumulative += e.bucketCounts[i].Load()
				out += fmt.Sprintf("%s_bucket{le=\"%.3f\"} %d\n", prefix, b, cumulative)
			}
			cumulative += e.bucketCounts[len(h.buckets)].Load()
			out += fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d\n", prefix, cumulative)
			out += fmt.Sprintf("%s_sum %.6f\n", prefix, float64(e.sum.Load())/1e6)
			out += fmt.Sprintf("%s_count %d\n", prefix, e.count.Load())
		}
		h.mu.RUnlock()
	}

	return out
}

func sortedKeys(m map[string]*atomic.Int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// DefaultBuckets are latency buckets for HTTP request duration (in seconds).
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

// AppMetrics holds all application-level metrics.
type AppMetrics struct {
	Collector         *Collector
	RequestsTotal     *Counter
	RequestDuration   *Histogram
	RequestsInFlight  *Gauge
	ErrorsTotal       *Counter
	SecretsEncrypted  *Counter
	SecretsDecrypted  *Counter
	PromotionsTotal   *Counter
	KeyRotationsTotal *Counter
	AuthAttemptsTotal *Counter
	AuthFailuresTotal *Counter
	ActiveAPIKeys     *Gauge
	WebhookDeliveries *Counter
	RateLimitHits     *Counter
}

// NewAppMetrics creates and registers all application metrics.
func NewAppMetrics() *AppMetrics {
	c := NewCollector()
	return &AppMetrics{
		Collector:         c,
		RequestsTotal:     c.NewCounter("keepsave_http_requests_total", "Total HTTP requests"),
		RequestDuration:   c.NewHistogram("keepsave_http_request_duration_seconds", "HTTP request duration in seconds", DefaultBuckets),
		RequestsInFlight:  c.NewGauge("keepsave_http_requests_in_flight", "Current in-flight HTTP requests"),
		ErrorsTotal:       c.NewCounter("keepsave_http_errors_total", "Total HTTP error responses"),
		SecretsEncrypted:  c.NewCounter("keepsave_secrets_encrypted_total", "Total secrets encrypted"),
		SecretsDecrypted:  c.NewCounter("keepsave_secrets_decrypted_total", "Total secrets decrypted"),
		PromotionsTotal:   c.NewCounter("keepsave_promotions_total", "Total promotion operations"),
		KeyRotationsTotal: c.NewCounter("keepsave_key_rotations_total", "Total key rotation operations"),
		AuthAttemptsTotal: c.NewCounter("keepsave_auth_attempts_total", "Total authentication attempts"),
		AuthFailuresTotal: c.NewCounter("keepsave_auth_failures_total", "Total authentication failures"),
		ActiveAPIKeys:     c.NewGauge("keepsave_active_api_keys", "Current active API keys"),
		WebhookDeliveries: c.NewCounter("keepsave_webhook_deliveries_total", "Total webhook deliveries"),
		RateLimitHits:     c.NewCounter("keepsave_rate_limit_hits_total", "Total rate limit rejections"),
	}
}
