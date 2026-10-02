import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { ArrowUpRight, ArrowRight, Eye, EyeOff } from '@/components/icons';
import { login as apiLogin } from '../api/client';
import { AuthShell } from '../components/auth/AuthShell';
import { ProviderButtons } from '../components/auth/ProviderButtons';
import type { User } from '../types';

export function LoginPage({ onLogin }: { onLogin: (user: User, token: string) => void }) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [visible, setVisible] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [providerPending, setProviderPending] = useState(false);
  const navigate = useNavigate();
  async function handleSubmit(event: FormEvent) {
    event.preventDefault(); if (loading || providerPending) return;
    setError(''); setLoading(true);
    try { const result = await apiLogin(email, password); onLogin(result.user, result.token); navigate('/', { replace: true }); }
    catch (error) { setError(error instanceof Error ? error.message : 'Login failed. Please try again.'); }
    finally { setLoading(false); }
  }
  return <AuthShell>
    <h2>Sign in</h2><p className="ks-auth-lead">Continue to your KeepSave workspace.</p>
    <ProviderButtons busy={loading} onBusyChange={setProviderPending} />
    <div className="ks-auth-divider"><span>or continue with email</span></div>
    {error && <div className="ks-auth-error" role="alert">{error}</div>}
    <form onSubmit={handleSubmit} className="ks-auth-form" aria-busy={loading}>
      <label htmlFor="login-email">Email</label>
      <input id="login-email" type="email" autoComplete="username" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="you@company.com" required disabled={loading || providerPending} />
      <label htmlFor="login-password">Password</label>
      <div className="ks-auth-password"><input id="login-password" type={visible ? 'text' : 'password'} autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} minLength={8} required disabled={loading || providerPending} placeholder="Enter your password" />
        <button type="button" aria-label={visible ? 'Hide password' : 'Show password'} aria-pressed={visible} onClick={() => setVisible(!visible)}>{visible ? <EyeOff size={18} /> : <Eye size={18} />}</button>
      </div>
      <p className="ks-auth-session">Your session stays in this browser tab.</p>
      <button type="submit" className="ks-auth-submit" disabled={loading || providerPending}>{loading ? 'Signing in…' : 'Enter the vault'}<ArrowRight size={17} /></button>
    </form>
    <p className="ks-auth-register">New to KeepSave? <Link to="/register">Open an account <ArrowUpRight size={14} /></Link></p>
  </AuthShell>;
}
