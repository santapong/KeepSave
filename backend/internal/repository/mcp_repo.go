package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type MCPRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewMCPRepository(db *sql.DB, dialect Dialect) *MCPRepository {
	return &MCPRepository{db: db, dialect: dialect}
}

// MCP Server CRUD

func (r *MCPRepository) CreateServer(server *models.MCPServer) error {
	id := uuid.New()
	envMappings, _ := json.Marshal(server.EnvMappings)
	toolDefs, _ := json.Marshal(server.ToolDefinitions)

	query := Q(r.dialect, `INSERT INTO mcp_servers (id, name, description, owner_id, github_url, github_branch, entry_command, transport, icon_url, version, status, env_mappings, tool_definitions, is_public)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`)
	_, err := r.db.Exec(query, id, server.Name, server.Description, server.OwnerID, server.GitHubURL, server.GitHubBranch,
		server.EntryCommand, server.Transport, server.IconURL, server.Version, server.Status,
		string(envMappings), string(toolDefs), server.IsPublic)
	if err != nil {
		return fmt.Errorf("creating mcp server: %w", err)
	}
	server.ID = id
	server.CreatedAt = time.Now()
	server.UpdatedAt = time.Now()
	return nil
}

func (r *MCPRepository) GetServer(id uuid.UUID) (*models.MCPServer, error) {
	server := &models.MCPServer{}
	query := Q(r.dialect, `SELECT id, name, description, owner_id, github_url, github_branch, entry_command, transport, icon_url, version, status, build_log, env_mappings, tool_definitions, last_synced_at, install_count, is_public, created_at, updated_at
		FROM mcp_servers WHERE id = $1`)
	err := r.db.QueryRow(query, id).Scan(
		&server.ID, &server.Name, &server.Description, &server.OwnerID, &server.GitHubURL, &server.GitHubBranch,
		&server.EntryCommand, &server.Transport, &server.IconURL, &server.Version, &server.Status, &server.BuildLog,
		&server.EnvMappings, &server.ToolDefinitions, &server.LastSyncedAt, &server.InstallCount, &server.IsPublic,
		&server.CreatedAt, &server.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting mcp server: %w", err)
	}
	return server, nil
}

func (r *MCPRepository) ListPublicServers() ([]models.MCPServer, error) {
	query := Q(r.dialect, `SELECT id, name, description, owner_id, github_url, github_branch, entry_command, transport, icon_url, version, status, build_log, env_mappings, tool_definitions, last_synced_at, install_count, is_public, created_at, updated_at
		FROM mcp_servers WHERE is_public = TRUE AND status = 'ready' ORDER BY install_count DESC, created_at DESC`)
	return r.scanServers(query)
}

func (r *MCPRepository) ListServersByOwner(ownerID uuid.UUID) ([]models.MCPServer, error) {
	query := Q(r.dialect, `SELECT id, name, description, owner_id, github_url, github_branch, entry_command, transport, icon_url, version, status, build_log, env_mappings, tool_definitions, last_synced_at, install_count, is_public, created_at, updated_at
		FROM mcp_servers WHERE owner_id = $1 ORDER BY created_at DESC`)
	return r.scanServersWithArg(query, ownerID)
}

func (r *MCPRepository) UpdateServerStatus(id uuid.UUID, status, buildLog string) error {
	query := Q(r.dialect, `UPDATE mcp_servers SET status = $1, build_log = $2, updated_at = NOW() WHERE id = $3`)
	_, err := r.db.Exec(query, status, buildLog, id)
	return err
}

