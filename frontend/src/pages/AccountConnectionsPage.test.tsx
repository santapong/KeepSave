import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { AccountConnectionsPage } from './AccountConnectionsPage';
import { getAccountSessions, revokeAccountSession, invalidateBrowserSession } from '../api/client';
import { getConnections } from '../api/socialAuth';
vi.mock('../api/client', () => ({ getAccountSessions: vi.fn(), revokeAccountSession: vi.fn(), invalidateBrowserSession: vi.fn() }));
vi.mock('../api/socialAuth', () => ({ getConnections: vi.fn(), startSocialLogin: vi.fn() }));
const sessions = [{ id: 'current', current: true, created_at: '2026-10-01T00:00:00Z', expires_at: '2026-10-02T00:00:00Z', status: 'active' as const, ip_address: '', user_agent: '' }];
beforeEach(() => { vi.clearAllMocks(); vi.mocked(getConnections).mockResolvedValue({ connected: [], available: { github: false, google: false } }); vi.mocked(getAccountSessions).mockResolvedValue({ sessions }); });
it('shows current session and invalidates browser state after a confirmed revoke', async () => {
 vi.mocked(revokeAccountSession).mockResolvedValue(undefined); render(<BrowserRouter><AccountConnectionsPage /></BrowserRouter>);
 fireEvent.click(await screen.findByRole('button', { name: 'Sign out' }));
 await waitFor(() => expect(invalidateBrowserSession).toHaveBeenCalledTimes(1)); expect(revokeAccountSession).toHaveBeenCalledWith('current');
});
it('keeps browser state when session revocation fails', async () => {
 vi.mocked(revokeAccountSession).mockRejectedValue(new Error('unavailable')); render(<BrowserRouter><AccountConnectionsPage /></BrowserRouter>);
 fireEvent.click(await screen.findByRole('button', { name: 'Sign out' }));
 expect(await screen.findByRole('alert')).toHaveTextContent('not been confirmed'); expect(invalidateBrowserSession).not.toHaveBeenCalled();
});
