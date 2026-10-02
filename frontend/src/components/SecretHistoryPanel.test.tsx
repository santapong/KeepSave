import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { SecretHistoryPanel } from './SecretHistoryPanel';
import { listSecretHistory, restoreSecretVersion } from '../api/client';
vi.mock('../api/client', () => ({ listSecretHistory: vi.fn(), restoreSecretVersion: vi.fn() }));
vi.mock('./TypedConfirmModal', () => ({ TypedConfirmModal: ({ open, onConfirm }: { open: boolean; onConfirm: () => void }) => open ? <button onClick={onConfirm}>Confirm restoration</button> : null }));
const secret = { id: 's', project_id: 'p', environment_id: 'e', key: 'DATABASE_URL', revision: 3, value: 'secret-current-never-in-history', created_at: '', updated_at: '' };
beforeEach(() => { vi.clearAllMocks(); vi.mocked(listSecretHistory).mockResolvedValue([{ revision: 1, operation: 'baseline', created_at: '2026-10-01T00:00:00Z' }, { revision: 3, operation: 'update', created_at: '2026-10-01T01:00:00Z' }]); });
it('restores the chosen metadata revision with the expected current revision', async () => {
 const restored = vi.fn(); vi.mocked(restoreSecretVersion).mockResolvedValue({ id: 's', project_id: 'p', environment_id: 'e', environment: 'alpha', key: secret.key, revision: 4, deleted: false, created_at: '', updated_at: '' });
 render(<SecretHistoryPanel projectId="p" secret={secret} onClose={vi.fn()} onRestored={restored} />);
 fireEvent.click(await screen.findByRole('button', { name: 'Restore this revision' })); expect(restoreSecretVersion).not.toHaveBeenCalled();
 expect(screen.queryByText(secret.value)).not.toBeInTheDocument(); fireEvent.click(screen.getByRole('button', { name: 'Confirm restoration' }));
 await waitFor(() => expect(restored).toHaveBeenCalled()); expect(restoreSecretVersion).toHaveBeenCalledWith('p', 's', 1, 3);
});
it('shows a stale restore without replaying the mutation', async () => {
 vi.mocked(restoreSecretVersion).mockRejectedValue(new Error('conflict with current state')); const restored = vi.fn();
 render(<SecretHistoryPanel projectId="p" secret={secret} onClose={vi.fn()} onRestored={restored} />);
 fireEvent.click(await screen.findByRole('button', { name: 'Restore this revision' })); fireEvent.click(screen.getByRole('button', { name: 'Confirm restoration' }));
 expect(await screen.findByRole('alert')).toHaveTextContent('never retried automatically'); expect(restoreSecretVersion).toHaveBeenCalledTimes(1); expect(restored).not.toHaveBeenCalled();
});
