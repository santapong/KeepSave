import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setToken, clearToken, isAuthenticated, migrateLegacyJWTKey, JWT_STORAGE_KEY } from './client';

// Helper: create a fake JWT with a future exp claim
function makeFakeJWT(expSeconds: number): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + expSeconds }));
  return `${header}.${payload}.fake-signature`;
}

describe('API Client Auth', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it('isAuthenticated returns false when no token', () => {
    expect(isAuthenticated()).toBe(false);
  });

  it('isAuthenticated returns true after setToken with valid JWT', () => {
    const token = makeFakeJWT(3600); // expires in 1 hour
    setToken(token);
    expect(isAuthenticated()).toBe(true);
    // Token is stored tab-scoped in sessionStorage, not persistent localStorage.
    expect(sessionStorage.getItem('keepsave_token')).toBe(token);
    expect(localStorage.getItem('keepsave_token')).toBeNull();
  });

  it('isAuthenticated returns false for expired JWT', () => {
    const token = makeFakeJWT(-100); // already expired
    setToken(token);
    expect(isAuthenticated()).toBe(false);
  });

  it('clearToken removes token and auth state', () => {
    const token = makeFakeJWT(3600);
    setToken(token);
    clearToken();
    expect(isAuthenticated()).toBe(false);
    expect(sessionStorage.getItem('keepsave_token')).toBeNull();
  });

  it('JWT_STORAGE_KEY is the canonical key', () => {
    expect(JWT_STORAGE_KEY).toBe('keepsave_token');
  });

  it('migrateLegacyJWTKey copies legacy `jwt` value into the session store', () => {
    const token = makeFakeJWT(3600);
    localStorage.setItem('jwt', token);
    migrateLegacyJWTKey();
    expect(sessionStorage.getItem('keepsave_token')).toBe(token);
    expect(localStorage.getItem('jwt')).toBeNull();
  });

  it('migrateLegacyJWTKey moves a persisted localStorage token into sessionStorage', () => {
    const token = makeFakeJWT(3600);
    // Simulate an already-signed-in user from the old localStorage scheme.
    localStorage.setItem('keepsave_token', token);
    migrateLegacyJWTKey();
    expect(sessionStorage.getItem('keepsave_token')).toBe(token);
    expect(localStorage.getItem('keepsave_token')).toBeNull();
    expect(isAuthenticated()).toBe(true);
  });

  it('migrateLegacyJWTKey is a no-op when canonical key already exists', () => {
    const token = makeFakeJWT(3600);
    setToken(token);
    localStorage.setItem('jwt', 'leftover');
    migrateLegacyJWTKey();
    expect(sessionStorage.getItem('keepsave_token')).toBe(token);
    // legacy key is still cleaned up to prevent confusion
    expect(localStorage.getItem('jwt')).toBeNull();
  });

  it('migrateLegacyJWTKey does nothing when no keys are set', () => {
    migrateLegacyJWTKey();
    expect(sessionStorage.getItem('keepsave_token')).toBeNull();
    expect(localStorage.getItem('keepsave_token')).toBeNull();
  });
});

describe('API Client requests', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
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
    setToken(makeFakeJWT(3600));

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
          Authorization: expect.stringContaining('Bearer '),
        }),
      })
    );
  });

  it('throws error on non-OK response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ error: 'invalid credentials' }),
    });

    const { login } = await import('./client');
    await expect(login('bad@example.com', 'wrong')).rejects.toThrow('invalid credentials');
  });

  it('throws session expired on 401 response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ error: 'unauthorized' }),
    });

    const { listProjects } = await import('./client');
    await expect(listProjects()).rejects.toThrow('Session expired');
  });

  it('handles 204 No Content responses', async () => {
    setToken(makeFakeJWT(3600));

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    const { deleteProject } = await import('./client');
    await deleteProject('123');

    expect(global.fetch).toHaveBeenCalledWith(
      '/api/v1/projects/123',
      expect.objectContaining({ method: 'DELETE' })
    );
  });
});


describe('server session controls', () => {
  beforeEach(() => { localStorage.clear(); sessionStorage.clear(); vi.restoreAllMocks(); });
  it('retains identity and provider proofs when logout is unavailable', async () => {
    const { logoutCurrentSession, getAuthToken } = await import('./client');
    setToken(makeFakeJWT(3600)); const token = getAuthToken();
    localStorage.setItem('keepsave_user', '{"id":"owner"}');
    sessionStorage.setItem('keepsave_oauth:pending', 'proof');
    global.fetch = vi.fn().mockResolvedValue({ status: 503, ok: false, json: async () => ({ error: 'unavailable' }) });
    await expect(logoutCurrentSession()).rejects.toThrow('unavailable');
    expect(getAuthToken()).toBe(token); expect(localStorage.getItem('keepsave_user')).not.toBeNull();
    expect(sessionStorage.getItem('keepsave_oauth:pending')).toBe('proof');
  });
  it('clears identity and link proofs only after confirmed logout', async () => {
    const { logoutCurrentSession, getAuthToken } = await import('./client');
    setToken(makeFakeJWT(3600)); localStorage.setItem('keepsave_user', '{"id":"owner"}');
    sessionStorage.setItem('keepsave_oauth:pending', 'proof');
    global.fetch = vi.fn().mockResolvedValue({ status: 204, ok: true });
    await logoutCurrentSession(); expect(getAuthToken()).toBeNull();
    expect(localStorage.getItem('keepsave_user')).toBeNull(); expect(sessionStorage.getItem('keepsave_oauth:pending')).toBeNull();
  });
  it('clears state on a protected 401 even when its response is not JSON', async () => {
    const { listProjects, getAuthToken } = await import('./client'); setToken(makeFakeJWT(3600));
    global.fetch = vi.fn().mockResolvedValue({ status: 401, ok: false, json: async () => { throw new Error('not JSON'); } });
    await expect(listProjects()).rejects.toThrow('Session expired'); expect(getAuthToken()).toBeNull();
  });
  it('a failed password sign-in does not invalidate another current session', async () => {
    const { login, getAuthToken } = await import('./client'); setToken(makeFakeJWT(3600)); const previous = getAuthToken();
    global.fetch = vi.fn().mockResolvedValue({ status: 401, ok: false, json: async () => ({ error: { message: 'Invalid credentials' } }) });
    await expect(login('bad@example.com', 'wrong')).rejects.toThrow('Invalid credentials'); expect(getAuthToken()).toBe(previous);
  });
  it('identity replacement discards pending provider proofs', () => {
    setToken(makeFakeJWT(3600)); sessionStorage.setItem('keepsave_oauth:pending', 'proof');
    setToken(makeFakeJWT(7200)); expect(sessionStorage.getItem('keepsave_oauth:pending')).toBeNull();
  });
});


it('an earlier denied request does not erase a newly established browser session', async () => {
 const { listProjects, getAuthToken } = await import('./client');
 sessionStorage.clear(); setToken(makeFakeJWT(3600));
 let finish!: (response: unknown) => void;
 global.fetch = vi.fn().mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
 const oldRequest = listProjects(); const fresh = makeFakeJWT(7200); setToken(fresh);
 finish({ status: 401, ok: false }); await expect(oldRequest).rejects.toThrow('Session expired'); expect(getAuthToken()).toBe(fresh);
});
