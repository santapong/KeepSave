package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a token bucket rate limiter per client IP.
type RateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*bucket
	rate     int           // tokens per interval
	interval time.Duration // refill interval
	burst    int           // max tokens (burst capacity)
	cleanup  time.Duration // how often to clean stale entries
}

type bucket struct {
	tokens    int
	lastFill  time.Time
	lastAccess time.Time
}

// NewRateLimiter creates a rate limiter.
// rate: requests allowed per interval, burst: max burst capacity.
func NewRateLimiter(rate int, interval time.Duration, burst int) *RateLimiter {
	rl := &RateLimiter{
		clients:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
		burst:    burst,
		cleanup:  5 * time.Minute,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.clients {
			if now.Sub(b.lastAccess) > rl.cleanup {
				delete(rl.clients, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.clients[key]
	if !exists {
		rl.clients[key] = &bucket{
			tokens:    rl.burst - 1,
			lastFill:  now,
			lastAccess: now,
		}
		return true
	}

	b.lastAccess = now

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastFill)
	refill := int(elapsed / rl.interval) * rl.rate
	if refill > 0 {
		b.tokens += refill
		if b.tokens > rl.burst {
			b.tokens = rl.burst
		}
		b.lastFill = now
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

// RateLimitMiddleware returns a Gin middleware that rate limits by client IP.
func RateLimitMiddleware(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		if !limiter.allow(clientIP) {
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, please retry later",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
