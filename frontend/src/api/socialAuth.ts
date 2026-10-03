import { BASE_URL, getAuthToken, invalidateBrowserSession } from './client';
import type { AuthResponse } from '../types';
import { validConsentReturn } from '../lib/mcpReturnContext';
export type Provider = 'github' | 'google';
export type Providers = Record<Provider, boolean>;
type Mode = 'login' | 'link';
interface PendingFlow { verifier: string; provider: Provider; mode: Mode; expires: number; returnTo?: string }
const prefix = 'keepsave_oauth:';

async function request<T>(path: string, body?: unknown, authenticated = false): Promise<T> {
  const token = authenticated ? getAuthToken() : null;
  if (authenticated && !token) throw new Error('Your session expired. Sign in again before connecting a provider.');
  const response = await fetch(`${BASE_URL}${path}`, {
    method: body ? 'POST' : 'GET', cache: 'no-store',
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    ...(body ? { body: JSON.stringify(body) } : {}),
  });
  if (authenticated && response.status === 401) {
    if (getAuthToken() === token) invalidateBrowserSession();
    throw new Error('Your session expired. Sign in again before connecting a provider.');
  }
  let data;
  try { data = await response.json(); } catch { throw new Error('Sign-in is temporarily unavailable. Please try again.'); }
  if (!response.ok) throw new Error(data?.error?.message || 'Sign-in could not be completed. Please try again.');
  return data as T;
}
export const getProviders = () => request<Providers>('/auth/providers');
export const getConnections = () => request<{ connected: Provider[]; available: Providers }>('/account/connections', undefined, true);
function flowPath(provider: Provider, mode: Mode) { return mode === 'link' ? `/account/connections/${provider}` : `/auth/social/${provider}`; }

export async function startSocialLogin(provider: Provider, mode: Mode = 'login', returnTo?: string): Promise<void> {
  try {
    sessionStorage.setItem(`${prefix}probe`, '1'); sessionStorage.removeItem(`${prefix}probe`);
    for (const key of Object.keys(sessionStorage)) {
      if (key.startsWith(prefix)) {
        try { if (JSON.parse(sessionStorage.getItem(key) || '{}').expires < Date.now()) sessionStorage.removeItem(key); }
        catch { sessionStorage.removeItem(key); }
      }
    }
  } catch { throw new Error('Allow storage for this site to sign in with a provider.'); }
  const verifier = Array.from(crypto.getRandomValues(new Uint8Array(32)), (byte) => byte.toString(16).padStart(2, '0')).join('');
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
  const challenge = btoa(String.fromCharCode(...new Uint8Array(digest))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
  const result = await request<{ authorization_url: string; state: string }>(`${flowPath(provider, mode)}/start`, { code_challenge: challenge }, mode === 'link');
  const destination = new URL(result.authorization_url);
  const expected = provider === 'github' ? 'https://github.com/login/oauth/authorize' : 'https://accounts.google.com/o/oauth2/v2/auth';
  if (`${destination.origin}${destination.pathname}` !== expected || !/^[a-f0-9]{64}$/.test(result.state)) throw new Error('The sign-in destination could not be verified.');
  const pending: PendingFlow = { verifier, provider, mode, expires: Date.now() + 600_000, ...(mode === 'login' && validConsentReturn(returnTo) ? { returnTo: validConsentReturn(returnTo) } : {}) };
  try { sessionStorage.setItem(`${prefix}${result.state}`, JSON.stringify(pending)); }
  catch { throw new Error('Allow storage for this site to sign in with a provider.'); }
  window.location.assign(result.authorization_url);
}

export async function completeSocialLogin(provider: string, query: string): Promise<{ mode: Mode; auth?: AuthResponse; returnTo?: string }> {
  const params = new URLSearchParams(query);
  const state = params.get('state') || '';
  if ((provider !== 'github' && provider !== 'google') || !/^[a-f0-9]{64}$/.test(state)) throw new Error('This sign-in attempt could not be verified. Please start again.');
  let pending: PendingFlow;
  try {
    pending = JSON.parse(sessionStorage.getItem(`${prefix}${state}`) || 'null');
    sessionStorage.removeItem(`${prefix}${state}`);
  } catch { throw new Error('This sign-in needs the browser tab where you started. Please start again.'); }
  if (!pending || pending.provider !== provider || pending.expires <= Date.now() || !/^[a-f0-9]{64}$/.test(pending.verifier) || !['login', 'link'].includes(pending.mode)) throw new Error('This sign-in attempt expired or belongs to another tab. Please start again.');
  if (params.has('error')) throw new Error('Sign-in was cancelled. You can try again or continue with email.');
  const code = params.get('code');
  if (!code) throw new Error('No sign-in code was received. Please start again.');
  const result = await request<AuthResponse>(`${flowPath(provider, pending.mode)}/complete`, { code, state, code_verifier: pending.verifier }, pending.mode === 'link');
  if (pending.mode === 'login' && (!result.token || !result.user?.id)) throw new Error('The sign-in response could not be verified. Please start again.');
  return { mode: pending.mode, ...(pending.mode === 'login' ? { auth: result, returnTo: validConsentReturn(pending.returnTo) } : {}) };
}
