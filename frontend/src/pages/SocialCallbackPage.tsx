import { ArrowLeft, ArrowRight } from '@/components/icons';
import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { completeSocialLogin } from '../api/socialAuth';
import { AuthShell } from '../components/auth/AuthShell';
import type { User } from '../types';
import { validConsentReturn } from '../lib/mcpReturnContext';

export function SocialCallbackPage({ onLogin }: { onLogin: (user: User, token: string) => void }) {
  const { provider = '' } = useParams();
  const [query] = useState(() => window.location.search);
  const [error, setError] = useState('');
  const attempt = useRef<ReturnType<typeof completeSocialLogin> | null>(null);
  const navigate = useNavigate();
  useEffect(() => {
    // Strip credentials before making further requests; referrer policy is set in index.html.
    window.history.replaceState(window.history.state, '', window.location.pathname);
    let active = true;
    // Share the promise across StrictMode's effect replay: one code exchange only.
    attempt.current ??= completeSocialLogin(provider, query);
    attempt.current.then((result) => {
      if (!active) return;
      if (result.auth) onLogin(result.auth.user, result.auth.token);
      navigate(result.mode === 'link' ? '/account?connected=1' : validConsentReturn(result.returnTo) || '/', { replace: true });
    }).catch((error) => { if (active) setError(error instanceof Error ? error.message : 'Sign-in could not be completed.'); });
    return () => { active = false; };
  }, [provider, query, onLogin, navigate]);
  return <AuthShell><h2>{error ? 'Let’s try again' : 'Connecting your account'}</h2>
    {error ? <p role="alert" className="ks-auth-error">{error}</p> : <p role="status" className="ks-auth-lead">Verifying your sign-in. This will only take a moment.</p>}
    {error && <><Link className="ks-auth-return" to="/login"><ArrowLeft size={14} /> Back to sign in</Link><br /><Link className="ks-auth-return" to="/account">Account connections <ArrowRight size={14} /></Link></>}
  </AuthShell>;
}
