import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { webcrypto } from 'node:crypto';
import { completeSocialLogin, startSocialLogin } from './socialAuth';
vi.mock('./client', () => ({ BASE_URL: '/api/v1', getAuthToken: () => 'existing-session', invalidateBrowserSession: vi.fn() }));
const state = 'a'.repeat(64);
const pending = { verifier: 'b'.repeat(64), provider: 'github', mode: 'login', expires: Date.now() + 600000 };
const fetchMock = vi.fn();
function save(value = pending) { sessionStorage.setItem(`keepsave_oauth:${state}`, JSON.stringify(value)); }
beforeEach(() => { sessionStorage.clear(); vi.stubGlobal('fetch', fetchMock); fetchMock.mockReset(); });
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });
describe('social sign-in binding', () => {
 it('sends code and verifier in POST body then removes pending proof', async () => {
  save(); fetchMock.mockResolvedValue({ ok: true, json: async () => ({ token: 'jwt', user: { id: 'owner' } }) });
  const result = await completeSocialLogin('github', `?code=code&state=${state}`);
  expect(result.auth?.token).toBe('jwt'); expect(sessionStorage.getItem(`keepsave_oauth:${state}`)).toBeNull();
  expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/auth/social/github/complete');
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ code: 'code', state, code_verifier: pending.verifier });
  expect(fetchMock.mock.calls[0][1].headers.Authorization).toBeUndefined();
 });
 it.each(['missing', 'expired', 'provider'])('rejects %s flow before requesting a session', async (mode) => {
  if (mode !== 'missing') save({ ...pending, ...(mode === 'expired' ? { expires: 0 } : { provider: 'google' }) });
  await expect(completeSocialLogin('github', `?code=code&state=${state}`)).rejects.toThrow(/expired|another tab/); expect(fetchMock).not.toHaveBeenCalled();
 });
 it('handles denial without displaying provider-supplied text', async () => {
  save(); await expect(completeSocialLogin('github', `?error=access_denied&error_description=untrusted&state=${state}`)).rejects.toThrow('Sign-in was cancelled'); expect(fetchMock).not.toHaveBeenCalled();
 });
 it('requires the existing session when completing a link', async () => {
  save({ ...pending, mode: 'link' }); fetchMock.mockResolvedValue({ ok: true, json: async () => ({ linked: true }) });
  expect((await completeSocialLogin('github', `?code=code&state=${state}`)).mode).toBe('link');
  expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/account/connections/github/complete');
  expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe('Bearer existing-session');
 });
 it('creates browser-bound PKCE and redirects only to the expected provider', async () => {
  vi.stubGlobal('crypto', webcrypto); const assign = vi.fn(); vi.stubGlobal('window', { location: { assign } });
  const url = `https://github.com/login/oauth/authorize?state=${state}`;
  fetchMock.mockResolvedValue({ ok: true, json: async () => ({ authorization_url: url, state }) });
  await startSocialLogin('github'); const proof = JSON.parse(sessionStorage.getItem(`keepsave_oauth:${state}`)!);
  expect(proof.verifier).toMatch(/^[a-f0-9]{64}$/); expect(assign).toHaveBeenCalledWith(url);
  const digest = await webcrypto.subtle.digest('SHA-256', new TextEncoder().encode(proof.verifier));
  expect(JSON.parse(fetchMock.mock.calls[0][1].body).code_challenge).toBe(Buffer.from(digest).toString('base64url'));
 });
 it('rejects an unexpected redirect destination', async () => {
  vi.stubGlobal('crypto', webcrypto);
  fetchMock.mockResolvedValue({ ok: true, json: async () => ({ authorization_url: 'https://example.com/steal', state }) });
  await expect(startSocialLogin('github')).rejects.toThrow('destination'); expect(sessionStorage.getItem(`keepsave_oauth:${state}`)).toBeNull();
 });
 it('fails closed when browser storage is unavailable', async () => {
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked'); });
  await expect(startSocialLogin('google')).rejects.toThrow('Allow storage'); expect(fetchMock).not.toHaveBeenCalled();
 });
});
