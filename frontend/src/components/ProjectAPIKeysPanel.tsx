import { useState, useEffect, type FormEvent } from 'react';
import { listAPIKeys, createAPIKey, deleteAPIKey } from '../api/client';
import type { APIKey } from '../types';

interface ProjectAPIKeysPanelProps {
  projectId: string;
}

const SCOPES = ['read', 'write', 'delete', 'promote'] as const;
const SCOPE_DESCRIPTIONS: Record<string, string> = {
  read: 'Fetch and list secrets, export .env files',
  write: 'Create and update secret values',
  delete: 'Permanently delete secrets from an environment',
  promote: 'Create promotion requests and approve/reject them',
};

export function ProjectAPIKeysPanel({ projectId }: ProjectAPIKeysPanelProps) {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteError, setDeleteError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [selectedScopes, setSelectedScopes] = useState<Set<string>>(new Set(['read']));
  const [environment, setEnvironment] = useState('');
  const [newRawKey, setNewRawKey] = useState('');
  const [creating, setCreating] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    listAPIKeys()
      .then((all) => setKeys(all.filter((k) => k.project_id === projectId)))
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load API keys'))
      .finally(() => setLoading(false));
  }, [projectId]);

  function toggleScope(scope: string) {
    setSelectedScopes((prev) => {
      const next = new Set(prev);
      if (next.has(scope)) next.delete(scope);
      else next.add(scope);
      return next;
    });
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setError('');
    setCreating(true);
    try {
      const resp = await createAPIKey(
        name,
        projectId,
        Array.from(selectedScopes).sort(),
        environment || undefined
      );
      setNewRawKey(resp.raw_key);
      setName('');
      setSelectedScopes(new Set(['read']));
      setEnvironment('');
      const all = await listAPIKeys();
      setKeys(all.filter((k) => k.project_id === projectId));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create API key');
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm('Delete this API key? This cannot be undone.')) return;
    setDeleteError('');
    try {
      await deleteAPIKey(id);
      setKeys((prev) => prev.filter((k) => k.id !== id));
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete API key');
    }
  }

  function handleToggleCreate() {
    setShowCreate((v) => !v);
    setNewRawKey('');
    setCopied(false);
  }

  async function handleCopy() {
    await navigator.clipboard.writeText(newRawKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
        <div>
          <h2 style={{ fontSize: 18, fontWeight: 600, margin: 0 }}>API Keys</h2>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginTop: 4, margin: '4px 0 0' }}>
            Scoped keys for AI agents, CI/CD pipelines, and services to access this project's secrets.
          </p>
        </div>
        <button onClick={handleToggleCreate} style={btnPrimary}>
          {showCreate ? 'Cancel' : 'Create API Key'}
        </button>
      </div>

      {/* Errors */}
      {error && <div style={errorStyle}>{error}</div>}
      {deleteError && <div style={errorStyle}>{deleteError}</div>}

      {/* One-time raw key success box */}
      {newRawKey && (
        <div style={successBox}>
          <p style={{ fontWeight: 600, marginBottom: 4, margin: '0 0 4px' }}>API Key Created</p>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginBottom: 10, margin: '0 0 10px' }}>
            Copy this key now — it will not be shown again.
          </p>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <code style={rawKeyCode}>{newRawKey}</code>
            <button onClick={handleCopy} style={btnSmallOutline}>
              {copied ? 'Copied!' : 'Copy'}
            </button>
          </div>
        </div>
      )}

      {/* Create form */}
      {showCreate && (
        <form onSubmit={handleCreate} style={formCard}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            {/* Key Name */}
            <label style={labelStyle}>
              Key Name
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                placeholder="my-agent-key"
                style={inputStyle}
              />
            </label>

            {/* Environment */}
            <label style={labelStyle}>
              Environment
              <select value={environment} onChange={(e) => setEnvironment(e.target.value)} style={inputStyle}>
                <option value="">All environments</option>
                <option value="alpha">Alpha</option>
                <option value="uat">UAT</option>
                <option value="prod">PROD</option>
              </select>
            </label>

            {/* Scopes — full-width checkbox group */}
            <div style={{ gridColumn: '1 / -1' }}>
              <span style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 8 }}>Scopes</span>
              <div style={{ display: 'flex', gap: 0, flexDirection: 'column' as const }}>
                {SCOPES.map((scope) => (
                  <label
                    key={scope}
                    style={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      gap: 8,
                      padding: '8px 10px',
                      borderRadius: 4,
                      cursor: 'pointer',
                      background: selectedScopes.has(scope) ? 'rgba(99,102,241,0.07)' : 'transparent',
                      border: `1px solid ${selectedScopes.has(scope) ? 'var(--color-primary)' : 'var(--color-border)'}`,
                      marginBottom: 6,
                      transition: 'background 0.1s, border-color 0.1s',
                    }}
                  >
                    <input
                      type="checkbox"
                      checked={selectedScopes.has(scope)}
                      onChange={() => toggleScope(scope)}
                      style={{ marginTop: 2, flexShrink: 0 }}
                    />
                    <div>
                      <code style={{ fontSize: 12, fontWeight: 600 }}>{scope}</code>
                      <span style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginLeft: 8 }}>
                        {SCOPE_DESCRIPTIONS[scope]}
                      </span>
                    </div>
                  </label>
                ))}
              </div>
            </div>
          </div>

          <button
            type="submit"
            disabled={creating || selectedScopes.size === 0}
            style={{
              ...btnPrimary,
              marginTop: 16,
              opacity: creating || selectedScopes.size === 0 ? 0.6 : 1,
              cursor: creating || selectedScopes.size === 0 ? 'not-allowed' : 'pointer',
            }}
          >
            {creating ? 'Creating...' : 'Create Key'}
          </button>
        </form>
      )}

      {/* Content */}
      {loading ? (
        <p style={{ color: 'var(--color-text-secondary)', fontSize: 14 }}>Loading API keys...</p>
      ) : keys.length === 0 ? (
        <div style={emptyState}>
          <p style={{ fontWeight: 600, marginBottom: 6, margin: '0 0 6px' }}>No API keys for this project</p>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', margin: 0 }}>
            Create a key above to allow agents, scripts, or CI/CD pipelines to access this project's secrets.
          </p>
        </div>
      ) : (
        <table style={tableStyle}>
          <thead>
            <tr>
              <th style={thStyle}>Name</th>
              <th style={thStyle}>Scopes</th>
              <th style={thStyle}>Environment</th>
              <th style={thStyle}>Created</th>
              <th style={thStyle}></th>
            </tr>
          </thead>
          <tbody>
            {keys.map((k) => (
              <tr key={k.id}>
                <td style={tdStyle}>
                  <strong style={{ fontSize: 13 }}>{k.name}</strong>
                </td>
                <td style={tdStyle}>
                  {k.scopes?.map((s) => (
                    <span key={s} style={scopeBadge}>{s}</span>
                  ))}
                </td>
                <td style={tdStyle}>
                  {k.environment ? (
                    <span style={{ fontSize: 12, fontWeight: 600, textTransform: 'uppercase' as const }}>
                      {k.environment}
                    </span>
                  ) : (
                    <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>All</span>
                  )}
                </td>
                <td style={tdStyle}>
                  <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                    {new Date(k.created_at).toLocaleDateString()}
                  </span>
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

/* Styles */

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 600,
  cursor: 'pointer',
  flexShrink: 0,
};

const btnSmallOutline: React.CSSProperties = {
  padding: '4px 10px',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  fontSize: 12,
  cursor: 'pointer',
  whiteSpace: 'nowrap' as const,
  flexShrink: 0,
};

const btnSmallDanger: React.CSSProperties = {
  padding: '4px 10px',
  background: 'transparent',
  color: 'var(--color-danger)',
  border: '1px solid var(--color-danger)',
  borderRadius: 4,
  fontSize: 12,
  cursor: 'pointer',
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
  background: 'var(--color-input-bg)',
};

const rawKeyCode: React.CSSProperties = {
  fontSize: 13,
  wordBreak: 'break-all' as const,
  background: 'var(--color-input-bg)',
  padding: '4px 8px',
  borderRadius: 4,
  border: '1px solid var(--color-border)',
  flex: 1,
};

const successBox: React.CSSProperties = {
  background: 'rgba(34, 197, 94, 0.1)',
  border: '1px solid rgba(34, 197, 94, 0.25)',
  borderRadius: 'var(--radius)',
  padding: 16,
  marginBottom: 16,
};

const emptyState: React.CSSProperties = {
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  border: '1px solid var(--color-border)',
  padding: '32px 24px',
  textAlign: 'center' as const,
};

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse' as const,
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
};

const thStyle: React.CSSProperties = {
  textAlign: 'left' as const,
  padding: '10px 12px',
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  borderBottom: '1px solid var(--color-border)',
  textTransform: 'uppercase' as const,
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

const errorStyle: React.CSSProperties = {
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
};
