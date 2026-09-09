import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BrowserRouter } from 'react-router-dom';
import { LoginPage } from './LoginPage';

vi.mock('../api/client', () => ({
  login: vi.fn(),
}));

import { login as apiLogin } from '../api/client';

describe('LoginPage', () => {
  const mockOnLogin = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  function renderLogin() {
    return render(
      <BrowserRouter>
        <LoginPage onLogin={mockOnLogin} />
      </BrowserRouter>
    );
  }

  it('renders login form', () => {
    renderLogin();
    expect(screen.getByText(/your orbit/i)).toBeInTheDocument();
    expect(screen.getByText(/sign in/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /enter the vault/i })).toBeInTheDocument();
  });

  it('calls onLogin on successful login', async () => {
    const user = userEvent.setup();
    const mockUser = { id: '1', email: 'test@test.com', created_at: '', updated_at: '' };
    (apiLogin as ReturnType<typeof vi.fn>).mockResolvedValue({
      user: mockUser,
      token: 'jwt-token',
    });

    renderLogin();

    await user.type(screen.getByLabelText(/email/i), 'test@test.com');
    await user.type(screen.getByLabelText(/password/i), 'password123');
    await user.click(screen.getByRole('button', { name: /enter the vault/i }));

    await waitFor(() => {
      expect(mockOnLogin).toHaveBeenCalledWith(mockUser, 'jwt-token');
    });
  });

  it('shows error message on failed login', async () => {
    const user = userEvent.setup();
    (apiLogin as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('invalid credentials'));

    renderLogin();

    await user.type(screen.getByLabelText(/email/i), 'bad@test.com');
    await user.type(screen.getByLabelText(/password/i), 'wrongpass');
    await user.click(screen.getByRole('button', { name: /enter the vault/i }));

    await waitFor(() => {
      expect(screen.getByText('invalid credentials')).toBeInTheDocument();
    });
  });

  it('has link to register page', () => {
    renderLogin();
    expect(screen.getByText(/open an account/i)).toBeInTheDocument();
  });
});
