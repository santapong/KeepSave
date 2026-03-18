import type {
  AuthResponse,
  Project,
  Secret,
  APIKey,
  AuditEntry,
  PromotionRequest,
  DiffEntry,
  Organization,
  OrgMember,
  SecretTemplate,
  DependencyNode,
  ImportResult,
  DashboardApplication,
} from '../types';
import type {
  OAuthClient,
  MCPServer,
  MCPInstallation,
  MCPServerWithTools,
  GatewayStats,
} from '../types/mcp';

const BASE_URL = '/api/v1';

function getToken(): string | null {
  return localStorage.getItem('keepsave_token');
}

export function setToken(token: string): void {
  localStorage.setItem('keepsave_token', token);
}

export function clearToken(): void {
  localStorage.removeItem('keepsave_token');
}

export function isAuthenticated(): boolean {
  return !!getToken();
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  };

  const token = getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const data = await response.json();

  if (!response.ok) {
    const errorMessage =
      typeof data.error === 'object' && data.error !== null
        ? data.error.message
        : data.error;
    throw new Error(errorMessage || `Request failed with status ${response.status}`);
  }

  return data;
}

// Auth
export async function register(email: string, password: string): Promise<AuthResponse> {
  return request<AuthResponse>('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function login(email: string, password: string): Promise<AuthResponse> {
  return request<AuthResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

// Projects
export async function listProjects(): Promise<Project[]> {
  const data = await request<{ projects: Project[] }>('/projects');
  return data.projects;
}

export async function getProject(id: string): Promise<Project> {
  const data = await request<{ project: Project }>(`/projects/${id}`);
  return data.project;
}

export async function createProject(name: string, description: string): Promise<Project> {
  const data = await request<{ project: Project }>('/projects', {
    method: 'POST',
    body: JSON.stringify({ name, description }),
  });
  return data.project;
}

export async function updateProject(id: string, name: string, description: string): Promise<Project> {
  const data = await request<{ project: Project }>(`/projects/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, description }),
  });
  return data.project;
}

export async function deleteProject(id: string): Promise<void> {
  await request(`/projects/${id}`, { method: 'DELETE' });
}

// Secrets
export async function listSecrets(projectId: string, environment: string): Promise<Secret[]> {
  const data = await request<{ secrets: Secret[] }>(
    `/projects/${projectId}/secrets?environment=${environment}`
  );
  return data.secrets;
}

export async function createSecret(
  projectId: string,
  key: string,
  value: string,
  environment: string
): Promise<Secret> {
  const data = await request<{ secret: Secret }>(`/projects/${projectId}/secrets`, {
    method: 'POST',
    body: JSON.stringify({ key, value, environment }),
  });
  return data.secret;
}

export async function updateSecret(
  projectId: string,
  secretId: string,
  value: string
): Promise<Secret> {
  const data = await request<{ secret: Secret }>(
    `/projects/${projectId}/secrets/${secretId}`,
    {
      method: 'PUT',
      body: JSON.stringify({ value }),
    }
  );
  return data.secret;
}

export async function deleteSecret(projectId: string, secretId: string): Promise<void> {
  await request(`/projects/${projectId}/secrets/${secretId}`, { method: 'DELETE' });
}

// Promotions
export async function promoteDiff(
  projectId: string,
  sourceEnvironment: string,
  targetEnvironment: string,
  keys?: string[]
): Promise<DiffEntry[]> {
  const data = await request<{ diff: DiffEntry[] }>(`/projects/${projectId}/promote/diff`, {
    method: 'POST',
    body: JSON.stringify({
      source_environment: sourceEnvironment,
      target_environment: targetEnvironment,
      keys,
    }),
  });
  return data.diff;
}

export async function promote(
  projectId: string,
  sourceEnvironment: string,
  targetEnvironment: string,
  overridePolicy: string,
  keys?: string[],
  notes?: string
): Promise<PromotionRequest> {
  const data = await request<{ promotion: PromotionRequest }>(`/projects/${projectId}/promote`, {
    method: 'POST',
    body: JSON.stringify({
      source_environment: sourceEnvironment,
      target_environment: targetEnvironment,
      override_policy: overridePolicy,
      keys,
      notes,
    }),
  });
  return data.promotion;
}

export async function listPromotions(projectId: string): Promise<PromotionRequest[]> {
  const data = await request<{ promotions: PromotionRequest[] }>(
    `/projects/${projectId}/promotions`
  );
  return data.promotions;
}

export async function approvePromotion(
  projectId: string,
  promotionId: string
): Promise<PromotionRequest> {
  const data = await request<{ promotion: PromotionRequest }>(
    `/projects/${projectId}/promotions/${promotionId}/approve`,
    { method: 'POST' }
  );
  return data.promotion;
}

export async function rejectPromotion(
  projectId: string,
  promotionId: string
): Promise<PromotionRequest> {
  const data = await request<{ promotion: PromotionRequest }>(
    `/projects/${projectId}/promotions/${promotionId}/reject`,
    { method: 'POST' }
  );
  return data.promotion;
}

export async function rollbackPromotion(
  projectId: string,
  promotionId: string
): Promise<void> {
  await request(`/projects/${projectId}/promotions/${promotionId}/rollback`, {
    method: 'POST',
  });
}

// Audit Log
export async function listAuditLog(projectId: string, limit = 50): Promise<AuditEntry[]> {
  const data = await request<{ audit_log: AuditEntry[] }>(
    `/projects/${projectId}/audit-log?limit=${limit}`
  );
  return data.audit_log;
}

// API Keys
export async function listAPIKeys(): Promise<APIKey[]> {
  const data = await request<{ api_keys: APIKey[] }>('/api-keys');
  return data.api_keys;
}

export async function createAPIKey(
  name: string,
  projectId: string,
  scopes: string[],
  environment?: string
): Promise<{ api_key: APIKey; raw_key: string }> {
  return request('/api-keys', {
    method: 'POST',
    body: JSON.stringify({
      name,
      project_id: projectId,
      scopes,
      environment,
    }),
  });
}

export async function deleteAPIKey(id: string): Promise<void> {
  await request(`/api-keys/${id}`, { method: 'DELETE' });
}

// User lookup
export async function lookupUserByEmail(email: string): Promise<{ id: string; email: string }> {
  const data = await request<{ user: { id: string; email: string } }>(`/users/lookup?email=${encodeURIComponent(email)}`);
  return data.user;
}

// Organizations
export async function listOrganizations(): Promise<Organization[]> {
  const data = await request<{ organizations: Organization[] }>('/organizations');
  return data.organizations || [];
}

export async function createOrganization(name: string): Promise<Organization> {
  const data = await request<{ organization: Organization }>('/organizations', {
    method: 'POST',
    body: JSON.stringify({ name }),
  });
  return data.organization;
}

export async function getOrganization(orgId: string): Promise<Organization> {
  const data = await request<{ organization: Organization }>(`/organizations/${orgId}`);
  return data.organization;
}

export async function updateOrganization(orgId: string, name: string): Promise<Organization> {
  const data = await request<{ organization: Organization }>(`/organizations/${orgId}`, {
    method: 'PUT',
    body: JSON.stringify({ name }),
  });
  return data.organization;
}

export async function deleteOrganization(orgId: string): Promise<void> {
  await request(`/organizations/${orgId}`, { method: 'DELETE' });
}

export async function listOrgMembers(orgId: string): Promise<OrgMember[]> {
  const data = await request<{ members: OrgMember[] }>(`/organizations/${orgId}/members`);
  return data.members || [];
}

export async function addOrgMember(orgId: string, userId: string, role: string): Promise<OrgMember> {
  const data = await request<{ member: OrgMember }>(`/organizations/${orgId}/members`, {
    method: 'POST',
    body: JSON.stringify({ user_id: userId, role }),
  });
  return data.member;
}

export async function updateOrgMemberRole(orgId: string, userId: string, role: string): Promise<OrgMember> {
  const data = await request<{ member: OrgMember }>(`/organizations/${orgId}/members/${userId}`, {
    method: 'PUT',
    body: JSON.stringify({ role }),
  });
  return data.member;
}

export async function removeOrgMember(orgId: string, userId: string): Promise<void> {
  await request(`/organizations/${orgId}/members/${userId}`, { method: 'DELETE' });
}

export async function assignProjectToOrg(orgId: string, projectId: string): Promise<void> {
  await request(`/organizations/${orgId}/projects`, {
    method: 'POST',
    body: JSON.stringify({ project_id: projectId }),
  });
}

export async function listOrgProjects(orgId: string): Promise<Project[]> {
  const data = await request<{ projects: Project[] }>(`/organizations/${orgId}/projects`);
  return data.projects || [];
}

// Templates
export async function listTemplates(organizationId?: string): Promise<SecretTemplate[]> {
  const query = organizationId ? `?organization_id=${organizationId}` : '';
  const data = await request<{ templates: SecretTemplate[] }>(`/templates${query}`);
  return data.templates || [];
}

export async function listBuiltinTemplates(): Promise<SecretTemplate[]> {
  const data = await request<{ templates: SecretTemplate[] }>('/templates/builtin');
  return data.templates || [];
}

export async function createTemplate(
  name: string,
  description: string,
  stack: string,
  keys: Record<string, unknown>,
  organizationId?: string,
  isGlobal?: boolean
): Promise<SecretTemplate> {
  const data = await request<{ template: SecretTemplate }>('/templates', {
    method: 'POST',
    body: JSON.stringify({
      name,
      description,
      stack,
      keys,
      organization_id: organizationId || '',
      is_global: isGlobal || false,
    }),
  });
  return data.template;
}

export async function deleteTemplate(templateId: string): Promise<void> {
  await request(`/templates/${templateId}`, { method: 'DELETE' });
}

export async function applyTemplate(
  templateId: string,
  projectId: string,
  environment: string
): Promise<Secret[]> {
  const data = await request<{ secrets: Secret[] }>(`/templates/${templateId}/apply`, {
    method: 'POST',
    body: JSON.stringify({ project_id: projectId, environment }),
  });
  return data.secrets;
}

// Import/Export .env
export async function exportEnv(projectId: string, environment: string): Promise<string> {
  const data = await request<{ content: string }>(
    `/projects/${projectId}/env-export?environment=${environment}`
  );
  return data.content;
}

export async function importEnv(
  projectId: string,
  environment: string,
  content: string,
  overwrite: boolean
): Promise<ImportResult> {
  const data = await request<{ result: ImportResult }>(`/projects/${projectId}/env-import`, {
    method: 'POST',
    body: JSON.stringify({ environment, content, overwrite }),
  });
  return data.result;
}

// Phase 7: Admin Dashboard & Observability
export async function getAdminDashboard(): Promise<Record<string, unknown>> {
  return request<Record<string, unknown>>('/admin/dashboard');
}

export async function getTraces(): Promise<Record<string, unknown>[]> {
  const data = await request<{ spans: Record<string, unknown>[] }>('/admin/traces');
  return data.spans || [];
}

// Phase 9: Enterprise - SSO
export async function configureSSOProvider(
  orgId: string,
  provider: string,
  issuerUrl: string,
  clientId: string,
  clientSecret: string
): Promise<Record<string, unknown>> {
  const data = await request<{ sso_config: Record<string, unknown> }>(`/organizations/${orgId}/sso`, {
    method: 'POST',
    body: JSON.stringify({ provider, issuer_url: issuerUrl, client_id: clientId, client_secret: clientSecret }),
  });
  return data.sso_config;
}

export async function listSSOConfigs(orgId: string): Promise<Record<string, unknown>[]> {
  const data = await request<{ sso_configs: Record<string, unknown>[] }>(`/organizations/${orgId}/sso`);
  return data.sso_configs || [];
}

// Phase 9: Enterprise - Compliance
export async function generateComplianceReport(orgId: string, reportType: string): Promise<Record<string, unknown>> {
  const data = await request<{ report: Record<string, unknown> }>(`/organizations/${orgId}/compliance`, {
    method: 'POST',
    body: JSON.stringify({ report_type: reportType }),
  });
  return data.report;
}

export async function listComplianceReports(orgId: string): Promise<Record<string, unknown>[]> {
  const data = await request<{ reports: Record<string, unknown>[] }>(`/organizations/${orgId}/compliance`);
  return data.reports || [];
}

// Phase 9: Enterprise - Backups
export async function createBackup(projectId: string, type = 'full'): Promise<Record<string, unknown>> {
  const data = await request<{ backup: Record<string, unknown> }>(`/projects/${projectId}/backups`, {
    method: 'POST',
    body: JSON.stringify({ type }),
  });
  return data.backup;
}

export async function listBackups(projectId: string): Promise<Record<string, unknown>[]> {
  const data = await request<{ backups: Record<string, unknown>[] }>(`/projects/${projectId}/backups`);
  return data.backups || [];
}

// Phase 9: Enterprise - Secret Policies
export async function getSecretPolicy(projectId: string): Promise<Record<string, unknown>> {
  const data = await request<{ policy: Record<string, unknown> }>(`/projects/${projectId}/policy`);
  return data.policy;
}

export async function setSecretPolicy(
  projectId: string,
  maxAgeDays: number,
  reminderDays: number,
  requireRotation: boolean
): Promise<Record<string, unknown>> {
  const data = await request<{ policy: Record<string, unknown> }>(`/projects/${projectId}/policy`, {
    method: 'PUT',
    body: JSON.stringify({
      max_age_days: maxAgeDays,
      rotation_reminder_days: reminderDays,
      require_rotation: requireRotation,
    }),
  });
  return data.policy;
}

// Phase 11: Agent Leases
export async function createLease(
  projectId: string,
  environment: string,
  secretKeys: string[],
  durationMinutes: number
): Promise<Record<string, unknown>> {
  const data = await request<{ lease: Record<string, unknown> }>(`/projects/${projectId}/leases`, {
    method: 'POST',
    body: JSON.stringify({
      environment,
      secret_keys: secretKeys,
      duration_minutes: durationMinutes,
    }),
  });
  return data.lease;
}

export async function listLeases(projectId: string): Promise<Record<string, unknown>[]> {
  const data = await request<{ leases: Record<string, unknown>[] }>(`/projects/${projectId}/leases`);
  return data.leases || [];
}

export async function revokeLease(projectId: string, leaseId: string): Promise<void> {
  await request(`/projects/${projectId}/leases/${leaseId}`, { method: 'DELETE' });
}

// Phase 11: Agent Analytics
export async function getAgentActivity(projectId: string): Promise<Record<string, unknown>[]> {
  const data = await request<{ activities: Record<string, unknown>[] }>(`/projects/${projectId}/agent-activity`);
  return data.activities || [];
}

export async function getAgentHeatmap(projectId: string): Promise<Record<string, unknown>[]> {
  const data = await request<{ heatmap: Record<string, unknown>[] }>(`/projects/${projectId}/agent-heatmap`);
  return data.heatmap || [];
}

// Phase 12: Platform - Events
export async function getEvents(): Promise<Record<string, unknown>[]> {
  const data = await request<{ events: Record<string, unknown>[] }>('/platform/events');
  return data.events || [];
}

export async function replayEvents(eventType: string): Promise<void> {
  await request('/platform/events/replay', {
    method: 'POST',
    body: JSON.stringify({ event_type: eventType }),
  });
}

// Phase 12: Platform - Plugins
export async function getPlugins(): Promise<Record<string, unknown>[]> {
  const data = await request<{ plugins: Record<string, unknown>[] }>('/platform/plugins');
  return data.plugins || [];
}

export async function registerPlugin(
  name: string,
  pluginType: string,
  version: string,
  config: Record<string, unknown> = {}
): Promise<Record<string, unknown>> {
  const data = await request<{ plugin: Record<string, unknown> }>('/platform/plugins', {
    method: 'POST',
    body: JSON.stringify({ name, plugin_type: pluginType, version, config }),
  });
  return data.plugin;
}

// Phase 12: Platform - Access Policies
export async function listAccessPolicies(projectId: string): Promise<Record<string, unknown>[]> {
  const data = await request<{ policies: Record<string, unknown>[] }>(`/projects/${projectId}/access-policies`);
  return data.policies || [];
}

export async function createAccessPolicy(
  projectId: string,
  policyType: string,
  config: Record<string, unknown>
): Promise<Record<string, unknown>> {
  const data = await request<{ policy: Record<string, unknown> }>(`/projects/${projectId}/access-policies`, {
    method: 'POST',
    body: JSON.stringify({ policy_type: policyType, config }),
  });
  return data.policy;
}

// Phase 13: OAuth Client Management
export async function listOAuthClients(): Promise<OAuthClient[]> {
  const data = await request<{ clients: OAuthClient[] }>('/oauth/clients');
  return data.clients || [];
}

export async function registerOAuthClient(
  name: string,
  description: string,
  redirectURIs: string[],
  scopes: string[],
  grantTypes: string[],
  isPublic: boolean
): Promise<{ client: OAuthClient; client_secret: string }> {
  return request('/oauth/clients', {
    method: 'POST',
    body: JSON.stringify({
      name,
      description,
      redirect_uris: redirectURIs,
      scopes,
      grant_types: grantTypes,
      is_public: isPublic,
    }),
  });
}

export async function deleteOAuthClient(clientId: string): Promise<void> {
  await request(`/oauth/clients/${clientId}`, { method: 'DELETE' });
}

// Phase 13: MCP Server Hub
export async function listPublicMCPServers(): Promise<MCPServer[]> {
  const data = await request<{ servers: MCPServer[] }>('/mcp/servers/public');
  return data.servers || [];
}

export async function listMyMCPServers(): Promise<MCPServer[]> {
  const data = await request<{ servers: MCPServer[] }>('/mcp/servers');
  return data.servers || [];
}

export async function getMCPServer(serverId: string): Promise<MCPServer> {
  const data = await request<{ server: MCPServer }>(`/mcp/servers/${serverId}`);
  return data.server;
}

export async function registerMCPServer(
  name: string,
  description: string,
  githubUrl: string,
  githubBranch: string,
  entryCommand: string,
  transport: string,
  envMappings: Record<string, unknown>,
  isPublic: boolean
): Promise<MCPServer> {
  const data = await request<{ server: MCPServer }>('/mcp/servers', {
    method: 'POST',
    body: JSON.stringify({
      name,
      description,
      github_url: githubUrl,
      github_branch: githubBranch,
      entry_command: entryCommand,
      transport,
      env_mappings: envMappings,
      is_public: isPublic,
    }),
  });
  return data.server;
}

export async function deleteMCPServer(serverId: string): Promise<void> {
  await request(`/mcp/servers/${serverId}`, { method: 'DELETE' });
}

export async function rebuildMCPServer(serverId: string): Promise<void> {
  await request(`/mcp/servers/${serverId}/rebuild`, { method: 'POST' });
}

// MCP Installations
export async function listMCPInstallations(): Promise<MCPInstallation[]> {
  const data = await request<{ installations: MCPInstallation[] }>('/mcp/installations');
  return data.installations || [];
}

export async function installMCPServer(
  mcpServerId: string,
  projectId?: string,
  config?: Record<string, unknown>
): Promise<MCPInstallation> {
  const data = await request<{ installation: MCPInstallation }>('/mcp/installations', {
    method: 'POST',
    body: JSON.stringify({
      mcp_server_id: mcpServerId,
      project_id: projectId || '',
      config: config || {},
    }),
  });
  return data.installation;
}

export async function uninstallMCPServer(installId: string): Promise<void> {
  await request(`/mcp/installations/${installId}`, { method: 'DELETE' });
}

// MCP Gateway
export async function listMCPTools(): Promise<MCPServerWithTools[]> {
  const data = await request<{ servers: MCPServerWithTools[] }>('/mcp/gateway/tools');
  return data.servers || [];
}

export async function getMCPGatewayStats(): Promise<GatewayStats[]> {
  const data = await request<{ stats: GatewayStats[] }>('/mcp/gateway/stats');
  return data.stats || [];
}

export async function getMCPConfig(): Promise<Record<string, unknown>> {
  return request<Record<string, unknown>>('/mcp/config');
}

// Dependency Graph
export async function analyzeDependencies(
  projectId: string,
  environment: string
): Promise<unknown[]> {
  const data = await request<{ dependencies: unknown[] }>(
    `/projects/${projectId}/dependencies/analyze?environment=${environment}`,
    { method: 'POST' }
  );
  return data.dependencies || [];
}

export async function getDependencyGraph(
  projectId: string,
  environment: string
): Promise<DependencyNode[]> {
  const data = await request<{ graph: DependencyNode[] }>(
    `/projects/${projectId}/dependencies/graph?environment=${environment}`
  );
  return data.graph || [];
}

// Phase 14: Application Dashboard

export async function listApplications(
  search?: string,
  category?: string,
  limit?: number,
  offset?: number
): Promise<{ applications: DashboardApplication[]; categories: string[]; total: number; limit: number; offset: number }> {
  const params = new URLSearchParams();
  if (search) params.set('search', search);
  if (category && category !== 'All') params.set('category', category);
  if (limit) params.set('limit', String(limit));
  if (offset) params.set('offset', String(offset));
  const query = params.toString() ? `?${params.toString()}` : '';
  return request<{ applications: DashboardApplication[]; categories: string[]; total: number; limit: number; offset: number }>(`/applications${query}`);
}

export async function createApplication(
  name: string,
  url: string,
  description: string,
  icon: string,
  category: string
): Promise<DashboardApplication> {
  const data = await request<{ application: DashboardApplication }>('/applications', {
    method: 'POST',
    body: JSON.stringify({ name, url, description, icon, category }),
  });
  return data.application;
}

export async function updateApplication(
  id: string,
  name: string,
  url: string,
  description: string,
  icon: string,
  category: string
): Promise<DashboardApplication> {
  const data = await request<{ application: DashboardApplication }>(`/applications/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, url, description, icon, category }),
  });
  return data.application;
}

export async function deleteApplication(id: string): Promise<void> {
  await request(`/applications/${id}`, { method: 'DELETE' });
}

export async function toggleApplicationFavorite(id: string): Promise<boolean> {
  const data = await request<{ is_favorite: boolean }>(`/applications/${id}/favorite`, {
    method: 'POST',
  });
  return data.is_favorite;
}
