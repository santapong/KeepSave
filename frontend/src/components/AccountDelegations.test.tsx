import { beforeEach, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { AccountDelegations } from './AccountDelegations';
import { getAuthToken, invalidateBrowserSession } from '../api/client';
import { useCapabilities } from '../hooks/useCapabilities';
import type { MCPDelegation } from '../api/coreTypes';

vi.mock('../api/client', () => ({ BASE_URL: '/api/v1', getAuthToken: vi.fn(), invalidateBrowserSession: vi.fn() }));
vi.mock('../hooks/useCapabilities', () => ({ useCapabilities: vi.fn() }));
const delegation: MCPDelegation = { family_id: 'family', client_id: 'keepsave-hermes-linux-v1', harness: 'hermes', resource: 'https://fixture.invalid/mcp', scope: 'keepsave:tools offline_access', created_at: '2026-10-02T00:00:00Z', expires_at: '2099-10-02T00:00:00Z', requires_refresh: true };
const fetchMock = vi.fn<typeof fetch>();
beforeEach(() => { vi.clearAllMocks(); vi.stubGlobal('fetch', fetchMock); vi.mocked(getAuthToken).mockReturnValue('fixture-token'); vi.mocked(useCapabilities).mockReturnValue({ capabilities: { version: 'v1', available: ['mcp_delegation'], release: 'fixture' } as never, error: false, enabled: () => true, unavailable: () => false }); fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ delegations: [delegation] }), { status: 200, headers: { 'Content-Type': 'application/json' } })); });
async function confirm() { fireEvent.click(await screen.findByRole('button', { name: 'Revoke connection' })); const buttons = await screen.findAllByRole('button', { name: 'Revoke connection' }); fireEvent.click(buttons[buttons.length - 1]); }

it('keeps a connection listed until server revocation commits', async () => {
  let settle!: (response: Response) => void; fetchMock.mockReturnValueOnce(new Promise((resolve) => { settle = resolve; }));
  render(<AccountDelegations />); expect(await screen.findByText('Waiting for the client to refresh this connection.')).toBeInTheDocument(); await confirm();
  await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2)); expect(screen.getByText(delegation.client_id)).toBeInTheDocument();
  settle(new Response(null, { status: 204 })); expect(await screen.findByRole('status')).toHaveTextContent('Hermes connection revoked'); expect(screen.queryByText(delegation.client_id)).not.toBeInTheDocument();
  expect(fetchMock.mock.calls[1][1]).toMatchObject({ method: 'DELETE', referrerPolicy: 'no-referrer', cache: 'no-store' });
});
it('retains the connection after an uncertain error', async () => {
  fetchMock.mockRejectedValueOnce(new Error('Disconnected')); render(<AccountDelegations />); await confirm();
  expect(await screen.findByRole('alert')).toHaveTextContent('Disconnected'); expect(screen.getByText(delegation.client_id)).toBeInTheDocument(); expect(invalidateBrowserSession).not.toHaveBeenCalled();
});
it('does not fetch connection inventory when its server capability is unavailable', async () => {
  vi.mocked(useCapabilities).mockReturnValue({ capabilities: { available: [] } as never, error: false, enabled: () => false, unavailable: () => true }); render(<AccountDelegations />);
  expect(screen.getByText('Tool connections are awaiting server setup.')).toBeInTheDocument(); expect(fetchMock).not.toHaveBeenCalled();
});
it('rejects a late account response after identity changes', async () => {
  fetchMock.mockReset(); let settle!: (response: Response) => void; fetchMock.mockReturnValue(new Promise((resolve) => { settle = resolve; }));
  render(<AccountDelegations />); await waitFor(() => expect(fetchMock).toHaveBeenCalledOnce()); vi.mocked(getAuthToken).mockReturnValue('different-account'); settle(new Response(JSON.stringify({ delegations: [delegation] }), { status: 200 }));
  expect(await screen.findByRole('alert')).toHaveTextContent('Your account changed'); expect(screen.queryByText(delegation.client_id)).not.toBeInTheDocument(); expect(invalidateBrowserSession).not.toHaveBeenCalled();
});
