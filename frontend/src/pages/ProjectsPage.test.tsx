import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BrowserRouter } from 'react-router-dom';
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
      <BrowserRouter>
        <ProjectsPage />
      </BrowserRouter>
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
      expect(screen.getByText(/empty shelf/i)).toBeInTheDocument();
    });
  });

  it('opens create form and creates project', async () => {
    const user = userEvent.setup();
    renderPage();
    await waitFor(() => expect(screen.getByText('My App')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /\+ new project/i }));
    await user.type(screen.getByPlaceholderText(/nexus-platform/i), 'New Project');
    await user.type(screen.getByPlaceholderText(/optional/i), 'desc');
    await user.click(screen.getByRole('button', { name: /create →/i }));

    expect(createProject).toHaveBeenCalledWith('New Project', 'desc');
  });

  it('displays project descriptions', async () => {
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('A test project')).toBeInTheDocument();
    });
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
