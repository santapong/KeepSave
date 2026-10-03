import { useEffect, useState, type FormEvent } from 'react';
import { listLifecycle, updateLifecycle } from '../api/teamVault';
import type { LifecycleItem } from '../api/coreTypes';
const dateValue = (value: string | null) => value?.slice(0, 10) ?? '';
export function SecretLifecyclePanel({ projectId }: { projectId: string }) {
  const [records, setRecords] = useState<LifecycleItem[]>([]);
  const [selected, setSelected] = useState<LifecycleItem | null>(null);
  const [error, setError] = useState(''); const [status, setStatus] = useState(''); const [busy, setBusy] = useState(false);
  const [owner, setOwner] = useState(''); const [expiry, setExpiry] = useState(''); const [renewal, setRenewal] = useState(''); const [provenance, setProvenance] = useState('');
  useEffect(() => { let active = true; listLifecycle(projectId).then(data => { if (active) setRecords(data.records); }).catch(error => { if (active) setError(error instanceof Error ? error.message : 'Could not load lifecycle metadata.'); }); return () => { active = false; }; }, [projectId]);
  function select(item: LifecycleItem) { setSelected(item); setOwner(item.lifecycle.responsible_user_id ?? ''); setExpiry(dateValue(item.lifecycle.declared_expires_at)); setRenewal(dateValue(item.lifecycle.renewal_at)); setProvenance(item.lifecycle.provenance); setError(''); setStatus(''); }
  async function save(event: FormEvent) { event.preventDefault(); if (!selected || busy) return; setBusy(true); setError(''); setStatus('');
    try { const life = await updateLifecycle(projectId, selected.lifecycle.secret_id, { expected_revision: selected.lifecycle.revision, responsible_user_id: owner.trim() || null, declared_expires_at: expiry ? `${expiry}T00:00:00Z` : null, renewal_at: renewal ? `${renewal}T00:00:00Z` : null, provenance }); const next = { ...selected, lifecycle: life }; setRecords(items => items.map(item => item.lifecycle.secret_id === life.secret_id ? next : item)); setSelected(next); setStatus('Lifecycle saved. The credential value and its history are unchanged.'); }
    catch (error) { setError(error instanceof Error ? error.message : 'Could not save lifecycle metadata.'); } finally { setBusy(false); }
  }
  return <section className="cz-card" style={{ padding: 24 }}><h2>Credential lifecycle</h2><p className="cz-muted">Record who renews a credential and when it is due. These are declared dates; KeepSave does not inspect the provider’s expiry. Renewal date takes priority for reminders.</p><p className="cz-muted">Showing up to 100 authorized records. Credential values are never loaded here.</p>
    {error && <p role="alert" className="ks-auth-error">{error}</p>}{status && <p role="status">{status}</p>}
    <div className="ks-connections-list">{records.map(item => <div className="ks-connections-row" key={item.lifecycle.secret_id}><div><strong>{item.key}</strong><p>{item.environment} · {item.lifecycle.known ? `Metadata revision ${item.lifecycle.revision}` : 'Lifecycle unknown'}</p></div><button className="cz-btn" onClick={() => select(item)} disabled={busy}>Edit lifecycle</button></div>)}</div>
    {selected && <form onSubmit={event => void save(event)} className="ks-platform-form"><h3>{selected.key} · {selected.environment}</h3><label>Responsible member ID<input className="cz-input" value={owner} onChange={event => setOwner(event.target.value)} placeholder="Current workspace member ID, or leave unassigned" /></label><label>Declared expiry (UTC)<input className="cz-input" type="date" value={expiry} onChange={event => setExpiry(event.target.value)} /></label><label>Renewal date (UTC)<input className="cz-input" type="date" value={renewal} onChange={event => setRenewal(event.target.value)} /></label><label>Provenance<input className="cz-input" maxLength={500} value={provenance} onChange={event => setProvenance(event.target.value)} placeholder="Where this credential was issued; do not enter its value" /></label><button className="cz-btn cz-btn-primary" disabled={busy}>{busy ? 'Saving…' : 'Save lifecycle'}</button></form>}
  </section>;
}
