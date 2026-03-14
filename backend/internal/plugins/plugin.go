package plugins

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// SecretProvider is the interface for external secret storage providers.
type SecretProvider interface {
	Name() string
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	List() (map[string]string, error)
}

// NotificationSender is the interface for notification plugins.
type NotificationSender interface {
	Name() string
	Send(eventType string, payload map[string]interface{}) error
}

// Validator is the interface for custom validation plugins.
type Validator interface {
	Name() string
	Validate(key, value string) error
}

// Registry manages registered plugins.
type Registry struct {
	mu                sync.RWMutex
	secretProviders   map[string]SecretProvider
	notificationSinks map[string]NotificationSender
	validators        map[string]Validator
	db                *sql.DB
}

// NewRegistry creates a new plugin registry.
func NewRegistry(db *sql.DB) *Registry {
	return &Registry{
		secretProviders:   make(map[string]SecretProvider),
		notificationSinks: make(map[string]NotificationSender),
		validators:        make(map[string]Validator),
		db:                db,
	}
}

// RegisterSecretProvider registers a secret provider plugin.
func (r *Registry) RegisterSecretProvider(provider SecretProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secretProviders[provider.Name()] = provider
}

// RegisterNotificationSender registers a notification plugin.
func (r *Registry) RegisterNotificationSender(sender NotificationSender) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notificationSinks[sender.Name()] = sender
}

// RegisterValidator registers a validation plugin.
func (r *Registry) RegisterValidator(validator Validator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validators[validator.Name()] = validator
}

// GetSecretProvider returns a registered secret provider by name.
func (r *Registry) GetSecretProvider(name string) (SecretProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.secretProviders[name]
	return p, ok
}

// GetNotificationSender returns a registered notification sender by name.
func (r *Registry) GetNotificationSender(name string) (NotificationSender, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.notificationSinks[name]
	return s, ok
}

// ListPlugins returns all registered plugins from the database.
func (r *Registry) ListPlugins() ([]models.Plugin, error) {
	if r.db == nil {
		return nil, nil
	}

	rows, err := r.db.Query(
		`SELECT id, name, plugin_type, version, config, enabled, created_at, updated_at
		FROM plugins ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing plugins: %w", err)
	}
	defer rows.Close()

	var plugins []models.Plugin
	for rows.Next() {
		var p models.Plugin
		if err := rows.Scan(&p.ID, &p.Name, &p.PluginType, &p.Version, &p.Config,
			&p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning plugin: %w", err)
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// RegisterPlugin adds a plugin to the database registry.
func (r *Registry) RegisterPlugin(name, pluginType, version string, config models.JSONMap) (*models.Plugin, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	plugin := &models.Plugin{}
	err := r.db.QueryRow(
		`INSERT INTO plugins (name, plugin_type, version, config, enabled)
		VALUES ($1, $2, $3, $4, TRUE)
		ON CONFLICT (name) DO UPDATE SET version = $3, config = $4, updated_at = NOW()
		RETURNING id, name, plugin_type, version, config, enabled, created_at, updated_at`,
		name, pluginType, version, config,
	).Scan(&plugin.ID, &plugin.Name, &plugin.PluginType, &plugin.Version, &plugin.Config,
		&plugin.Enabled, &plugin.CreatedAt, &plugin.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("registering plugin: %w", err)
	}
	return plugin, nil
}

// TogglePlugin enables or disables a plugin.
func (r *Registry) TogglePlugin(pluginID uuid.UUID, enabled bool) error {
	if r.db == nil {
		return fmt.Errorf("database not configured")
	}

	_, err := r.db.Exec(`UPDATE plugins SET enabled = $2, updated_at = NOW() WHERE id = $1`, pluginID, enabled)
	if err != nil {
		return fmt.Errorf("toggling plugin: %w", err)
	}
	return nil
}

// NotifyAll sends a notification to all enabled notification plugins.
func (r *Registry) NotifyAll(eventType string, payload map[string]interface{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, sender := range r.notificationSinks {
		go func(s NotificationSender) {
			_ = s.Send(eventType, payload)
		}(sender)
	}
}

// ValidateAll runs all validation plugins against a secret.
func (r *Registry) ValidateAll(key, value string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, v := range r.validators {
		if err := v.Validate(key, value); err != nil {
			return fmt.Errorf("validation plugin %s: %w", v.Name(), err)
		}
	}
	return nil
}
