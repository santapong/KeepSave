package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// WebhookEvent represents an event that triggers a webhook.
type WebhookEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	ProjectID uuid.UUID              `json:"project_id"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// WebhookConfig defines a webhook endpoint configuration.
type WebhookConfig struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
	Events []string `json:"events"` // e.g., "promotion.completed", "promotion.requested"
}

// WebhookService manages webhook notifications.
type WebhookService struct {
	mu       sync.RWMutex
	configs  map[uuid.UUID][]WebhookConfig // project_id -> configs
	client   *http.Client
	eventLog []WebhookDelivery
}

// WebhookDelivery records a webhook delivery attempt.
type WebhookDelivery struct {
	EventID    string    `json:"event_id"`
	URL        string    `json:"url"`
	StatusCode int       `json:"status_code"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	DeliveredAt time.Time `json:"delivered_at"`
}

// NewWebhookService creates a new webhook service.
func NewWebhookService() *WebhookService {
	return &WebhookService{
		configs: make(map[uuid.UUID][]WebhookConfig),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RegisterWebhook adds a webhook configuration for a project.
func (ws *WebhookService) RegisterWebhook(projectID uuid.UUID, config WebhookConfig) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.configs[projectID] = append(ws.configs[projectID], config)
}

// RemoveWebhooks removes all webhooks for a project.
func (ws *WebhookService) RemoveWebhooks(projectID uuid.UUID) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	delete(ws.configs, projectID)
}

// ListWebhooks returns all webhook configs for a project.
func (ws *WebhookService) ListWebhooks(projectID uuid.UUID) []WebhookConfig {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.configs[projectID]
}

// GetDeliveries returns recent webhook delivery records.
func (ws *WebhookService) GetDeliveries() []WebhookDelivery {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	result := make([]WebhookDelivery, len(ws.eventLog))
	copy(result, ws.eventLog)
	return result
}

// Notify sends a webhook event to all registered endpoints for the project.
func (ws *WebhookService) Notify(projectID uuid.UUID, eventType string, data map[string]interface{}) {
	ws.mu.RLock()
	configs := ws.configs[projectID]
	ws.mu.RUnlock()

	if len(configs) == 0 {
		return
	}

	event := WebhookEvent{
		ID:        uuid.New().String(),
		Type:      eventType,
		ProjectID: projectID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}

	for _, config := range configs {
		if !ws.shouldDeliver(config, eventType) {
			continue
		}
		go ws.deliver(event, config)
	}
}

func (ws *WebhookService) shouldDeliver(config WebhookConfig, eventType string) bool {
	if len(config.Events) == 0 {
		return true // deliver all events if no filter
	}
	for _, e := range config.Events {
		if e == eventType || e == "*" {
			return true
		}
	}
	return false
}

func (ws *WebhookService) deliver(event WebhookEvent, config WebhookConfig) {
	payload, err := json.Marshal(event)
	if err != nil {
		ws.recordDelivery(event.ID, config.URL, 0, false, err.Error())
		return
	}

	req, err := http.NewRequest("POST", config.URL, bytes.NewReader(payload))
	if err != nil {
		ws.recordDelivery(event.ID, config.URL, 0, false, err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-KeepSave-Event", event.Type)
	req.Header.Set("X-KeepSave-Delivery", event.ID)

	if config.Secret != "" {
		signature := computeHMAC(payload, []byte(config.Secret))
		req.Header.Set("X-KeepSave-Signature", fmt.Sprintf("sha256=%s", signature))
	}

	// Retry up to 3 times with exponential backoff
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
		}

		resp, err := ws.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			ws.recordDelivery(event.ID, config.URL, resp.StatusCode, true, "")
			return
		}
		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	ws.recordDelivery(event.ID, config.URL, 0, false, errMsg)
}

func (ws *WebhookService) recordDelivery(eventID, url string, statusCode int, success bool, errMsg string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.eventLog = append(ws.eventLog, WebhookDelivery{
		EventID:    eventID,
		URL:        url,
		StatusCode: statusCode,
		Success:    success,
		Error:      errMsg,
		DeliveredAt: time.Now(),
	})
	// Keep only last 1000 deliveries
	if len(ws.eventLog) > 1000 {
		ws.eventLog = ws.eventLog[len(ws.eventLog)-1000:]
	}
}

func computeHMAC(message, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
}
