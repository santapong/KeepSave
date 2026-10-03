import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { ProjectsPage } from './ProjectsPage';

vi.mock('../api/client', () => ({
  listProjects: vi.fn(),
  createProject: vi.fn(),
  deleteProject: vi.fn(),
  importEnv: vi.fn(),
}));

import { listProjects, createProject, importEnv } from '../api/client';

const mockProjects = [
  {
    id: 'p1abcdef',
    name: 'My App',
    description: 'A test project',
    owner_id: 'u1',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 'p2abcdef',
    name: 'Backend Service',
    description: '',
    owner_id: 'u1',
    created_at: '2026-01-02T00:00:00Z',
    updated_at: '2026-01-02T00:00:00Z',
  },
];

describe('ProjectsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (listProjects as ReturnType<typeof vi.fn>).mockResolvedValue(mockProjects);
    (createProject as ReturnType<typeof vi.fn>).mockResolvedValue(mockProjects[0]);
  });

  function renderPage() {
    return render(
      <MemoryRouter>
        <ProjectsPage />
      </MemoryRouter>
    );
  }

  it('renders project list', async () => {
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('My App')).toBeInTheDocument();
      expect(screen.getByText('Backend Service')).toBeInTheDocument();
    });
  });

  it('shows empty state when no projects', async () => {
    (listProjects as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/your first project starts here/i)).toBeInTheDocument();
    });
  });

  it('opens create form and creates project', async () => {
    const user = userEvent.setup();
    renderPage();
    await waitFor(() => expect(screen.getByText('My App')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'New project', exact: true }));
    await user.type(screen.getByPlaceholderText(/storefront-api/i), 'New Project');
    await user.type(screen.getByPlaceholderText(/optional/i), 'desc');
    await user.click(screen.getByRole('button', { name: 'Create project' }));

    expect(createProject).toHaveBeenCalledWith('New Project', 'desc');
  });

  it('displays project descriptions', async () => {
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('A test project')).toBeInTheDocument();
    });
  });

  it('filters descriptions and restores projects after clearing an empty search', async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText('My App');
    await user.type(screen.getByRole('textbox', { name: 'Filter projects' }), 'test project');
    expect(screen.getByRole('link', { name: 'Open My App' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Open Backend Service' })).not.toBeInTheDocument();
    await user.clear(screen.getByRole('textbox', { name: 'Filter projects' }));
    await user.type(screen.getByRole('textbox', { name: 'Filter projects' }), 'missing');
    expect(screen.getByText('No matching projects')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Clear search', exact: true }));
    expect(screen.getAllByRole('link', { name: /^Open / })).toHaveLength(2);
  });

  it('sorts projects and preserves project navigation in list view', async () => {
    const user = userEvent.setup();
    vi.mocked(listProjects).mockResolvedValue([
      { ...mockProjects[0], name: 'Alpha', updated_at: '2026-01-03T00:00:00Z' },
      { ...mockProjects[1], name: 'Zeta' },
    ]);
    renderPage();
    await screen.findByRole('link', { name: 'Open Alpha' });
    expect(screen.getAllByRole('link', { name: /^Open / })[0]).toHaveAttribute('aria-label', 'Open Alpha');
    await user.selectOptions(screen.getByRole('combobox', { name: 'Sort projects' }), 'created');
    expect(screen.getAllByRole('link', { name: /^Open / })[0]).toHaveAttribute('aria-label', 'Open Zeta');
    await user.selectOptions(screen.getByRole('combobox', { name: 'Sort projects' }), 'name');
    expect(screen.getAllByRole('link', { name: /^Open / })[0]).toHaveAttribute('aria-label', 'Open Alpha');
    await user.click(screen.getByRole('button', { name: 'List view' }));
    expect(screen.getByRole('button', { name: 'List view' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('link', { name: 'Open Alpha' })).toHaveAttribute('href', '/projects/p1abcdef');
  });

  it('recovers a failed project load through Retry', async () => {
    const user = userEvent.setup();
    vi.mocked(listProjects).mockRejectedValueOnce(new Error('Connection unavailable'));
    renderPage();
    expect(await screen.findByRole('alert')).toHaveTextContent('Connection unavailable');
    expect(screen.queryByText('Your first project starts here')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByText('My App')).toBeInTheDocument();
  });
  it.each([
    { created: ['IMPORTED_KEY'], updated: null, skipped: null },
    { created: null, updated: null, skipped: ['IMPORTED_KEY'] },
    { created: null, updated: ['IMPORTED_KEY'], skipped: null },
  ])('closes a successful import when unused result arrays are null: %j', async (result) => {
    vi.mocked(importEnv).mockResolvedValue(result);
    const user = userEvent.setup({ applyAccept: false });
    renderPage();
    await screen.findByText('My App');
    await user.click(screen.getByRole('button', { name: 'Import .env', exact: true }));
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    const content = `IMPORTED_KEY=${crypto.randomUUID()}`;
    const file = new File([content], '.env', { type: 'text/plain' });
    Object.defineProperty(file, 'text', { value: async () => content });
    await user.upload(input, file);
    await waitFor(() => expect(importEnv).toHaveBeenCalled());
    await waitFor(() => expect(screen.queryByLabelText('Environment')).not.toBeInTheDocument());
  });

});
