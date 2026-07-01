package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// ErrBuildQueueFull is returned by EnqueueBuild/EnqueueRebuild when the bounded
// concurrent-build semaphore is saturated. Callers (the MCP handlers) surface
// this as HTTP 429 so an authenticated user cannot spawn unbounded build
// goroutines / subprocesses (DoS bound).
var ErrBuildQueueFull = errors.New("mcp build queue is full; retry later")

const (
	// defaultMaxConcurrentBuilds bounds how many BuildServer/RebuildServer
	// goroutines (each of which spawns git/npm/go/pip subprocesses) may run at
	// once. Small by default; overridable via MCP_MAX_CONCURRENT_BUILDS.
	defaultMaxConcurrentBuilds = 3
	// defaultBuildTimeout bounds a single build's total subprocess wall-time.
	// Overridable via MCP_BUILD_TIMEOUT (Go duration, e.g. "5m", "90s").
	defaultBuildTimeout = 5 * time.Minute
)

type MCPBuilderService struct {
	mcpRepo  *repository.MCPRepository
	buildDir string

	// sem bounds concurrent builds; a slot is held for the lifetime of one
	// build goroutine (DoS bound). buildTimeout caps a single build's total
	// subprocess time via exec.CommandContext.
	sem          chan struct{}
	buildTimeout time.Duration

	// baseCtx is the parent context for every build. main.go sets it to the
	// server's background/shutdown context so SIGTERM cancels in-flight builds.
	// A per-build context with buildTimeout is derived from it. Guarded by mu
	// so SetBaseContext can be called during wiring without a data race.
	mu      sync.RWMutex
	baseCtx context.Context
}

func NewMCPBuilderService(mcpRepo *repository.MCPRepository) *MCPBuilderService {
	buildDir := os.Getenv("MCP_BUILD_DIR")
	if buildDir == "" {
		buildDir = "/tmp/keepsave-mcp-builds"
	}
	os.MkdirAll(buildDir, 0755)

	maxConcurrent := defaultMaxConcurrentBuilds
	if v := os.Getenv("MCP_MAX_CONCURRENT_BUILDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConcurrent = n
		}
	}
	buildTimeout := defaultBuildTimeout
	if v := os.Getenv("MCP_BUILD_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			buildTimeout = d
		}
	}

	return &MCPBuilderService{
		mcpRepo:      mcpRepo,
		buildDir:     buildDir,
		sem:          make(chan struct{}, maxConcurrent),
		buildTimeout: buildTimeout,
		baseCtx:      context.Background(),
	}
}

// SetBaseContext binds the builder's goroutines to a parent context (typically
// the server's background/shutdown context). When that context is cancelled
// (SIGTERM), in-flight builds observe cancellation via exec.CommandContext and
// abort rather than orphaning subprocesses.
func (s *MCPBuilderService) SetBaseContext(ctx context.Context) {
	if ctx == nil {
		return
	}
	s.mu.Lock()
	s.baseCtx = ctx
	s.mu.Unlock()
}

func (s *MCPBuilderService) getBaseContext() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.baseCtx == nil {
		return context.Background()
	}
	return s.baseCtx
}

// EnqueueBuild starts a bounded, shutdown-aware build in a new goroutine. It
// returns ErrBuildQueueFull (without starting anything) when the concurrency
// limit is already reached, so the caller can respond 429. A background build's
// own errors are recorded on the server row by BuildServer, not returned here.
func (s *MCPBuilderService) EnqueueBuild(serverID uuid.UUID) error {
	return s.enqueue(serverID, false)
}

// EnqueueRebuild is EnqueueBuild for the rebuild (clean + build) path.
func (s *MCPBuilderService) EnqueueRebuild(serverID uuid.UUID) error {
	return s.enqueue(serverID, true)
}

