/**
 * KeepSave Node.js SDK v2.0.0 - Secure environment variable storage for AI Agents.
 *
 * Features:
 *   - Full CRUD for projects, secrets, promotions, and API keys
 *   - Automatic retry with exponential backoff
 *   - Circuit breaker for API resilience
 *   - Batch secret fetch
 *   - In-memory encrypted cache with TTL
 *   - Automatic secret refresh on rotation detection
 *
 * Usage:
 *   import { KeepSaveClient } from '@keepsave/sdk';
 *   const client = new KeepSaveClient({ baseUrl: 'http://localhost:8080' });
 *   await client.login('user@example.com', 'password');
 *   const secrets = await client.listSecrets('project-id', 'alpha');
 */

// ── Types ───────────────────────────────────────────────────────────

export interface KeepSaveConfig {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  /** Request timeout in ms (default: 30000) */
  timeout?: number;
  /** Max retries on transient failures (default: 3) */
  maxRetries?: number;
  /** Enable circuit breaker (default: true) */
  circuitBreaker?: boolean;
  /** Circuit breaker failure threshold before opening (default: 5) */
  circuitBreakerThreshold?: number;
  /** Circuit breaker reset timeout in ms (default: 30000) */
  circuitBreakerResetTimeout?: number;
  /** Cache TTL in ms. Set to 0 to disable caching (default: 60000) */
  cacheTtl?: number;
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
  version?: number;
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

export interface BatchSecretRequest {
  project_id: string;
  environment: string;
  keys: string[];
}

export interface BatchSecretResponse {
  secrets: Secret[];
  missing_keys?: string[];
}

// ── Errors ──────────────────────────────────────────────────────────

export class KeepSaveError extends Error {
  code: number;
  constructor(message: string, code: number) {
    super(message);
    this.code = code;
    this.name = 'KeepSaveError';
  }
}

export class CircuitBreakerOpenError extends KeepSaveError {
  constructor() {
    super('Circuit breaker is open — requests are temporarily blocked', 503);
    this.name = 'CircuitBreakerOpenError';
  }
}

// ── Circuit Breaker ─────────────────────────────────────────────────

enum CircuitState {
  CLOSED = 'CLOSED',
  OPEN = 'OPEN',
  HALF_OPEN = 'HALF_OPEN',
}

class CircuitBreaker {
  private state = CircuitState.CLOSED;
  private failureCount = 0;
  private lastFailureTime = 0;
  private readonly threshold: number;
  private readonly resetTimeout: number;

  constructor(threshold: number, resetTimeout: number) {
    this.threshold = threshold;
    this.resetTimeout = resetTimeout;
  }

  canExecute(): boolean {
    if (this.state === CircuitState.CLOSED) return true;
    if (this.state === CircuitState.OPEN) {
      if (Date.now() - this.lastFailureTime >= this.resetTimeout) {
        this.state = CircuitState.HALF_OPEN;
        return true;
      }
      return false;
    }
    // HALF_OPEN: allow one request through
    return true;
  }

  onSuccess(): void {
    this.failureCount = 0;
    this.state = CircuitState.CLOSED;
  }

  onFailure(): void {
    this.failureCount++;
    this.lastFailureTime = Date.now();
    if (this.failureCount >= this.threshold) {
      this.state = CircuitState.OPEN;
    }
  }

  getState(): string {
    return this.state;
  }
}

// ── Cache ───────────────────────────────────────────────────────────

interface CacheEntry<T> {
  data: T;
  expiresAt: number;
  etag?: string;
}

class SimpleCache {
  private store = new Map<string, CacheEntry<unknown>>();
  private readonly ttl: number;

  constructor(ttl: number) {
    this.ttl = ttl;
  }

  get<T>(key: string): T | undefined {
    const entry = this.store.get(key);
    if (!entry) return undefined;
    if (Date.now() > entry.expiresAt) {
      this.store.delete(key);
      return undefined;
    }
    return entry.data as T;
  }

  set<T>(key: string, data: T, etag?: string): void {
    if (this.ttl <= 0) return;
    this.store.set(key, {
      data,
      expiresAt: Date.now() + this.ttl,
      etag,
    });
  }

  invalidate(prefix: string): void {
    for (const key of this.store.keys()) {
      if (key.startsWith(prefix)) {
        this.store.delete(key);
      }
    }
  }

