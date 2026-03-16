package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type MCPService struct {
	mcpRepo     *repository.MCPRepository
	secretRepo  *repository.SecretRepository
	projectRepo *repository.ProjectRepository
	envRepo     *repository.EnvironmentRepository
}

func NewMCPService(mcpRepo *repository.MCPRepository, secretRepo *repository.SecretRepository, projectRepo *repository.ProjectRepository, envRepo *repository.EnvironmentRepository) *MCPService {
	return &MCPService{
		mcpRepo:     mcpRepo,
		secretRepo:  secretRepo,
		projectRepo: projectRepo,
		envRepo:     envRepo,
	}
}

// Server Management

func (s *MCPService) RegisterServer(name, description string, ownerID uuid.UUID, githubURL, githubBranch, entryCommand, transport, iconURL, version string, envMappings models.JSONMap, isPublic bool) (*models.MCPServer, error) {
	if transport == "" {
		transport = "stdio"
	}
	if githubBranch == "" {
		githubBranch = "main"
	}
	if version == "" {
		version = "1.0.0"
	}

	server := &models.MCPServer{
		Name:         name,
		Description:  description,
		OwnerID:      ownerID,
		GitHubURL:    githubURL,
		GitHubBranch: githubBranch,
		EntryCommand: entryCommand,
		Transport:    transport,
		IconURL:      iconURL,
		Version:      version,
		Status:       "pending",
		EnvMappings:  envMappings,
		IsPublic:     isPublic,
	}

	if err := s.mcpRepo.CreateServer(server); err != nil {
		return nil, fmt.Errorf("creating mcp server: %w", err)
	}

	return server, nil
}

func (s *MCPService) GetServer(id uuid.UUID) (*models.MCPServer, error) {
	return s.mcpRepo.GetServer(id)
}

func (s *MCPService) ListPublicServers() ([]models.MCPServer, error) {
	return s.mcpRepo.ListPublicServers()
}

func (s *MCPService) ListMyServers(ownerID uuid.UUID) ([]models.MCPServer, error) {
	return s.mcpRepo.ListServersByOwner(ownerID)
}

func (s *MCPService) UpdateServer(server *models.MCPServer) error {
	return s.mcpRepo.UpdateServer(server)
}

func (s *MCPService) DeleteServer(id, ownerID uuid.UUID) error {
	return s.mcpRepo.DeleteServer(id, ownerID)
}

func (s *MCPService) UpdateServerStatus(id uuid.UUID, status, buildLog string) error {
	return s.mcpRepo.UpdateServerStatus(id, status, buildLog)
}

func (s *MCPService) SyncServerTools(id uuid.UUID, toolDefs models.JSONMap) error {
	return s.mcpRepo.UpdateServerSync(id, toolDefs)
}

// Installation Management

func (s *MCPService) InstallServer(userID, mcpServerID uuid.UUID, projectID *uuid.UUID, config models.JSONMap) (*models.MCPInstallation, error) {
	// Verify server exists and is ready
	server, err := s.mcpRepo.GetServer(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}
	if server.Status != "ready" && server.Status != "pending" {
		return nil, fmt.Errorf("server is not available (status: %s)", server.Status)
	}

	inst := &models.MCPInstallation{
		UserID:      userID,
		MCPServerID: mcpServerID,
		ProjectID:   projectID,
		Enabled:     true,
		Config:      config,
	}

	if err := s.mcpRepo.CreateInstallation(inst); err != nil {
		return nil, fmt.Errorf("creating installation: %w", err)
	}

	s.mcpRepo.IncrementInstallCount(mcpServerID)

	return inst, nil
}

func (s *MCPService) ListInstallations(userID uuid.UUID) ([]models.MCPInstallation, error) {
	return s.mcpRepo.ListInstallationsByUser(userID)
}

func (s *MCPService) UpdateInstallation(id uuid.UUID, enabled bool, config models.JSONMap) error {
	return s.mcpRepo.UpdateInstallation(id, enabled, config)
}

func (s *MCPService) UninstallServer(id, userID uuid.UUID) error {
	return s.mcpRepo.DeleteInstallation(id, userID)
}

// Gateway Operations

func (s *MCPService) ListUserTools(userID uuid.UUID) ([]models.MCPServerWithTools, error) {
	installations, err := s.mcpRepo.ListEnabledInstallationsForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("listing installations: %w", err)
	}

	var result []models.MCPServerWithTools
	for _, inst := range installations {
		server, err := s.mcpRepo.GetServer(inst.MCPServerID)
		if err != nil {
			continue
		}

		swt := models.MCPServerWithTools{
			MCPServer: *server,
		}

		// Parse tool definitions from server
		if toolsRaw, ok := server.ToolDefinitions["tools"]; ok {
			if toolsSlice, ok := toolsRaw.([]interface{}); ok {
				for _, t := range toolsSlice {
					if toolMap, ok := t.(map[string]interface{}); ok {
						tool := models.MCPToolDefinition{
							Name:        fmt.Sprintf("%v", toolMap["name"]),
							Description: fmt.Sprintf("%v", toolMap["description"]),
						}
						if schema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
							tool.InputSchema = schema
						}
						swt.Tools = append(swt.Tools, tool)
					}
				}
			}
		}

		result = append(result, swt)
	}

	return result, nil
}

func (s *MCPService) LogGatewayRequest(log *models.MCPGatewayLog) error {
	return s.mcpRepo.LogGatewayRequest(log)
}

func (s *MCPService) GetGatewayStats(userID uuid.UUID) ([]map[string]interface{}, error) {
	return s.mcpRepo.GetGatewayStats(userID)
}
