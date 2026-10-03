import { useState } from 'react';
import { createEncryptedBackup, verifyEncryptedBackup, previewBackupRestore, restoreBackupRecords } from '../api/client';
import type { Bundle, Verification, RestorePreview } from '../api/coreTypes';
import { TypedConfirmModal } from './TypedConfirmModal';

const MAX_BACKUP_FILE = 64 * 1024 * 1024;
export function ProjectRecoveryPanel({ projectId, projectName }: { projectId: string; projectName: string }) {
  const [bundle, setBundle] = useState<Bundle | null>(null);
  const [verification, setVerification] = useState<Verification | null>(null);
  const [preview, setPreview] = useState<RestorePreview | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [metadataSelection, setMetadataSelection] = useState<Set<string>>(new Set());
  const [owners, setOwners] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [confirm, setConfirm] = useState(false);
  async function download() {
    if (busy) return; setBusy(true); setError(''); setMessage('');
    try {
      const data = await createEncryptedBackup(projectId);
      const url = URL.createObjectURL(new Blob([JSON.stringify(data)], { type: 'application/json' }));
      const link = document.createElement('a'); link.href = url; link.download = `keepsave-${projectId}-encrypted-vault.json`;
      document.body.appendChild(link); link.click(); link.remove(); URL.revokeObjectURL(url);
      setMessage('Encrypted backup downloaded. Keep the documented recovery material separately.');
    } catch (error) { setError(error instanceof Error ? error.message : 'Could not create the backup.'); }
    finally { setBusy(false); }
  }
  async function chooseFile(file?: File) {
    setBundle(null); setVerification(null); setPreview(null); setSelected(new Set()); setMetadataSelection(new Set()); setOwners({}); setMessage(''); setError('');
    if (!file) return;
    setBusy(true);
    try {
      if (file.size > MAX_BACKUP_FILE) throw new Error('Choose an encrypted backup smaller than 64 MiB.');
      const data: unknown = JSON.parse(await file.text());
      if (!data || typeof data !== 'object') throw new Error('Choose a KeepSave encrypted backup file.');
      const value = data as Partial<Bundle>;
      if (!['keepsave.encrypted-vault.v1', 'keepsave.encrypted-vault.v2'].includes(value.format ?? '') || typeof value.ciphertext !== 'string' || typeof value.nonce !== 'string' || typeof value.sha256 !== 'string') throw new Error('Choose a KeepSave encrypted backup file.');
      setBundle(value as Bundle);
    } catch (error) { setError(error instanceof Error ? error.message : 'Could not read that backup.'); }
    finally { setBusy(false); }
  }
  async function verify() {
    if (!bundle || busy) return; setBusy(true); setError(''); setMessage(''); setPreview(null); setSelected(new Set());
    try {
      const metadata = await verifyEncryptedBackup(projectId, bundle); setVerification(metadata);
      setPreview(await previewBackupRestore(projectId, bundle));
    } catch (error) { setVerification(null); setError(error instanceof Error ? error.message : 'Backup verification failed.'); }
    finally { setBusy(false); }
  }
  async function restore() {
    if (!bundle || !preview || busy || selected.size === 0) return;
    const chosen = preview.records.filter(record => record.restorable && selected.has(record.secret_id));
    if (chosen.some(record => metadataSelection.has(record.secret_id) && record.backup_lifecycle?.responsible_user_id && !owners[record.secret_id]?.trim())) { setError('Choose a current responsible member for each selected lifecycle restore.'); return; }
    const records = chosen.map(record => ({ secret_id: record.secret_id, backup_revision: record.backup_revision, expected_current_revision: record.current_revision, ...(metadataSelection.has(record.secret_id) ? { restore_lifecycle: true, expected_metadata_revision: record.current_metadata_revision ?? 0, ...(owners[record.secret_id]?.trim() ? { mapped_responsible_user_id: owners[record.secret_id].trim() } : {}) } : {}) }));
    if (!records.length) return; setBusy(true); setError(''); setMessage('');
    try {
      const restored = await restoreBackupRecords(projectId, bundle, records); setPreview(null); setSelected(new Set());
      setMessage(`Restored ${restored.records.length} selected records as new revisions. Open Secrets to inspect the current metadata.`);
    } catch (error) {
      setPreview(null); setSelected(new Set());
      setError(`${error instanceof Error ? error.message : 'Restore failed.'} Verify and preview again before another restore.`);
    } finally { setBusy(false); }
  }
  return <section className="cz-card" style={{ padding: 24 }} aria-labelledby="project-recovery-title">
    <h2 id="project-recovery-title">Encrypted backups and recovery</h2>
    <p className="cz-muted">Project administrators can download an encrypted vault backup, verify it, and restore explicitly selected active records. Deleted or absent records remain unchanged. Recovery keys stay on the server.</p>
    <p className="cz-muted">Use the operator recovery drill to verify a backup in an isolated target before a live restore.</p>
    <button className="cz-btn" disabled={busy} onClick={() => void download()}>{busy ? 'Working…' : 'Download encrypted backup'}</button>
    <div style={{ marginTop: 24 }}><label htmlFor="encrypted-backup-file">Select an encrypted KeepSave backup</label><input id="encrypted-backup-file" type="file" accept="application/json,.json" disabled={busy} onChange={(event) => void chooseFile(event.currentTarget.files?.[0])} className="block my-3" /><button className="cz-btn" disabled={!bundle || busy} onClick={() => void verify()}>Verify and preview</button></div>
    {error && <p role="alert" className="ks-auth-error">{error}</p>}{message && <p role="status">{message}</p>}
    {verification && <p role="status">Verified {verification.entries} records, {verification.revisions} revisions, {verification.keys} keys, and {verification.snapshots} promotion snapshots, and {verification.lifecycle_records ?? 0} lifecycle records. Backup created {new Date(verification.created_at).toLocaleString()}.</p>}
    {preview && <div style={{ marginTop: 20 }}><h3>Choose records to restore</h3><p className="cz-muted">Credential values restore as new revisions. Lifecycle metadata stays current unless explicitly selected below.</p><div className="ks-connections-list">{preview.records.map((record) => <div key={record.secret_id}><label className="ks-connections-row">
      <div><strong>{record.key} · {record.environment}</strong><p>Backup revision {record.backup_revision} · Current revision {record.current_revision} · {record.status}</p></div>
      <input type="checkbox" aria-label={`Restore ${record.key} in ${record.environment}`} checked={selected.has(record.secret_id)} disabled={!record.restorable || busy} onChange={(event) => { const next = new Set(selected); if (event.currentTarget.checked) next.add(record.secret_id); else next.delete(record.secret_id); setSelected(next); }} />
    </label>{selected.has(record.secret_id) && record.backup_lifecycle && <div className="ks-platform-form"><p>Lifecycle revision {record.backup_lifecycle.revision} in backup · {record.current_metadata_revision ?? 0} current. Renewal {record.backup_lifecycle.renewal_at || 'unknown'} · declared expiry {record.backup_lifecycle.declared_expires_at || 'unknown'}.</p><label><input type="checkbox" checked={metadataSelection.has(record.secret_id)} onChange={event => { const next = new Set(metadataSelection); if (event.target.checked) next.add(record.secret_id); else next.delete(record.secret_id); setMetadataSelection(next); }} /> Restore lifecycle for {record.key}</label>{metadataSelection.has(record.secret_id) && record.backup_lifecycle.responsible_user_id && <label>Current responsible member for {record.key}<input value={owners[record.secret_id] || ''} onChange={event => setOwners({ ...owners, [record.secret_id]: event.target.value })} placeholder="Explicit current member ID" /></label>}</div>}</div>)}</div><button className="cz-btn" style={{ marginTop: 16 }} disabled={selected.size === 0 || busy} onClick={() => setConfirm(true)}>Restore {selected.size} selected records</button></div>}
    <TypedConfirmModal open={confirm} onOpenChange={setConfirm} title="Restore selected records" description={`Append new revisions for ${selected.size} selected records in ${projectName}. KeepSave checks every expected current revision before applying the selection.`} confirmPhrase={projectName} confirmLabel="Restore selected records" onConfirm={() => void restore()} />
  </section>;
}
