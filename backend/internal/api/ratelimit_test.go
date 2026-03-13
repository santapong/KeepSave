package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterAllow(t *testing.T) {
	tests := []struct {
		name      string
		rate      int
		burst     int
		requests  int
		wantAllow int
	}{
		{"single request within burst", 10, 10, 1, 1},
		{"requests at burst limit", 10, 5, 5, 5},
		{"requests exceed burst", 10, 3, 10, 3},
		{"burst of 1", 10, 1, 3, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewRateLimiter(tt.rate, time.Second, tt.burst)
			allowed := 0
			for i := 0; i < tt.requests; i++ {
				if limiter.allow("test-client") {
					allowed++
				}
			}
			if allowed != tt.wantAllow {
				t.Errorf("allowed %d requests, want %d", allowed, tt.wantAllow)
			}
		})
	}
}

func TestRateLimiterRefill(t *testing.T) {
	limiter := NewRateLimiter(10, 50*time.Millisecond, 5)

	// Exhaust tokens
	for i := 0; i < 5; i++ {
		limiter.allow("client1")
	}

	// Should be blocked
	if limiter.allow("client1") {
		t.Error("should be rate limited after exhausting burst")
	}

	// Wait for refill
	time.Sleep(60 * time.Millisecond)

	// Should have tokens again
	if !limiter.allow("client1") {
		t.Error("should be allowed after refill interval")
	}
}

func TestRateLimiterIsolation(t *testing.T) {
	limiter := NewRateLimiter(10, time.Second, 2)

	// Exhaust client1
	limiter.allow("client1")
	limiter.allow("client1")
	if limiter.allow("client1") {
		t.Error("client1 should be rate limited")
	}

	// client2 should still have tokens
	if !limiter.allow("client2") {
		t.Error("client2 should not be rate limited")
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	limiter := NewRateLimiter(100, time.Second, 50)

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.allow("concurrent-client") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if allowed > 50 {
		t.Errorf("allowed %d requests, should not exceed burst of 50", allowed)
	}
	if allowed < 1 {
		t.Error("should have allowed at least 1 request")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewRateLimiter(10, time.Second, 2)
	r := gin.New()
	r.Use(RateLimitMiddleware(limiter))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First two requests should pass
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: got status %d, want 200", i+1, w.Code)
		}
	}

	// Third request should be rate limited
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}
