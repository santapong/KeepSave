import { beforeEach, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { OrganizationSafetyPanel } from './OrganizationSafetyPanel';
import { getIdentityAvailability, offboardMember, previewOffboarding, sendWorkspaceInvitation, IdentityPlatformError, type Offboarding } from '../api/identityPlatform';
import type { Organization, OrgMember } from '../types';

vi.mock('../api/identityPlatform', async (original) => ({ ...await original<typeof import('../api/identityPlatform')>(), getIdentityAvailability: vi.fn(), offboardMember: vi.fn(), previewOffboarding: vi.fn(), sendWorkspaceInvitation: vi.fn() }));
const org: Organization = { id: 'org', owner_id: 'owner', name: 'Fixture workspace', slug: 'fixture', created_at: '', updated_at: '' };
const members: OrgMember[] = ['owner', 'teammate'].map((user_id) => ({ id: user_id, organization_id: org.id, user_id, role: 'admin', created_at: '', updated_at: '' }));
const impact: Offboarding = { preview_id: 'preview', preview_expires_at: '2099-10-02T12:00:00Z', organization_id: org.id, user_id: 'teammate', authority_epoch: 4, keys: 2, leases: 3, runs: 1, status: 'preview', projects_requiring_reassignment: [{ project_id: 'project', name: 'Owned project', consequence: 'Stored ownership requires reassignment.' }], credentials_requiring_reassignment: [{ project_id: 'project', secret_id: 'secret', key: 'APP_TOKEN', environment: 'production', lifecycle_revision: 2, secret_revision: 4, consequence: 'Credential responsibility requires reassignment.' }] };
beforeEach(() => { vi.clearAllMocks(); vi.mocked(getIdentityAvailability).mockResolvedValue({ identity: true, email: true }); vi.mocked(previewOffboarding).mockResolvedValue({ offboarding: impact }); });
async function openReview() { const buttons = await screen.findAllByRole('button', { name: 'Review offboarding' }); expect(buttons[0]).toBeDisabled(); fireEvent.click(buttons[1]); await screen.findByRole('heading', { name: 'Review affected access' }); }
async function confirm(label = 'Confirm offboarding') { fireEvent.click(screen.getByRole('button', { name: label })); fireEvent.click(await screen.findByRole('button', { name: 'Revoke workspace access' })); }

it('shows retained ownership and retries an uncertain execution with the same key before refreshing membership', async () => {
  const onChanged = vi.fn().mockResolvedValue(undefined);
  vi.mocked(offboardMember).mockRejectedValueOnce(new Error('Connection closed')).mockResolvedValueOnce({ offboarding: { ...impact, id: 'receipt', status: 'locally_revoked' } });
  render(<OrganizationSafetyPanel organization={org} members={members} isAdmin onChanged={onChanged} />); await openReview();
  expect(screen.getByText('Owned project')).toBeInTheDocument(); expect(screen.getByText('APP_TOKEN · production')).toBeInTheDocument();
  await confirm(); expect(await screen.findByRole('alert')).toHaveTextContent('Revocation was not confirmed'); expect(onChanged).not.toHaveBeenCalled();
  const key = vi.mocked(offboardMember).mock.calls[0][1]; expect(key).toMatch(/^[a-f\d-]{36}$/i);
  await confirm('Retry the same review'); await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
  expect(offboardMember).toHaveBeenLastCalledWith(impact, key); expect(await screen.findByText(/Local revocation receipt receipt/)).toBeInTheDocument();
});
it('requires a fresh review when affected inventory changes and never refreshes membership on rejection', async () => {
  const onChanged = vi.fn(); vi.mocked(offboardMember).mockRejectedValue(new IdentityPlatformError(409, 'stale review'));
  render(<OrganizationSafetyPanel organization={org} members={members} isAdmin onChanged={onChanged} />); await openReview(); await confirm();
  expect(await screen.findByRole('alert')).toHaveTextContent('Prepare a new review'); expect(screen.queryByRole('heading', { name: 'Review affected access' })).not.toBeInTheDocument(); expect(onChanged).not.toHaveBeenCalled();
});
it('gates invitations on email readiness and protects the organization owner', async () => {
  vi.mocked(getIdentityAvailability).mockResolvedValue({ identity: true, email: false });
  render(<OrganizationSafetyPanel organization={org} members={members} isAdmin onChanged={vi.fn()} />);
  expect(await screen.findByRole('button', { name: 'Send invitation' })).toBeDisabled(); const buttons = screen.getAllByRole('button', { name: 'Review offboarding' }); expect(buttons[0]).toBeDisabled(); fireEvent.click(buttons[0]); expect(previewOffboarding).not.toHaveBeenCalled(); expect(sendWorkspaceInvitation).not.toHaveBeenCalled();
});
it('does not carry an unfinished review into another organization', async () => {
  let settle!: (value: { offboarding: Offboarding }) => void; vi.mocked(previewOffboarding).mockReturnValueOnce(new Promise((resolve) => { settle = resolve; }));
  const { rerender } = render(<OrganizationSafetyPanel organization={org} members={members} isAdmin onChanged={vi.fn()} />);
  fireEvent.click((await screen.findAllByRole('button', { name: 'Review offboarding' }))[1]);
  rerender(<OrganizationSafetyPanel organization={{ ...org, id: 'other-org' }} members={members} isAdmin onChanged={vi.fn()} />);
  settle({ offboarding: impact }); await waitFor(() => expect(screen.getAllByRole('button', { name: 'Review offboarding' })[1]).not.toBeDisabled());
  expect(screen.queryByRole('heading', { name: 'Review affected access' })).not.toBeInTheDocument(); expect(offboardMember).not.toHaveBeenCalled();
});