func (r *MCPRepository) UpdateServer(server *models.MCPServer) error {
	envMappings, _ := json.Marshal(server.EnvMappings)
	toolDefs, _ := json.Marshal(server.ToolDefinitions)

	query := Q(r.dialect, `UPDATE mcp_servers SET name = $1, description = $2, github_branch = $3, entry_command = $4, transport = $5, icon_url = $6, version = $7, env_mappings = $8, tool_definitions = $9, is_public = $10, updated_at = NOW()
		WHERE id = $11 AND owner_id = $12`)
	result, err := r.db.Exec(query, server.Name, server.Description, server.GitHubBranch, server.EntryCommand, server.Transport,
		server.IconURL, server.Version, string(envMappings), string(toolDefs), server.IsPublic, server.ID, server.OwnerID)
	if err != nil {
		return fmt.Errorf("updating mcp server: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("mcp server not found")
	}
	return nil
}

func (r *MCPRepository) UpdateServerSync(id uuid.UUID, toolDefs models.JSONMap) error {
	toolDefsJSON, _ := json.Marshal(toolDefs)
	now := time.Now()
	query := Q(r.dialect, `UPDATE mcp_servers SET tool_definitions = $1, last_synced_at = $2, status = 'ready', updated_at = $3 WHERE id = $4`)
	_, err := r.db.Exec(query, string(toolDefsJSON), now, now, id)
	return err
}

func (r *MCPRepository) DeleteServer(id uuid.UUID, ownerID uuid.UUID) error {
	query := Q(r.dialect, `DELETE FROM mcp_servers WHERE id = $1 AND owner_id = $2`)
	result, err := r.db.Exec(query, id, ownerID)
	if err != nil {
		return fmt.Errorf("deleting mcp server: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("mcp server not found")
	}
	return nil
}

func (r *MCPRepository) IncrementInstallCount(id uuid.UUID) error {
	query := Q(r.dialect, `UPDATE mcp_servers SET install_count = install_count + 1 WHERE id = $1`)
	_, err := r.db.Exec(query, id)
	return err
}

// Installation operations

func (r *MCPRepository) CreateInstallation(inst *models.MCPInstallation) error {
	id := uuid.New()
	configJSON, _ := json.Marshal(inst.Config)
	query := Q(r.dialect, `INSERT INTO mcp_installations (id, user_id, mcp_server_id, project_id, enabled, config)
		VALUES ($1, $2, $3, $4, $5, $6)`)
	_, err := r.db.Exec(query, id, inst.UserID, inst.MCPServerID, inst.ProjectID, inst.Enabled, string(configJSON))
	if err != nil {
		return fmt.Errorf("creating mcp installation: %w", err)
	}
	inst.ID = id
	inst.CreatedAt = time.Now()
	return nil
}

func (r *MCPRepository) ListInstallationsByUser(userID uuid.UUID) ([]models.MCPInstallation, error) {
	query := Q(r.dialect, `SELECT id, user_id, mcp_server_id, project_id, enabled, config, created_at, updated_at
		FROM mcp_installations WHERE user_id = $1 ORDER BY created_at DESC`)
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing installations: %w", err)
	}
	defer rows.Close()

	var installations []models.MCPInstallation
	for rows.Next() {
		var inst models.MCPInstallation
		if err := rows.Scan(&inst.ID, &inst.UserID, &inst.MCPServerID, &inst.ProjectID, &inst.Enabled, &inst.Config, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning installation: %w", err)
		}
		installations = append(installations, inst)
	}
	return installations, nil
}

func (r *MCPRepository) GetInstallation(userID, mcpServerID uuid.UUID) (*models.MCPInstallation, error) {
	inst := &models.MCPInstallation{}
	query := Q(r.dialect, `SELECT id, user_id, mcp_server_id, project_id, enabled, config, created_at, updated_at
		FROM mcp_installations WHERE user_id = $1 AND mcp_server_id = $2`)
	err := r.db.QueryRow(query, userID, mcpServerID).Scan(
		&inst.ID, &inst.UserID, &inst.MCPServerID, &inst.ProjectID, &inst.Enabled, &inst.Config, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting installation: %w", err)
	}
	return inst, nil
}

func (r *MCPRepository) UpdateInstallation(id uuid.UUID, enabled bool, config models.JSONMap) error {
	configJSON, _ := json.Marshal(config)
	query := Q(r.dialect, `UPDATE mcp_installations SET enabled = $1, config = $2, updated_at = NOW() WHERE id = $3`)
	_, err := r.db.Exec(query, enabled, string(configJSON), id)
	return err
}

func (r *MCPRepository) DeleteInstallation(id uuid.UUID, userID uuid.UUID) error {
	query := Q(r.dialect, `DELETE FROM mcp_installations WHERE id = $1 AND user_id = $2`)
	result, err := r.db.Exec(query, id, userID)
	if err != nil {
		return fmt.Errorf("deleting installation: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("installation not found")
	}
	return nil
}

func (r *MCPRepository) ListEnabledInstallationsForUser(userID uuid.UUID) ([]models.MCPInstallation, error) {
	query := Q(r.dialect, `SELECT i.id, i.user_id, i.mcp_server_id, i.project_id, i.enabled, i.config, i.created_at, i.updated_at
		FROM mcp_installations i
		INNER JOIN mcp_servers s ON s.id = i.mcp_server_id
		WHERE i.user_id = $1 AND i.enabled = TRUE AND s.status = 'ready'
		ORDER BY i.created_at DESC`)
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing enabled installations: %w", err)
	}
	defer rows.Close()

	var installations []models.MCPInstallation
	for rows.Next() {
		var inst models.MCPInstallation
		if err := rows.Scan(&inst.ID, &inst.UserID, &inst.MCPServerID, &inst.ProjectID, &inst.Enabled, &inst.Config, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning installation: %w", err)
		}
		installations = append(installations, inst)
	}
	return installations, nil
}

// Gateway log operations

func (r *MCPRepository) LogGatewayRequest(log *models.MCPGatewayLog) error {
	id := uuid.New()
	payloadJSON, _ := json.Marshal(log.RequestPayload)
	query := Q(r.dialect, `INSERT INTO mcp_gateway_log (id, user_id, mcp_server_id, tool_name, request_payload, response_status, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`)
	_, err := r.db.Exec(query, id, log.UserID, log.MCPServerID, log.ToolName, string(payloadJSON), log.ResponseStatus, log.DurationMs)
	return err
}

func (r *MCPRepository) GetGatewayStats(userID uuid.UUID) ([]map[string]interface{}, error) {
	query := Q(r.dialect, `SELECT s.name, COUNT(*) as calls, AVG(g.duration_ms) as avg_duration
		FROM mcp_gateway_log g
		INNER JOIN mcp_servers s ON s.id = g.mcp_server_id
		WHERE g.user_id = $1
		GROUP BY s.name
		ORDER BY calls DESC
		LIMIT 20`)
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("getting gateway stats: %w", err)
	}
	defer rows.Close()

	var stats []map[string]interface{}
	for rows.Next() {
		var name string
		var calls int
		var avgDuration float64
		if err := rows.Scan(&name, &calls, &avgDuration); err != nil {
			return nil, err
		}
		stats = append(stats, map[string]interface{}{
			"server_name":  name,
			"total_calls":  calls,
			"avg_duration": avgDuration,
		})
	}
	return stats, nil
}

// Internal scan helpers

func (r *MCPRepository) scanServers(query string) ([]models.MCPServer, error) {
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying mcp servers: %w", err)
	}
	defer rows.Close()
	return r.scanServerRows(rows)
}

func (r *MCPRepository) scanServersWithArg(query string, arg interface{}) ([]models.MCPServer, error) {
	rows, err := r.db.Query(query, arg)
	if err != nil {
		return nil, fmt.Errorf("querying mcp servers: %w", err)
	}
	defer rows.Close()
	return r.scanServerRows(rows)
}

func (r *MCPRepository) scanServerRows(rows *sql.Rows) ([]models.MCPServer, error) {
	var servers []models.MCPServer
	for rows.Next() {
		var s models.MCPServer
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Description, &s.OwnerID, &s.GitHubURL, &s.GitHubBranch,
			&s.EntryCommand, &s.Transport, &s.IconURL, &s.Version, &s.Status, &s.BuildLog,
			&s.EnvMappings, &s.ToolDefinitions, &s.LastSyncedAt, &s.InstallCount, &s.IsPublic,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning mcp server: %w", err)
		}
		servers = append(servers, s)
	}
	return servers, nil
}