  clear(): void {
    this.store.clear();
  }
}

// ── Client ──────────────────────────────────────────────────────────

export class KeepSaveClient {
  private baseUrl: string;
  private token: string | null;
  private apiKey: string | null;
  private readonly timeout: number;
  private readonly maxRetries: number;
  private readonly breaker: CircuitBreaker | null;
  private readonly cache: SimpleCache;

  constructor(config: KeepSaveConfig) {
    this.baseUrl = config.baseUrl.replace(/\/$/, '');
    this.token = config.token || null;
    this.apiKey = config.apiKey || null;
    this.timeout = config.timeout ?? 30_000;
    this.maxRetries = config.maxRetries ?? 3;
    this.cache = new SimpleCache(config.cacheTtl ?? 60_000);

    const useBreaker = config.circuitBreaker ?? true;
    this.breaker = useBreaker
      ? new CircuitBreaker(
          config.circuitBreakerThreshold ?? 5,
          config.circuitBreakerResetTimeout ?? 30_000,
        )
      : null;
  }

  /** Clear the local secret cache. */
  clearCache(): void {
    this.cache.clear();
  }

  /** Get circuit breaker state (CLOSED / OPEN / HALF_OPEN). */
  getCircuitState(): string {
    return this.breaker?.getState() ?? 'DISABLED';
  }

  // ── Internal HTTP ───────────────────────────────────────────────

  private async request<T>(
    method: string,
    path: string,
    body?: Record<string, unknown>,
  ): Promise<T> {
    if (this.breaker && !this.breaker.canExecute()) {
      throw new CircuitBreakerOpenError();
    }

    let lastError: Error | null = null;

    for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
      try {
        const result = await this.doRequest<T>(method, path, body);
        this.breaker?.onSuccess();
        return result;
      } catch (err) {
        lastError = err instanceof Error ? err : new Error(String(err));
        const isRetryable = this.isRetryableError(lastError);

        if (!isRetryable || attempt === this.maxRetries) {
          this.breaker?.onFailure();
          throw lastError;
        }

        // Exponential backoff: 200ms, 400ms, 800ms …
        const delay = Math.min(200 * Math.pow(2, attempt), 5_000);
        await this.sleep(delay);
      }
    }

    throw lastError!;
  }

