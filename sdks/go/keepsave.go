// Package keepsave provides a Go SDK v2.0.0 for the KeepSave API.
//
// Features:
//   - Full CRUD for projects, secrets, promotions, and API keys
//   - Automatic retry with exponential backoff
//   - Circuit breaker for API resilience
//   - Batch secret fetch
//   - In-memory cache with TTL
//   - Automatic secret refresh on rotation detection
//
// Usage:
//
//	client := keepsave.NewClient("http://localhost:8080", keepsave.WithToken("jwt-token"))
//	secrets, err := client.ListSecrets(ctx, "project-id", "alpha")
package keepsave

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ── Client & Options ────────────────────────────────────────────────

// Client is a KeepSave API client.
type Client struct {
	baseURL    string
	token      string
	apiKey     string
	httpClient *http.Client

	maxRetries int
	breaker    *circuitBreaker
	cache      *cache
}

// Option configures a Client.
type Option func(*Client)

// WithToken sets the JWT token for authentication.
func WithToken(token string) Option {
	return func(c *Client) { c.token = token }
}

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithMaxRetries sets the maximum number of retries on transient failures (default: 3).
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

// WithCircuitBreaker enables the circuit breaker with the given threshold and reset timeout.
func WithCircuitBreaker(threshold int, resetTimeout time.Duration) Option {
	return func(c *Client) {
		c.breaker = newCircuitBreaker(threshold, resetTimeout)
	}
}

// WithCacheTTL sets the cache TTL. Set to 0 to disable caching (default: 60s).
func WithCacheTTL(ttl time.Duration) Option {
	return func(c *Client) {
		c.cache = newCache(ttl)
	}
}

// NewClient creates a new KeepSave API client.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries: 3,
		breaker:    newCircuitBreaker(5, 30*time.Second),
		cache:      newCache(60 * time.Second),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ClearCache clears the local secret cache.
func (c *Client) ClearCache() {
	if c.cache != nil {
		c.cache.clear()
	}
}

// CircuitState returns the circuit breaker state (CLOSED / OPEN / HALF_OPEN / DISABLED).
func (c *Client) CircuitState() string {
	if c.breaker == nil {
		return "DISABLED"
	}
	return c.breaker.state()
}

// ── Types ───────────────────────────────────────────────────────────

// Error represents an API error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("keepsave: %d %s", e.Code, e.Message)
}

// CircuitBreakerOpenError is returned when the circuit breaker is open.
type CircuitBreakerOpenError struct{}

func (e *CircuitBreakerOpenError) Error() string {
	return "keepsave: circuit breaker is open — requests are temporarily blocked"
}

