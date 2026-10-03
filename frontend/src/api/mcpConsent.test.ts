import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { decideMCPConsent, getMCPConsent, verifiedConsentCallback, type MCPConsent } from './mcpConsent';
const session = vi.hoisted(() => ({ token: 'initial' }));
vi.mock('./client', () => ({ BASE_URL: '/api/v1', getAuthToken: () => session.token, invalidateBrowserSession: vi.fn() }));
const id = '55555555-1111-2222-3333-444444444444';
const consent: MCPConsent = { request_id: id, client_id: 'keepsave-hermes-linux-v1', harness: 'hermes', resource: 'https://app.keepsave.example/mcp', redirect_uri: 'http://127.0.0.1:17702/callback', scope: 'keepsave:tools offline_access', expires_at: new Date(Date.now() + 300000).toISOString() };
const fetchMock = vi.fn();
beforeEach(() => { session.token = 'initial'; vi.stubGlobal('fetch', fetchMock); fetchMock.mockReset(); });
afterEach(() => vi.unstubAllGlobals());
it('accepts only the registered client callback with canonical issuer and bounded response', () => {
  const callback = `${consent.redirect_uri}?code=${'a'.repeat(43)}&state=${'s'.repeat(32)}&iss=${encodeURIComponent('https://app.keepsave.example')}`;
  expect(verifiedConsentCallback(callback, consent)).toBe(callback);
  for (const wrong of [callback.replace('17702', '17701'), callback.replace('127.0.0.1', 'evil.example'), callback.replace('https%3A%2F%2Fapp.keepsave.example', 'https%3A%2F%2Fevil.example'), `${callback}&code=${'b'.repeat(43)}`, `${callback}&access_token=private`]) expect(() => verifiedConsentCallback(wrong, consent)).toThrow('verified');
});
it('does not accept a late preview or decision for a different browser identity', async () => {
  let finish!: (value: unknown) => void;
  fetchMock.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  const pending = getMCPConsent(id);
  session.token = 'other';
  finish({ ok: true, json: async () => consent });
  await expect(pending).rejects.toThrow('account changed');
  fetchMock.mockImplementation(async () => ({ ok: true, json: async () => { session.token = 'third'; return { redirect_url: 'private' }; } }));
  await expect(decideMCPConsent(id, true)).rejects.toThrow('account changed');
});
it('never sends a decision until explicitly invoked and never trusts malformed client metadata', async () => {
  fetchMock.mockResolvedValue({ ok: true, json: async () => ({ ...consent, redirect_uri: 'https://evil.example' }) });
  await expect(getMCPConsent(id)).rejects.toThrow('verified');
  expect(fetchMock.mock.calls[0][1].method).toBe('GET');
  expect(fetchMock).toHaveBeenCalledTimes(1);
});
