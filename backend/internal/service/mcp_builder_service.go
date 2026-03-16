package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type MCPBuilderService struct {
	mcpRepo  *repository.MCPRepository
	buildDir string
}

func NewMCPBuilderService(mcpRepo *repository.MCPRepository) *MCPBuilderService {
	buildDir := os.Getenv("MCP_BUILD_DIR")
	if buildDir == "" {
		buildDir = "/tmp/keepsave-mcp-builds"
	}
	os.MkdirAll(buildDir, 0755)
	return &MCPBuilderService{mcpRepo: mcpRepo, buildDir: buildDir}
}

// BuildServer clones the GitHub repo, detects the project type, installs dependencies, and discovers tools.
func (s *MCPBuilderService) BuildServer(serverID uuid.UUID) error {
	server, err := s.mcpRepo.GetServer(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	s.mcpRepo.UpdateServerStatus(serverID, "building", "Starting build...")

	cloneDir := filepath.Join(s.buildDir, serverID.String())

	// Clean up any previous build
	os.RemoveAll(cloneDir)

	// Clone the repository
	buildLog := &strings.Builder{}

	buildLog.WriteString("Cloning repository...\n")
	if err := s.gitClone(server.GitHubURL, server.GitHubBranch, cloneDir); err != nil {
		errMsg := fmt.Sprintf("Clone failed: %v", err)
		buildLog.WriteString(errMsg + "\n")
		s.mcpRepo.UpdateServerStatus(serverID, "error", buildLog.String())
		return fmt.Errorf("cloning repository: %w", err)
	}
	buildLog.WriteString("Clone successful.\n")

	// Detect project type and install dependencies
	projectType := s.detectProjectType(cloneDir)
	buildLog.WriteString(fmt.Sprintf("Detected project type: %s\n", projectType))

	if err := s.installDependencies(cloneDir, projectType, buildLog); err != nil {
		buildLog.WriteString(fmt.Sprintf("Dependency installation failed: %v\n", err))
		s.mcpRepo.UpdateServerStatus(serverID, "error", buildLog.String())
		return fmt.Errorf("installing dependencies: %w", err)
	}

	// Discover entry command if not set
	if server.EntryCommand == "" {
		entryCmd := s.discoverEntryCommand(cloneDir, projectType)
		buildLog.WriteString(fmt.Sprintf("Discovered entry command: %s\n", entryCmd))
		server.EntryCommand = entryCmd
	}

	// Discover tool definitions
	toolDefs := s.discoverTools(cloneDir, projectType)
	buildLog.WriteString(fmt.Sprintf("Discovered %d tools\n", len(toolDefs)))

	toolDefsMap := models.JSONMap{"tools": toolDefs}
	if err := s.mcpRepo.UpdateServerSync(serverID, toolDefsMap); err != nil {
		buildLog.WriteString(fmt.Sprintf("Failed to save tools: %v\n", err))
		s.mcpRepo.UpdateServerStatus(serverID, "error", buildLog.String())
		return err
	}

	buildLog.WriteString("Build completed successfully.\n")
	s.mcpRepo.UpdateServerStatus(serverID, "ready", buildLog.String())

	return nil
}

// RebuildServer triggers a fresh build for the server.
func (s *MCPBuilderService) RebuildServer(serverID uuid.UUID) error {
	cloneDir := filepath.Join(s.buildDir, serverID.String())
	os.RemoveAll(cloneDir)
	return s.BuildServer(serverID)
}

// GetBuildDir returns the build directory path for a server.
func (s *MCPBuilderService) GetBuildDir(serverID uuid.UUID) string {
	return filepath.Join(s.buildDir, serverID.String())
}

// CleanupBuild removes the build directory for a server.
func (s *MCPBuilderService) CleanupBuild(serverID uuid.UUID) {
	cloneDir := filepath.Join(s.buildDir, serverID.String())
	os.RemoveAll(cloneDir)
}

// Internal helpers

func (s *MCPBuilderService) gitClone(repoURL, branch, destDir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch, repoURL, destDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", string(output), err)
	}
	return nil
}

func (s *MCPBuilderService) detectProjectType(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "nodejs"
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		return "rust"
	}
	return "unknown"
}

