import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setToken, clearToken, isAuthenticated } from './client';

describe('API Client Auth', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('isAuthenticated returns false when no token', () => {
    expect(isAuthenticated()).toBe(false);
  });

  it('isAuthenticated returns true after setToken', () => {
    setToken('test-jwt-token');
    expect(isAuthenticated()).toBe(true);
    expect(localStorage.getItem('keepsave_token')).toBe('test-jwt-token');
  });

  it('clearToken removes token and auth state', () => {
    setToken('test-jwt-token');
    clearToken();
    expect(isAuthenticated()).toBe(false);
    expect(localStorage.getItem('keepsave_token')).toBeNull();
  });
});

describe('API Client requests', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('login sends correct request and returns auth response', async () => {
    const mockResponse = {
      user: { id: '123', email: 'test@example.com', created_at: '', updated_at: '' },
      token: 'jwt-token',
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockResponse),
    });

    const { login } = await import('./client');
    const result = await login('test@example.com', 'password123');

    expect(global.fetch).toHaveBeenCalledWith(
      '/api/v1/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'test@example.com', password: 'password123' }),
      })
    );
    expect(result.token).toBe('jwt-token');
    expect(result.user.email).toBe('test@example.com');
  });

  it('listProjects sends auth header when token is set', async () => {
    setToken('my-token');

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ projects: [] }),
    });

    const { listProjects } = await import('./client');
    await listProjects();

    expect(global.fetch).toHaveBeenCalledWith(
      '/api/v1/projects',
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer my-token',
        }),
      })
    );
  });

  it('throws error on non-OK response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ error: 'invalid credentials' }),
    });

    const { login } = await import('./client');
    await expect(login('bad@example.com', 'wrong')).rejects.toThrow('invalid credentials');
  });

  it('handles 204 No Content responses', async () => {
    setToken('my-token');

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    const { deleteProject } = await import('./client');
    await expect(deleteProject('some-id')).resolves.toBeUndefined();
  });
});
