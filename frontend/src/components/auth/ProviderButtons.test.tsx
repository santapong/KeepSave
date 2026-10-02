import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProviderButtons } from './ProviderButtons';
import { getProviders, startSocialLogin } from '../../api/socialAuth';
vi.mock('../../api/socialAuth', () => ({ getProviders: vi.fn(), startSocialLogin: vi.fn() }));
beforeEach(() => { vi.clearAllMocks(); });
it('explains unavailable providers and keeps them disabled', async () => {
 vi.mocked(getProviders).mockResolvedValue({ github: false, google: false }); render(<ProviderButtons />);
 expect(await screen.findByText(/awaiting setup/)).toBeInTheDocument();
 expect(screen.getByRole('button', { name: /GitHub/ })).toBeDisabled(); expect(screen.getByRole('button', { name: /Google/ })).toBeDisabled();
});
it('starts the selected provider and recovers after an error', async () => {
 vi.mocked(getProviders).mockResolvedValue({ github: true, google: true }); vi.mocked(startSocialLogin).mockRejectedValue(new Error('Please try again.'));
 render(<ProviderButtons />); const button = screen.getByRole('button', { name: /Google/ });
 await waitFor(() => expect(button).toBeEnabled()); await userEvent.click(button);
 expect(startSocialLogin).toHaveBeenCalledWith('google'); expect(await screen.findByRole('alert')).toHaveTextContent('Please try again.'); expect(button).toBeEnabled();
});
it('explains service failure without blocking the email flow', async () => {
 vi.mocked(getProviders).mockRejectedValue(new Error('offline')); render(<ProviderButtons />);
 expect(await screen.findByRole('alert')).toHaveTextContent('continue with email');
});