func (s *MCPBuilderService) enqueue(serverID uuid.UUID, rebuild bool) error {
	select {
	case s.sem <- struct{}{}:
		// Acquired a slot.
	default:
		return ErrBuildQueueFull
	}
	go func() {
		defer func() { <-s.sem }()
		ctx, cancel := context.WithTimeout(s.getBaseContext(), s.buildTimeout)
		defer cancel()
		if rebuild {
			_ = s.RebuildServerCtx(ctx, serverID)
		} else {
			_ = s.BuildServerCtx(ctx, serverID)
		}
	}()
	return nil
}

// BuildServer clones the GitHub repo, detects the project type, installs
// dependencies, and discovers tools. It uses a background context with the
// configured build timeout; prefer BuildServerCtx (or EnqueueBuild) to bind the
// build to the server's shutdown context.
func (s *MCPBuilderService) BuildServer(serverID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(s.getBaseContext(), s.buildTimeout)
	defer cancel()
	return s.BuildServerCtx(ctx, serverID)
}

// BuildServerCtx is BuildServer with an explicit context. Every subprocess it
// spawns (git/npm/go/pip) runs under ctx, so a cancelled ctx (shutdown) or an
// exceeded deadline (build timeout) terminates the subprocess and fails the
// build cleanly instead of orphaning it.
func (s *MCPBuilderService) BuildServerCtx(ctx context.Context, serverID uuid.UUID) error {
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
	if err := s.gitClone(ctx, server.GitHubURL, server.GitHubBranch, cloneDir); err != nil {
		errMsg := fmt.Sprintf("Clone failed: %v", err)
		buildLog.WriteString(errMsg + "\n")
		s.mcpRepo.UpdateServerStatus(serverID, "error", buildLog.String())
		return fmt.Errorf("cloning repository: %w", err)
	}
	buildLog.WriteString("Clone successful.\n")

	// Detect project type and install dependencies
	projectType := s.detectProjectType(cloneDir)
	buildLog.WriteString(fmt.Sprintf("Detected project type: %s\n", projectType))

	if err := s.installDependencies(ctx, cloneDir, projectType, buildLog); err != nil {
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

// RebuildServer triggers a fresh build for the server using a background
// context with the configured build timeout.
func (s *MCPBuilderService) RebuildServer(serverID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(s.getBaseContext(), s.buildTimeout)
	defer cancel()
	return s.RebuildServerCtx(ctx, serverID)
}

// RebuildServerCtx is RebuildServer with an explicit context.
func (s *MCPBuilderService) RebuildServerCtx(ctx context.Context, serverID uuid.UUID) error {
	cloneDir := filepath.Join(s.buildDir, serverID.String())
	os.RemoveAll(cloneDir)
	return s.BuildServerCtx(ctx, serverID)
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

func (s *MCPBuilderService) gitClone(ctx context.Context, repoURL, branch, destDir string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, repoURL, destDir)
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

func (s *MCPBuilderService) installDependencies(ctx context.Context, dir, projectType string, buildLog *strings.Builder) error {
	var cmd *exec.Cmd
	switch projectType {
	case "nodejs":
		buildLog.WriteString("Running npm install...\n")
		cmd = exec.CommandContext(ctx, "npm", "install", "--production")
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
			buildCmd := exec.CommandContext(ctx, "npm", "run", "build")
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
		cmd = exec.CommandContext(ctx, "go", "build", "./...")
		cmd.Dir = dir
	case "python":
		buildLog.WriteString("Setting up Python environment...\n")
		// Create venv and install
		venvCmd := exec.CommandContext(ctx, "python3", "-m", "venv", filepath.Join(dir, ".venv"))
		if output, err := venvCmd.CombinedOutput(); err != nil {
			buildLog.WriteString(string(output))
			return fmt.Errorf("venv creation failed: %w", err)
		}
		pip := filepath.Join(dir, ".venv", "bin", "pip")
		reqFile := filepath.Join(dir, "requirements.txt")
		if _, err := os.Stat(reqFile); err == nil {
			cmd = exec.CommandContext(ctx, pip, "install", "-r", "requirements.txt")
			cmd.Dir = dir
		} else {
			cmd = exec.CommandContext(ctx, pip, "install", "-e", ".")
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
