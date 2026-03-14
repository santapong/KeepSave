package events

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// EventType constants for the event bus.
const (
	SecretCreated       = "secret.created"
	SecretUpdated       = "secret.updated"
	SecretDeleted       = "secret.deleted"
	PromotionRequested  = "promotion.requested"
	PromotionCompleted  = "promotion.completed"
	PromotionApproved   = "promotion.approved"
	PromotionRejected   = "promotion.rejected"
	PromotionRolledBack = "promotion.rolled_back"
	KeyRotated          = "key.rotated"
	UserRegistered      = "user.registered"
	UserLoggedIn        = "user.logged_in"
	APIKeyCreated       = "api_key.created"
	APIKeyDeleted       = "api_key.deleted"
	BackupCreated       = "backup.created"
	LeaseCreated        = "lease.created"
	LeaseRevoked        = "lease.revoked"
)

// Handler is a function that processes an event.
type Handler func(event models.Event)

// Bus is an in-process event bus with persistent event log.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	db       *sql.DB
}

// NewBus creates a new event bus.
func NewBus(db *sql.DB) *Bus {
	return &Bus{
		handlers: make(map[string][]Handler),
		db:       db,
	}
}

// Subscribe registers a handler for an event type.
func (b *Bus) Subscribe(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Publish emits an event to all subscribers and persists it.
func (b *Bus) Publish(eventType string, aggregateID uuid.UUID, payload models.JSONMap) error {
	event := models.Event{
		ID:          uuid.New(),
		EventType:   eventType,
		AggregateID: aggregateID,
		Payload:     payload,
		Published:   false,
		CreatedAt:   time.Now(),
	}

	// Persist to event log
	if b.db != nil {
		_, err := b.db.Exec(
			`INSERT INTO event_log (id, event_type, aggregate_id, payload, published)
			VALUES ($1, $2, $3, $4, $5)`,
			event.ID, event.EventType, event.AggregateID, event.Payload, false,
		)
		if err != nil {
			return fmt.Errorf("persisting event: %w", err)
		}
	}

	// Dispatch to handlers
	b.mu.RLock()
	handlers := b.handlers[eventType]
	wildcardHandlers := b.handlers["*"]
	b.mu.RUnlock()

	for _, h := range handlers {
		go h(event)
	}
	for _, h := range wildcardHandlers {
		go h(event)
	}

	// Mark as published
	if b.db != nil {
		_, _ = b.db.Exec(`UPDATE event_log SET published = TRUE WHERE id = $1`, event.ID)
	}

	return nil
}

// GetUnpublished returns events that haven't been successfully published.
func (b *Bus) GetUnpublished(limit int) ([]models.Event, error) {
	if b.db == nil {
		return nil, nil
	}

	rows, err := b.db.Query(
		`SELECT id, event_type, aggregate_id, payload, published, created_at
		FROM event_log WHERE published = FALSE ORDER BY created_at ASC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying unpublished events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.EventType, &e.AggregateID, &e.Payload, &e.Published, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

// Replay re-emits historical events to handlers since the given time.
func (b *Bus) Replay(eventType string, since time.Time) error {
	if b.db == nil {
		return nil
	}

	rows, err := b.db.Query(
		`SELECT id, event_type, aggregate_id, payload, published, created_at
		FROM event_log WHERE event_type = $1 AND created_at >= $2 ORDER BY created_at ASC`,
		eventType, since,
	)
	if err != nil {
		return fmt.Errorf("querying events for replay: %w", err)
	}
	defer rows.Close()

	b.mu.RLock()
	handlers := b.handlers[eventType]
	b.mu.RUnlock()

	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.EventType, &e.AggregateID, &e.Payload, &e.Published, &e.CreatedAt); err != nil {
			return fmt.Errorf("scanning event for replay: %w", err)
		}
		for _, h := range handlers {
			h(e)
		}
	}
	return nil
}

// GetRecentEvents returns the most recent events.
func (b *Bus) GetRecentEvents(limit int) ([]models.Event, error) {
	if b.db == nil {
		return nil, nil
	}

	rows, err := b.db.Query(
		`SELECT id, event_type, aggregate_id, payload, published, created_at
		FROM event_log ORDER BY created_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying recent events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.EventType, &e.AggregateID, &e.Payload, &e.Published, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}
