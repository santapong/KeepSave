export interface Secret {
  id: string;
  project_id: string;
  environment_id: string;
  key: string;
  value?: string;
  created_at: string;
  updated_at: string;
}

export class KeepSaveAPI {
  private baseUrl: string;
  private token: string | null = null;
  private apiKey: string | null = null;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl.replace(/\/+$/, '');
  }

  setToken(token: string): void {
    this.token = token;
    this.apiKey = null;
  }

  setApiKey(apiKey: string): void {
    this.apiKey = apiKey;
    this.token = null;
  }

  isAuthenticated(): boolean {
    return !!(this.token || this.apiKey);
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...((options.headers as Record<string, string>) || {}),
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    } else if (this.apiKey) {
      headers['X-API-Key'] = this.apiKey;
    }

    const response = await fetch(`${this.baseUrl}${path}`, {
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

  async listSecrets(projectId: string, environment: string): Promise<Secret[]> {
    const data = await this.request<{ secrets: Secret[] }>(
      `/api/v1/projects/${projectId}/secrets?environment=${encodeURIComponent(environment)}`
    );
    return data.secrets;
  }

  async createSecret(projectId: string, key: string, value: string, environment: string): Promise<Secret> {
    const data = await this.request<{ secret: Secret }>(`/api/v1/projects/${projectId}/secrets`, {
      method: 'POST',
      body: JSON.stringify({ key, value, environment }),
    });
    return data.secret;
  }

  async updateSecret(projectId: string, secretId: string, value: string): Promise<Secret> {
    const data = await this.request<{ secret: Secret }>(
      `/api/v1/projects/${projectId}/secrets/${secretId}`,
      {
        method: 'PUT',
        body: JSON.stringify({ value }),
      }
    );
    return data.secret;
  }

  async deleteSecret(projectId: string, secretId: string): Promise<void> {
    await this.request(`/api/v1/projects/${projectId}/secrets/${secretId}`, {
      method: 'DELETE',
    });
  }
}
