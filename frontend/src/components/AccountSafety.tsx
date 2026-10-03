import { useEffect, useState, type FormEvent } from 'react';
import { getAccountMethods, getIdentityAvailability, getProofDelivery, getVerifiedContacts, removeAccountMethod, requestContactProof, requestFreshProof, IdentityPlatformError, type AccountMethod, type IdentityAvailability, type ProofDelivery, type ProofRequest, type VerifiedContact } from '../api/identityPlatform';
import { ConfirmDialog } from './ConfirmDialog';
import '../styles/auth.css';

const label = (name: string) => name === 'password' ? 'Password' : name === 'google' ? 'Google' : 'GitHub';
export function AccountSafety() {
  const [availability, setAvailability] = useState<IdentityAvailability | null>(null);
  const [methods, setMethods] = useState<AccountMethod[]>([]);
  const [contacts, setContacts] = useState<VerifiedContact[]>([]);
  const [contact, setContact] = useState('');
  const [proof, setProof] = useState<ProofRequest | null>(null);
  const [delivery, setDelivery] = useState<ProofDelivery | null>(null);
  const [remove, setRemove] = useState<AccountMethod | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  useEffect(() => {
    let active = true;
    getIdentityAvailability().then(async (ready) => {
      if (active) setAvailability(ready);
      if (!ready.identity) return;
      const [methodData, contactData] = await Promise.all([getAccountMethods(), getVerifiedContacts()]);
      if (active) { setMethods(methodData.methods); setContacts(contactData.contacts); }
    }).catch(() => { if (active) setError('Could not load account safety. Reload to try again.'); });
    return () => { active = false; };
  }, []);
  async function verifyContact(event: FormEvent) {
    event.preventDefault(); if (busy || !availability?.email) return;
    setBusy(true); setError(''); setMessage('');
    try { const data = await requestContactProof(contact); setProof(data.request); setDelivery(null); setMessage('Confirmation requested. Open the email in this signed-in browser tab.'); }
    catch (e) { setError(e instanceof IdentityPlatformError && e.status === 403 ? 'Sign in again before adding a recovery email.' : 'Could not request confirmation. Try again when email delivery is available.'); }
    finally { setBusy(false); }
  }
  async function checkDelivery() {
    if (!proof || busy) return; setBusy(true); setError('');
    try { setDelivery((await getProofDelivery(proof.id)).delivery); }
    catch { setError('Could not check delivery. A new email has not been sent.'); }
    finally { setBusy(false); }
  }
  async function freshProof() {
    if (!proof || busy || !availability?.email) return; setBusy(true); setError('');
    try { setProof((await requestFreshProof(proof.id)).request); setDelivery(null); setMessage('A fresh confirmation was requested. Use only the newest email.'); }
    catch { setError('Could not request a fresh confirmation. Check delivery again before retrying.'); }
    finally { setBusy(false); }
  }
  async function removeMethod(method: AccountMethod) {
    if (busy) return; setBusy(true); setError(''); setMessage('');
    try { await removeAccountMethod(method.name); setMethods((await getAccountMethods()).methods); setMessage(`${label(method.name)} removed. Other browser sessions have been revoked.`); }
    catch (e) { setError(e instanceof IdentityPlatformError && e.status === 403 ? 'Sign in using another remaining method, then return here to remove this method.' : e instanceof IdentityPlatformError && e.status === 409 ? 'Keep at least one usable sign-in method.' : 'The method could not be removed. No removal has been confirmed.'); }
    finally { setBusy(false); }
  }
  const usable = methods.filter((method) => method.usable).length;
  return <section className="cz-card ks-connections" style={{ marginTop: 36, padding: 24 }} aria-labelledby="account-safety-title">
    <h2 id="account-safety-title">Account safety</h2>
    {error && <p role="alert" className="ks-auth-error">{error}</p>}{message && <p role="status">{message}</p>}
    {!availability && !error && <p role="status">Checking account safety…</p>}
    {availability && !availability.identity && <p className="cz-muted">Account safety is awaiting server setup.</p>}
    {availability?.identity && <>
      <p className="cz-muted">Keep a way back into your account. To remove a sign-in method, first sign in using another remaining method.</p>
      <div className="ks-connections-list">{methods.map((method) => <div className="ks-connections-row" key={method.name}><div><strong>{label(method.name)}</strong><p>{method.usable ? 'Available for sign-in' : method.name === 'password' ? 'Password sign-in is not set' : 'Awaiting provider setup'}</p></div><button className="cz-btn" disabled={busy || (method.name === 'password' && !method.usable) || (method.usable && usable <= 1)} onClick={() => setRemove(method)}>Remove {label(method.name)}</button></div>)}</div>
      <h3 style={{ marginTop: 28 }}>Verified recovery emails</h3>
      {contacts.length ? <ul>{contacts.map((entry) => <li key={entry.contact}>{entry.contact} · Verified {new Date(entry.verified_at).toLocaleDateString()}</li>)}</ul> : <p className="cz-muted">No verified recovery email yet.</p>}
      {!availability.email && <p className="cz-muted">Email confirmation is awaiting delivery setup.</p>}
      <form className="ks-auth-form" onSubmit={verifyContact} aria-busy={busy}>
        <label htmlFor="account-recovery-contact">Recovery email</label><input id="account-recovery-contact" type="email" autoComplete="email" required value={contact} disabled={busy || !availability.email} onChange={(event) => setContact(event.target.value)} />
        <button className="cz-btn" type="submit" disabled={busy || !availability.email}>Request confirmation</button>
      </form>
      {proof && <div style={{ marginTop: 20 }}><button className="cz-btn" disabled={busy} onClick={() => void checkDelivery()}>Check delivery</button>
        {delivery && <p role="status">{delivery.state === 'sent' ? 'Email accepted for delivery. Check your inbox.' : delivery.state === 'uncertain' ? 'Delivery could not be confirmed. KeepSave will not resend this email automatically.' : delivery.state === 'failed' ? 'Email was not accepted for delivery.' : delivery.state === 'cancelled' ? 'This confirmation is no longer active.' : 'Delivery is pending.'}</p>}
        {delivery && ['uncertain', 'failed', 'cancelled'].includes(delivery.state) && <button className="cz-btn" disabled={busy || !availability.email} onClick={() => void freshProof()}>Request a fresh confirmation</button>}
      </div>}
    </>}
    <ConfirmDialog open={remove !== null} onOpenChange={(open) => { if (!open) setRemove(null); }} title={`Remove ${remove ? label(remove.name) : 'sign-in method'}?`} description="KeepSave will check that you recently signed in using another usable method and revoke your other sessions." confirmLabel="Remove method" onConfirm={() => { if (remove) void removeMethod(remove); }} />
  </section>;
}
