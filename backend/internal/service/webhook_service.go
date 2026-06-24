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
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
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
	URL    string   `json:"url"`
	Secret string   `json:"secret"`
	Events []string `json:"events"` // e.g., "promotion.completed", "promotion.requested"
}

// WebhookService manages webhook notifications.
type WebhookService struct {
	mu        sync.RWMutex
	configs   map[uuid.UUID][]WebhookConfig // project_id -> configs
	client    *http.Client
	eventLog  []WebhookDelivery
	auditRepo *repository.AuditRepository
}

// WebhookDelivery records a webhook delivery attempt. ProjectID scopes the
// record to its tenant so the delivery log (a process-global slice) can be
// filtered per caller instead of exposing every tenant's target URLs.
type WebhookDelivery struct {
	EventID     string    `json:"event_id"`
	ProjectID   uuid.UUID `json:"project_id"`
	URL         string    `json:"url"`
	StatusCode  int       `json:"status_code"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	DeliveredAt time.Time `json:"delivered_at"`
}

// NewWebhookService creates a new webhook service.
func NewWebhookService(auditRepo *repository.AuditRepository) *WebhookService {
	return &WebhookService{
		configs: make(map[uuid.UUID][]WebhookConfig),
		client: &http.Client{
			Timeout: 10 * time.Second,
			// Do not follow redirects (API-F05): a webhook target that 3xx's to
			// an internal address (e.g. 169.254.169.254) would otherwise bypass
			// the registration-time SSRF allow-policy. ErrUseLastResponse makes
			// Do return the redirect response itself, which is treated as a
			// non-2xx failed delivery.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		auditRepo: auditRepo,
	}
}

// RegisterWebhook adds a webhook configuration for a project. The URL is
// validated against the SSRF allow-policy at registration time (ADR-0013 /
// audit S-H2) so an attacker cannot register an internal target and trigger
// it later via a state-mutating call. Delivery does NOT re-validate -
// re-resolving on every call costs latency and only protects against DNS
// rebinding, which we accept as out-of-scope for this round.
func (ws *WebhookService) RegisterWebhook(projectID uuid.UUID, config WebhookConfig, actorID uuid.UUID, ipAddr string) error {
	if err := ValidateWebhookURL(config.URL); err != nil {
		return err
	}
	ws.mu.Lock()
	ws.configs[projectID] = append(ws.configs[projectID], config)
	ws.mu.Unlock()

	// Never log the webhook secret; the URL/events are non-sensitive metadata.
	emitAudit(ws.auditRepo, &actorID, &projectID, "webhook.registered", "",
		models.JSONMap{"project_id": projectID.String(), "url": config.URL, "events": config.Events}, ipAddr)
	return nil
}

// RemoveWebhooks removes all webhooks for a project.
func (ws *WebhookService) RemoveWebhooks(projectID uuid.UUID, actorID uuid.UUID, ipAddr string) {
	ws.mu.Lock()
	removed := len(ws.configs[projectID])
	delete(ws.configs, projectID)
	ws.mu.Unlock()

	emitAudit(ws.auditRepo, &actorID, &projectID, "webhook.removed", "",
		models.JSONMap{"project_id": projectID.String(), "removed_count": removed}, ipAddr)
}

// ListWebhooks returns all webhook configs for a project.
func (ws *WebhookService) ListWebhooks(projectID uuid.UUID) []WebhookConfig {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.configs[projectID]
}

// GetDeliveries returns recent webhook delivery records for the given projects
// only (the caller's accessible set). Passing no projects returns nothing — a
// caller never sees another tenant's delivery records.
func (ws *WebhookService) GetDeliveries(projectIDs []uuid.UUID) []WebhookDelivery {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	allow := make(map[uuid.UUID]bool, len(projectIDs))
	for _, id := range projectIDs {
		allow[id] = true
	}
	result := make([]WebhookDelivery, 0)
	for _, d := range ws.eventLog {
		if allow[d.ProjectID] {
			result = append(result, d)
		}
	}
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
		ws.recordDelivery(event.ID, event.ProjectID, config.URL, 0, false, err.Error())
		return
	}

	req, err := http.NewRequest("POST", config.URL, bytes.NewReader(payload))
	if err != nil {
		ws.recordDelivery(event.ID, event.ProjectID, config.URL, 0, false, err.Error())
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
			ws.recordDelivery(event.ID, event.ProjectID, config.URL, resp.StatusCode, true, "")
			return
		}
		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	ws.recordDelivery(event.ID, event.ProjectID, config.URL, 0, false, errMsg)
}

func (ws *WebhookService) recordDelivery(eventID string, projectID uuid.UUID, url string, statusCode int, success bool, errMsg string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.eventLog = append(ws.eventLog, WebhookDelivery{
		EventID:     eventID,
		ProjectID:   projectID,
		URL:         url,
		StatusCode:  statusCode,
		Success:     success,
		Error:       errMsg,
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
