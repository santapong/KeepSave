/**
 * KeepSave Node.js SDK - Secure environment variable storage for AI Agents.
 *
 * Usage:
 *   import { KeepSaveClient } from '@keepsave/sdk';
 *   const client = new KeepSaveClient({ baseUrl: 'http://localhost:8080' });
 *   await client.login('user@example.com', 'password');
 *   const secrets = await client.listSecrets('project-id', 'alpha');
 */

export interface KeepSaveConfig {
  baseUrl: string;
  token?: string;
  apiKey?: string;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  owner_id: string;
  created_at: string;
  updated_at: string;
}

export interface Secret {
  id: string;
  project_id: string;
  environment_id: string;
  key: string;
  value?: string;
  created_at: string;
  updated_at: string;
}

export interface APIKeyResponse {
  api_key: { id: string; name: string; project_id: string };
  raw_key: string;
}

export interface DiffEntry {
  key: string;
  action: 'add' | 'update' | 'no_change';
  source_value?: string;
  target_value?: string;
}

export interface PromotionRequest {
  id: string;
  project_id: string;
  source_environment: string;
  target_environment: string;
  status: string;
}

export class KeepSaveError extends Error {
  code: number;
  constructor(message: string, code: number) {
    super(message);
    this.code = code;
    this.name = 'KeepSaveError';
  }
}

export class KeepSaveClient {
  private baseUrl: string;
  private token: string | null;
  private apiKey: string | null;

  constructor(config: KeepSaveConfig) {
    this.baseUrl = config.baseUrl.replace(/\/$/, '');
    this.token = config.token || null;
    this.apiKey = config.apiKey || null;
  }

  private async request<T>(method: string, path: string, body?: Record<string, unknown>): Promise<T> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.apiKey) {
      headers['X-API-Key'] = this.apiKey;
    } else if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const resp = await fetch(`${this.baseUrl}/api/v1${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (resp.status === 204) return {} as T;

    const data = await resp.json();
    if (!resp.ok) {
      const msg = data?.error?.message || data?.error || `Request failed: ${resp.status}`;
      throw new KeepSaveError(msg, resp.status);
    }
    return data as T;
  }

  // Authentication

  async register(email: string, password: string): Promise<{ user: { id: string; email: string }; token: string }> {
    const resp = await this.request<{ user: { id: string; email: string }; token: string }>(
      'POST', '/auth/register', { email, password }
    );
    this.token = resp.token;
    return resp;
  }

  async login(email: string, password: string): Promise<{ user: { id: string; email: string }; token: string }> {
    const resp = await this.request<{ user: { id: string; email: string }; token: string }>(
      'POST', '/auth/login', { email, password }
    );
    this.token = resp.token;
    return resp;
  }

  // Projects

  async listProjects(): Promise<Project[]> {
    const resp = await this.request<{ projects: Project[] }>('GET', '/projects');
    return resp.projects || [];
  }

  async createProject(name: string, description = ''): Promise<Project> {
    const resp = await this.request<{ project: Project }>('POST', '/projects', { name, description });
    return resp.project;
  }

  async getProject(id: string): Promise<Project> {
    const resp = await this.request<{ project: Project }>('GET', `/projects/${id}`);
    return resp.project;
  }

  async deleteProject(id: string): Promise<void> {
    await this.request('DELETE', `/projects/${id}`);
  }

  // Secrets

  async listSecrets(projectId: string, environment: string): Promise<Secret[]> {
    const resp = await this.request<{ secrets: Secret[] }>('GET', `/projects/${projectId}/secrets?environment=${environment}`);
    return resp.secrets || [];
  }

  async createSecret(projectId: string, key: string, value: string, environment: string): Promise<Secret> {
    const resp = await this.request<{ secret: Secret }>('POST', `/projects/${projectId}/secrets`, {
      key, value, environment,
    });
    return resp.secret;
  }

  async updateSecret(projectId: string, secretId: string, value: string): Promise<Secret> {
    const resp = await this.request<{ secret: Secret }>('PUT', `/projects/${projectId}/secrets/${secretId}`, { value });
    return resp.secret;
  }

  async deleteSecret(projectId: string, secretId: string): Promise<void> {
    await this.request('DELETE', `/projects/${projectId}/secrets/${secretId}`);
  }

  // Promotions

  async promote(
    projectId: string, source: string, target: string,
    overridePolicy = 'skip', keys?: string[], notes?: string
  ): Promise<PromotionRequest> {
    const body: Record<string, unknown> = {
      source_environment: source,
      target_environment: target,
      override_policy: overridePolicy,
    };
    if (keys) body.keys = keys;
    if (notes) body.notes = notes;
    const resp = await this.request<{ promotion: PromotionRequest }>('POST', `/projects/${projectId}/promote`, body);
    return resp.promotion;
  }

  async promoteDiff(projectId: string, source: string, target: string): Promise<DiffEntry[]> {
    const resp = await this.request<{ diff: DiffEntry[] }>('POST', `/projects/${projectId}/promote/diff`, {
      source_environment: source, target_environment: target,
    });
    return resp.diff || [];
  }

  // API Keys

  async listAPIKeys(): Promise<{ id: string; name: string }[]> {
    const resp = await this.request<{ api_keys: { id: string; name: string }[] }>('GET', '/api-keys');
    return resp.api_keys || [];
  }

  async createAPIKey(name: string, projectId: string, scopes: string[] = ['read'], environment?: string): Promise<APIKeyResponse> {
    const body: Record<string, unknown> = { name, project_id: projectId, scopes };
    if (environment) body.environment = environment;
    return this.request<APIKeyResponse>('POST', '/api-keys', body);
  }

  async deleteAPIKey(id: string): Promise<void> {
    await this.request('DELETE', `/api-keys/${id}`);
  }

  // Key Rotation

  async rotateKeys(projectId: string): Promise<Record<string, unknown>> {
    return this.request('POST', `/projects/${projectId}/rotate-keys`);
  }

  // Import/Export

  async exportEnv(projectId: string, environment: string): Promise<string> {
    const resp = await this.request<{ content: string }>('GET', `/projects/${projectId}/env-export?environment=${environment}`);
    return resp.content || '';
  }

  async importEnv(projectId: string, environment: string, content: string, overwrite = false): Promise<Record<string, unknown>> {
    const resp = await this.request<{ result: Record<string, unknown> }>('POST', `/projects/${projectId}/env-import`, {
      environment, content, overwrite,
    });
    return resp.result || {};
  }
}
