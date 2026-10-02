import { act, renderHook } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { useAuth } from './useAuth';
import { getAuthToken, setToken } from '../api/client';
import { toast } from './useToast';
vi.mock('./useToast', () => ({ toast: vi.fn() }));
const token = () => `e30.${btoa(JSON.stringify({ exp: Date.now() / 1000 + 3600 }))}.synthetic`;
beforeEach(() => { localStorage.clear(); sessionStorage.clear(); vi.clearAllMocks(); });
it('keeps an active session after failed server logout and explains the unconfirmed revocation', async () => {
 setToken(token()); localStorage.setItem('keepsave_user', JSON.stringify({ id: 'owner' }));
 const fetch = vi.fn().mockResolvedValue({ status: 503, ok: false, json: async () => ({ error: 'unavailable' }) }); vi.stubGlobal('fetch', fetch);
 const { result } = renderHook(useAuth); await act(() => result.current.logout());
 expect(result.current.authenticated).toBe(true); expect(getAuthToken()).not.toBeNull();
 expect(toast).toHaveBeenCalledWith(expect.objectContaining({ description: expect.stringContaining('not been confirmed') }));
 vi.unstubAllGlobals();
});
it('updates the hook only after server logout is committed', async () => {
 setToken(token()); localStorage.setItem('keepsave_user', JSON.stringify({ id: 'owner' }));
 vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ status: 204, ok: true }));
 const { result } = renderHook(useAuth); await act(() => result.current.logout());
 expect(result.current.authenticated).toBe(false); expect(result.current.user).toBeNull(); expect(getAuthToken()).toBeNull();
 vi.unstubAllGlobals();
});
