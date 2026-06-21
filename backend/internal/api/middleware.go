package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// RequirePlatformAdmin restricts a route group to platform administrators —
// users whose JWT email is in the configured allowlist
// (KEEPSAVE_PLATFORM_ADMIN_EMAILS). It MUST be mounted after JWTAuthMiddleware
// (which sets the "email" claim). It fails closed: an empty allowlist denies
// everyone, so /admin is never world-readable by default (DB-06).
func RequirePlatformAdmin(adminEmails []string) gin.HandlerFunc {
	allow := make(map[string]struct{}, len(adminEmails))
	for _, e := range adminEmails {
		if v := strings.ToLower(strings.TrimSpace(e)); v != "" {
			allow[v] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		email, _ := c.Get("email")
		es, _ := email.(string)
		if es == "" {
			WrapError(c, ErrUnauthorized)
			c.Abort()
			return
		}
		if _, ok := allow[strings.ToLower(es)]; !ok {
			WrapError(c, ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// TrustedProxyMiddleware extracts the real client IP from reverse proxy headers
// (X-Forwarded-For, X-Real-IP) and sets X-Forwarded-Proto awareness.
// This allows KeepSave to work correctly behind nginx, Traefik, Kong, etc.
func TrustedProxyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prefer X-Real-IP (set by nginx), then X-Forwarded-For (first IP in chain)
		if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
			c.Set("client_ip", realIP)
		} else if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
			// X-Forwarded-For may contain comma-separated list; first is the client
			parts := strings.SplitN(forwarded, ",", 2)
			c.Set("client_ip", strings.TrimSpace(parts[0]))
		} else {
			c.Set("client_ip", c.ClientIP())
		}

		// Set scheme awareness for redirect URLs (OAuth flows, etc.)
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			c.Set("scheme", proto)
		} else {
			if c.Request.TLS != nil {
				c.Set("scheme", "https")
			} else {
				c.Set("scheme", "http")
			}
		}

		c.Next()
	}
}

// CORSMiddleware reflects an Origin header that matches the configured
// CORS_ORIGINS allow-list. Three formats are supported:
//   - "*" (legacy / dev): mirrors the request Origin. NOT permitted in
//     production - rejected at config-load time.
//   - exact comma-separated list: "https://app.example.com,https://x.io"
//   - glob patterns with a single "*" in the host: useful for Vercel
//     preview subdomains, e.g. "https://keepsave-uat-*-yourteam.vercel.app"
//
// Bearer-token only - Access-Control-Allow-Credentials stays false; do
// not change without a Type-1 ADR (cookies + CORS open up CSRF surface).
func CORSMiddleware(allowedOrigins string) gin.HandlerFunc {
	patterns := compileOriginPatterns(allowedOrigins)
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := matchOrigin(origin, allowedOrigins, patterns)
		if allowed != "" {
			c.Header("Access-Control-Allow-Origin", allowed)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// matchOrigin returns the Access-Control-Allow-Origin value to echo for
// this request, or "" if the origin is not allowed.
//   - cors=="*" mirrors the Origin (legacy dev mode)
//   - exact-match against the comma-list
//   - glob match against any pattern containing "*"
func matchOrigin(origin, cors string, patterns []*originPattern) string {
	if cors == "*" {
		if origin == "" {
			return "*"
		}
		return origin
	}
	if origin == "" {
		return ""
	}
	for _, raw := range strings.Split(cors, ",") {
		if strings.TrimSpace(raw) == origin {
			return origin
		}
	}
	for _, p := range patterns {
		if p.matches(origin) {
			return origin
		}
	}
	return ""
}

type originPattern struct {
	prefix string
	suffix string
}

func (p *originPattern) matches(origin string) bool {
	if len(origin) < len(p.prefix)+len(p.suffix) {
		return false
	}
	return strings.HasPrefix(origin, p.prefix) && strings.HasSuffix(origin, p.suffix)
}

// compileOriginPatterns returns a slice of prefix/suffix splits for every
// entry in the comma-list that contains "*". Entries with two or more "*"
// are rejected (too permissive, hard to reason about).
func compileOriginPatterns(cors string) []*originPattern {
	var out []*originPattern
	for _, raw := range strings.Split(cors, ",") {
		s := strings.TrimSpace(raw)
		if s == "" || s == "*" || !strings.Contains(s, "*") {
			continue
		}
		if strings.Count(s, "*") != 1 {
			continue
		}
		idx := strings.IndexByte(s, '*')
		out = append(out, &originPattern{prefix: s[:idx], suffix: s[idx+1:]})
	}
	return out
}

func JWTAuthMiddleware(jwtService *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			RespondError(c, http.StatusUnauthorized, "authorization header required")
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			RespondError(c, http.StatusUnauthorized, "invalid authorization format")
			c.Abort()
			return
		}

		claims, err := jwtService.ValidateToken(parts[1])
		if err != nil {
			RespondError(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}

func APIKeyAuthMiddleware(jwtService *auth.JWTService, apikeyRepo *repository.APIKeyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try API key first
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			hashedKey := auth.HashAPIKey(apiKey)
			key, err := apikeyRepo.GetByHashedKey(hashedKey)
			if err != nil {
				RespondError(c, http.StatusUnauthorized, "invalid api key")
				c.Abort()
				return
			}

			if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
				RespondError(c, http.StatusUnauthorized, "api key expired")
				c.Abort()
				return
			}

			c.Set("user_id", key.UserID)
			c.Set("api_key_project_id", key.ProjectID)
			c.Set("api_key_scopes", key.Scopes)
			if key.Environment != nil {
				c.Set("api_key_environment", *key.Environment)
			}
			c.Next()
			return
		}

		// Fall back to JWT
		JWTAuthMiddleware(jwtService)(c)
	}
}
