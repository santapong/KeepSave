package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// TestParseScope covers the action[:keyGlob] split (ADR-0022).
func TestParseScope(t *testing.T) {
	cases := []struct{ in, action, glob string }{
		{"read", "read", ""},
		{"read:DB_*", "read", "DB_*"},
		{"write:APP_CONFIG", "write", "APP_CONFIG"},
		{"read:*", "read", "*"},
		{"read:a:b", "read", "a:b"}, // only the first ':' splits
	}
	for _, tc := range cases {
		a, g := parseScope(tc.in)
		if a != tc.action || g != tc.glob {
			t.Errorf("parseScope(%q) = (%q,%q), want (%q,%q)", tc.in, a, g, tc.action, tc.glob)
		}
	}
}

// TestMatchKeyGlob covers the "*"-only glob matcher.
func TestMatchKeyGlob(t *testing.T) {
	cases := []struct {
		pattern, key string
		want         bool
	}{
		{"", "ANYTHING", true},             // empty = all (legacy)
		{"*", "ANYTHING", true},            // star = all
		{"DB_*", "DB_URL", true},           // prefix
		{"DB_*", "DBURL", false},           // prefix must match literally
		{"DB_*", "STRIPE_KEY", false},      // no match
		{"*_URL", "DB_URL", true},          // suffix
		{"*_URL", "DB_URLX", false},        // suffix anchored to end
		{"DB_URL", "DB_URL", true},         // exact
		{"DB_URL", "DB_URL2", false},       // exact, no partial
		{"*CONFIG*", "APP_CONFIG_X", true}, // contains
		{"*CONFIG*", "APP_CFG_X", false},   // contains miss
		{"A*B*C", "AxxByyC", true},         // multi-segment in order
		{"A*B*C", "AxxCyyB", false},        // out of order
	}
	for _, tc := range cases {
		if got := matchKeyGlob(tc.pattern, tc.key); got != tc.want {
			t.Errorf("matchKeyGlob(%q,%q) = %v, want %v", tc.pattern, tc.key, got, tc.want)
		}
	}
}

// TestApiKeyScopeAllowsKey covers per-key grants incl. the write⇒delete
// implication and legacy bare-scope back-compat.
func TestApiKeyScopeAllowsKey(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		action string
		key    string
		want   bool
	}{
		{"bare read matches any key", []string{"read"}, "read", "STRIPE_KEY", true},
		{"scoped read matches glob", []string{"read:DB_*"}, "read", "DB_URL", true},
		{"scoped read misses other key", []string{"read:DB_*"}, "read", "STRIPE_KEY", false},
		{"scoped read denies write", []string{"read:DB_*"}, "write", "DB_URL", false},
		{"write implies delete on same glob", []string{"write:DB_*"}, "delete", "DB_URL", true},
		{"write does not imply delete off-glob", []string{"write:DB_*"}, "delete", "X", false},
		{"multiple scopes union", []string{"read:DB_*", "write:APP_*"}, "write", "APP_CONFIG", true},
		{"bare write implies delete all", []string{"write"}, "delete", "ANY", true},
		{"no scopes", []string{}, "read", "K", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apiKeyScopeAllowsKey(models.StringList(tc.scopes), tc.action, tc.key); got != tc.want {
				t.Errorf("apiKeyScopeAllowsKey(%v,%q,%q) = %v, want %v", tc.scopes, tc.action, tc.key, got, tc.want)
			}
		})
	}
}

// TestApiKeyHasScope_GrammarActionGate confirms the coarse action gate parses
// the action prefix of grammar scopes (so a key-scoped key still passes the
// method gate, with per-key precision enforced later in handlers).
func TestApiKeyHasScope_GrammarActionGate(t *testing.T) {
	if !apiKeyHasScope(models.StringList{"read:DB_*"}, "read") {
		t.Error("read:DB_* should pass the read action gate")
	}
	if apiKeyHasScope(models.StringList{"read:DB_*"}, "write") {
		t.Error("read:DB_* must not pass the write action gate")
	}
	if !apiKeyHasScope(models.StringList{"write:APP_*"}, "delete") {
		t.Error("write:APP_* should pass the delete action gate (write⇒delete)")
	}
}

// TestAPIKeyScopeAllowsKey_NoopForJWT verifies the handler helper is a no-op
// (allow) when the caller is not API-key authenticated — the back-compat
// guarantee for JWT/user callers (ADR-0022).
func TestAPIKeyScopeAllowsKey_NoopForJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// No api_key_scopes set ⇒ allow.
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if !APIKeyScopeAllowsKey(c, "write", "ANYTHING") {
		t.Error("helper should allow when no api_key_scopes present (JWT caller)")
	}

	// With a key-scoped api key, the helper enforces.
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Set("api_key_scopes", models.StringList{"read:DB_*"})
	if APIKeyScopeAllowsKey(c2, "read", "STRIPE_KEY") {
		t.Error("helper should deny an out-of-scope key for an API-key caller")
	}
	if !APIKeyScopeAllowsKey(c2, "read", "DB_URL") {
		t.Error("helper should allow an in-scope key")
	}
}
