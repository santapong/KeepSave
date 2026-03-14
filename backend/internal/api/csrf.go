package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// CSRFMiddleware provides Cross-Site Request Forgery protection.
// It exempts API key-authenticated requests (machine-to-machine).
func CSRFMiddleware() gin.HandlerFunc {
	store := &csrfStore{tokens: make(map[string]bool)}

	return func(c *gin.Context) {
		// Skip CSRF for API key-authenticated requests
		if c.GetHeader("X-API-Key") != "" {
			c.Next()
			return
		}

		// Skip safe methods
		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			// Issue a CSRF token for subsequent mutation requests
			token := store.generate()
			c.Header("X-CSRF-Token", token)
			c.Next()
			return
		}

		// Validate CSRF token on mutation requests
		token := c.GetHeader("X-CSRF-Token")
		if token == "" {
			token = c.PostForm("_csrf_token")
		}

		if token == "" {
			RespondError(c, http.StatusForbidden, "CSRF token required")
			c.Abort()
			return
		}

		if !store.validate(token) {
			RespondError(c, http.StatusForbidden, "invalid CSRF token")
			c.Abort()
			return
		}

		c.Next()
	}
}

type csrfStore struct {
	mu     sync.RWMutex
	tokens map[string]bool
}

func (s *csrfStore) generate() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	token := hex.EncodeToString(b)

	s.mu.Lock()
	s.tokens[token] = true
	// Keep store bounded
	if len(s.tokens) > 100000 {
		count := 0
		for k := range s.tokens {
			delete(s.tokens, k)
			count++
			if count >= 50000 {
				break
			}
		}
	}
	s.mu.Unlock()

	return token
}

func (s *csrfStore) validate(token string) bool {
	s.mu.RLock()
	_, exists := s.tokens[token]
	s.mu.RUnlock()

	if !exists {
		return false
	}

	// Single-use tokens
	s.mu.Lock()
	delete(s.tokens, token)
	s.mu.Unlock()

	return true
}

// ConstantTimeCompare compares two strings in constant time.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
