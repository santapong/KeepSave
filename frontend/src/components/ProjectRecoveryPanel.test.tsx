import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { ProjectRecoveryPanel } from './ProjectRecoveryPanel';
import { verifyEncryptedBackup, previewBackupRestore, restoreBackupRecords } from '../api/client';
vi.mock('../api/client', () => ({ createEncryptedBackup: vi.fn(), verifyEncryptedBackup: vi.fn(), previewBackupRestore: vi.fn(), restoreBackupRecords: vi.fn() }));
vi.mock('./TypedConfirmModal', () => ({ TypedConfirmModal: ({ open, onConfirm }: { open: boolean; onConfirm: () => void }) => open ? <button onClick={onConfirm}>Confirm selected restore</button> : null }));
const bundle = { format: 'keepsave.encrypted-vault.v1', ciphertext: 'synthetic-encrypted-only', nonce: 'synthetic', sha256: 'synthetic' };
beforeEach(() => {
 vi.clearAllMocks();
 vi.mocked(verifyEncryptedBackup).mockResolvedValue({ project_id: 'p', created_at: '2026-10-01T00:00:00Z', entries: 2, revisions: 4, keys: 1, snapshots: 0 });
 vi.mocked(previewBackupRestore).mockResolvedValue({ project_id: 'p', backup_created_at: '2026-10-01T00:00:00Z', records: [{ secret_id: 'a', environment: 'alpha', key: 'ACTIVE', backup_revision: 2, current_revision: 3, status: 'different', restorable: true }, { secret_id: 'd', environment: 'alpha', key: 'DELETED', backup_revision: 1, current_revision: 0, status: 'deleted', restorable: false }] });
});
async function loadPreview() {
 render(<ProjectRecoveryPanel projectId="p" projectName="project" />);
 const file = new File([JSON.stringify(bundle)], 'encrypted.json', { type: 'application/json' }); Object.defineProperty(file, 'text', { value: async () => JSON.stringify(bundle) });
 fireEvent.change(screen.getByLabelText('Select an encrypted KeepSave backup'), { target: { files: [file] } });
 await waitFor(() => expect(screen.getByRole('button', { name: 'Verify and preview' })).toBeEnabled()); fireEvent.click(screen.getByRole('button', { name: 'Verify and preview' }));
 await screen.findByRole('checkbox', { name: 'Restore ACTIVE in alpha' });
}
it('requires a selected eligible record and confirmation, then sends only exact revisions', async () => {
 vi.mocked(restoreBackupRecords).mockResolvedValue({ records: [] }); await loadPreview();
 expect(screen.getByRole('checkbox', { name: 'Restore DELETED in alpha' })).toBeDisabled();
 expect(screen.getByRole('button', { name: 'Restore 0 selected records' })).toBeDisabled(); expect(restoreBackupRecords).not.toHaveBeenCalled();
 fireEvent.click(screen.getByRole('checkbox', { name: 'Restore ACTIVE in alpha' })); fireEvent.click(screen.getByRole('button', { name: 'Restore 1 selected records' })); expect(restoreBackupRecords).not.toHaveBeenCalled();
 fireEvent.click(screen.getByRole('button', { name: 'Confirm selected restore' }));
 await waitFor(() => expect(restoreBackupRecords).toHaveBeenCalledWith('p', bundle, [{ secret_id: 'a', backup_revision: 2, expected_current_revision: 3 }]));
});
it('discards stale selection after restore failure and requires another preview', async () => {
 vi.mocked(restoreBackupRecords).mockRejectedValue(new Error('conflict')); await loadPreview();
 fireEvent.click(screen.getByRole('checkbox', { name: 'Restore ACTIVE in alpha' })); fireEvent.click(screen.getByRole('button', { name: 'Restore 1 selected records' })); fireEvent.click(screen.getByRole('button', { name: 'Confirm selected restore' }));
 expect(await screen.findByRole('alert')).toHaveTextContent('Verify and preview again'); expect(screen.queryByRole('checkbox')).not.toBeInTheDocument(); expect(restoreBackupRecords).toHaveBeenCalledTimes(1);
});
