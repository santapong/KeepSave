import { useEffect, useState } from 'react';
import { listSecretHistory, restoreSecretVersion } from '../api/client';
import type { Revision } from '../api/coreTypes';
import type { Secret } from '../types';
import { TypedConfirmModal } from './TypedConfirmModal';

export function SecretHistoryPanel({ projectId, secret, onClose, onRestored }: { projectId: string; secret: Secret; onClose: () => void; onRestored: () => void }) {
  const [history, setHistory] = useState<Revision[] | null>(null);
  const [error, setError] = useState('');
  const [selected, setSelected] = useState<Revision | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let active = true;
    listSecretHistory(projectId, secret.id).then((rows) => { if (active) setHistory(rows); })
      .catch((error) => { if (active) setError(error instanceof Error ? error.message : 'Could not load history.'); });
    return () => { active = false; };
  }, [projectId, secret.id]);
  async function restore() {
    if (!selected || !secret.revision || busy) return;
    setBusy(true); setError('');
    try { await restoreSecretVersion(projectId, secret.id, selected.revision, secret.revision); onRestored(); }
    catch (error) { setError(`${error instanceof Error ? error.message : 'Restore failed.'} Reload the secret before trying again; a stale restore is never retried automatically.`); }
    finally { setBusy(false); }
  }
  return <section className="cz-card" aria-labelledby="secret-history-title" style={{ padding: 24, marginTop: 20 }}>
    <div className="flex justify-between items-center gap-3"><h2 id="secret-history-title">History · {secret.key}</h2><button className="cz-btn" onClick={onClose} disabled={busy}>Close history</button></div>
    <p className="cz-muted">Current revision {secret.revision}. Restoration appends a new revision. Values stay hidden in this history view.</p>
    {error && <p role="alert" className="ks-auth-error">{error}</p>}
    {!history && !error && <p role="status">Loading history…</p>}
    <div className="ks-connections-list">{history?.map((version) => <div className="ks-connections-row" key={version.revision}>
      <div><strong>Revision {version.revision}</strong><p>{version.operation} · {new Date(version.created_at).toLocaleString()}</p></div>
      <button className="cz-btn" disabled={busy || version.revision === secret.revision || !secret.revision} onClick={() => setSelected(version)}>{version.revision === secret.revision ? 'Current' : 'Restore this revision'}</button>
    </div>)}</div>
    <TypedConfirmModal open={selected !== null} onOpenChange={(open) => { if (!open) setSelected(null); }} title="Restore a secret revision" description={`Restore revision ${selected?.revision} of ${secret.key} as a new current revision. KeepSave will reject this request if revision ${secret.revision} has changed.`} confirmPhrase={secret.key} confirmLabel="Restore revision" onConfirm={() => void restore()} />
  </section>;
}
