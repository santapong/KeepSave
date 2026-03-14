// Package keepsave provides a Go SDK for the KeepSave API.
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
	"net/http"
	"time"
)

// Client is a KeepSave API client.
type Client struct {
	baseURL    string
	token      string
	apiKey     string
	httpClient *http.Client
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

// NewClient creates a new KeepSave API client.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Error represents an API error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("keepsave: %d %s", e.Code, e.Message)
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
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
	Token string `json:"token"`
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

// Login authenticates and stores the JWT token.
func (c *Client) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	data, err := c.do(ctx, "POST", "/auth/login", map[string]string{
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
	data, err := c.do(ctx, "POST", "/auth/register", map[string]string{
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

// ListProjects returns all projects.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	data, err := c.do(ctx, "GET", "/projects", nil)
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
	data, err := c.do(ctx, "POST", "/projects", map[string]string{
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

// ListSecrets returns secrets for a project and environment.
func (c *Client) ListSecrets(ctx context.Context, projectID, environment string) ([]Secret, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/projects/%s/secrets?environment=%s", projectID, environment), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Secrets []Secret `json:"secrets"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return resp.Secrets, nil
}

// CreateSecret stores a new secret.
func (c *Client) CreateSecret(ctx context.Context, projectID, key, value, environment string) (*Secret, error) {
	data, err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/secrets", projectID), map[string]string{
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
	data, err := c.do(ctx, "PUT", fmt.Sprintf("/projects/%s/secrets/%s", projectID, secretID), map[string]string{
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
	_, err := c.do(ctx, "DELETE", fmt.Sprintf("/projects/%s/secrets/%s", projectID, secretID), nil)
	return err
}

// Promote initiates a promotion between environments.
func (c *Client) Promote(ctx context.Context, projectID, source, target, overridePolicy string) error {
	_, err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/promote", projectID), map[string]string{
		"source_environment": source,
		"target_environment": target,
		"override_policy":    overridePolicy,
	})
	return err
}

// PromoteDiff previews promotion changes.
func (c *Client) PromoteDiff(ctx context.Context, projectID, source, target string) ([]DiffEntry, error) {
	data, err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/promote/diff", projectID), map[string]string{
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

// RotateKeys rotates encryption keys for a project.
func (c *Client) RotateKeys(ctx context.Context, projectID string) error {
	_, err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/rotate-keys", projectID), nil)
	return err
}

// ExportEnv exports secrets as .env content.
func (c *Client) ExportEnv(ctx context.Context, projectID, environment string) (string, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/projects/%s/env-export?environment=%s", projectID, environment), nil)
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
