import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SecretsPanel } from './SecretsPanel';

vi.mock('../api/client', () => ({
  listSecrets: vi.fn(),
  createSecret: vi.fn(),
  updateSecret: vi.fn(),
  deleteSecret: vi.fn(),
}));

import { listSecrets, createSecret } from '../api/client';

const mockSecrets = [
  {
    id: 's1',
    project_id: 'p1',
    environment_id: 'e1',
    key: 'DATABASE_URL',
    value: 'postgres://localhost/db',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 's2',
    project_id: 'p1',
    environment_id: 'e1',
    key: 'API_KEY',
    value: 'secret-value',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
];

describe('SecretsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (listSecrets as ReturnType<typeof vi.fn>).mockResolvedValue(mockSecrets);
    (createSecret as ReturnType<typeof vi.fn>).mockResolvedValue({
      id: 's3',
      project_id: 'p1',
      environment_id: 'e1',
      key: 'NEW_KEY',
      value: 'new-value',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    });
  });

  it('renders environment tabs', async () => {
    render(<SecretsPanel projectId="p1" />);
    expect(screen.getByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('uat')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
  });

  it('loads and displays secrets', async () => {
    render(<SecretsPanel projectId="p1" />);
    await waitFor(() => {
      expect(screen.getByText('DATABASE_URL')).toBeInTheDocument();
      expect(screen.getByText('API_KEY')).toBeInTheDocument();
    });
    expect(listSecrets).toHaveBeenCalledWith('p1', 'alpha');
  });

  it('switches environments when tab is clicked', async () => {
    const user = userEvent.setup();
    render(<SecretsPanel projectId="p1" />);

    await waitFor(() => {
      expect(screen.getByText('DATABASE_URL')).toBeInTheDocument();
    });

    await user.click(screen.getByText('uat'));
    expect(listSecrets).toHaveBeenCalledWith('p1', 'uat');
  });

  it('reveals and hides secret values', async () => {
    const user = userEvent.setup();
    render(<SecretsPanel projectId="p1" />);

    await waitFor(() => {
      expect(screen.getByText('DATABASE_URL')).toBeInTheDocument();
    });

    const revealButtons = screen.getAllByText('Reveal');
    await user.click(revealButtons[0]);

    expect(screen.getByText('postgres://localhost/db')).toBeInTheDocument();

    await user.click(screen.getByText('Hide'));
    expect(screen.queryByText('postgres://localhost/db')).not.toBeInTheDocument();
  });

  it('shows add secret form and creates secret', async () => {
    const user = userEvent.setup();
    render(<SecretsPanel projectId="p1" />);

    await waitFor(() => {
      expect(screen.getByText('DATABASE_URL')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Add Secret'));

    const keyInput = screen.getByPlaceholderText('KEY_NAME');
    const valueInput = screen.getByPlaceholderText('Secret value');

    await user.type(keyInput, 'NEW_KEY');
    await user.type(valueInput, 'new-value');
    await user.click(screen.getByText('Save'));

    expect(createSecret).toHaveBeenCalledWith('p1', 'NEW_KEY', 'new-value', 'alpha');
  });

  it('displays empty state when no secrets', async () => {
    (listSecrets as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    render(<SecretsPanel projectId="p1" />);

    await waitFor(() => {
      expect(screen.getByText(/No secrets in ALPHA environment/)).toBeInTheDocument();
    });
  });

  it('displays error when loading fails', async () => {
    (listSecrets as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));
    render(<SecretsPanel projectId="p1" />);

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });
  });
});
