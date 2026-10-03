import { beforeEach, afterEach, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { RecoveryPage } from './RecoveryPage';
vi.mock('../api/client', () => ({ BASE_URL: '/api/v1', getAuthToken: () => 'current-session', invalidateBrowserSession: vi.fn() }));
const fetchMock = vi.fn();
beforeEach(() => { fetchMock.mockReset(); vi.stubGlobal('fetch', fetchMock); }); afterEach(() => { vi.unstubAllGlobals(); });
it.each(['known', 'unknown'])('uses the same generic recovery wording for a %s contact', async () => {
  fetchMock.mockImplementation(async (_url, options) => ({ ok: true, status: options.method === 'POST' ? 202 : 200, json: async () => options.method === 'POST' ? { request: { id: 'nonsecret-id', status: 'accepted' } } : { available: ['identity_platform', 'identity_email_proofs'] } }));
  render(<BrowserRouter><RecoveryPage /></BrowserRouter>); const button = await screen.findByRole('button', { name: 'Request recovery' }); await waitFor(() => expect(button).toBeEnabled());
  fireEvent.change(screen.getByLabelText('Verified recovery email'), { target: { value: 'recovery@example.invalid' } }); fireEvent.click(button);
  expect(await screen.findByRole('status')).toHaveTextContent('If this is a verified recovery email'); const call = fetchMock.mock.calls.find(([, options]) => options.method === 'POST'); expect(call?.[1].headers.Authorization).toBeUndefined(); expect(call?.[0]).toBe('/api/v1/auth/recovery/request');
});
it('keeps recovery disabled when email delivery is unavailable', async () => {
  fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ available: ['identity_platform'] }) }); render(<BrowserRouter><RecoveryPage /></BrowserRouter>);
  expect(await screen.findByText(/awaiting email delivery setup/)).toBeInTheDocument(); expect(screen.getByRole('button', { name: 'Request recovery' })).toBeDisabled(); expect(fetchMock).toHaveBeenCalledOnce();
});
