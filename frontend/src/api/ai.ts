// Phase 15: AI Intelligence API client
import type {
  AIProviderStatus,
  DriftCheck,
  Anomaly,
  UsageTrend,
  UsageForecast,
  SecretRecommendation,
  NLPQueryResult,
} from '../types/ai';

const BASE_URL = '/api/v1';

function getToken(): string | null {
  return localStorage.getItem('keepsave_token');
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  };
  const token = getToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const response = await fetch(`${BASE_URL}${path}`, { ...options, headers });
  if (response.status === 204) return undefined as T;
  const data = await response.json();
  if (!response.ok) {
    const msg = typeof data.error === 'object' ? data.error.message : data.error;
    throw new Error(msg || `Request failed: ${response.status}`);
  }
  return data;
}

// AI Providers
export async function listAIProviders(): Promise<{ providers: AIProviderStatus[]; has_provider: boolean }> {
  return request('/ai/providers');
}

// Drift Detection
export async function detectDrift(projectId: string, sourceEnv: string, targetEnv: string): Promise<DriftCheck> {
  const data = await request<{ drift_check: DriftCheck }>(`/projects/${projectId}/drift`, {
    method: 'POST',
    body: JSON.stringify({ source_env: sourceEnv, target_env: targetEnv }),
  });
  return data.drift_check;
}

export async function listDriftChecks(projectId: string): Promise<DriftCheck[]> {
  const data = await request<{ drift_checks: DriftCheck[] }>(`/projects/${projectId}/drift`);
  return data.drift_checks || [];
}

// Anomaly Detection
export async function runAnomalyDetection(projectId: string): Promise<{ anomalies: Anomaly[]; count: number }> {
  return request(`/projects/${projectId}/anomalies/scan`, { method: 'POST' });
}

export async function listAnomalies(projectId?: string, status?: string): Promise<Anomaly[]> {
  const params = new URLSearchParams();
  if (projectId) params.set('project_id', projectId);
  if (status) params.set('status', status);
  const q = params.toString() ? `?${params.toString()}` : '';
  const data = await request<{ anomalies: Anomaly[] }>(`/ai/anomalies${q}`);
  return data.anomalies || [];
}

export async function acknowledgeAnomaly(id: string): Promise<void> {
  await request(`/ai/anomalies/${id}/acknowledge`, { method: 'PUT' });
}

export async function resolveAnomaly(id: string): Promise<void> {
  await request(`/ai/anomalies/${id}/resolve`, { method: 'PUT' });
}

// Usage Analytics
export async function getUsageTrends(projectId: string, period?: string, days?: number): Promise<UsageTrend[]> {
  const params = new URLSearchParams();
  if (period) params.set('period', period);
  if (days) params.set('days', String(days));
  const q = params.toString() ? `?${params.toString()}` : '';
  const data = await request<{ trends: UsageTrend[] }>(`/projects/${projectId}/analytics/trends${q}`);
  return data.trends || [];
}

export async function getUsageForecast(projectId: string, days?: number): Promise<UsageForecast[]> {
  const q = days ? `?days=${days}` : '';
  const data = await request<{ forecasts: UsageForecast[] }>(`/projects/${projectId}/analytics/forecast${q}`);
  return data.forecasts || [];
}

// Recommendations
export async function generateRecommendations(projectId: string): Promise<SecretRecommendation[]> {
  const data = await request<{ recommendations: SecretRecommendation[] }>(`/projects/${projectId}/recommendations/generate`, { method: 'POST' });
  return data.recommendations || [];
}

export async function listRecommendations(projectId: string, status?: string): Promise<SecretRecommendation[]> {
  const q = status ? `?status=${status}` : '';
  const data = await request<{ recommendations: SecretRecommendation[] }>(`/projects/${projectId}/recommendations${q}`);
  return data.recommendations || [];
}

export async function dismissRecommendation(projectId: string, recId: string): Promise<void> {
  await request(`/projects/${projectId}/recommendations/${recId}`, { method: 'DELETE' });
}

// NLP Query
export async function nlpQuery(query: string): Promise<NLPQueryResult> {
  const data = await request<{ result: NLPQueryResult }>('/ai/query', {
    method: 'POST',
    body: JSON.stringify({ query }),
  });
  return data.result;
}