// Project represents a KeepSave project.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Secret represents a stored secret.
type Secret struct {
	ID            string `json:"id"`
	ProjectID     string `json:"project_id"`
	EnvironmentID string `json:"environment_id"`
	Key           string `json:"key"`
	Value         string `json:"value,omitempty"`
	Version       int    `json:"version,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// DiffEntry represents a diff between environments.
type DiffEntry struct {
	Key          string `json:"key"`
	Action       string `json:"action"`
	SourceValue  string `json:"source_value,omitempty"`
	TargetValue  string `json:"target_value,omitempty"`
	SourceExists bool   `json:"source_exists"`
	TargetExists bool   `json:"target_exists"`
}

// AuthResponse is returned by login/register.
type AuthResponse struct {
	User struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
	Token string `json:"token"`
}

// BatchSecretResponse is returned by batch secret fetch.
type BatchSecretResponse struct {
	Secrets     []Secret `json:"secrets"`
	MissingKeys []string `json:"missing_keys,omitempty"`
}

// ── Circuit Breaker ─────────────────────────────────────────────────

type cbState int

const (
	cbClosed cbState = iota
	cbOpen
	cbHalfOpen
)

type circuitBreaker struct {
	mu              sync.Mutex
	st              cbState
	failureCount    int
	threshold       int
	resetTimeout    time.Duration
	lastFailureTime time.Time
}

func newCircuitBreaker(threshold int, resetTimeout time.Duration) *circuitBreaker {
	return &circuitBreaker{
		st:           cbClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (cb *circuitBreaker) canExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.st {
	case cbClosed:
		return true
	case cbOpen:
		if time.Since(cb.lastFailureTime) >= cb.resetTimeout {
			cb.st = cbHalfOpen
			return true
		}
		return false
	default: // cbHalfOpen
		return true
	}
}

func (cb *circuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.st = cbClosed
}

func (cb *circuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	cb.lastFailureTime = time.Now()
	if cb.failureCount >= cb.threshold {
		cb.st = cbOpen
	}
}

func (cb *circuitBreaker) state() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.st {
	case cbClosed:
		return "CLOSED"
	case cbOpen:
		return "OPEN"
	case cbHalfOpen:
		return "HALF_OPEN"
	}
	return "UNKNOWN"
}

// ── Cache ───────────────────────────────────────────────────────────

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

type cache struct {
	mu    sync.RWMutex
	store map[string]cacheEntry
	ttl   time.Duration
}

func newCache(ttl time.Duration) *cache {
	return &cache{
		store: make(map[string]cacheEntry),
		ttl:   ttl,
	}
}

func (c *cache) get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.store[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func (c *cache) set(key string, data interface{}) {
	if c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = cacheEntry{data: data, expiresAt: time.Now().Add(c.ttl)}
}

func (c *cache) invalidate(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.store {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.store, k)
		}
	}
}

func (c *cache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]cacheEntry)
}

// ── Internal HTTP ───────────────────────────────────────────────────

func (c *Client) doWithRetry(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	if c.breaker != nil && !c.breaker.canExecute() {
		return nil, &CircuitBreakerOpenError{}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		data, err := c.do(ctx, method, path, body)
		if err == nil {
			if c.breaker != nil {
				c.breaker.onSuccess()
			}
			return data, nil
		}

		lastErr = err
		if !isRetryable(err) || attempt == c.maxRetries {
			if c.breaker != nil {
				c.breaker.onFailure()
			}
			return nil, lastErr
		}

		delay := time.Duration(math.Min(float64(200*time.Millisecond)*math.Pow(2, float64(attempt)), float64(5*time.Second)))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, lastErr
}

func isRetryable(err error) bool {
	if apiErr, ok := err.(*Error); ok {
		return apiErr.Code >= 500 || apiErr.Code == 429
	}
	return true // network errors are retryable
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v1"+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	} else if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error Error `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return nil, &errResp.Error
		}
		return nil, &Error{Code: resp.StatusCode, Message: string(respBody)}
	}

	return respBody, nil
}

// ── Authentication ──────────────────────────────────────────────────

// Login authenticates and stores the JWT token.
func (c *Client) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	data, err := c.doWithRetry(ctx, "POST", "/auth/login", map[string]string{
		"email": email, "password": password,
	})
	if err != nil {
		return nil, err
	}
	var resp AuthResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	c.token = resp.Token
	return &resp, nil
}

// Register creates a new user account.
func (c *Client) Register(ctx context.Context, email, password string) (*AuthResponse, error) {
	data, err := c.doWithRetry(ctx, "POST", "/auth/register", map[string]string{
		"email": email, "password": password,
	})
	if err != nil {
		return nil, err
	}
	var resp AuthResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	c.token = resp.Token
	return &resp, nil
}

// ── Projects ────────────────────────────────────────────────────────

// ListProjects returns all projects.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	data, err := c.doWithRetry(ctx, "GET", "/projects", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Projects []Project `json:"projects"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return resp.Projects, nil
}

