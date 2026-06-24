package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// TestEnforceAPIKeyScope_Method covers AUTH-01: an API key is held to its scope
// per HTTP method, and a JWT caller (no api_key_scopes) is unaffected.
func TestEnforceAPIKeyScope_Method(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(scopes []string, method string, apiKey bool) int {
		r := gin.New()
		r.Handle(method, "/x",
			func(c *gin.Context) {
				if apiKey {
					c.Set("api_key_scopes", models.StringList(scopes))
				}
				c.Next()
			},
			EnforceAPIKeyScope(),
			func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
		)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(method, "/x", nil)
		r.ServeHTTP(w, req)
		return w.Code
	}

	cases := []struct {
		name   string
		scopes []string
		method string
		want   int
	}{
		{"read GET ok", []string{"read"}, http.MethodGet, http.StatusOK},
		{"read POST denied", []string{"read"}, http.MethodPost, http.StatusForbidden},
		{"write POST ok", []string{"write"}, http.MethodPost, http.StatusOK},
		{"write DELETE ok", []string{"write"}, http.MethodDelete, http.StatusOK},
		{"read DELETE denied", []string{"read"}, http.MethodDelete, http.StatusForbidden},
		{"delete DELETE ok", []string{"delete"}, http.MethodDelete, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := run(tc.scopes, tc.method, true); code != tc.want {
				t.Errorf("status = %d, want %d", code, tc.want)
			}
		})
	}

	// JWT caller (no api_key_scopes) is unaffected regardless of method.
	if code := run(nil, http.MethodDelete, false); code != http.StatusOK {
		t.Errorf("JWT DELETE status = %d, want 200", code)
	}
}

// TestEnforceAPIKeyScope_Environment covers AUTH-02: an environment-locked key
// may only target its environment, read from the query or the JSON body.
func TestEnforceAPIKeyScope_Environment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(keyEnv string, setEnv bool, method, path, body string) int {
		r := gin.New()
		r.Handle(method, "/x",
			func(c *gin.Context) {
				c.Set("api_key_scopes", models.StringList{"read", "write"})
				if setEnv {
					c.Set("api_key_environment", keyEnv)
				}
				c.Next()
			},
			EnforceAPIKeyScope(),
			func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
		)
		w := httptest.NewRecorder()
		var req *http.Request
		if body != "" {
			req, _ = http.NewRequest(method, "/x"+path, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, "/x"+path, nil)
		}
		r.ServeHTTP(w, req)
		return w.Code
	}

	if code := run("alpha", true, http.MethodPost, "", `{"environment":"prod","value":"x"}`); code != http.StatusForbidden {
		t.Errorf("alpha key, prod body = %d, want 403", code)
	}
	if code := run("alpha", true, http.MethodPost, "", `{"environment":"alpha","value":"x"}`); code != http.StatusOK {
		t.Errorf("alpha key, alpha body = %d, want 200", code)
	}
	if code := run("alpha", true, http.MethodPost, "", `{"value":"x"}`); code != http.StatusOK {
		t.Errorf("alpha key, no env in body = %d, want 200 (skipped)", code)
	}
	if code := run("alpha", true, http.MethodGet, "?environment=prod", ""); code != http.StatusForbidden {
		t.Errorf("alpha key, prod query = %d, want 403", code)
	}
	if code := run("alpha", false, http.MethodPost, "", `{"environment":"prod"}`); code != http.StatusOK {
		t.Errorf("unbound key = %d, want 200", code)
	}
}

// TestEnforceAPIKeyScope_PreservesBody ensures the body peek for the
// environment check does not consume the body the handler later binds.
func TestEnforceAPIKeyScope_PreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x",
		func(c *gin.Context) {
			c.Set("api_key_scopes", models.StringList{"write"})
			c.Set("api_key_environment", "alpha")
			c.Next()
		},
		EnforceAPIKeyScope(),
		func(c *gin.Context) {
			var body struct {
				Environment string `json:"environment"`
				Value       string `json:"value"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"err": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"value": body.Value})
		},
	)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{"environment":"alpha","value":"hunter2"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hunter2") {
		t.Errorf("handler did not see body after middleware peek: %s", w.Body.String())
	}
}
