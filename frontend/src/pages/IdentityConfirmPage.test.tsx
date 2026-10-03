import { StrictMode } from 'react';
import { beforeEach, afterEach, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { IdentityConfirmPage } from './IdentityConfirmPage';
import { getAuthToken, invalidateBrowserSession } from '../api/client';
vi.mock('../api/client', () => ({ BASE_URL: '/api/v1', getAuthToken: vi.fn(() => 'current-session'), invalidateBrowserSession: vi.fn() }));
const fetchMock = vi.fn();
const id = '12345678-1111-2222-3333-444444444444', proof = 'a'.repeat(43);
function response(data: unknown, status = 200) { return { ok: status < 400, status, json: async () => data }; }
beforeEach(() => { vi.clearAllMocks(); vi.mocked(getAuthToken).mockReturnValue('current-session'); fetchMock.mockReset(); vi.stubGlobal('fetch', fetchMock); sessionStorage.clear(); localStorage.clear(); });
afterEach(() => { vi.unstubAllGlobals(); });
it('clears the fragment before calls and submits a contact proof only in JSON once', async () => {
  window.history.replaceState({}, '', `/identity/confirm#id=${id}&purpose=contact_verify&proof=${proof}`);
  fetchMock.mockImplementation(async (_url, options) => { expect(window.location.hash).toBe(''); return response(options.method === 'POST' ? null : { available: ['identity_platform', 'identity_email_proofs'] }, options.method === 'POST' ? 204 : 200); });
  render(<StrictMode><BrowserRouter><IdentityConfirmPage /></BrowserRouter></StrictMode>);
  const button = await screen.findByRole('button', { name: 'Confirm email' }); await waitFor(() => expect(button).toBeEnabled()); fireEvent.click(button);
  expect(await screen.findByText('Your recovery email has been verified.')).toBeInTheDocument();
  const posts = fetchMock.mock.calls.filter(([, options]) => options.method === 'POST'); expect(posts).toHaveLength(1);
  expect(posts[0][0]).toBe(`/api/v1/account/contact-proofs/${id}/confirm`); expect(JSON.parse(posts[0][1].body)).toEqual({ proof });
  expect(posts[0][1].referrerPolicy).toBe('no-referrer'); expect(posts[0][1].redirect).toBe('error'); expect(document.body.textContent).not.toContain(proof); expect(sessionStorage.length).toBe(0); expect(localStorage.length).toBe(0);
});
it('public password reset sends no browser token and invalidates sessions only after success', async () => {
  window.history.replaceState({}, '', `/identity/confirm#id=${id}&purpose=password_reset&proof=${proof}`);
  fetchMock.mockImplementation(async (_url, options) => response(options.method === 'POST' ? null : { available: ['identity_platform', 'identity_email_proofs'] }, options.method === 'POST' ? 204 : 200));
  render(<BrowserRouter><IdentityConfirmPage /></BrowserRouter>);
  await waitFor(() => expect(screen.getByRole('button', { name: 'Save new password' })).toBeEnabled());
  fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'A-safe-test-password-2026!' } }); fireEvent.change(screen.getByLabelText('Repeat password'), { target: { value: 'A-safe-test-password-2026!' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save new password' })); await waitFor(() => expect(invalidateBrowserSession).toHaveBeenCalledOnce());
  const call = fetchMock.mock.calls.find(([, options]) => options.method === 'POST'); expect(call?.[0]).toBe('/api/v1/auth/recovery/confirm'); expect(call?.[1].headers.Authorization).toBeUndefined();
});
it('rejects a malformed proof after clearing it without a request', async () => {
  window.history.replaceState({}, '', '/identity/confirm#id=bad&purpose=password_reset&proof=private');
  render(<BrowserRouter><IdentityConfirmPage /></BrowserRouter>); expect(await screen.findByRole('alert')).toHaveTextContent('incomplete'); expect(window.location.hash).toBe(''); expect(fetchMock).not.toHaveBeenCalled();
});
it('does not sign out a newer account when an earlier reset finishes', async () => {
  window.history.replaceState({}, '', `/identity/confirm#id=${id}&purpose=password_reset&proof=${proof}`);
  let finish!: (value: ReturnType<typeof response>) => void;
  fetchMock.mockImplementation((_url, options) => options.method === 'POST' ? new Promise(resolve => { finish = resolve; }) : Promise.resolve(response({ available: ['identity_platform', 'identity_email_proofs'] })));
  render(<BrowserRouter><IdentityConfirmPage /></BrowserRouter>);
  await waitFor(() => expect(screen.getByRole('button', { name: 'Save new password' })).toBeEnabled());
  fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'A-safe-test-password-2026!' } });
  fireEvent.change(screen.getByLabelText('Repeat password'), { target: { value: 'A-safe-test-password-2026!' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save new password' }));
  await waitFor(() => expect(finish).toBeTypeOf('function'));
  vi.mocked(getAuthToken).mockReturnValue('new-account-session');
  finish(response(null, 204));
  await screen.findByRole('heading', { name: 'Confirmation complete' });
  expect(invalidateBrowserSession).not.toHaveBeenCalled();
  expect(getAuthToken()).toBe('new-account-session');
});
