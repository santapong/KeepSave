package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type MCPGatewayHandler struct {
	mcpService     *service.MCPService
	builderService *service.MCPBuilderService
	mcpRepo        *repository.MCPRepository
	secretRepo     *repository.SecretRepository
	projectRepo    *repository.ProjectRepository
	envRepo        *repository.EnvironmentRepository
	cryptoSvc      *crypto.Service
}

func NewMCPGatewayHandler(
	mcpService *service.MCPService,
	builderService *service.MCPBuilderService,
	mcpRepo *repository.MCPRepository,
	secretRepo *repository.SecretRepository,
	projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository,
	cryptoSvc *crypto.Service,
) *MCPGatewayHandler {
	return &MCPGatewayHandler{
		mcpService:     mcpService,
		builderService: builderService,
		mcpRepo:        mcpRepo,
		secretRepo:     secretRepo,
		projectRepo:    projectRepo,
		envRepo:        envRepo,
		cryptoSvc:      cryptoSvc,
	}
}

// HandleToolCall proxies an MCP tool call to the appropriate MCP server.
func (h *MCPGatewayHandler) HandleToolCall(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var req models.MCPGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, models.MCPGatewayResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &models.MCPError{Code: -32700, Message: "parse error"},
		})
		return
	}

	if req.Method != "tools/call" {
		c.JSON(http.StatusOK, models.MCPGatewayResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &models.MCPError{Code: -32601, Message: "method not found"},
		})
		return
	}

	toolName, _ := req.Params["name"].(string)
	toolArgs, _ := req.Params["arguments"].(map[string]interface{})

	if toolName == "" {
		c.JSON(http.StatusOK, models.MCPGatewayResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &models.MCPError{Code: -32602, Message: "tool name required"},
		})
		return
	}

	// Find which server handles this tool
	serversWithTools, err := h.mcpService.ListUserTools(userID)
	if err != nil {
		c.JSON(http.StatusOK, models.MCPGatewayResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &models.MCPError{Code: -32603, Message: "internal error"},
		})
		return
	}

	var targetServer *models.MCPServerWithTools
	for i, swt := range serversWithTools {
		for _, tool := range swt.Tools {
			if tool.Name == toolName {
				targetServer = &serversWithTools[i]
				break
			}
		}
		if targetServer != nil {
			break
		}
	}

	if targetServer == nil {
		c.JSON(http.StatusOK, models.MCPGatewayResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &models.MCPError{Code: -32601, Message: fmt.Sprintf("tool '%s' not found in any installed server", toolName)},
		})
		return
	}

	// Inject secrets as environment variables
	envVars, err := h.resolveSecretEnvVars(targetServer, userID)
	if err != nil {
		c.JSON(http.StatusOK, models.MCPGatewayResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &models.MCPError{Code: -32603, Message: "failed to resolve secrets"},
		})
		return
	}

	// Execute the tool call via the MCP server
	start := time.Now()
	result, err := h.executeMCPToolCall(targetServer, toolName, toolArgs, envVars)
	duration := time.Since(start)

	// Log the gateway request
	status := "success"
	if err != nil {
		status = "error"
	}
	h.mcpService.LogGatewayRequest(&models.MCPGatewayLog{
		UserID:         &userID,
		MCPServerID:    &targetServer.ID,
		ToolName:       toolName,
		RequestPayload: models.JSONMap{"arguments": toolArgs},
		ResponseStatus: status,
		DurationMs:     int(duration.Milliseconds()),
	})

	if err != nil {
		c.JSON(http.StatusOK, models.MCPGatewayResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &models.MCPError{Code: -32603, Message: err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, models.MCPGatewayResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

// ListTools returns all available tools across installed MCP servers.
func (h *MCPGatewayHandler) ListTools(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	serversWithTools, err := h.mcpService.ListUserTools(userID)
	if err != nil {
		c.JSON(http.StatusOK, models.MCPGatewayResponse{
			JSONRPC: "2.0",
			Error:   &models.MCPError{Code: -32603, Message: "internal error"},
		})
		return
	}

	var allTools []map[string]interface{}
	for _, swt := range serversWithTools {
		for _, tool := range swt.Tools {
			allTools = append(allTools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
				"server_id":   swt.ID.String(),
				"server_name": swt.Name,
			})
		}
	}

	c.JSON(http.StatusOK, models.MCPGatewayResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"tools": allTools,
		},
	})
}

