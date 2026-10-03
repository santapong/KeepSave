import { useEffect, useRef, useState, type FormEvent } from 'react';
import type { Organization, OrgMember } from '../types';
import { getIdentityAvailability, offboardMember, previewOffboarding, sendWorkspaceInvitation, IdentityPlatformError, type IdentityAvailability, type Offboarding } from '../api/identityPlatform';
import { ConfirmDialog } from './ConfirmDialog';
import '../styles/auth.css';

interface Props { organization: Organization; members: Array<OrgMember & { email?: string }>; isAdmin: boolean; onChanged: () => Promise<void> }
type Pending = { preview: Offboarding; key: string; uncertain: boolean };
export function OrganizationSafetyPanel({ organization, members, isAdmin, onChanged }: Props) {
  const currentOrganization = useRef(organization.id);
  currentOrganization.current = organization.id;
  const [ready, setReady] = useState<IdentityAvailability | null>(null);
  const [contact, setContact] = useState('');
  const [role, setRole] = useState<'viewer' | 'editor' | 'promoter' | 'admin'>('viewer');
  const [busy, setBusy] = useState(false);
  const [pending, setPending] = useState<Pending | null>(null);
  const [receipt, setReceipt] = useState<Offboarding | null>(null);
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  useEffect(() => { setPending(null); setReceipt(null); setConfirm(false); setError(''); setMessage(''); }, [organization.id]);
  useEffect(() => { let active = true; getIdentityAvailability().then((value) => { if (active) setReady(value); }).catch(() => { if (active) setError('Could not check workspace safety availability.'); }); return () => { active = false; }; }, []);
  async function invite(event: FormEvent) {
    event.preventDefault(); if (busy || !ready?.email || !isAdmin) return; setBusy(true); setError(''); setMessage('');
    const scope = organization.id;
    try { await sendWorkspaceInvitation(scope, contact, role); if (currentOrganization.current !== scope) return; setContact(''); setMessage('Invitation requested. The teammate receives an email and must accept using the intended account.'); }
    catch { if (currentOrganization.current === scope) setError('Invitation delivery was not confirmed. Check email before requesting another invitation. A recent admin sign-in may be required.'); }
    finally { setBusy(false); }
  }
  async function preview(member: OrgMember) {
    if (busy || member.user_id === organization.owner_id || !isAdmin || !ready?.identity) return; setBusy(true); setError(''); setMessage(''); setReceipt(null); setPending(null);
    const scope = organization.id;
    try { const data = await previewOffboarding(scope, member.user_id); if (currentOrganization.current !== scope) return; if (data.offboarding.organization_id !== scope || data.offboarding.user_id !== member.user_id) throw new Error('Invalid preview'); setPending({ preview: data.offboarding, key: crypto.randomUUID(), uncertain: false }); }
    catch { if (currentOrganization.current === scope) setError('Could not prepare the offboarding review. No access removal has been confirmed.'); }
    finally { setBusy(false); }
  }
  async function execute() {
    if (!pending || busy || pending.preview.organization_id !== organization.id) return; setBusy(true); setError('');
    const scope = organization.id;
    try {
      const result = (await offboardMember(pending.preview, pending.key)).offboarding;
      if (currentOrganization.current !== scope) return;
      if (result.organization_id !== scope || result.user_id !== pending.preview.user_id || result.status !== 'locally_revoked') throw new Error('Invalid receipt');
      setReceipt(result); setPending(null); setMessage('Workspace access has been revoked. Stored ownership and credential responsibility still need explicit reassignment.');
      try { await onChanged(); } catch { setError('Access was revoked, but the member list could not refresh. Reload to check it.'); }
    } catch (e) {
      if (currentOrganization.current !== scope) return;
      if (e instanceof IdentityPlatformError && e.status === 409) { setPending(null); setError('The affected access changed or this review expired. Prepare a new review before removing access.'); }
      else { setPending((value) => value ? { ...value, uncertain: true } : null); setError('Revocation was not confirmed. Retry this same review to check its result; the member list is unchanged until confirmation.'); }
    } finally { setBusy(false); }
  }
  const expired = pending ? Date.parse(pending.preview.preview_expires_at) <= Date.now() : false;
  return <section className="cz-card ks-connections" style={{ marginTop: 28, padding: 24, maxWidth: 'none' }} aria-labelledby="workspace-safety-title">
    <h2 id="workspace-safety-title">Workspace invitations and offboarding</h2>
    {error && <p role="alert" className="ks-auth-error">{error}</p>}{message && <p role="status">{message}</p>}
    {!ready && !error && <p role="status">Checking workspace safety…</p>}
    {ready && !ready.identity && <p className="cz-muted">Workspace safety is awaiting server setup.</p>}
    {ready?.identity && !isAdmin && <p className="cz-muted">An organization administrator manages invitations and offboarding.</p>}
    {ready?.identity && isAdmin && <>
      <h3>Invite a teammate</h3><p className="cz-muted">The teammate must verify the intended account before receiving workspace access.</p>
      {!ready.email && <p className="cz-muted">Invitations are awaiting email delivery setup.</p>}
      <form className="ks-auth-form" onSubmit={invite} aria-busy={busy}>
        <label htmlFor="workspace-invitation-contact">Teammate email</label><input id="workspace-invitation-contact" type="email" required disabled={busy || !ready.email} value={contact} onChange={(event) => setContact(event.target.value)} />
        <label htmlFor="workspace-invitation-role">Workspace role</label><select id="workspace-invitation-role" className="cz-input" disabled={busy || !ready.email} value={role} onChange={(event) => setRole(event.target.value as typeof role)}>{['viewer', 'editor', 'promoter', 'admin'].map((value) => <option key={value} value={value}>{value[0].toUpperCase() + value.slice(1)}</option>)}</select>
        <button type="submit" className="cz-btn" disabled={busy || !ready.email}>Send invitation</button>
      </form>
      <h3 style={{ marginTop: 28 }}>Review a teammate’s access</h3><p className="cz-muted">Review affected workspace access before removal. The organization owner requires a separate succession process.</p>
      <div className="ks-connections-list">{members.map((member) => <div key={member.user_id} className="ks-connections-row"><div><strong>{member.email || member.user_id.slice(0, 8)}</strong><p>{member.role}{member.user_id === organization.owner_id ? ' · Organization owner' : ''}</p></div><button className="cz-btn" disabled={busy || member.user_id === organization.owner_id} onClick={() => void preview(member)}>Review offboarding</button></div>)}</div>
      {pending && <div style={{ marginTop: 24 }} aria-labelledby="offboarding-review-title">
        <h3 id="offboarding-review-title">Review affected access</h3><p>{pending.preview.keys} keys · {pending.preview.leases} leases · {pending.preview.runs} active runs</p>
        <p className="cz-muted">Review expires {new Date(pending.preview.preview_expires_at).toLocaleTimeString()}. Access changes require a fresh review.</p>
        <h4>Project ownership requiring reassignment</h4>{pending.preview.projects_requiring_reassignment.length ? <ul>{pending.preview.projects_requiring_reassignment.map((project) => <li key={project.project_id}><strong>{project.name}</strong> — {project.consequence}</li>)}</ul> : <p>No project ownership needs reassignment.</p>}
        <h4>Credential responsibility requiring reassignment</h4>{pending.preview.credentials_requiring_reassignment.length ? <ul>{pending.preview.credentials_requiring_reassignment.map((credential) => <li key={credential.secret_id}><strong>{credential.key} · {credential.environment}</strong> — {credential.consequence}</li>)}</ul> : <p>No credential responsibility needs reassignment.</p>}
        <p className="cz-muted">This removes organization access. Other workspaces, personal projects and browser sessions stay available. Provider cleanup has a separate status.</p>
        <button className="cz-btn cz-btn-danger" disabled={busy || (expired && !pending.uncertain)} onClick={() => setConfirm(true)}>{pending.uncertain ? 'Retry the same review' : 'Confirm offboarding'}</button>
        <button className="cz-btn" style={{ marginLeft: 12 }} disabled={busy} onClick={() => setPending(null)}>Close review</button>
      </div>}
      {receipt && <p role="status">Local revocation receipt {receipt.id}. {receipt.keys} keys, {receipt.leases} leases and {receipt.runs} runs were included.</p>}
    </>}
    <ConfirmDialog open={confirm} onOpenChange={setConfirm} title="Revoke workspace access?" description="KeepSave will recheck this review and current permissions. Project ownership and credential responsibility are retained for explicit reassignment." confirmLabel="Revoke workspace access" onConfirm={() => void execute()} />
  </section>;
}
