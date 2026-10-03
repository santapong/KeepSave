import { beforeEach, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { SecretLifecyclePanel } from './SecretLifecyclePanel';
import { SafeAuditPanel } from './SafeAuditPanel';
import * as api from '../api/teamVault';
vi.mock('../api/teamVault', () => ({ listLifecycle: vi.fn(), updateLifecycle: vi.fn(), searchAudit: vi.fn(), createAuditExport: vi.fn(), auditExportStatus: vi.fn(), downloadAuditExport: vi.fn() }));
const lifecycle = { secret_id: 'secret', project_id: 'project', responsible_user_id: null, declared_expires_at: null, renewal_at: null, provenance: '', revision: 3, known: true, updated_at: null };
beforeEach(() => { vi.clearAllMocks(); vi.mocked(api.listLifecycle).mockResolvedValue({ records: [{ key: 'GITHUB', environment: 'alpha', lifecycle }] }); vi.mocked(api.searchAudit).mockResolvedValue({ entries: [] }); });
it('uses metadata revision and retains the edit after a stale update is rejected', async () => {
  vi.mocked(api.updateLifecycle).mockRejectedValue(new Error('Revision conflict. Reload before saving.'));
  render(<SecretLifecyclePanel projectId="project" />); fireEvent.click(await screen.findByRole('button', { name: /Edit/ }));
  fireEvent.change(screen.getByLabelText('Provenance'), { target: { value: 'GitHub App installation' } }); fireEvent.click(screen.getByRole('button', { name: 'Save lifecycle' }));
  await screen.findByRole('alert'); expect(api.updateLifecycle).toHaveBeenCalledWith('project', 'secret', expect.objectContaining({ expected_revision: 3, provenance: 'GitHub App installation' })); expect(screen.getByLabelText('Provenance')).toHaveValue('GitHub App installation');
  expect(screen.queryByText('Lifecycle saved.')).not.toBeInTheDocument();
});
it('does not allow download until the export is published', async () => {
  vi.mocked(api.createAuditExport).mockResolvedValue({ id: 'export', status: 'pending', rows: 2 } as never);
  vi.mocked(api.auditExportStatus).mockResolvedValue({ id: 'export', status: 'failed', rows: 2 } as never);
  render(<SafeAuditPanel projectId="project" />); await waitFor(() => expect(api.searchAudit).toHaveBeenCalled()); fireEvent.click(screen.getByRole('button', { name: 'Create safe metadata export' }));
  await screen.findByText(/Export pending/); expect(screen.queryByRole('button', { name: 'Download export' })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Check status' })); await screen.findByText(/Export failed/); expect(api.downloadAuditExport).not.toHaveBeenCalled();
});
