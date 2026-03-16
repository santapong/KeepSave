import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { listSecrets, createSecret, updateSecret, deleteSecret } from '../api/client';
import { formatDate } from '../utils/formatDate';
import type { Secret } from '../types';

const ENVIRONMENTS = ['alpha', 'uat', 'prod'] as const;
type Environment = (typeof ENVIRONMENTS)[number];

const ENV_COLORS: Record<Environment, string> = {
  alpha: '#22c55e',
  uat: '#6366f1',
  prod: '#f59e0b',
};

interface SecretsPanelProps {
  projectId: string;
}

export function SecretsPanel({ projectId }: SecretsPanelProps) {
  const [env, setEnv] = useState<Environment>('alpha');
  const [secrets, setSecrets] = useState<Secret[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showAdd, setShowAdd] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');
  const [revealed, setRevealed] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const loadSecrets = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const data = await listSecrets(projectId, env);
      setSecrets(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load secrets');
    } finally {
      setLoading(false);
    }
  }, [projectId, env]);

  useEffect(() => {
    loadSecrets();
    setRevealed(new Set());
    setEditing(null);
    setEditValue('');
    setSearchQuery('');
  }, [loadSecrets]);

  const filteredSecrets = secrets.filter((s) =>
    s.key.toLowerCase().includes(searchQuery.toLowerCase())
  );

  async function handleAdd(e: FormEvent) {
    e.preventDefault();
    try {
      await createSecret(projectId, newKey.toUpperCase(), newValue, env);
      setNewKey('');
      setNewValue('');
      setShowAdd(false);
      loadSecrets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create secret');
    }
  }

  async function handleUpdate(secretId: string) {
    try {
      await updateSecret(projectId, secretId, editValue);
      setEditing(null);
      setEditValue('');
      loadSecrets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update secret');
    }
  }

  async function handleDelete(secretId: string) {
    if (!window.confirm('Delete this secret? This action cannot be undone.')) return;
    try {
      await deleteSecret(projectId, secretId);
      loadSecrets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete secret');
    }
  }

  function toggleReveal(id: string) {
    setRevealed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleRevealAll() {
    if (revealed.size === filteredSecrets.length && filteredSecrets.length > 0) {
      setRevealed(new Set());
    } else {
      setRevealed(new Set(filteredSecrets.map((s) => s.id)));
    }
  }

  async function copyToClipboard(secretId: string, value: string | undefined) {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopiedId(secretId);
      setTimeout(() => setCopiedId(null), 2000);
    } catch {
      // Fallback for environments without clipboard API
      const textarea = document.createElement('textarea');
      textarea.value = value;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
      setCopiedId(secretId);
      setTimeout(() => setCopiedId(null), 2000);
    }
  }

  function maskValue(): string {
    return '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022';
  }

  const allRevealed = filteredSecrets.length > 0 && revealed.size === filteredSecrets.length;
  const envColor = ENV_COLORS[env];

  return (
    <div>
      {/* Environment Tabs */}
      <div style={envTabsRow}>
        {ENVIRONMENTS.map((e) => {
          const color = ENV_COLORS[e];
          const isActive = env === e;
          return (
            <button
              key={e}
              onClick={() => setEnv(e)}
              style={{
                padding: '8px 20px',
                borderRadius: 'var(--radius)',
                border: '1px solid',
                borderColor: isActive ? color : 'var(--color-border)',
                background: isActive ? color : 'var(--color-surface)',
                color: isActive ? '#fff' : 'var(--color-text-secondary)',
                fontSize: 13,
                fontWeight: 600,
                textTransform: 'uppercase',
                cursor: 'pointer',
                transition: 'all 0.15s ease',
                letterSpacing: '0.05em',
              }}
            >
              {e}
            </button>
          );
        })}
        <div style={{ flex: 1 }} />
        <button
          onClick={() => setShowAdd(!showAdd)}
          style={{
            ...btnPrimary,
            background: showAdd ? 'transparent' : 'var(--color-primary)',
            color: showAdd ? 'var(--color-text-secondary)' : '#fff',
            border: showAdd ? '1px solid var(--color-border)' : 'none',
          }}
        >
          {showAdd ? 'Cancel' : '+ Add Secret'}
        </button>
      </div>

      {/* Error Banner */}
      {error && (
        <div style={errorStyle}>
          <span style={{ marginRight: 8 }}>!</span>
          {error}
          <button
            onClick={() => setError('')}
            style={{
              marginLeft: 'auto',
              background: 'none',
              border: 'none',
              color: 'var(--color-danger)',
              cursor: 'pointer',
              fontSize: 14,
              fontWeight: 700,
              padding: '0 4px',
            }}
          >
            x
          </button>
        </div>
      )}

      {/* Add Secret Form */}
      {showAdd && (
        <form onSubmit={handleAdd} style={addFormCard}>
          <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 12 }}>
            Add new secret to <span style={{ color: envColor, textTransform: 'uppercase' }}>{env}</span>
          </div>
          <div style={addFormFieldsGrid}>
            <div style={formField}>
              <label style={labelStyle}>Key</label>
              <input
                placeholder="e.g. DATABASE_URL"
                value={newKey}
                onChange={(e) => setNewKey(e.target.value.toUpperCase())}
                required
                style={{
                  ...inputStyle,
                  fontFamily: 'monospace',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                }}
              />
            </div>
            <div style={{ ...formField, flex: 2 }}>
              <label style={labelStyle}>Value</label>
              <input
                placeholder="Enter the secret value"
                value={newValue}
                onChange={(e) => setNewValue(e.target.value)}
                required
                type="password"
                style={inputStyle}
              />
            </div>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 4 }}>
            <button type="button" onClick={() => setShowAdd(false)} style={btnSmallOutline}>
              Cancel
            </button>
            <button type="submit" style={btnPrimary}>
              Save Secret
            </button>
          </div>
        </form>
      )}

      {/* Summary Bar + Search */}
      {!loading && (
        <div style={summaryBar}>
          <div style={summaryLeft}>
            <span style={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              minWidth: 24,
              height: 24,
              borderRadius: 12,
              background: envColor,
              color: '#fff',
              fontSize: 12,
              fontWeight: 700,
              marginRight: 8,
            }}>
              {secrets.length}
            </span>
            <span style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
              {secrets.length === 1 ? 'secret' : 'secrets'} in{' '}
              <span style={{ color: envColor, fontWeight: 600, textTransform: 'uppercase' }}>
                {env}
              </span>
            </span>
          </div>
          <div style={summaryRight}>
            {secrets.length > 0 && (
              <>
                <button onClick={toggleRevealAll} style={btnSmallOutline}>
                  {allRevealed ? 'Hide All' : 'Reveal All'}
                </button>
                <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
                  <svg
                    style={{
                      position: 'absolute',
                      left: 10,
                      width: 14,
                      height: 14,
                      pointerEvents: 'none',
                    }}
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="var(--color-text-secondary)"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <circle cx="11" cy="11" r="8" />
                    <path d="m21 21-4.35-4.35" />
                  </svg>
                  <input
                    type="text"
                    placeholder="Filter by key name..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    style={searchInput}
                  />
                  {searchQuery && (
                    <button
                      onClick={() => setSearchQuery('')}
                      style={{
                        position: 'absolute',
                        right: 8,
                        background: 'none',
                        border: 'none',
                        color: 'var(--color-text-secondary)',
                        cursor: 'pointer',
                        fontSize: 14,
                        padding: '0 4px',
                      }}
                    >
                      x
                    </button>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {/* Search results info */}
      {searchQuery && !loading && (
        <div style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 8 }}>
          Showing {filteredSecrets.length} of {secrets.length} secrets matching &quot;{searchQuery}&quot;
        </div>
      )}

      {/* Content */}
      {loading ? (
        <div style={loadingContainer}>
          <div style={loadingSpinner} />
          <span style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
            Loading secrets...
          </span>
        </div>
      ) : secrets.length === 0 ? (
        <div style={emptyStateCard}>
          <div style={emptyStateIcon}>
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke={envColor} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
          </div>
          <h3 style={{ fontSize: 16, fontWeight: 600, margin: '0 0 8px 0' }}>
            No secrets in {env.toUpperCase()}
          </h3>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', margin: '0 0 20px 0', maxWidth: 360, lineHeight: 1.6 }}>
            Secrets are encrypted key-value pairs stored securely for this environment.
            Add your first secret to get started with the{' '}
            <span style={{ color: envColor, fontWeight: 600, textTransform: 'uppercase' }}>{env}</span>{' '}
            environment.
          </p>
          <button
            onClick={() => setShowAdd(true)}
            style={{
              ...btnPrimary,
              padding: '10px 24px',
              fontSize: 14,
            }}
          >
            + Add Your First Secret
          </button>
        </div>
      ) : filteredSecrets.length === 0 ? (
        <div style={emptyStateCard}>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', margin: 0 }}>
            No secrets matching &quot;{searchQuery}&quot; in {env.toUpperCase()}.
          </p>
        </div>
      ) : (
        <div style={tableWrapper}>
          <table style={tableStyle}>
            <thead>
              <tr>
                <th style={thStyle}>Key</th>
                <th style={thStyle}>Value</th>
                <th style={{ ...thStyle, width: 160 }}>Updated</th>
                <th style={{ ...thStyle, width: 220 }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredSecrets.map((s) => (
                <tr
                  key={s.id}
                  style={{
                    transition: 'background 0.1s ease',
                  }}
                  onMouseEnter={(e) => {
                    (e.currentTarget as HTMLTableRowElement).style.background = 'var(--color-surface-hover)';
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLTableRowElement).style.background = 'transparent';
                  }}
                >
                  <td style={tdStyle}>
                    <code style={keyCodeStyle}>{s.key}</code>
                  </td>
                  <td style={tdStyle}>
                    {editing === s.id ? (
                      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                        <input
                          value={editValue}
                          onChange={(e) => setEditValue(e.target.value)}
                          style={{ ...inputStyle, flex: 1 }}
                          autoFocus
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') handleUpdate(s.id);
                            if (e.key === 'Escape') setEditing(null);
                          }}
                        />
                        <button onClick={() => handleUpdate(s.id)} style={btnSmall}>
                          Save
                        </button>
                        <button onClick={() => setEditing(null)} style={btnSmallOutline}>
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <code style={valueCodeStyle}>
                        {revealed.has(s.id) ? s.value : maskValue()}
                      </code>
                    )}
                  </td>
                  <td style={tdStyle}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                      {formatDate(s.updated_at)}
                    </span>
                  </td>
                  <td style={tdStyle}>
                    <div style={{ display: 'flex', gap: 6 }}>
                      <button
                        onClick={() => toggleReveal(s.id)}
                        style={btnSmallOutline}
                        title={revealed.has(s.id) ? 'Hide value' : 'Reveal value'}
                      >
                        {revealed.has(s.id) ? 'Hide' : 'Reveal'}
                      </button>
                      {revealed.has(s.id) && (
                        <button
                          onClick={() => copyToClipboard(s.id, s.value)}
                          style={{
                            ...btnSmallOutline,
                            color: copiedId === s.id ? '#22c55e' : 'var(--color-text-secondary)',
                            borderColor: copiedId === s.id ? '#22c55e' : 'var(--color-border)',
                          }}
                          title="Copy to clipboard"
                        >
                          {copiedId === s.id ? 'Copied!' : 'Copy'}
                        </button>
                      )}
                      <button
                        onClick={() => {
                          setEditing(s.id);
                          setEditValue(s.value || '');
                        }}
                        style={btnSmallOutline}
                        title="Edit value"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(s.id)}
                        style={btnSmallDanger}
                        title="Delete secret"
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

/* --- Style Constants --- */

const envTabsRow: React.CSSProperties = {
  display: 'flex',
  gap: 8,
  marginBottom: 16,
  alignItems: 'center',
};

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 600,
  cursor: 'pointer',
  transition: 'opacity 0.15s ease',
};

const btnSmall: React.CSSProperties = {
  padding: '4px 10px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 4,
  fontSize: 12,
  cursor: 'pointer',
  fontWeight: 500,
};

const btnSmallOutline: React.CSSProperties = {
  padding: '4px 10px',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  fontSize: 12,
  cursor: 'pointer',
  fontWeight: 500,
  transition: 'all 0.1s ease',
};

const btnSmallDanger: React.CSSProperties = {
  padding: '4px 10px',
  background: 'transparent',
  color: 'var(--color-danger)',
  border: '1px solid var(--color-danger)',
  borderRadius: 4,
  fontSize: 12,
  cursor: 'pointer',
  fontWeight: 500,
};

const addFormCard: React.CSSProperties = {
  marginBottom: 16,
  background: 'var(--color-surface)',
  padding: 20,
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  border: '1px solid var(--color-border)',
};

const addFormFieldsGrid: React.CSSProperties = {
  display: 'flex',
  gap: 12,
  marginBottom: 12,
};

const formField: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 6,
  flex: 1,
};

const labelStyle: React.CSSProperties = {
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
};

const inputStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  background: 'var(--color-input-bg)',
  color: 'inherit',
  outline: 'none',
};

const summaryBar: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: 12,
  gap: 12,
};

