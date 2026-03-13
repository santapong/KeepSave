package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWebhookServiceRegisterAndList(t *testing.T) {
	ws := NewWebhookService()
	projectID := uuid.New()

	config := WebhookConfig{
		URL:    "https://example.com/webhook",
		Secret: "test-secret",
		Events: []string{"promotion.completed"},
	}

	ws.RegisterWebhook(projectID, config)

	configs := ws.ListWebhooks(projectID)
	if len(configs) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(configs))
	}
	if configs[0].URL != "https://example.com/webhook" {
		t.Errorf("URL = %q, want %q", configs[0].URL, "https://example.com/webhook")
	}
	if configs[0].Events[0] != "promotion.completed" {
		t.Errorf("event = %q, want %q", configs[0].Events[0], "promotion.completed")
	}
}

func TestWebhookServiceRemove(t *testing.T) {
	ws := NewWebhookService()
	projectID := uuid.New()

	ws.RegisterWebhook(projectID, WebhookConfig{URL: "https://example.com/hook"})
	ws.RemoveWebhooks(projectID)

	configs := ws.ListWebhooks(projectID)
	if len(configs) != 0 {
		t.Errorf("expected 0 webhooks after removal, got %d", len(configs))
	}
}

func TestWebhookServiceListEmpty(t *testing.T) {
	ws := NewWebhookService()
	configs := ws.ListWebhooks(uuid.New())
	if configs != nil {
		t.Errorf("expected nil for unknown project, got %v", configs)
	}
}

func TestWebhookServiceNotify(t *testing.T) {
	var receivedCount int32
	var receivedPayload []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&receivedCount, 1)

		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}
		if r.Header.Get("X-KeepSave-Event") == "" {
			t.Error("expected X-KeepSave-Event header")
		}
		if r.Header.Get("X-KeepSave-Delivery") == "" {
			t.Error("expected X-KeepSave-Delivery header")
		}

		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedPayload = buf[:n]

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ws := NewWebhookService()
	projectID := uuid.New()

	ws.RegisterWebhook(projectID, WebhookConfig{
		URL:    server.URL,
		Events: []string{"promotion.completed"},
	})

	ws.Notify(projectID, "promotion.completed", map[string]interface{}{
		"source": "alpha",
		"target": "uat",
	})

	// Wait for async delivery
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&receivedCount) != 1 {
		t.Errorf("expected 1 delivery, got %d", receivedCount)
	}

	var event WebhookEvent
	if err := json.Unmarshal(receivedPayload, &event); err != nil {
		t.Fatalf("failed to parse webhook payload: %v", err)
	}

	if event.Type != "promotion.completed" {
		t.Errorf("event type = %q, want %q", event.Type, "promotion.completed")
	}
	if event.ProjectID != projectID {
		t.Errorf("project ID mismatch")
	}
}

func TestWebhookServiceEventFilter(t *testing.T) {
	var receivedCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&receivedCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ws := NewWebhookService()
	projectID := uuid.New()

	ws.RegisterWebhook(projectID, WebhookConfig{
		URL:    server.URL,
		Events: []string{"promotion.completed"},
	})

	// This event type should NOT be delivered
	ws.Notify(projectID, "promotion.requested", map[string]interface{}{})

	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&receivedCount) != 0 {
		t.Errorf("filtered event should not be delivered, got %d deliveries", receivedCount)
	}
}

func TestWebhookServiceWildcardEvents(t *testing.T) {
	var receivedCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&receivedCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ws := NewWebhookService()
	projectID := uuid.New()

	ws.RegisterWebhook(projectID, WebhookConfig{
		URL:    server.URL,
		Events: []string{"*"},
	})

	ws.Notify(projectID, "any.event.type", map[string]interface{}{})

	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&receivedCount) != 1 {
		t.Errorf("wildcard should match any event, got %d deliveries", receivedCount)
	}
}

func TestWebhookServiceHMACSignature(t *testing.T) {
	var signature string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signature = r.Header.Get("X-KeepSave-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ws := NewWebhookService()
	projectID := uuid.New()

	ws.RegisterWebhook(projectID, WebhookConfig{
		URL:    server.URL,
		Secret: "webhook-secret-key",
		Events: []string{"*"},
	})

	ws.Notify(projectID, "test.event", map[string]interface{}{"key": "value"})

	time.Sleep(200 * time.Millisecond)

	if signature == "" {
		t.Error("expected HMAC signature header")
	}
	if len(signature) < 10 {
		t.Error("signature seems too short")
	}
	if signature[:7] != "sha256=" {
		t.Errorf("signature should start with 'sha256=', got %q", signature[:7])
	}
}

func TestWebhookServiceDeliveryLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ws := NewWebhookService()
	projectID := uuid.New()

	ws.RegisterWebhook(projectID, WebhookConfig{
		URL:    server.URL,
		Events: []string{"*"},
	})

	ws.Notify(projectID, "test.event", map[string]interface{}{})
	time.Sleep(200 * time.Millisecond)

	deliveries := ws.GetDeliveries()
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery record, got %d", len(deliveries))
	}
	if !deliveries[0].Success {
		t.Error("delivery should be successful")
	}
}

func TestComputeHMAC(t *testing.T) {
	message := []byte("test payload")
	key := []byte("secret-key")

	sig1 := computeHMAC(message, key)
	sig2 := computeHMAC(message, key)

	if sig1 != sig2 {
		t.Error("same input should produce same HMAC")
	}

	sig3 := computeHMAC(message, []byte("different-key"))
	if sig1 == sig3 {
		t.Error("different keys should produce different HMACs")
	}

	sig4 := computeHMAC([]byte("different payload"), key)
	if sig1 == sig4 {
		t.Error("different payloads should produce different HMACs")
	}
}
