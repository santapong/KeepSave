import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BrowserRouter } from 'react-router-dom';
import { MCPConsentPage } from './MCPConsentPage';
import { decideMCPConsent, getMCPConsent } from '../api/mcpConsent';
vi.mock('../api/client', () => ({ getAuthToken: () => 'synthetic-human' }));
vi.mock('../api/mcpConsent', () => ({ getMCPConsent: vi.fn(), decideMCPConsent: vi.fn(), verifiedConsentCallback: () => { throw new Error('The client callback could not be verified.'); } }));
const id = '55555555-1111-2222-3333-444444444444';
const consent = { request_id: id, client_id: 'keepsave-hermes-linux-v1', harness: 'hermes' as const, resource: 'https://app.keepsave.example/mcp', redirect_uri: 'http://127.0.0.1:17702/callback', scope: 'keepsave:tools offline_access', expires_at: new Date(Date.now() + 300000).toISOString() };
beforeEach(() => { vi.clearAllMocks(); window.history.replaceState({}, '', `/mcp/consent?request_id=${id}`); vi.mocked(getMCPConsent).mockResolvedValue(consent); });
it('asks for sign-in and preserves only the validated consent request', () => {
  render(<BrowserRouter><MCPConsentPage authenticated={false} /></BrowserRouter>);
  expect(screen.getByRole('link', { name: /Sign in to review/ })).toHaveAttribute('href', `/login?next=${encodeURIComponent(`/mcp/consent?request_id=${id}`)}`);
  expect(getMCPConsent).not.toHaveBeenCalled(); expect(decideMCPConsent).not.toHaveBeenCalled();
});
it('shows bounded read permissions and requires an explicit human decision', async () => {
  const user = userEvent.setup();
  vi.mocked(decideMCPConsent).mockResolvedValue({ redirect_url: 'https://evil.example' });
  render(<BrowserRouter><MCPConsentPage authenticated email="owner@example.invalid" /></BrowserRouter>);
  expect(await screen.findByRole('heading', { name: 'Connect Hermes?' })).toBeInTheDocument();
  expect(screen.getByText(/Read approved repository files/)).toBeInTheDocument(); expect(decideMCPConsent).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Deny' }));
  expect(decideMCPConsent).toHaveBeenCalledWith(id, false); expect(await screen.findByRole('alert')).toHaveTextContent('callback could not be verified');
});
it('refuses expired requests without issuing a decision', async () => {
  vi.mocked(getMCPConsent).mockResolvedValue({ ...consent, expires_at: new Date(0).toISOString() });
  render(<BrowserRouter><MCPConsentPage authenticated /></BrowserRouter>);
  expect(await screen.findByRole('button', { name: /Allow connection/ })).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Deny' })).toBeDisabled(); expect(decideMCPConsent).not.toHaveBeenCalled();
});
