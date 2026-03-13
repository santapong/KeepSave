import { describe, it, expect, vi, beforeEach } from 'vitest';
import { KeepSaveAPI } from './api';

describe('KeepSaveAPI', () => {
  let api: KeepSaveAPI;

  beforeEach(() => {
    api = new KeepSaveAPI('https://api.example.com');
    vi.restoreAllMocks();
  });

  describe('authentication', () => {
    it('starts unauthenticated', () => {
      expect(api.isAuthenticated()).toBe(false);
    });

    it('is authenticated after setToken', () => {
      api.setToken('test-token');
      expect(api.isAuthenticated()).toBe(true);
    });

    it('is authenticated after setApiKey', () => {
      api.setApiKey('ks_test');
      expect(api.isAuthenticated()).toBe(true);
    });

    it('sends Bearer token in Authorization header', async () => {
      api.setToken('jwt-123');
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ secrets: [] }),
      });
      vi.stubGlobal('fetch', mockFetch);

      await api.listSecrets('proj-1', 'alpha');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.example.com/api/v1/projects/proj-1/secrets?environment=alpha',
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: 'Bearer jwt-123',
          }),
        })
      );
    });

    it('sends API key in X-API-Key header', async () => {
      api.setApiKey('ks_test_key');
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ secrets: [] }),
      });
      vi.stubGlobal('fetch', mockFetch);

      await api.listSecrets('proj-1', 'alpha');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.objectContaining({
            'X-API-Key': 'ks_test_key',
          }),
        })
      );
    });

    it('setToken clears apiKey and vice versa', async () => {
      api.setApiKey('ks_key');
      api.setToken('jwt-token');

      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ secrets: [] }),
      });
      vi.stubGlobal('fetch', mockFetch);

      await api.listSecrets('proj-1', 'alpha');

      const headers = mockFetch.mock.calls[0][1].headers;
      expect(headers.Authorization).toBe('Bearer jwt-token');
      expect(headers['X-API-Key']).toBeUndefined();
    });
  });

  describe('listSecrets', () => {
    it('returns secrets array', async () => {
      const mockSecrets = [
        { id: '1', project_id: 'p1', environment_id: 'e1', key: 'DB_HOST', value: 'localhost', created_at: '', updated_at: '' },
      ];

      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ secrets: mockSecrets }),
      }));

      api.setToken('token');
      const result = await api.listSecrets('p1', 'alpha');
      expect(result).toEqual(mockSecrets);
    });

    it('throws on error response', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: 'Unauthorized' }),
      }));

      api.setToken('bad-token');
      await expect(api.listSecrets('p1', 'alpha')).rejects.toThrow('Unauthorized');
    });
  });

  describe('createSecret', () => {
    it('sends POST with correct body', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ secret: { id: 'new-1', key: 'API_KEY' } }),
      });
      vi.stubGlobal('fetch', mockFetch);

      api.setToken('token');
      await api.createSecret('p1', 'API_KEY', 'secret-value', 'alpha');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.example.com/api/v1/projects/p1/secrets',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ key: 'API_KEY', value: 'secret-value', environment: 'alpha' }),
        })
      );
    });
  });

  describe('updateSecret', () => {
    it('sends PUT with value', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ secret: { id: 's1', key: 'DB_HOST' } }),
      });
      vi.stubGlobal('fetch', mockFetch);

      api.setToken('token');
      await api.updateSecret('p1', 's1', 'new-value');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.example.com/api/v1/projects/p1/secrets/s1',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ value: 'new-value' }),
        })
      );
    });
  });

  describe('deleteSecret', () => {
    it('sends DELETE request', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 204,
        json: () => Promise.resolve({}),
      });
      vi.stubGlobal('fetch', mockFetch);

      api.setToken('token');
      await api.deleteSecret('p1', 's1');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.example.com/api/v1/projects/p1/secrets/s1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });

  describe('URL handling', () => {
    it('strips trailing slashes from base URL', async () => {
      const trailingApi = new KeepSaveAPI('https://api.example.com///');
      trailingApi.setToken('token');

      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ secrets: [] }),
      });
      vi.stubGlobal('fetch', mockFetch);

      await trailingApi.listSecrets('p1', 'alpha');

      expect(mockFetch.mock.calls[0][0]).toBe(
        'https://api.example.com/api/v1/projects/p1/secrets?environment=alpha'
      );
    });
  });
});
