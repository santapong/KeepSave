package mcpauth

import "net/http"

// OriginAllowed permits native clients without Origin, and rejects any present
// origin outside the exact operator allowlist. It is not bearer authorization.
func OriginAllowed(cfg Config, r *http.Request) bool {
	values := r.Header.Values("Origin")
	if len(values) == 0 {
		return true
	}
	if len(values) != 1 || values[0] == "" {
		return false
	}
	for _, allowed := range append([]string{cfg.AppURL}, cfg.AllowedOrigins...) {
		if values[0] == allowed {
			return true
		}
	}
	return false
}
func GuardHTTP(cfg Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !OriginAllowed(cfg, r) {
			http.Error(w, "origin denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