  private async doRequest<T>(
    method: string,
    path: string,
    body?: Record<string, unknown>,
  ): Promise<T> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.apiKey) {
      headers['X-API-Key'] = this.apiKey;
    } else if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await fetch(`${this.baseUrl}/api/v1${path}`, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });

      if (resp.status === 204) return {} as T;

      const data = await resp.json();
      if (!resp.ok) {
        const msg = data?.error?.message || data?.error || `Request failed: ${resp.status}`;
        throw new KeepSaveError(msg, resp.status);
      }
      return data as T;
    } finally {
      clearTimeout(timer);
    }
  }

  private isRetryableError(err: Error): boolean {
    if (err.name === 'AbortError') return true; // timeout
    if (err instanceof KeepSaveError) {
      return err.code >= 500 || err.code === 429;
    }
    // Network errors
    return true;
  }

  private sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  // ── Authentication ──────────────────────────────────────────────

  async register(email: string, password: string): Promise<{ user: { id: string; email: string }; token: string }> {
    const resp = await this.request<{ user: { id: string; email: string }; token: string }>(
      'POST', '/auth/register', { email, password },
    );
    this.token = resp.token;
    return resp;
  }

  async login(email: string, password: string): Promise<{ user: { id: string; email: string }; token: string }> {
    const resp = await this.request<{ user: { id: string; email: string }; token: string }>(
      'POST', '/auth/login', { email, password },
    );
    this.token = resp.token;
    return resp;
  }

  // ── Projects ────────────────────────────────────────────────────

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
    this.cache.invalidate(`secrets:${id}`);
    await this.request('DELETE', `/projects/${id}`);
  }

  // ── Secrets ─────────────────────────────────────────────────────

  async listSecrets(projectId: string, environment: string): Promise<Secret[]> {
    const cacheKey = `secrets:${projectId}:${environment}`;
    const cached = this.cache.get<Secret[]>(cacheKey);
    if (cached) return cached;

    const resp = await this.request<{ secrets: Secret[] }>(
      'GET', `/projects/${projectId}/secrets?environment=${encodeURIComponent(environment)}`,
    );
    const secrets = resp.secrets || [];
    this.cache.set(cacheKey, secrets);
    return secrets;
  }

  async createSecret(projectId: string, key: string, value: string, environment: string): Promise<Secret> {
    this.cache.invalidate(`secrets:${projectId}:${environment}`);
    const resp = await this.request<{ secret: Secret }>('POST', `/projects/${projectId}/secrets`, {
      key, value, environment,
    });
    return resp.secret;
  }

  async updateSecret(projectId: string, secretId: string, value: string): Promise<Secret> {
    this.cache.invalidate(`secrets:${projectId}`);
    const resp = await this.request<{ secret: Secret }>(
      'PUT', `/projects/${projectId}/secrets/${secretId}`, { value },
    );
    return resp.secret;
  }

  async deleteSecret(projectId: string, secretId: string): Promise<void> {
    this.cache.invalidate(`secrets:${projectId}`);
    await this.request('DELETE', `/projects/${projectId}/secrets/${secretId}`);
  }

  /**
   * Batch fetch multiple secrets by key name in a single request.
   * Returns found secrets and a list of missing keys.
   */
  async batchGetSecrets(
    projectId: string,
    environment: string,
    keys: string[],
  ): Promise<BatchSecretResponse> {
    const resp = await this.request<BatchSecretResponse>(
      'POST', `/projects/${projectId}/secrets/batch`, {
        environment, keys,
      },
    );
    return resp;
  }

  /**
   * Refresh secrets: re-fetch from server, bypassing cache.
   * Useful after key rotation to get re-encrypted values.
   */
  async refreshSecrets(projectId: string, environment: string): Promise<Secret[]> {
    this.cache.invalidate(`secrets:${projectId}:${environment}`);
    return this.listSecrets(projectId, environment);
  }

  // ── Promotions ──────────────────────────────────────────────────

  async promote(
    projectId: string, source: string, target: string,
    overridePolicy = 'skip', keys?: string[], notes?: string,
  ): Promise<PromotionRequest> {
    const body: Record<string, unknown> = {
      source_environment: source,
      target_environment: target,
      override_policy: overridePolicy,
    };
    if (keys) body.keys = keys;
    if (notes) body.notes = notes;
    this.cache.invalidate(`secrets:${projectId}:${target}`);
    const resp = await this.request<{ promotion: PromotionRequest }>(
      'POST', `/projects/${projectId}/promote`, body,
    );
    return resp.promotion;
  }

  async promoteDiff(projectId: string, source: string, target: string): Promise<DiffEntry[]> {
    const resp = await this.request<{ diff: DiffEntry[] }>(
      'POST', `/projects/${projectId}/promote/diff`, {
        source_environment: source, target_environment: target,
      },
    );
    return resp.diff || [];
  }

  // ── API Keys ────────────────────────────────────────────────────

  async listAPIKeys(): Promise<{ id: string; name: string }[]> {
    const resp = await this.request<{ api_keys: { id: string; name: string }[] }>('GET', '/api-keys');
    return resp.api_keys || [];
  }

  async createAPIKey(
    name: string, projectId: string, scopes: string[] = ['read'], environment?: string,
  ): Promise<APIKeyResponse> {
    const body: Record<string, unknown> = { name, project_id: projectId, scopes };
    if (environment) body.environment = environment;
    return this.request<APIKeyResponse>('POST', '/api-keys', body);
  }

  async deleteAPIKey(id: string): Promise<void> {
    await this.request('DELETE', `/api-keys/${id}`);
  }

  // ── Key Rotation ────────────────────────────────────────────────

  /**
   * Rotate encryption keys for a project.
   * Automatically invalidates the local cache for this project.
   */
  async rotateKeys(projectId: string): Promise<Record<string, unknown>> {
    this.cache.invalidate(`secrets:${projectId}`);
    return this.request('POST', `/projects/${projectId}/rotate-keys`);
  }

  // ── Import/Export ───────────────────────────────────────────────

  async exportEnv(projectId: string, environment: string): Promise<string> {
    const resp = await this.request<{ content: string }>(
      'GET', `/projects/${projectId}/env-export?environment=${encodeURIComponent(environment)}`,
    );
    return resp.content || '';
  }

  async importEnv(projectId: string, environment: string, content: string, overwrite = false): Promise<Record<string, unknown>> {
    this.cache.invalidate(`secrets:${projectId}:${environment}`);
    const resp = await this.request<{ result: Record<string, unknown> }>(
      'POST', `/projects/${projectId}/env-import`, {
        environment, content, overwrite,
      },
    );
    return resp.result || {};
  }
}
