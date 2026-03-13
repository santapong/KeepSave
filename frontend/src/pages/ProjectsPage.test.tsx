import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BrowserRouter } from 'react-router-dom';
import { ProjectsPage } from './ProjectsPage';

vi.mock('../api/client', () => ({
  listProjects: vi.fn(),
  createProject: vi.fn(),
  deleteProject: vi.fn(),
}));

import { listProjects, createProject } from '../api/client';

const mockProjects = [
  {
    id: 'p1',
    name: 'My App',
    description: 'A test project',
    owner_id: 'u1',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 'p2',
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
      expect(screen.getByText(/No projects yet/)).toBeInTheDocument();
    });
  });

  it('opens create form and creates project', async () => {
    const user = userEvent.setup();
    renderPage();
    await waitFor(() => expect(screen.getByText('My App')).toBeInTheDocument());

    await user.click(screen.getByText('New Project'));
    await user.type(screen.getByPlaceholderText('Project name'), 'New Project');
    await user.type(screen.getByPlaceholderText('Description (optional)'), 'desc');
    await user.click(screen.getByText('Create'));

    expect(createProject).toHaveBeenCalledWith('New Project', 'desc');
  });

  it('displays project descriptions', async () => {
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('A test project')).toBeInTheDocument();
    });
  });
});
