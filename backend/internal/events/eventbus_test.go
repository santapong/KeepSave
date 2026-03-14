package events

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

func TestBusPublishSubscribe(t *testing.T) {
	bus := NewBus(nil)

	var received atomic.Int32

	bus.Subscribe("test.event", func(event models.Event) {
		received.Add(1)
	})

	err := bus.Publish("test.event", uuid.New(), models.JSONMap{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for async handler
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 event received, got %d", received.Load())
	}
}

func TestBusWildcardSubscriber(t *testing.T) {
	bus := NewBus(nil)

	var received atomic.Int32

	bus.Subscribe("*", func(event models.Event) {
		received.Add(1)
	})

	_ = bus.Publish("event.a", uuid.New(), nil)
	_ = bus.Publish("event.b", uuid.New(), nil)

	time.Sleep(50 * time.Millisecond)

	if received.Load() != 2 {
		t.Errorf("expected 2 wildcard events, got %d", received.Load())
	}
}

func TestBusMultipleSubscribers(t *testing.T) {
	bus := NewBus(nil)

	var count1, count2 atomic.Int32

	bus.Subscribe("test", func(event models.Event) {
		count1.Add(1)
	})
	bus.Subscribe("test", func(event models.Event) {
		count2.Add(1)
	})

	_ = bus.Publish("test", uuid.New(), nil)

	time.Sleep(50 * time.Millisecond)

	if count1.Load() != 1 || count2.Load() != 1 {
		t.Errorf("expected both subscribers to receive event")
	}
}

func TestBusNoSubscribers(t *testing.T) {
	bus := NewBus(nil)

	err := bus.Publish("nobody.listens", uuid.New(), nil)
	if err != nil {
		t.Fatalf("expected no error for events with no subscribers: %v", err)
	}
}

func TestBusEventPayload(t *testing.T) {
	bus := NewBus(nil)

	var receivedPayload models.JSONMap
	done := make(chan struct{})

	bus.Subscribe("payload.test", func(event models.Event) {
		receivedPayload = event.Payload
		close(done)
	})

	payload := models.JSONMap{"project_id": "abc", "action": "promote"}
	_ = bus.Publish("payload.test", uuid.New(), payload)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}

	if receivedPayload["project_id"] != "abc" {
		t.Errorf("expected project_id=abc, got %v", receivedPayload["project_id"])
	}
}