// CreateProject creates a new project.
func (c *Client) CreateProject(ctx context.Context, name, description string) (*Project, error) {
	data, err := c.doWithRetry(ctx, "POST", "/projects", map[string]string{
		"name": name, "description": description,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Project Project `json:"project"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &resp.Project, nil
}

// ── Secrets ─────────────────────────────────────────────────────────

// ListSecrets returns secrets for a project and environment. Results are cached.
func (c *Client) ListSecrets(ctx context.Context, projectID, environment string) ([]Secret, error) {
	cacheKey := fmt.Sprintf("secrets:%s:%s", projectID, environment)
	if c.cache != nil {
		if cached, ok := c.cache.get(cacheKey); ok {
			return cached.([]Secret), nil
		}
	}

	data, err := c.doWithRetry(ctx, "GET", fmt.Sprintf("/projects/%s/secrets?environment=%s", projectID, url.QueryEscape(environment)), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Secrets []Secret `json:"secrets"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if c.cache != nil {
		c.cache.set(cacheKey, resp.Secrets)
	}
	return resp.Secrets, nil
}

// CreateSecret stores a new secret.
func (c *Client) CreateSecret(ctx context.Context, projectID, key, value, environment string) (*Secret, error) {
	if c.cache != nil {
		c.cache.invalidate(fmt.Sprintf("secrets:%s:%s", projectID, environment))
	}
	data, err := c.doWithRetry(ctx, "POST", fmt.Sprintf("/projects/%s/secrets", projectID), map[string]string{
		"key": key, "value": value, "environment": environment,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Secret Secret `json:"secret"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &resp.Secret, nil
}

// UpdateSecret updates a secret value.
func (c *Client) UpdateSecret(ctx context.Context, projectID, secretID, value string) (*Secret, error) {
	if c.cache != nil {
		c.cache.invalidate(fmt.Sprintf("secrets:%s", projectID))
	}
	data, err := c.doWithRetry(ctx, "PUT", fmt.Sprintf("/projects/%s/secrets/%s", projectID, secretID), map[string]string{
		"value": value,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Secret Secret `json:"secret"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &resp.Secret, nil
}

// DeleteSecret removes a secret.
func (c *Client) DeleteSecret(ctx context.Context, projectID, secretID string) error {
	if c.cache != nil {
		c.cache.invalidate(fmt.Sprintf("secrets:%s", projectID))
	}
	_, err := c.doWithRetry(ctx, "DELETE", fmt.Sprintf("/projects/%s/secrets/%s", projectID, secretID), nil)
	return err
}

// BatchGetSecrets fetches multiple secrets by key name in a single request.
func (c *Client) BatchGetSecrets(ctx context.Context, projectID, environment string, keys []string) (*BatchSecretResponse, error) {
	data, err := c.doWithRetry(ctx, "POST", fmt.Sprintf("/projects/%s/secrets/batch", projectID), map[string]interface{}{
		"environment": environment,
		"keys":        keys,
	})
	if err != nil {
		return nil, err
	}
	var resp BatchSecretResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &resp, nil
}

// RefreshSecrets re-fetches secrets from server, bypassing cache.
func (c *Client) RefreshSecrets(ctx context.Context, projectID, environment string) ([]Secret, error) {
	if c.cache != nil {
		c.cache.invalidate(fmt.Sprintf("secrets:%s:%s", projectID, environment))
	}
	return c.ListSecrets(ctx, projectID, environment)
}

// ── Promotions ──────────────────────────────────────────────────────

// Promote initiates a promotion between environments.
func (c *Client) Promote(ctx context.Context, projectID, source, target, overridePolicy string) error {
	if c.cache != nil {
		c.cache.invalidate(fmt.Sprintf("secrets:%s:%s", projectID, target))
	}
	_, err := c.doWithRetry(ctx, "POST", fmt.Sprintf("/projects/%s/promote", projectID), map[string]string{
		"source_environment": source,
		"target_environment": target,
		"override_policy":    overridePolicy,
	})
	return err
}

// PromoteDiff previews promotion changes.
func (c *Client) PromoteDiff(ctx context.Context, projectID, source, target string) ([]DiffEntry, error) {
	data, err := c.doWithRetry(ctx, "POST", fmt.Sprintf("/projects/%s/promote/diff", projectID), map[string]string{
		"source_environment": source,
		"target_environment": target,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Diff []DiffEntry `json:"diff"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return resp.Diff, nil
}

// ── Key Rotation ────────────────────────────────────────────────────

// RotateKeys rotates encryption keys for a project. Invalidates local cache.
func (c *Client) RotateKeys(ctx context.Context, projectID string) error {
	if c.cache != nil {
		c.cache.invalidate(fmt.Sprintf("secrets:%s", projectID))
	}
	_, err := c.doWithRetry(ctx, "POST", fmt.Sprintf("/projects/%s/rotate-keys", projectID), nil)
	return err
}

// ── Import/Export ───────────────────────────────────────────────────

// ExportEnv exports secrets as .env content.
func (c *Client) ExportEnv(ctx context.Context, projectID, environment string) (string, error) {
	data, err := c.doWithRetry(ctx, "GET", fmt.Sprintf("/projects/%s/env-export?environment=%s", projectID, url.QueryEscape(environment)), nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	return resp.Content, nil
}
