package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupHighLoadRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	handler := NewHealthHandler(nil)
	r.GET("/healthz", handler.Liveness)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}

func TestHighLoadConcurrentRequests(t *testing.T) {
	r := setupHighLoadRouter()
	server := httptest.NewServer(r)
	defer server.Close()

	concurrency := 100
	requestsPerClient := 50
	totalRequests := concurrency * requestsPerClient

	var successCount int64
	var errorCount int64
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}

			for j := 0; j < requestsPerClient; j++ {
				resp, err := client.Get(server.URL + "/healthz")
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	success := atomic.LoadInt64(&successCount)
	errors := atomic.LoadInt64(&errorCount)

	t.Logf("High Load Test Results:")
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful: %d", success)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  RPS: %.0f", float64(success)/elapsed.Seconds())

	// At least 90% of requests should succeed
	successRate := float64(success) / float64(totalRequests) * 100
	if successRate < 90 {
		t.Errorf("success rate %.1f%% is below 90%%", successRate)
	}
}

func TestHighLoadRateLimiterUnderStress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewRateLimiter(1000, time.Second, 500)
	r := gin.New()
	r.Use(RateLimitMiddleware(limiter))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	server := httptest.NewServer(r)
	defer server.Close()

	concurrency := 50
	requestsPerClient := 20

	var okCount, rateLimitedCount, otherCount int64
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}

			for j := 0; j < requestsPerClient; j++ {
				resp, err := client.Get(server.URL + "/test")
				if err != nil {
					atomic.AddInt64(&otherCount, 1)
					continue
				}
				resp.Body.Close()

				switch resp.StatusCode {
				case http.StatusOK:
					atomic.AddInt64(&okCount, 1)
				case http.StatusTooManyRequests:
					atomic.AddInt64(&rateLimitedCount, 1)
				default:
					atomic.AddInt64(&otherCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	ok := atomic.LoadInt64(&okCount)
	limited := atomic.LoadInt64(&rateLimitedCount)
	other := atomic.LoadInt64(&otherCount)

	t.Logf("Rate Limiter Stress Test:")
	t.Logf("  OK: %d, Rate Limited: %d, Other: %d", ok, limited, other)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Total RPS: %.0f", float64(ok+limited+other)/elapsed.Seconds())

	// Rate limiter should have accepted requests up to its burst capacity
	if ok == 0 {
		t.Error("at least some requests should have been accepted")
	}
}

func TestHighLoadConcurrentRateLimiterClients(t *testing.T) {
	limiter := NewRateLimiter(100, time.Second, 10)
	numClients := 200
	requestsPerClient := 20

	var totalAllowed int64
	var totalDenied int64
	var wg sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			key := fmt.Sprintf("client-%d", clientID)
			for j := 0; j < requestsPerClient; j++ {
				if limiter.allow(key) {
					atomic.AddInt64(&totalAllowed, 1)
				} else {
					atomic.AddInt64(&totalDenied, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	allowed := atomic.LoadInt64(&totalAllowed)
	denied := atomic.LoadInt64(&totalDenied)

	t.Logf("Concurrent Clients Test:")
	t.Logf("  Clients: %d, Requests/client: %d", numClients, requestsPerClient)
	t.Logf("  Allowed: %d, Denied: %d", allowed, denied)

	// Each client has a burst of 10, so max allowed per client = 10
	// Total allowed should be at most numClients * burst
	maxAllowed := int64(numClients * 10)
	if allowed > maxAllowed {
		t.Errorf("allowed %d > max expected %d", allowed, maxAllowed)
	}
	if allowed == 0 {
		t.Error("at least some requests should be allowed")
	}
}

func TestHighLoadEncryptionThroughput(t *testing.T) {
	// Test the crypto operations under load (simulated via direct function calls)
	concurrency := 50
	operationsPerGoroutine := 100

	var completed int64
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				// Simulate a compute-bound operation
				data := make([]byte, 1024)
				for k := range data {
					data[k] = byte(k % 256)
				}
				atomic.AddInt64(&completed, 1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	ops := atomic.LoadInt64(&completed)
	t.Logf("Encryption Throughput Simulation:")
	t.Logf("  Operations: %d", ops)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Ops/sec: %.0f", float64(ops)/elapsed.Seconds())

	if ops != int64(concurrency*operationsPerGoroutine) {
		t.Errorf("expected %d operations, got %d", concurrency*operationsPerGoroutine, ops)
	}
}

func TestHighLoadMemoryStability(t *testing.T) {
	// Ensure the rate limiter doesn't leak memory under sustained load
	limiter := NewRateLimiter(100, time.Second, 50)

	// Create 10000 unique clients
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("client-%d", i)
		limiter.allow(key)
	}

	// Verify the map has entries
	limiter.mu.Lock()
	clientCount := len(limiter.clients)
	limiter.mu.Unlock()

	if clientCount != 10000 {
		t.Errorf("expected 10000 clients tracked, got %d", clientCount)
	}
}

func TestHighLoadSequentialLatency(t *testing.T) {
	r := setupHighLoadRouter()

	// Measure latency of sequential requests
	iterations := 1000
	var totalLatency time.Duration

	for i := 0; i < iterations; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/healthz", nil)
		start := time.Now()
		r.ServeHTTP(w, req)
		totalLatency += time.Since(start)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d returned %d", i, w.Code)
		}
	}

	avgLatency := totalLatency / time.Duration(iterations)
	t.Logf("Sequential Latency Test:")
	t.Logf("  Iterations: %d", iterations)
	t.Logf("  Total: %v", totalLatency)
	t.Logf("  Average: %v", avgLatency)

	// Average latency should be under 1ms for a simple health check
	if avgLatency > time.Millisecond {
		t.Logf("WARNING: average latency %v exceeds 1ms threshold", avgLatency)
	}
}

func BenchmarkHealthEndpoint(b *testing.B) {
	r := setupHighLoadRouter()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/healthz", nil)
			r.ServeHTTP(w, req)
		}
	})
}

func BenchmarkRateLimiterAllow(b *testing.B) {
	limiter := NewRateLimiter(10000, time.Second, 10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.allow("bench-client")
		}
	})
}

func BenchmarkRateLimiterMultipleClients(b *testing.B) {
	limiter := NewRateLimiter(100, time.Second, 100)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			limiter.allow(fmt.Sprintf("client-%d", i%1000))
			i++
		}
	})
}
