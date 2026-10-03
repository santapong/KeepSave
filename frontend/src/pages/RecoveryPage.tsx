import { useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { getIdentityAvailability, requestAccountRecovery, type IdentityAvailability } from '../api/identityPlatform';
import { AuthShell } from '../components/auth/AuthShell';

export function RecoveryPage() {
  const [ready, setReady] = useState<IdentityAvailability | null>(null);
  const [contact, setContact] = useState('');
  const [busy, setBusy] = useState(false);
  const [accepted, setAccepted] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    getIdentityAvailability().then((value) => { if (active) setReady(value); }).catch(() => { if (active) setError('Recovery is temporarily unavailable. Please try again later.'); });
    return () => { active = false; };
  }, []);
  async function submit(event: FormEvent) {
    event.preventDefault(); if (busy || !ready?.identity || !ready.email) return;
    setBusy(true); setError('');
    try { await requestAccountRecovery(contact); setAccepted(true); }
    catch { setError('Recovery could not be requested. Please try again later.'); }
    finally { setBusy(false); }
  }
  return <AuthShell><h2>Recover your account</h2><p className="ks-auth-lead">Use an email you previously verified for this KeepSave account.</p>
    {error && <p role="alert" className="ks-auth-error">{error}</p>}
    {!ready && !error && <p role="status">Checking recovery availability…</p>}
    {ready && (!ready.identity || !ready.email) && <p className="ks-auth-lead">Password recovery is awaiting email delivery setup. Use an existing sign-in method.</p>}
    {accepted ? <p role="status" className="ks-auth-lead">If this is a verified recovery email, a confirmation will arrive shortly. Check your inbox and use the newest link within 15 minutes.</p> : <form className="ks-auth-form" onSubmit={submit} aria-busy={busy}>
      <label htmlFor="recovery-email">Verified recovery email</label><input id="recovery-email" type="email" autoComplete="email" required value={contact} disabled={busy || !ready?.email || !ready.identity} onChange={(event) => setContact(event.target.value)} />
      <button className="ks-auth-submit" disabled={busy || !ready?.email || !ready.identity}>{busy ? 'Requesting…' : 'Request recovery'}</button>
    </form>}
    <p className="ks-auth-register"><Link to="/login">Back to sign in</Link></p>
  </AuthShell>;
}
