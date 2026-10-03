import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { getAuthToken, invalidateBrowserSession } from '../api/client';
import { acceptInvitation, captureIdentityProof, confirmVerifiedContact, getIdentityAvailability, resetAccountPassword, type IdentityAvailability, type IdentityProof } from '../api/identityPlatform';
import { AuthShell } from '../components/auth/AuthShell';

export function IdentityConfirmPage() {
  const captured = useRef<{ loaded: boolean; proof: IdentityProof | null }>({ loaded: false, proof: null });
  const [proof, setProof] = useState<IdentityProof | null>(null);
  const [ready, setReady] = useState<IdentityAvailability | null>(null);
  const [password, setPassword] = useState('');
  const [repeat, setRepeat] = useState('');
  const [busy, setBusy] = useState(false);
  const [complete, setComplete] = useState(false);
  const [resultMessage, setResultMessage] = useState('');
  const [error, setError] = useState('');
  useEffect(() => {
    // Capture once across StrictMode effect replay, clear before any API call.
    if (!captured.current.loaded) captured.current = { loaded: true, proof: captureIdentityProof() };
    setProof(captured.current.proof);
    if (!captured.current.proof) { setError('This confirmation link is incomplete. Request a fresh link.'); return; }
    let active = true;
    getIdentityAvailability().then((value) => { if (active) setReady(value); }).catch(() => { if (active) setError('Confirmation is temporarily unavailable. Reopen the email link to try again.'); });
    return () => { active = false; };
  }, []);
  async function confirm(event: FormEvent) {
    event.preventDefault(); if (!proof || busy || !ready?.identity || !ready.email) return;
    if (proof.purpose === 'password_reset' && password !== repeat) { setError('The passwords do not match.'); return; }
    if (proof.purpose === 'password_reset' && new TextEncoder().encode(password).length > 72) { setError('Choose a shorter password.'); return; }
    const startedToken = getAuthToken();
    setBusy(true); setError('');
    try {
      if (proof.purpose === 'password_reset') { await resetAccountPassword(proof, password); if (getAuthToken() === startedToken) invalidateBrowserSession(); }
      else if (proof.purpose === 'contact_verify') await confirmVerifiedContact(proof);
      else await acceptInvitation(proof);
      setResultMessage(proof.purpose === 'password_reset' ? 'Your password has been saved. Existing sessions, API keys and delegated access have been revoked.' : proof.purpose === 'invitation' ? 'Your workspace invitation has been accepted.' : 'Your recovery email has been verified.');
      captured.current.proof = null; setProof(null); setPassword(''); setRepeat(''); setComplete(true);
    } catch { setError('The link could not be confirmed. It may have expired, been used, or belong to another account or browser session. Request a fresh link to continue.'); }
    finally { setBusy(false); }
  }
  const reset = proof?.purpose === 'password_reset';
  const signedIn = !!getAuthToken();
  const title = complete ? 'Confirmation complete' : reset ? 'Choose a new password' : proof?.purpose === 'invitation' ? 'Accept workspace invitation' : 'Verify your recovery email';
  return <AuthShell><h2>{title}</h2>
    {error && <p role="alert" className="ks-auth-error">{error}</p>}
    {complete ? <><p role="status" className="ks-auth-lead">{resultMessage}</p><Link className="ks-auth-return" to={signedIn ? '/account' : '/login'}>{signedIn ? 'Continue to account' : 'Continue to sign in'}</Link></> : <>
      {!ready && proof && !error && <p role="status">Checking confirmation availability…</p>}
      {ready && (!ready.identity || !ready.email) && <p className="ks-auth-lead">Email confirmation is awaiting server setup.</p>}
      {proof && !reset && !signedIn && <><p className="ks-auth-lead">Sign in to the intended account, then open the email link again. Recovery email confirmation needs the browser tab where you requested it.</p><Link className="ks-auth-return" to="/login">Sign in</Link></>}
      {proof && (reset || signedIn) && <form className="ks-auth-form" onSubmit={confirm} aria-busy={busy}>
        {reset ? <>
          <p className="ks-auth-lead">Use at least 8 characters with an uppercase letter, lowercase letter, number and symbol.</p>
          <label htmlFor="recovery-new-password">New password</label><input id="recovery-new-password" type="password" autoComplete="new-password" required minLength={8} value={password} disabled={busy} onChange={(event) => setPassword(event.target.value)} />
          <label htmlFor="recovery-repeat-password">Repeat password</label><input id="recovery-repeat-password" type="password" autoComplete="new-password" required minLength={8} value={repeat} disabled={busy} onChange={(event) => setRepeat(event.target.value)} />
        </> : <p className="ks-auth-lead">{proof.purpose === 'invitation' ? 'KeepSave will check the invitation and current workspace permissions before adding this account.' : 'Confirm that this email belongs to your KeepSave account.'}</p>}
        <button className="ks-auth-submit" type="submit" disabled={busy || !ready?.identity || !ready.email}>{busy ? 'Confirming…' : reset ? 'Save new password' : proof.purpose === 'invitation' ? 'Accept invitation' : 'Confirm email'}</button>
      </form>}
      <p className="ks-auth-register"><Link to="/account">Account safety</Link> · <Link to="/auth/recovery">Request password recovery</Link></p>
    </>}
  </AuthShell>;
}
