import { beforeEach, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { DeveloperAccessPage } from './DeveloperAccessPage';
import * as api from '../api/toolPlatform';
import { listProjects } from '../api/client';

vi.mock('../api/client', () => ({ listProjects: vi.fn(), getAuthToken: () => 'fixture' }));
vi.mock('../api/toolPlatform', () => ({ toolWorkspace: vi.fn(), checkToolConnection: vi.fn(), createToolRun: vi.fn(), cancelToolRun: vi.fn(), toolReceipts: vi.fn(), createToolResource: vi.fn(), approveToolResource: vi.fn() }));
const fixture = {
  artifacts: [], profiles: [], packages: [], connections: [], workloads: [], grants: [], runs: [], delegations: [],
  bindings: [{ id: 'binding', project_id: 'project', environment_id: 'environment', connection_id: 'connection', target: { repository_id: 42, owner: 'team', repository: 'allowed', reference: 'main' } }],
};
beforeEach(() => { vi.clearAllMocks(); vi.mocked(listProjects).mockResolvedValue([{ id: 'project', name: 'Team vault' } as never]); vi.mocked(api.toolWorkspace).mockResolvedValue(fixture); });
const open = () => render(<MemoryRouter initialEntries={['/developer-access?project=project']}><DeveloperAccessPage /></MemoryRouter>);
it('loads metadata without reading GitHub or creating authority', async () => {
  open(); await screen.findByRole('heading', { name: 'Review run' });
  expect(api.checkToolConnection).not.toHaveBeenCalled(); expect(api.createToolResource).not.toHaveBeenCalled(); expect(api.approveToolResource).not.toHaveBeenCalled();
});
it('discloses the external read and waits for confirmation before checking a connection', async () => {
  vi.mocked(api.checkToolConnection).mockResolvedValue({ status: 'succeeded' } as never);
  open(); await screen.findByRole('heading', { name: 'Review run' }); fireEvent.click(screen.getByRole('button', { name: 'Repository', exact: true }));
  fireEvent.click(await screen.findByRole('button', { name: 'Check connection' }));
  expect(screen.getByText(/mint a repository-scoped read token/)).toBeInTheDocument(); expect(api.checkToolConnection).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Confirm', exact: true }));
  await waitFor(() => expect(api.checkToolConnection).toHaveBeenCalledWith('project', 'connection', 'binding'));
});
it('keeps the request identifier stable after a lost run response', async () => {
  vi.mocked(api.toolWorkspace).mockResolvedValue({ ...fixture, grants: [{ id: 'grant', client_id: 'keepsave-codex-linux-v1' } as never], delegations: [{ family_id: 'family', client_id: 'keepsave-codex-linux-v1', harness: 'codex', requires_refresh: false } as never] });
  vi.mocked(api.createToolRun).mockRejectedValue(new Error('Disconnected before confirmation'));
  open(); await screen.findByRole('heading', { name: 'Review run' });
  fireEvent.change(screen.getByLabelText('Granted access'), { target: { value: 'grant' } }); fireEvent.change(screen.getByLabelText('Your connected client'), { target: { value: 'family' } });
  const key = (screen.getByLabelText('Run request identifier') as HTMLInputElement).value;
  fireEvent.click(screen.getByRole('button', { name: 'Save review run' })); await screen.findByRole('alert');
  expect((screen.getByLabelText('Run request identifier') as HTMLInputElement).value).toBe(key);
  expect(api.createToolRun).toHaveBeenCalledWith('project', expect.objectContaining({ request_key: key, family_id: 'family', client_id: 'keepsave-codex-linux-v1', duration_seconds: 600 }));
});