const summaryLeft: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
};

const summaryRight: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 8,
};

const searchInput: React.CSSProperties = {
  padding: '6px 12px 6px 32px',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  background: 'var(--color-input-bg)',
  color: 'inherit',
  outline: 'none',
  width: 220,
};

const loadingContainer: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  padding: 48,
  gap: 16,
};

const loadingSpinner: React.CSSProperties = {
  width: 24,
  height: 24,
  border: '3px solid var(--color-border)',
  borderTopColor: 'var(--color-primary)',
  borderRadius: '50%',
  animation: 'spin 0.8s linear infinite',
};

const emptyStateCard: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  padding: '48px 24px',
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  border: '1px solid var(--color-border)',
  textAlign: 'center',
};

const emptyStateIcon: React.CSSProperties = {
  marginBottom: 16,
  opacity: 0.7,
};

const tableWrapper: React.CSSProperties = {
  borderRadius: 'var(--radius)',
  overflow: 'hidden',
  boxShadow: 'var(--shadow)',
  border: '1px solid var(--color-border)',
};

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  background: 'var(--color-surface)',
};

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '10px 16px',
  fontSize: 11,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  borderBottom: '1px solid var(--color-border)',
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
};

const tdStyle: React.CSSProperties = {
  padding: '10px 16px',
  borderBottom: '1px solid var(--color-border)',
};

const keyCodeStyle: React.CSSProperties = {
  fontSize: 13,
  fontWeight: 600,
  fontFamily: 'monospace',
  letterSpacing: '0.02em',
};

const valueCodeStyle: React.CSSProperties = {
  fontSize: 13,
  fontFamily: 'monospace',
  color: 'var(--color-text-secondary)',
};

const errorStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '10px 14px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
  border: '1px solid var(--color-danger)',
  fontWeight: 500,
};
