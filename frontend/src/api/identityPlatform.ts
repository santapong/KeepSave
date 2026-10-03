import { BASE_URL, getAuthToken, invalidateBrowserSession } from './client';

import type { IdentityMethod, IdentityContact, IdentityProofRequest, IdentityDelivery, IdentityOffboarding, IdentityInvitationCreated } from './coreTypes';
export type AccountMethod = IdentityMethod;
export type VerifiedContact = IdentityContact;
export type ProofRequest = IdentityProofRequest;
export type ProofDelivery = IdentityDelivery;
export type Offboarding = IdentityOffboarding;
export interface IdentityAvailability { identity: boolean; email: boolean }
export interface IdentityProof { id: string; purpose: 'contact_verify' | 'password_reset' | 'invitation'; proof: string }

export class IdentityPlatformError extends Error {
  constructor(public readonly status: number, message: string) { super(message); this.name = 'IdentityPlatformError'; }
}

// Same-origin proof requests have no URL credentials, telemetry, or local storage.
async function request<T>(path: string, method = 'GET', body?: unknown, authenticated = true, key?: string): Promise<T> {
  const base = new URL(BASE_URL, window.location.origin);
  if (base.origin !== window.location.origin) throw new IdentityPlatformError(503, 'Account safety requires this app’s secure connection.');
  const token = authenticated ? getAuthToken() : null;
  if (authenticated && !token) throw new IdentityPlatformError(401, 'Sign in again to continue.');
  const response = await fetch(`${BASE_URL}${path}`, { method, cache: 'no-store', credentials: 'same-origin', referrerPolicy: 'no-referrer', redirect: 'error',
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(key ? { 'Idempotency-Key': key } : {}) },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  });
  if (authenticated && getAuthToken() !== token) throw new IdentityPlatformError(409, 'Your account changed. Reload this page to continue.');
  if (authenticated && response.status === 401) { if (getAuthToken() === token) invalidateBrowserSession(); throw new IdentityPlatformError(401, 'Your session expired. Sign in again to continue.'); }
  if (response.status === 204) return undefined as T;
  const data = await response.json().catch(() => null);
  if (authenticated && getAuthToken() !== token) throw new IdentityPlatformError(409, 'Your account changed. Reload this page to continue.');
  if (!response.ok) throw new IdentityPlatformError(response.status, typeof data?.error?.message === 'string' ? data.error.message : 'Account safety is temporarily unavailable.');
  return data as T;
}
export async function getIdentityAvailability(): Promise<IdentityAvailability> {
  const data = await request<{ available: string[] }>('/capabilities', 'GET', undefined, false);
  return { identity: data.available.includes('identity_platform'), email: data.available.includes('identity_email_proofs') };
}
export const getAccountMethods = () => request<{ methods: AccountMethod[] }>('/account/methods');
export const getVerifiedContacts = () => request<{ contacts: VerifiedContact[] }>('/account/contacts');
export const removeAccountMethod = (name: AccountMethod['name']) => request<void>(`/account/methods/${name}`, 'DELETE');
export const requestContactProof = (contact: string) => request<{ request: ProofRequest }>('/account/contact-proofs', 'POST', { contact });
export const getProofDelivery = (id: string) => request<{ delivery: ProofDelivery }>(`/account/proofs/${encodeURIComponent(id)}`);
export const requestFreshProof = (id: string) => request<{ request: ProofRequest }>(`/account/proofs/${encodeURIComponent(id)}/resend`, 'POST');
export const requestAccountRecovery = (contact: string) => request<{ request: ProofRequest }>('/auth/recovery/request', 'POST', { contact }, false);
export const confirmVerifiedContact = (proof: IdentityProof) => request<void>(`/account/contact-proofs/${encodeURIComponent(proof.id)}/confirm`, 'POST', { proof: proof.proof });
export const acceptInvitation = (proof: IdentityProof) => request<void>(`/invitations/${encodeURIComponent(proof.id)}/accept`, 'POST', { proof: proof.proof });
export const sendWorkspaceInvitation = (org: string, contact: string, role: 'viewer' | 'editor' | 'promoter' | 'admin') => request<IdentityInvitationCreated>(`/organizations/${encodeURIComponent(org)}/invitations`, 'POST', { contact, role });
export const resetAccountPassword = (proof: IdentityProof, password: string) => request<void>('/auth/recovery/confirm', 'POST', { id: proof.id, proof: proof.proof, password }, false);
export const previewOffboarding = (org: string, user: string) => request<{ offboarding: Offboarding }>(`/organizations/${encodeURIComponent(org)}/members/${encodeURIComponent(user)}/offboarding-preview`, 'POST');
export const offboardMember = (preview: Offboarding, key: string) => request<{ offboarding: Offboarding }>(`/organizations/${encodeURIComponent(preview.organization_id)}/members/${encodeURIComponent(preview.user_id)}/offboarding`, 'POST', { preview_id: preview.preview_id, expected_authority_epoch: preview.authority_epoch }, true, key);

// Call before any request or navigation. The proof stays only in component memory.
export function captureIdentityProof(): IdentityProof | null {
  const fragment = window.location.hash;
  window.history.replaceState(window.history.state, '', window.location.pathname);
  const parts = new URLSearchParams(fragment.replace(/^#/, ''));
  if (parts.getAll('id').length !== 1 || parts.getAll('purpose').length !== 1 || parts.getAll('proof').length !== 1) return null;
  const id = parts.get('id') || '', purpose = parts.get('purpose') || '', proof = parts.get('proof') || '';
  if (!/^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/.test(id) || !/^[A-Za-z0-9_-]{43}$/.test(proof) || !['contact_verify', 'password_reset', 'invitation'].includes(purpose)) return null;
  return { id, purpose: purpose as IdentityProof['purpose'], proof };
}
