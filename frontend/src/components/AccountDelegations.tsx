import { useEffect, useState } from 'react';
import { BASE_URL, getAuthToken, invalidateBrowserSession } from '../api/client';
import type { MCPDelegation, MCPDelegations } from '../api/coreTypes';
import { useCapabilities } from '../hooks/useCapabilities';
import { ConfirmDialog } from './ConfirmDialog';
import '../styles/auth.css';

async function request<T>(path: string, method = 'GET'): Promise<T> {
  const token = getAuthToken();
  if (!token || new URL(BASE_URL, window.location.origin).origin !== window.location.origin) throw new Error('Connection management is unavailable.');
  const response = await fetch(`${BASE_URL}${path}`, { method, cache: 'no-store', referrerPolicy: 'no-referrer', redirect: 'error', credentials: 'same-origin', headers: { Authorization: `Bearer ${token}` } });
  if (getAuthToken() !== token) throw new Error('Your account changed. Reload to check its connections.');
  if (response.status === 401) { invalidateBrowserSession(); throw new Error('Your session expired. Sign in again.'); }
  if (!response.ok) throw new Error('The connection change could not be confirmed. Please try again.');
  if (response.status === 204) return undefined as T;
  const data = await response.json() as T;
  if (getAuthToken() !== token) throw new Error('Your account changed. Reload to check its connections.');
  return data;
}

export function AccountDelegations() {
  const { capabilities, enabled } = useCapabilities();
  const available = enabled('mcp_delegation');
  const [records, setRecords] = useState<MCPDelegation[] | null>(null);
  const [revoke, setRevoke] = useState<MCPDelegation | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  useEffect(() => {
    if (!available) return;
    let active = true;
    request<MCPDelegations>('/account/delegations').then((data) => { if (active) setRecords(data.delegations); }).catch((e) => { if (active) setError(e instanceof Error ? e.message : 'Could not load your tool connections.'); });
    return () => { active = false; };
  }, [available]);
  async function revokeConnection(record: MCPDelegation) {
    if (busy) return; setBusy(true); setError(''); setMessage('');
    try {
      await request<void>(`/account/delegations/${encodeURIComponent(record.family_id)}`, 'DELETE');
      setRecords((value) => value?.filter((item) => item.family_id !== record.family_id) ?? null);
      setMessage(`${record.harness === 'codex' ? 'Codex' : 'Hermes'} connection revoked. Subsequent access through it is denied.`);
    } catch (e) { setError(e instanceof Error ? e.message : 'Revocation was not confirmed. The connection remains listed.'); }
    finally { setBusy(false); }
  }
  return <section className="cz-card ks-connections" style={{ padding: 24, marginTop: 36 }} aria-labelledby="account-delegations-title">
    <h2 id="account-delegations-title">Tool connections</h2>
    <p className="cz-muted">Review the clients allowed to use your approved KeepSave tools.</p>
    {!capabilities && <p role="status">Checking tool connection availability…</p>}
    {capabilities && !available && <p className="cz-muted">Tool connections are awaiting server setup.</p>}
    {error && <p role="alert" className="ks-auth-error">{error}</p>}{message && <p role="status">{message}</p>}
    {available && !records && !error && <p role="status">Loading your tool connections…</p>}
    {available && records?.length === 0 && <p>No active tool connections.</p>}
    {available && <div className="ks-connections-list">{records?.map((record) => <div className="ks-connections-row" key={record.family_id}><div>
      <strong>{record.harness === 'codex' ? 'Codex' : 'Hermes'}</strong><p>{record.client_id}</p><p>{record.resource}</p>
      <p>Connected {new Date(record.created_at).toLocaleString()} · Expires {new Date(record.expires_at).toLocaleString()}</p>
      <p>{record.requires_refresh ? 'Waiting for the client to refresh this connection.' : 'Approved tool access is available.'}</p>
      {record.scope.includes('offline_access') && <p>This client can renew its connection until expiry.</p>}
    </div><button className="cz-btn" disabled={busy} onClick={() => setRevoke(record)}>Revoke connection</button></div>)}</div>}
    <ConfirmDialog open={revoke !== null} onOpenChange={(open) => { if (!open) setRevoke(null); }} title="Revoke this tool connection?" description="Subsequent tool access through this connection will be denied. The client will need a new approved connection." confirmLabel="Revoke connection" onConfirm={() => { if (revoke) void revokeConnection(revoke); }} />
  </section>;
}
