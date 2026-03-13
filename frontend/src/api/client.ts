import type {
  AuthResponse,
  Project,
  Secret,
  APIKey,
  AuditEntry,
  PromotionRequest,
  DiffEntry,
} from '../types';

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
    throw new Error(data.error || `Request failed with status ${response.status}`);
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
