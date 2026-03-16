export interface OAuthClient {
  id: string;
  client_id: string;
  name: string;
  description: string;
  owner_id: string;
  redirect_uris: string[];
  scopes: string[];
  grant_types: string[];
  logo_url: string;
  homepage_url: string;
  is_public: boolean;
  created_at: string;
  updated_at: string;
}

export interface MCPServer {
  id: string;
  name: string;
  description: string;
  owner_id: string;
  github_url: string;
  github_branch: string;
  entry_command: string;
  transport: string;
  icon_url: string;
  version: string;
  status: 'pending' | 'building' | 'ready' | 'error';
  build_log: string;
  env_mappings: Record<string, unknown>;
  tool_definitions: Record<string, unknown>;
  last_synced_at: string | null;
  install_count: number;
  is_public: boolean;
  created_at: string;
  updated_at: string;
}

export interface MCPInstallation {
  id: string;
  user_id: string;
  mcp_server_id: string;
  project_id: string | null;
  enabled: boolean;
  config: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface MCPToolDefinition {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
  server_id?: string;
  server_name?: string;
}

export interface MCPServerWithTools extends MCPServer {
  tools: MCPToolDefinition[];
}

export interface GatewayStats {
  server_name: string;
  total_calls: number;
  avg_duration: number;
}
