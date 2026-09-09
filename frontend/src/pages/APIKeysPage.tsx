import { useState, useEffect, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { Plus, Trash2 } from 'lucide-react';
import { listAPIKeys, createAPIKey, deleteAPIKey, listProjects } from '../api/client';
import type { APIKey, Project } from '../types';
import { TypedConfirmModal } from '../components/TypedConfirmModal';
import { HoldToReveal } from '../components/cosmic/HoldToReveal';
import {
  Page,
  PageHeader,
  KpiStrip,
  Kpi,
  Chip,
  EmptyState,
} from '../components/cosmic/primitives';

export function APIKeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [projectId, setProjectId] = useState('');
  const [scopes, setScopes] = useState('read');
  const [environment, setEnvironment] = useState('');
  const [newRawKey, setNewRawKey] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<APIKey | null>(null);

  useEffect(() => {
    Promise.all([listAPIKeys(), listProjects()])
      .then(([keysData, projectsData]) => {
        setKeys(keysData);
        setProjects(projectsData);
        if (projectsData.length > 0) setProjectId(projectsData[0].id);
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load data'))
      .finally(() => setLoading(false));
  }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      const resp = await createAPIKey(
        name,
        projectId,
        scopes.split(',').map((s) => s.trim()).filter(Boolean),
        environment || undefined,
      );
      setNewRawKey(resp.raw_key);
      setName('');
      const updatedKeys = await listAPIKeys();
      setKeys(updatedKeys);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create API key');
    }
  }

  async function performDelete(id: string) {
    try {
      await deleteAPIKey(id);
      setKeys(keys.filter((k) => k.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete API key');
    }
  }

  function projectName(id: string): string {
    return projects.find((p) => p.id === id)?.name || id.slice(0, 8);
  }

  // KPIs derived from real data (no fabricated metrics).
  const readScoped = keys.filter((k) => k.scopes?.includes('read')).length;
  const scopedToEnv = keys.filter((k) => !!k.environment).length;

  return (
    <Page>
      <PageHeader
        eyebrow="Vault · access"
        title="API keys"
        sub="Keys are scoped per-project and per-environment. Prefer short expirations — the vault records every use."
        actions={
          <button
            type="button"
            className={`cz-btn ${showCreate ? '' : 'cz-btn-primary'}`}
            onClick={() => {
              setShowCreate((v) => !v);
              setNewRawKey('');
            }}
          >
            {showCreate ? 'Cancel' : (<><Plus size={15} /> Create API key</>)}
          </button>
        }
      />

      <KpiStrip>
        <Kpi label="Active keys" value={loading ? '—' : keys.length} hint="across this org" />
        <Kpi label="Projects" value={loading ? '—' : projects.length} hint="with vault access" />
        <Kpi label="Read-scoped" value={loading ? '—' : readScoped} hint="of all keys" />
        <Kpi label="Env-scoped" value={loading ? '—' : scopedToEnv} hint="pinned to one env" />
      </KpiStrip>

      {error && <div className="cz-login-error" style={{ marginBottom: 16 }}>{error}</div>}

      {newRawKey && (
        <div className="cz-card" style={{ padding: 18, marginBottom: 18, borderColor: 'var(--cz-go)' }}>
          <p style={{ fontWeight: 600, marginBottom: 4 }}>API key created</p>
          <p className="cz-mute" style={{ fontSize: 13, marginBottom: 12 }}>
            Copy this key now — it will not be shown again. Press and hold to reveal it.
          </p>
          <HoldToReveal value={newRawKey} bricks={16} />
        </div>
      )}

      {showCreate && projects.length === 0 && (
        <div className="cz-login-error" style={{ marginBottom: 16 }}>
          You must create a project before you can create an API key.{' '}
          <Link to="/" style={{ color: 'inherit', fontWeight: 600 }}>Go to Projects →</Link>
        </div>
      )}

      {showCreate && projects.length > 0 && (
        <form onSubmit={handleCreate} className="cz-card" style={{ padding: 20, marginBottom: 24 }}>
          <div className="cz-key-fields">
            <label className="cz-login-field">
              <span style={{ display: 'block', marginBottom: 7 }}>Name</span>
              <input value={name} onChange={(e) => setName(e.target.value)} required placeholder="my-agent-key" className="cz-input" />
            </label>
            <label className="cz-login-field">
              <span style={{ display: 'block', marginBottom: 7 }}>Project</span>
              <select value={projectId} onChange={(e) => setProjectId(e.target.value)} className="cz-input">
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            </label>
            <label className="cz-login-field">
              <span style={{ display: 'block', marginBottom: 7 }}>Scopes</span>
              <input value={scopes} onChange={(e) => setScopes(e.target.value)} placeholder="read,write" className="cz-input" />
            </label>
            <label className="cz-login-field">
              <span style={{ display: 'block', marginBottom: 7 }}>Environment (optional)</span>
              <select value={environment} onChange={(e) => setEnvironment(e.target.value)} className="cz-input">
                <option value="">All environments</option>
                <option value="alpha">Alpha</option>
                <option value="uat">UAT</option>
                <option value="prod">PROD</option>
              </select>
            </label>
          </div>
          <button type="submit" className="cz-btn cz-btn-primary" style={{ marginTop: 14 }}>Create key →</button>
        </form>
      )}

      {loading ? (
        <p className="cz-mute">Loading API keys…</p>
      ) : keys.length === 0 ? (
        <EmptyState title="No API keys yet">
          Issue a scoped key to let agents, scripts, or CI/CD pipelines read your secrets.
        </EmptyState>
      ) : (
        <div className="cz-card cz-secrets">
          <table className="cz-dtable">
            <thead>
              <tr>
                <th>Name</th>
                <th>Project</th>
                <th>Scopes</th>
                <th>Environment</th>
                <th>Created</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {keys.map((k) => (
                <tr key={k.id}>
                  <td><span className="cz-secret-key">{k.name}</span></td>
                  <td className="cz-mute">{projectName(k.project_id)}</td>
                  <td>
                    <div className="cz-envs">
                      {k.scopes?.map((s) => (
                        <Chip key={s} variant={s === 'admin' || s === 'delete' ? 'on' : undefined}>{s}</Chip>
                      ))}
                    </div>
                  </td>
                  <td>
                    {k.environment ? (
                      <Chip variant={k.environment === 'prod' ? 'prod' : undefined}>{k.environment}</Chip>
                    ) : (
                      <span className="cz-faint" style={{ fontFamily: 'var(--cz-mono)', fontSize: 12 }}>all</span>
                    )}
                  </td>
                  <td className="cz-faint" style={{ fontFamily: 'var(--cz-mono)', fontSize: 12 }}>
                    {new Date(k.created_at).toLocaleDateString()}
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <button
                      type="button"
                      className="cz-btn cz-btn-danger"
                      style={{ padding: '6px 12px', fontSize: 11 }}
                      onClick={() => setDeleteTarget(k)}
                    >
                      <Trash2 size={13} /> Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <TypedConfirmModal
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete API key"
        description={
          deleteTarget
            ? `Permanently delete "${deleteTarget.name}". Agents and services using this key will lose access immediately. This cannot be undone.`
            : ''
        }
        confirmPhrase={deleteTarget?.name ?? ''}
        confirmLabel="Delete key"
        onConfirm={() => {
          if (deleteTarget) performDelete(deleteTarget.id);
        }}
      />
    </Page>
  );
}