func (s *MCPBuilderService) installDependencies(dir, projectType string, buildLog *strings.Builder) error {
	var cmd *exec.Cmd
	switch projectType {
	case "nodejs":
		buildLog.WriteString("Running npm install...\n")
		cmd = exec.Command("npm", "install", "--production")
		cmd.Dir = dir
		// Also try to build if there's a build script
		output, err := cmd.CombinedOutput()
		buildLog.WriteString(string(output))
		if err != nil {
			return fmt.Errorf("npm install failed: %w", err)
		}
		// Check for TypeScript build
		if _, tsErr := os.Stat(filepath.Join(dir, "tsconfig.json")); tsErr == nil {
			buildLog.WriteString("Running npm run build...\n")
			buildCmd := exec.Command("npm", "run", "build")
			buildCmd.Dir = dir
			buildOutput, buildErr := buildCmd.CombinedOutput()
			buildLog.WriteString(string(buildOutput))
			if buildErr != nil {
				buildLog.WriteString("Build step failed (may not be required).\n")
			}
		}
		return nil
	case "go":
		buildLog.WriteString("Running go build...\n")
		cmd = exec.Command("go", "build", "./...")
		cmd.Dir = dir
	case "python":
		buildLog.WriteString("Setting up Python environment...\n")
		// Create venv and install
		venvCmd := exec.Command("python3", "-m", "venv", filepath.Join(dir, ".venv"))
		if output, err := venvCmd.CombinedOutput(); err != nil {
			buildLog.WriteString(string(output))
			return fmt.Errorf("venv creation failed: %w", err)
		}
		pip := filepath.Join(dir, ".venv", "bin", "pip")
		reqFile := filepath.Join(dir, "requirements.txt")
		if _, err := os.Stat(reqFile); err == nil {
			cmd = exec.Command(pip, "install", "-r", "requirements.txt")
			cmd.Dir = dir
		} else {
			cmd = exec.Command(pip, "install", "-e", ".")
			cmd.Dir = dir
		}
	default:
		buildLog.WriteString("Unknown project type, skipping dependency installation.\n")
		return nil
	}

	if cmd != nil {
		output, err := cmd.CombinedOutput()
		buildLog.WriteString(string(output))
		if err != nil {
			return fmt.Errorf("dependency install failed: %w", err)
		}
	}
	return nil
}

func (s *MCPBuilderService) discoverEntryCommand(dir, projectType string) string {
	switch projectType {
	case "nodejs":
		// Check package.json for main or bin
		data, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err == nil {
			var pkg map[string]interface{}
			if json.Unmarshal(data, &pkg) == nil {
				// Check bin field
				if bin, ok := pkg["bin"].(map[string]interface{}); ok {
					for _, v := range bin {
						return fmt.Sprintf("node %s", v)
					}
				}
				if bin, ok := pkg["bin"].(string); ok {
					return fmt.Sprintf("node %s", bin)
				}
				if main, ok := pkg["main"].(string); ok {
					return fmt.Sprintf("node %s", main)
				}
			}
		}
		// Check common entry points
		for _, entry := range []string{"dist/index.js", "build/index.js", "index.js", "src/index.js"} {
			if _, err := os.Stat(filepath.Join(dir, entry)); err == nil {
				return fmt.Sprintf("node %s", entry)
			}
		}
		return "npx ."
	case "go":
		return "go run ."
	case "python":
		// Check for common entry points
		for _, entry := range []string{"server.py", "main.py", "app.py", "src/server.py"} {
			if _, err := os.Stat(filepath.Join(dir, entry)); err == nil {
				return fmt.Sprintf(".venv/bin/python %s", entry)
			}
		}
		return ".venv/bin/python -m server"
	default:
		return ""
	}
}

func (s *MCPBuilderService) discoverTools(dir, projectType string) []map[string]interface{} {
	var tools []map[string]interface{}

	// Try to find tool definitions by scanning source files for common MCP patterns
	switch projectType {
	case "nodejs":
		tools = s.discoverNodeJSTools(dir)
	case "python":
		tools = s.discoverPythonTools(dir)
	case "go":
		tools = s.discoverGoTools(dir)
	}

	if len(tools) == 0 {
		// Return a placeholder indicating tools need manual configuration
		tools = []map[string]interface{}{
			{
				"name":        "discover",
				"description": "Tools will be discovered at runtime via MCP protocol",
				"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
			},
		}
	}

	return tools
}

func (s *MCPBuilderService) discoverNodeJSTools(dir string) []map[string]interface{} {
	var tools []map[string]interface{}

	// Look for tool definitions in common patterns
	// Check for a tools.json or mcp.json config
	for _, configFile := range []string{"tools.json", "mcp.json", ".mcp/config.json"} {
		data, err := os.ReadFile(filepath.Join(dir, configFile))
		if err != nil {
			continue
		}
		var config map[string]interface{}
		if json.Unmarshal(data, &config) == nil {
			if toolsList, ok := config["tools"].([]interface{}); ok {
				for _, t := range toolsList {
					if toolMap, ok := t.(map[string]interface{}); ok {
						tools = append(tools, toolMap)
					}
				}
			}
		}
	}

	return tools
}

func (s *MCPBuilderService) discoverPythonTools(dir string) []map[string]interface{} {
	return s.discoverNodeJSTools(dir) // Same config file check
}

func (s *MCPBuilderService) discoverGoTools(dir string) []map[string]interface{} {
	return s.discoverNodeJSTools(dir) // Same config file check
}
