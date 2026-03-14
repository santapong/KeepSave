import { useState, useEffect, type FormEvent } from 'react';
import {
  listAPIKeys,
  createAPIKey,
  deleteAPIKey,
  listProjects,
} from '../api/client';
import type { APIKey, Project } from '../types';

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
        scopes.split(',').map((s) => s.trim()),
        environment || undefined
      );
      setNewRawKey(resp.raw_key);
      setName('');
      const updatedKeys = await listAPIKeys();
      setKeys(updatedKeys);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create API key');
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm('Delete this API key? This cannot be undone.')) return;
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

  if (loading) return <p>Loading API keys...</p>;

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700 }}>API Keys</h1>
        <button onClick={() => { setShowCreate(!showCreate); setNewRawKey(''); }} style={btnPrimary}>
          {showCreate ? 'Cancel' : 'Create API Key'}
        </button>
      </div>

      {error && <div style={errorStyle}>{error}</div>}

      {newRawKey && (
        <div style={successBox}>
          <p style={{ fontWeight: 600, marginBottom: 4 }}>API Key Created</p>
          <p style={{ fontSize: 13, marginBottom: 8 }}>
            Copy this key now. You won&apos;t be able to see it again.
          </p>
          <code style={{ fontSize: 13, wordBreak: 'break-all', background: 'var(--color-input-bg)', padding: '4px 8px', borderRadius: 4, border: '1px solid var(--color-border)' }}>
            {newRawKey}
          </code>
        </div>
      )}

      {showCreate && (
        <form onSubmit={handleCreate} style={formCard}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <label style={labelStyle}>
              Name
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                placeholder="my-agent-key"
                style={inputStyle}
              />
            </label>
            <label style={labelStyle}>
              Project
              <select value={projectId} onChange={(e) => setProjectId(e.target.value)} style={inputStyle}>
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            </label>
            <label style={labelStyle}>
              Scopes
              <input
                value={scopes}
                onChange={(e) => setScopes(e.target.value)}
                placeholder="read,write"
                style={inputStyle}
              />
            </label>
            <label style={labelStyle}>
              Environment (optional)
              <select value={environment} onChange={(e) => setEnvironment(e.target.value)} style={inputStyle}>
                <option value="">All environments</option>
                <option value="alpha">Alpha</option>
                <option value="uat">UAT</option>
                <option value="prod">PROD</option>
              </select>
            </label>
          </div>
          <button type="submit" style={{ ...btnPrimary, marginTop: 12 }}>Create Key</button>
        </form>
      )}

      {keys.length === 0 ? (
        <p style={{ color: 'var(--color-text-secondary)' }}>No API keys yet.</p>
      ) : (
        <table style={tableStyle}>
          <thead>
            <tr>
              <th style={thStyle}>Name</th>
              <th style={thStyle}>Project</th>
              <th style={thStyle}>Scopes</th>
              <th style={thStyle}>Environment</th>
              <th style={thStyle}>Created</th>
              <th style={thStyle}></th>
            </tr>
          </thead>
          <tbody>
            {keys.map((k) => (
              <tr key={k.id}>
                <td style={tdStyle}><strong>{k.name}</strong></td>
                <td style={tdStyle}>{projectName(k.project_id)}</td>
                <td style={tdStyle}>
                  {k.scopes?.map((s) => (
                    <span key={s} style={scopeBadge}>{s}</span>
                  ))}
                </td>
                <td style={tdStyle}>
                  {k.environment ? k.environment.toUpperCase() : 'All'}
                </td>
                <td style={tdStyle}>
                  <span style={{ fontSize: 12 }}>{new Date(k.created_at).toLocaleDateString()}</span>
                </td>
                <td style={tdStyle}>
                  <button onClick={() => handleDelete(k.id)} style={btnSmallDanger}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 600,
};

const btnSmallDanger: React.CSSProperties = {
  padding: '4px 10px',
  background: 'transparent',
  color: 'var(--color-danger)',
  border: '1px solid var(--color-danger)',
  borderRadius: 4,
  fontSize: 12,
};

const formCard: React.CSSProperties = {
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  padding: 20,
  marginBottom: 24,
};

const labelStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 4,
  fontSize: 13,
  fontWeight: 500,
};

const inputStyle: React.CSSProperties = {
  padding: '6px 10px',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  fontSize: 13,
};

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
};

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '10px 12px',
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  borderBottom: '1px solid var(--color-border)',
  textTransform: 'uppercase',
};

const tdStyle: React.CSSProperties = {
  padding: '10px 12px',
  borderBottom: '1px solid var(--color-border)',
};

const scopeBadge: React.CSSProperties = {
  padding: '1px 6px',
  borderRadius: 3,
  fontSize: 11,
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  marginRight: 4,
};

const successBox: React.CSSProperties = {
  background: 'rgba(34, 197, 94, 0.1)',
  border: '1px solid rgba(34, 197, 94, 0.25)',
  borderRadius: 'var(--radius)',
  padding: 16,
  marginBottom: 16,
};

const errorStyle: React.CSSProperties = {
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
};
