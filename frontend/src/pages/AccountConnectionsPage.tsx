import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { getConnections, startSocialLogin, type Provider, type Providers } from '../api/socialAuth';
import { getAccountSessions, revokeAccountSession, invalidateBrowserSession, type AccountSession } from '../api/client';
import '../styles/auth.css';

export function AccountConnectionsPage() {
  const [connections, setConnections] = useState<{ connected: Provider[]; available: Providers } | null>(null);
  const [pending, setPending] = useState<Provider | null>(null);
  const [error, setError] = useState('');
  const [sessions, setSessions] = useState<AccountSession[] | null>(null);
  const [sessionError, setSessionError] = useState('');
  const [revoking, setRevoking] = useState<string | null>(null);
  const [params] = useSearchParams();
  useEffect(() => {
    let active = true;
    getConnections().then((data) => { if (active) setConnections(data); }).catch(() => { if (active) setError('Could not load your sign-in connections. Please reload to try again.'); });
    getAccountSessions().then((data) => { if (active) setSessions(data.sessions); }).catch(() => { if (active) setSessionError('Could not load your sessions. Please reload to try again.'); });
    return () => { active = false; };
  }, []);
  async function connect(provider: Provider) {
    setError(''); setPending(provider);
    try { await startSocialLogin(provider, 'link'); }
    catch (error) { setError(error instanceof Error ? error.message : 'Could not connect this provider.'); setPending(null); }
  }
  async function revoke(session: AccountSession) {
    setSessionError(''); setRevoking(session.id);
    try {
      await revokeAccountSession(session.id);
      if (session.current) { invalidateBrowserSession(); return; }
      const result = await getAccountSessions(); setSessions(result.sessions);
    } catch { setSessionError('Could not revoke this session. Try again; server revocation has not been confirmed.'); }
    finally { setRevoking(null); }
  }
  return <div className="cz-page ks-connections"><h1>Account connections</h1>
    <p className="cz-muted">Connect another way to sign in to this KeepSave account.</p>
    {params.has('connected') && connections && <p role="status">Your sign-in connection has been saved.</p>}
    {error && <p className="ks-auth-error" role="alert">{error}</p>}
    {!connections && !error && <p role="status">Loading your connections…</p>}
    <div className="ks-connections-list">{(['github', 'google'] as const).map((provider) => {
      const connected = connections?.connected.includes(provider);
      const available = connections?.available[provider];
      return <div key={provider} className="ks-connections-row"><div><strong>{provider === 'github' ? 'GitHub' : 'Google'}</strong><p>{connected ? 'Connected to this account' : available ? 'Use this provider to sign in next time.' : 'Awaiting provider setup.'}</p></div>
        <button className="cz-btn" disabled={!!pending || !available || connected} onClick={() => void connect(provider)}>{connected ? 'Connected' : pending === provider ? 'Connecting…' : 'Connect'}</button></div>;
    })}</div>
    <section aria-labelledby="account-sessions-title" style={{ marginTop: 36 }}>
      <h2 id="account-sessions-title">Your sessions</h2>
      <p className="cz-muted">Sessions expire after 24 hours. Revoking a session blocks its next request.</p>
      {sessionError && <p className="ks-auth-error" role="alert">{sessionError}</p>}
      {!sessions && !sessionError && <p role="status">Loading your sessions…</p>}
      <div className="ks-connections-list">{sessions?.map((session) => <div className="ks-connections-row" key={session.id}>
        <div><strong>{session.current ? 'This browser session' : 'Another session'}</strong>
          <p>{session.status} · Started {new Date(session.created_at).toLocaleString()}</p>
          <p>{session.user_agent || 'Client information unavailable'}{session.ip_address ? ` · ${session.ip_address}` : ''}</p>
        </div>
        <button className="cz-btn" disabled={revoking !== null || session.status !== 'active'} onClick={() => void revoke(session)}>
          {revoking === session.id ? 'Revoking…' : session.status !== 'active' ? session.status : session.current ? 'Sign out' : 'Revoke'}
        </button>
      </div>)}</div>
    </section>
  </div>;
}
