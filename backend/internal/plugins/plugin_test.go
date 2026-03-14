package plugins

import (
	"fmt"
	"testing"
)

type mockSecretProvider struct {
	name    string
	secrets map[string]string
}

func (m *mockSecretProvider) Name() string                     { return m.name }
func (m *mockSecretProvider) Get(key string) (string, error)   { return m.secrets[key], nil }
func (m *mockSecretProvider) Set(key, value string) error      { m.secrets[key] = value; return nil }
func (m *mockSecretProvider) Delete(key string) error          { delete(m.secrets, key); return nil }
func (m *mockSecretProvider) List() (map[string]string, error) { return m.secrets, nil }

type mockNotificationSender struct {
	name string
	sent int
}

func (m *mockNotificationSender) Name() string { return m.name }
func (m *mockNotificationSender) Send(eventType string, payload map[string]interface{}) error {
	m.sent++
	return nil
}

type mockValidator struct {
	name    string
	rejects bool
}

func (m *mockValidator) Name() string { return m.name }
func (m *mockValidator) Validate(key, value string) error {
	if m.rejects {
		return fmt.Errorf("rejected by %s", m.name)
	}
	return nil
}

func TestRegistrySecretProvider(t *testing.T) {
	registry := NewRegistry(nil)

	provider := &mockSecretProvider{
		name:    "vault",
		secrets: map[string]string{"DB_URL": "postgres://..."},
	}
	registry.RegisterSecretProvider(provider)

	found, ok := registry.GetSecretProvider("vault")
	if !ok {
		t.Fatal("expected to find vault provider")
	}
	if found.Name() != "vault" {
		t.Errorf("expected name vault, got %s", found.Name())
	}

	val, _ := found.Get("DB_URL")
	if val != "postgres://..." {
		t.Errorf("expected postgres://..., got %s", val)
	}
}

func TestRegistryNotificationSender(t *testing.T) {
	registry := NewRegistry(nil)

	sender := &mockNotificationSender{name: "slack"}
	registry.RegisterNotificationSender(sender)

	found, ok := registry.GetNotificationSender("slack")
	if !ok {
		t.Fatal("expected to find slack sender")
	}
	if found.Name() != "slack" {
		t.Errorf("expected name slack, got %s", found.Name())
	}
}

func TestRegistryNotifyAll(t *testing.T) {
	registry := NewRegistry(nil)

	s1 := &mockNotificationSender{name: "slack"}
	s2 := &mockNotificationSender{name: "pagerduty"}
	registry.RegisterNotificationSender(s1)
	registry.RegisterNotificationSender(s2)

	registry.NotifyAll("secret.created", map[string]interface{}{"key": "DB_URL"})
}

func TestRegistryValidateAll(t *testing.T) {
	registry := NewRegistry(nil)

	v1 := &mockValidator{name: "format", rejects: false}
	registry.RegisterValidator(v1)

	err := registry.ValidateAll("DB_URL", "postgres://...")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	v2 := &mockValidator{name: "strict", rejects: true}
	registry.RegisterValidator(v2)

	err = registry.ValidateAll("DB_URL", "bad-value")
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestRegistryProviderNotFound(t *testing.T) {
	registry := NewRegistry(nil)

	_, ok := registry.GetSecretProvider("nonexistent")
	if ok {
		t.Error("expected provider not found")
	}
}
