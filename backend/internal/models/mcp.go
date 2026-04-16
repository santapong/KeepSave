package models

import (
	"time"

	"github.com/google/uuid"
)

// MCPServer represents a registered MCP server in the hub.
type MCPServer struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	OwnerID         uuid.UUID  `json:"owner_id"`
	GitHubURL       string     `json:"github_url"`
	GitHubBranch    string     `json:"github_branch"`
	EntryCommand    string     `json:"entry_command"`
	Transport       string     `json:"transport"` // stdio, sse, streamable-http
	IconURL         string     `json:"icon_url,omitempty"`
	Version         string     `json:"version"`
	Status          string     `json:"status"` // pending, building, ready, error
	BuildLog        string     `json:"build_log,omitempty"`
	EnvMappings     JSONMap    `json:"env_mappings"`
	ToolDefinitions JSONMap    `json:"tool_definitions"`
	LastSyncedAt    *time.Time `json:"last_synced_at,omitempty"`
	InstallCount    int        `json:"install_count"`
	IsPublic        bool       `json:"is_public"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// MCPInstallation represents a user's installation of an MCP server.
type MCPInstallation struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	MCPServerID uuid.UUID  `json:"mcp_server_id"`
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	Enabled     bool       `json:"enabled"`
	Config      JSONMap    `json:"config"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// MCPGatewayLog represents a logged MCP tool call through the gateway.
type MCPGatewayLog struct {
	ID             uuid.UUID  `json:"id"`
	UserID         *uuid.UUID `json:"user_id,omitempty"`
	MCPServerID    *uuid.UUID `json:"mcp_server_id,omitempty"`
	ToolName       string     `json:"tool_name"`
	RequestPayload JSONMap    `json:"request_payload"`
	ResponseStatus string     `json:"response_status"`
	DurationMs     int        `json:"duration_ms"`
	CreatedAt      time.Time  `json:"created_at"`
}

// MCPToolDefinition represents a tool exposed by an MCP server.
type MCPToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema JSONMap `json:"inputSchema"`
}

// MCPServerWithTools combines server info with resolved tool list.
type MCPServerWithTools struct {
	MCPServer
	Tools []MCPToolDefinition `json:"tools"`
}

// MCPGatewayRequest represents an incoming MCP tool call to the gateway.
type MCPGatewayRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// MCPGatewayResponse represents the gateway's response to an MCP call.
type MCPGatewayResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP protocol error.
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
