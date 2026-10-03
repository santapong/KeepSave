import { BASE_URL, getAuthToken, invalidateBrowserSession } from './client';

export interface MCPConsent {
  request_id: string;
  client_id: string;
  harness: 'codex' | 'hermes';
  resource: string;
  redirect_uri: string;
  scope: string;
  expires_at: string;
}
const callbacks: Record<string, string> = {
  'keepsave-codex-linux-v1': 'http://127.0.0.1:17701/callback',
  'keepsave-hermes-linux-v1': 'http://127.0.0.1:17702/callback',
};
function canonicalResource(raw: string): URL {
  const resource = new URL(raw);
  if (resource.protocol !== 'https:' || resource.username || resource.password || resource.pathname !== '/mcp' || resource.search || resource.hash) throw new Error('The connection destination could not be verified.');
  return resource;
}
export function verifyConsent(value: MCPConsent, id: string): MCPConsent {
  canonicalResource(value.resource);
  if (value.request_id !== id || !callbacks[value.client_id] || callbacks[value.client_id] !== value.redirect_uri || value.harness !== (value.client_id.includes('hermes') ? 'hermes' : 'codex') || !['keepsave:tools', 'keepsave:tools offline_access'].includes(value.scope) || !Number.isFinite(Date.parse(value.expires_at))) throw new Error('This connection request could not be verified.');
  return value;
}
export function verifiedConsentCallback(raw: string, consent: MCPConsent): string {
  const destination = new URL(raw);
  const resource = canonicalResource(consent.resource);
  const params = destination.searchParams;
  const validResponse = params.getAll('code').length === 1 && /^[A-Za-z0-9_-]{43}$/.test(params.get('code') || '') && !params.has('error') || params.getAll('error').length === 1 && params.get('error') === 'access_denied' && !params.has('code');
  if (`${destination.origin}${destination.pathname}` !== callbacks[consent.client_id] || destination.username || destination.password || destination.hash || params.getAll('iss').length !== 1 || params.get('iss') !== resource.origin || params.getAll('state').length !== 1 || (params.get('state') || '').length < 16 || (params.get('state') || '').length > 1024 || [...params.keys()].some((key) => !['state', 'iss', 'code', 'error'].includes(key)) || !validResponse) throw new Error('The client callback could not be verified.');
  return destination.toString();
}
async function request<T>(path: string, body?: unknown): Promise<T> {
  const token = getAuthToken();
  if (!token) throw new Error('Sign in to review this connection.');
  const response = await fetch(`${BASE_URL}${path}`, { method: body ? 'POST' : 'GET', cache: 'no-store', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, ...(body ? { body: JSON.stringify(body) } : {}) });
  if (getAuthToken() !== token) throw new Error('Your account changed. Open this connection request again.');
  if (response.status === 401) { invalidateBrowserSession(); throw new Error('Your session expired. Sign in again.'); }
  if (!response.ok) throw new Error(response.status === 503 ? 'Connections are unavailable on this installation.' : 'This request expired or needs a recent sign-in. Sign in again or start a new connection in your client.');
  const result = await response.json() as T;
  if (getAuthToken() !== token) throw new Error('Your account changed. Open this connection request again.');
  return result;
}
export async function getMCPConsent(id: string): Promise<MCPConsent> {
  return verifyConsent(await request<MCPConsent>(`/mcp/consent?request_id=${encodeURIComponent(id)}`), id);
}
export const decideMCPConsent = (id: string, approve: boolean) => request<{ redirect_url: string }>('/mcp/consent', { request_id: id, approve });
