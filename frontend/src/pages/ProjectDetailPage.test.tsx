import { beforeEach, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useNavigate } from 'react-router-dom';
import { ProjectDetailPage } from './ProjectDetailPage';
import * as client from '../api/client';
import * as team from '../api/teamVault';

vi.mock('../api/client', async () => ({ ...await vi.importActual('../api/client'), getProject: vi.fn(), listSecrets: vi.fn(), exportEnv: vi.fn() }));
vi.mock('../api/teamVault', () => ({ listLifecycle: vi.fn(), searchAudit: vi.fn() }));
vi.mock('../hooks/useCapabilities', () => ({ useCapabilities: () => ({ enabled: () => true }) }));

const project = (id: string) => ({ id, name: `Project ${id}`, description: 'Synthetic fixture', created_at: '2026-10-02T00:00:00Z' });
function Fixture({ path }: { path: string }) {
  return <MemoryRouter initialEntries={[path]}><Routes><Route path="/projects/:id/*" element={<><SwitchProject /><ProjectDetailPage /></>} /></Routes></MemoryRouter>;
}
function SwitchProject() {
  const navigate = useNavigate();
  return <button onClick={() => navigate('/projects/b/lifecycle')}>Switch project fixture</button>;
}
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(client.getProject).mockImplementation(async id => project(id) as never);
  vi.mocked(client.listSecrets).mockResolvedValue([]);
  vi.mocked(team.listLifecycle).mockResolvedValue({ records: [] });
  vi.mocked(team.searchAudit).mockResolvedValue({ entries: [] });
});

it('opens lifecycle and audit directly without requesting credential values', async () => {
  render(<Fixture path="/projects/a/lifecycle" />);
  await screen.findByRole('heading', { name: 'Credential lifecycle' });
  await waitFor(() => expect(team.listLifecycle).toHaveBeenCalledWith('a'));
  fireEvent.click(screen.getByRole('link', { name: 'Audit', exact: true }));
  await screen.findByRole('heading', { name: 'Audit activity' });
  await waitFor(() => expect(team.searchAudit).toHaveBeenCalled());
  expect(client.listSecrets).not.toHaveBeenCalled();
  expect(client.exportEnv).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('link', { name: 'Secrets', exact: true }));
  await waitFor(() => expect(client.listSecrets).toHaveBeenCalledWith('a', 'alpha'));
});

it('ignores a previous project response after navigation', async () => {
  let resolveOld!: (value: never) => void;
  vi.mocked(client.getProject).mockImplementation(id => id === 'a' ? new Promise(resolve => { resolveOld = resolve; }) : Promise.resolve(project(id) as never));
  render(<Fixture path="/projects/a/lifecycle" />);
  await waitFor(() => expect(client.getProject).toHaveBeenCalledWith('a'));
  fireEvent.click(screen.getByRole('button', { name: 'Switch project fixture' }));
  await screen.findByRole('heading', { name: 'Project b', exact: true });
  await act(async () => { resolveOld(project('a') as never); });
  await waitFor(() => expect(screen.getByRole('heading', { name: 'Project b', exact: true })).toBeInTheDocument());
  expect(screen.queryByRole('heading', { name: 'Project a', exact: true })).not.toBeInTheDocument();
  expect(client.listSecrets).not.toHaveBeenCalled();
});
