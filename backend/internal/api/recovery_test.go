package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestPanicRecoveryReturnsStructured500 covers audit B-M1: a handler
// panic must produce the canonical {"error":{"code":500,"message":...,
// "error_code":"INTERNAL"}} shape and never echo the panic value.
func TestPanicRecoveryReturnsStructured500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(PanicRecoveryMiddleware())
	r.GET("/boom", func(c *gin.Context) {
		panic("hunter2 secret in panic message")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/boom", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Fatal("empty body")
	}
	// Never echo the panic value itself.
	if contains(body, "hunter2") {
		t.Errorf("response body leaked panic value: %s", body)
	}
	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v - body=%s", err, body)
	}
	if resp.Error.Code != http.StatusInternalServerError {
		t.Errorf("Code = %d, want 500", resp.Error.Code)
	}
	if resp.Error.ErrorCode != ErrInternal.Symbol {
		t.Errorf("ErrorCode = %q, want %q", resp.Error.ErrorCode, ErrInternal.Symbol)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
