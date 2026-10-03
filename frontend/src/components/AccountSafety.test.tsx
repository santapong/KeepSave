import { beforeEach, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { AccountSafety } from './AccountSafety';
import { getAccountMethods, getIdentityAvailability, getProofDelivery, getVerifiedContacts, requestContactProof, requestFreshProof } from '../api/identityPlatform';
vi.mock('../api/identityPlatform', async (importOriginal) => ({ ...await importOriginal<typeof import('../api/identityPlatform')>(), getAccountMethods: vi.fn(), getIdentityAvailability: vi.fn(), getProofDelivery: vi.fn(), getVerifiedContacts: vi.fn(), requestContactProof: vi.fn(), requestFreshProof: vi.fn(), removeAccountMethod: vi.fn() }));
beforeEach(() => { vi.clearAllMocks(); vi.mocked(getIdentityAvailability).mockResolvedValue({ identity: true, email: true }); vi.mocked(getAccountMethods).mockResolvedValue({ methods: [{ name: 'password', usable: true }] }); vi.mocked(getVerifiedContacts).mockResolvedValue({ contacts: [] }); });
it('prevents removing the last usable method', async () => { render(<AccountSafety />); expect(await screen.findByRole('button', { name: 'Remove Password' })).toBeDisabled(); });
it('offers an explicit fresh proof after uncertainty and never resends automatically', async () => {
  vi.mocked(requestContactProof).mockResolvedValue({ request: { id: 'nonsecret-id', status: 'pending' } }); vi.mocked(getProofDelivery).mockResolvedValue({ delivery: { state: 'uncertain', reason_code: 'smtp_acceptance_unknown' } }); vi.mocked(requestFreshProof).mockResolvedValue({ request: { id: 'new-nonsecret-id', status: 'pending' } });
  render(<AccountSafety />); await screen.findByRole('button', { name: 'Remove Password' }); fireEvent.change(screen.getByLabelText('Recovery email'), { target: { value: 'safe@example.invalid' } }); fireEvent.click(screen.getByRole('button', { name: 'Request confirmation' }));
  fireEvent.click(await screen.findByRole('button', { name: 'Check delivery' })); expect(await screen.findByText(/will not resend this email automatically/)).toBeInTheDocument(); expect(requestFreshProof).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Request a fresh confirmation' })); await waitFor(() => expect(requestFreshProof).toHaveBeenCalledWith('nonsecret-id')); expect(await screen.findByText(/Use only the newest email/)).toBeInTheDocument();
});
it('does not query protected inventory when identity is disabled', async () => { vi.mocked(getIdentityAvailability).mockResolvedValue({ identity: false, email: false }); render(<AccountSafety />); expect(await screen.findByText('Account safety is awaiting server setup.')).toBeInTheDocument(); expect(getAccountMethods).not.toHaveBeenCalled(); expect(getVerifiedContacts).not.toHaveBeenCalled(); });
