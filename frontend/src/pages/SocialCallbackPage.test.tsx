import { StrictMode } from 'react';
import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { SocialCallbackPage } from './SocialCallbackPage';
import { completeSocialLogin } from '../api/socialAuth';
vi.mock('../api/socialAuth', () => ({ completeSocialLogin: vi.fn() }));
beforeEach(() => { vi.clearAllMocks(); window.history.replaceState({}, '', '/auth/callback/github?code=private&state=state'); });
it('exchanges only once in StrictMode, strips the URL, and opens the workspace', async () => {
 const user = { id: '1', email: 'user@example.com', created_at: '', updated_at: '' }; const login = vi.fn();
 vi.mocked(completeSocialLogin).mockResolvedValue({ mode: 'login', auth: { user, token: 'jwt' } });
 render(<StrictMode><BrowserRouter><Routes><Route path="/auth/callback/:provider" element={<SocialCallbackPage onLogin={login} />} /><Route path="/" element={<p>Workspace</p>} /></Routes></BrowserRouter></StrictMode>);
 expect(window.location.search).toBe(''); expect(await screen.findByText('Workspace')).toBeInTheDocument();
 expect(completeSocialLogin).toHaveBeenCalledTimes(1); expect(login).toHaveBeenCalledWith(user, 'jwt');
});
it('shows a retry path after a denied sign-in', async () => {
 vi.mocked(completeSocialLogin).mockRejectedValue(new Error('Sign-in was cancelled.'));
 render(<BrowserRouter><Routes><Route path="/auth/callback/:provider" element={<SocialCallbackPage onLogin={vi.fn()} />} /></Routes></BrowserRouter>);
 await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Sign-in was cancelled.'));
 expect(screen.getByRole('link', { name: /Back to sign in/ })).toHaveAttribute('href', '/login');
});
