import { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { ArrowRight } from '@/components/icons';
import { AuthShell } from '../components/auth/AuthShell';
import { getAuthToken } from '../api/client';
import { decideMCPConsent, getMCPConsent, verifiedConsentCallback, type MCPConsent } from '../api/mcpConsent';
import { consentLoginPath, consentRequestID } from '../lib/mcpReturnContext';
import '../styles/mcpConsent.css';

export function MCPConsentPage({ authenticated, email }: { authenticated: boolean; email?: string }) {
  const location = useLocation();
  const id = consentRequestID(location.search);
  const identity = authenticated ? getAuthToken() : null;
  const [loaded, setLoaded] = useState<{ id: string; identity: string | null; value?: MCPConsent; error?: string }>();
  const [busy, setBusy] = useState(false);
  const [decisionError, setDecisionError] = useState('');
  const [now, setNow] = useState(Date.now);
  const submitting = useRef(false);
  const consent = loaded && loaded.id === id && loaded.identity === identity ? loaded.value : undefined;
  const error = !id ? 'This connection link is incomplete. Start a new connection from your client.' : loaded && loaded.id === id && loaded.identity === identity ? loaded.error : undefined;
  useEffect(() => {
    if (!authenticated || !id) return;
    let active = true;
    getMCPConsent(id).then((value) => { if (active) setLoaded({ id, identity, value }); }).catch((error) => { if (active) setLoaded({ id, identity, error: error instanceof Error ? error.message : 'The connection request is unavailable.' }); });
    return () => { active = false; };
  }, [authenticated, id, identity]);
  useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer); }, []);
  const expired = consent ? Date.parse(consent.expires_at) <= now : false;
  async function decide(approve: boolean) {
    if (!consent || !id || expired || submitting.current) return;
    submitting.current = true; setBusy(true); setDecisionError('');
    try {
      const result = await decideMCPConsent(id, approve);
      if (getAuthToken() !== identity) throw new Error('Your account changed. Open this request again.');
      window.location.assign(verifiedConsentCallback(result.redirect_url, consent));
    } catch (error) { setDecisionError(error instanceof Error ? error.message : 'The connection could not be confirmed.'); submitting.current = false; setBusy(false); }
  }
  return <AuthShell>
    <h2>{consent ? `Connect ${consent.harness === 'hermes' ? 'Hermes' : 'Codex'}?` : 'Connect your client'}</h2>
    {!authenticated && id && <><p className="ks-auth-lead">Sign in to review the access requested by your coding client.</p><Link className="ks-auth-submit" to={consentLoginPath(id)}>Sign in to review <ArrowRight size={17} /></Link></>}
    {authenticated && !consent && !error && <p className="ks-auth-lead" role="status">Loading the connection request…</p>}
    {error && <p className="ks-auth-error" role="alert">{error}</p>}
    {consent && <>
      <p className="ks-auth-lead">{email ? `Using ${email}.` : 'Using your current KeepSave account.'} Review the access before continuing.</p>
      <div className="ks-consent-detail"><span>KeepSave installation</span><p>{new URL(consent.resource).origin}</p></div>
      <ul className="ks-consent-access"><li>See runs you approved for this client.</li><li>Read approved repository files at the selected commit.</li><li>Check results and cancel your runs.</li></ul>
      <p className="ks-auth-session">Access ends within eight hours and follows your current account permissions. Signing out of this session revokes the connection. Your GitHub credentials stay with KeepSave.</p>
      {expired && <p className="ks-auth-error" role="alert">This request expired. Start a new connection in your client.</p>}
      {decisionError && <p className="ks-auth-error" role="alert">{decisionError}</p>}
      <div className="ks-consent-actions" aria-busy={busy}><button className="ks-auth-submit" disabled={busy || expired} onClick={() => void decide(true)}>{busy ? 'Continuing…' : 'Allow connection'} <ArrowRight size={17} /></button><button className="ks-consent-deny" disabled={busy || expired} onClick={() => void decide(false)}>Deny</button></div>
    </>}
    {authenticated && id && (error || decisionError) && <Link className="ks-auth-return" to={consentLoginPath(id)}>Sign in again</Link>}
  </AuthShell>;
}
