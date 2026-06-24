package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/models"
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

// scopeForMethod maps an HTTP method to the API-key scope required to perform
// it. The scope vocabulary (read/write/delete/promote) matches validation.go.
func scopeForMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return "read"
	case http.MethodDelete:
		return "delete"
	default: // POST, PUT, PATCH
		return "write"
	}
}

// parseScope splits a scope token into its action and optional key glob
// (ADR-0022). "read" → ("read",""); "read:DB_*" → ("read","DB_*"). An empty glob
// means "all keys" (legacy semantics), so bare scopes are unchanged.
func parseScope(s string) (action, keyGlob string) {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// actionGrants reports whether a scope's action grants the required action,
// honouring the write⇒delete implication (operators commonly issue ["write"]
// expecting full mutate access).
func actionGrants(scopeAction, required string) bool {
	return scopeAction == required || (required == "delete" && scopeAction == "write")
}

// apiKeyHasScope reports whether scopes grant the required action on at least
// one key. It is the coarse action gate (used where the target key is not yet
// known, e.g. list); per-key precision is apiKeyScopeAllowsKey.
func apiKeyHasScope(scopes models.StringList, required string) bool {
	for _, s := range scopes {
		if a, _ := parseScope(s); actionGrants(a, required) {
			return true
		}
	}
	return false
}

// matchKeyGlob matches a secret key against a glob whose only metacharacter is
// "*" (any run of characters, including empty). An empty pattern matches every
// key (legacy bare-scope semantics).
func matchKeyGlob(pattern, key string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	// No "*": exact match.
	if len(parts) == 1 {
		return pattern == key
	}
	// Anchor the first segment to the start.
	if parts[0] != "" {
		if !strings.HasPrefix(key, parts[0]) {
			return false
		}
		key = key[len(parts[0]):]
	}
	// Anchor the last segment to the end.
	last := parts[len(parts)-1]
	if last != "" {
		if !strings.HasSuffix(key, last) {
			return false
		}
		key = key[:len(key)-len(last)]
	}
	// Middle segments must appear in order.
	for _, seg := range parts[1 : len(parts)-1] {
		if seg == "" {
			continue
		}
		idx := strings.Index(key, seg)
		if idx < 0 {
			return false
		}
		key = key[idx+len(seg):]
	}
	return true
}

// apiKeyScopeAllowsKey reports whether scopes grant the required action on the
// specific key (ADR-0022): some scope's action must grant the action AND its
// key glob must match. A bare/`*` glob matches every key (back-compat).
func apiKeyScopeAllowsKey(scopes models.StringList, action, key string) bool {
	for _, s := range scopes {
		a, glob := parseScope(s)
		if actionGrants(a, action) && matchKeyGlob(glob, key) {
			return true
		}
	}
	return false
}

// APIKeyScopeAllowsKey is the handler-facing per-key check (ADR-0022). It is a
// no-op (returns true) for callers that are not API-key authenticated — JWT/user
// callers carry no api_key_scopes and are unaffected.
func APIKeyScopeAllowsKey(c *gin.Context, action, key string) bool {
	scopesVal, ok := c.Get("api_key_scopes")
	if !ok {
		return true
	}
	scopes, _ := scopesVal.(models.StringList)
	return apiKeyScopeAllowsKey(scopes, action, key)
}

// targetEnvironment best-effort extracts the environment a request targets,
// from the ?environment query param or a JSON body "environment" field. The
// body is read and restored so the handler's own bind is unaffected. Returns
// "" when the request does not name an environment (e.g. routes keyed only by
// :secretId), in which case the environment check is skipped at this layer.
func targetEnvironment(c *gin.Context) string {
	if env := c.Query("environment"); env != "" {
		return env
	}
	if c.Request.Body == nil || !strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		return ""
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
	if len(raw) == 0 {
		return ""
	}
	var probe struct {
		Environment string `json:"environment"`
	}
	if json.Unmarshal(raw, &probe) == nil {
		return probe.Environment
	}
	return ""
}

// EnforceAPIKeyScope restricts API-key callers to their granted scope and, when
// the key is environment-locked, to that environment (AUTH-01/02). It is a
// no-op for JWT callers (which never set api_key_scopes), so it must be mounted
// after APIKeyAuthMiddleware.
func EnforceAPIKeyScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		scopesVal, ok := c.Get("api_key_scopes")
		if !ok {
			c.Next() // not API-key auth — unaffected
			return
		}
		scopes, _ := scopesVal.(models.StringList)
		if !apiKeyHasScope(scopes, scopeForMethod(c.Request.Method)) {
			WrapError(c, ErrForbidden)
			c.Abort()
			return
		}
		if envVal, ok := c.Get("api_key_environment"); ok {
			keyEnv, _ := envVal.(string)
			if target := targetEnvironment(c); target != "" && !strings.EqualFold(target, keyEnv) {
				WrapError(c, ErrForbidden)
				c.Abort()
				return
			}
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

		// Agent tokens (ADR-0021) are a confined principal, NOT a general user
		// credential. They may only READ the secrets their lease names, in the
		// lease's project + environment. Default-deny: reject on any route
		// outside the read-secret allowlist (this single chokepoint also blocks
		// lease/token management, promotion, org/admin, applications, etc., and
		// closes the privilege-escalation where a lease-minted token inherited
		// the owning user's full access). Authorization is then re-derived from
		// the embedded lease scope by reusing the API-key enforcement path
		// (RequireProjectAccess + EnforceAPIKeyScope + APIKeyScopeAllowsKey).
		if claims.TokenType == "agent" {
			if !agentTokenRouteAllowed(c.Request.Method, c.FullPath()) {
				RespondError(c, http.StatusForbidden, "agent token not permitted for this operation")
				c.Abort()
				return
			}
			c.Set("user_id", claims.UserID)
			c.Set("email", claims.Email)
			c.Set("is_agent_token", true)
			if claims.ProjectID != nil {
				c.Set("api_key_project_id", *claims.ProjectID)
			}
			c.Set("api_key_scopes", agentReadScopes(claims.SecretKeys))
			if claims.Environment != "" {
				c.Set("api_key_environment", claims.Environment)
			}
			c.Next()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}

// agentTokenRouteAllowed is the allowlist of routes an ADR-0021 agent token may
// reach. It is intentionally minimal — only reading secrets — because an agent
// token is a lease-scoped read credential, never a management or write
// credential. Matching is on gin's matched route pattern (c.FullPath()), not the
// raw path, so it cannot be fooled by path tricks. Everything not listed here is
// denied, including the lease/agent-token endpoints themselves (no privilege
// re-delegation) and every write method on the secret routes.
func agentTokenRouteAllowed(method, fullPath string) bool {
	if method != http.MethodGet {
		return false
	}
	switch fullPath {
	case "/api/v1/projects/:id/secrets", "/api/v1/projects/:id/secrets/:secretId":
		return true
	default:
		return false
	}
}

// agentReadScopes turns a lease's secret keys into ADR-0022 read scopes
// (read:<key>), so the existing per-key enforcement restricts an agent token to
// exactly its leased keys and the coarse action gate forbids any write/delete.
func agentReadScopes(keys []string) models.StringList {
	scopes := make(models.StringList, 0, len(keys))
	for _, k := range keys {
		scopes = append(scopes, "read:"+k)
	}
	return scopes
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
