package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthLiveness(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHealthHandler(nil) // nil db is fine for liveness
	r := gin.New()
	r.GET("/healthz", handler.Liveness)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}

	if body["version"] != "0.5.0" {
		t.Errorf("expected version '0.5.0', got %q", body["version"])
	}

	if _, ok := body["uptime"]; !ok {
		t.Error("response should contain uptime field")
	}
}
