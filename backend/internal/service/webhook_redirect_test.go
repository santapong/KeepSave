package service

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestWebhookClient_DoesNotFollowRedirect pins API-F05: the webhook HTTP client
// must not follow a 3xx, so a target that redirects to an internal address
// cannot bypass the registration-time SSRF allow-policy. We assert the internal
// target is never contacted and the redirect status is surfaced as-is.
func TestWebhookClient_DoesNotFollowRedirect(t *testing.T) {
	var internalHits int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&internalHits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer internal.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL, http.StatusFound)
	}))
	defer redirector.Close()

	ws := NewWebhookService(nil)
	resp, err := ws.client.Get(redirector.URL)
	if err != nil {
		t.Fatalf("client.Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302 (redirect was followed)", resp.StatusCode)
	}
	if n := atomic.LoadInt32(&internalHits); n != 0 {
		t.Errorf("internal target hit %d times; redirect should not be followed", n)
	}
}