// MCPConfig generates the MCP configuration JSON for the user.
func (h *MCPGatewayHandler) MCPConfig(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	installations, err := h.mcpService.ListInstallations(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	baseURL := c.Request.Host
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}

	mcpServers := map[string]interface{}{}

	// Gateway server (unified endpoint)
	mcpServers["keepsave-gateway"] = map[string]interface{}{
		"command": "npx",
		"args":    []string{"-y", "@keepsave/mcp-client", "--gateway", fmt.Sprintf("%s://%s/api/v1/mcp/gateway", scheme, baseURL)},
	}

	// Individual server configs
	for _, inst := range installations {
		if !inst.Enabled {
			continue
		}
		server, err := h.mcpRepo.GetServer(inst.MCPServerID)
		if err != nil || server.Status != "ready" {
			continue
		}
		mcpServers[server.Name] = map[string]interface{}{
			"server_id":  server.ID.String(),
			"github_url": server.GitHubURL,
			"transport":  server.Transport,
			"enabled":    inst.Enabled,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"mcpServers": mcpServers,
	})
}

// Internal helpers

func (h *MCPGatewayHandler) resolveSecretEnvVars(server *models.MCPServerWithTools, userID uuid.UUID) ([]string, error) {
	var envVars []string

	if server.EnvMappings == nil {
		return envVars, nil
	}

	// EnvMappings format: {"ENV_VAR_NAME": {"project_id": "...", "environment": "...", "secret_key": "..."}}
	for envName, mappingRaw := range server.EnvMappings {
		mapping, ok := mappingRaw.(map[string]interface{})
		if !ok {
			continue
		}

		projectIDStr, _ := mapping["project_id"].(string)
		environment, _ := mapping["environment"].(string)
		secretKey, _ := mapping["secret_key"].(string)

		if projectIDStr == "" || environment == "" || secretKey == "" {
			continue
		}

		projectID, err := uuid.Parse(projectIDStr)
		if err != nil {
			continue
		}

		// Get project DEK
		project, err := h.projectRepo.GetByID(projectID)
		if err != nil {
			continue
		}

		dek, err := h.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
		if err != nil {
			continue
		}

		// Get environment
		env, err := h.envRepo.GetByProjectAndName(projectID, environment)
		if err != nil {
			continue
		}

		// Get secret
		secret, err := h.secretRepo.GetByEnvAndKey(env.ID, secretKey)
		if err != nil {
			continue
		}

		// Decrypt secret value
		value, err := crypto.Decrypt(dek, secret.EncryptedValue, secret.ValueNonce)
		if err != nil {
			continue
		}

		envVars = append(envVars, fmt.Sprintf("%s=%s", envName, string(value)))
	}

	return envVars, nil
}

func (h *MCPGatewayHandler) executeMCPToolCall(server *models.MCPServerWithTools, toolName string, args map[string]interface{}, envVars []string) (interface{}, error) {
	buildDir := h.builderService.GetBuildDir(server.ID)

	// Build the MCP JSON-RPC request
	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}

	requestJSON, err := json.Marshal(mcpRequest)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Parse entry command
	parts := strings.Fields(server.EntryCommand)
	if len(parts) == 0 {
		return nil, fmt.Errorf("no entry command configured")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = buildDir
	cmd.Stdin = strings.NewReader(string(requestJSON))

	// Set environment variables
	cmd.Env = append(cmd.Env, envVars...)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("executing tool: %w", err)
	}

	// Parse the MCP response
	var response map[string]interface{}
	if err := json.Unmarshal(output, &response); err != nil {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": string(output)},
			},
		}, nil
	}

	if result, ok := response["result"]; ok {
		return result, nil
	}

	return response, nil
}
