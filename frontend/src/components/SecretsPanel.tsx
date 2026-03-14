import { useState, useEffect, type FormEvent } from 'react';
import { listSecrets, createSecret, updateSecret, deleteSecret } from '../api/client';
import type { Secret } from '../types';

const ENVIRONMENTS = ['alpha', 'uat', 'prod'] as const;

interface SecretsPanelProps {
  projectId: string;
}

export function SecretsPanel({ projectId }: SecretsPanelProps) {
  const [env, setEnv] = useState<string>('alpha');
  const [secrets, setSecrets] = useState<Secret[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showAdd, setShowAdd] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');
  const [revealed, setRevealed] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');

  useEffect(() => {
    loadSecrets();
    setRevealed(new Set());
  }, [projectId, env]); // eslint-disable-line react-hooks/exhaustive-deps

  async function loadSecrets() {
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
  }

  async function handleAdd(e: FormEvent) {
    e.preventDefault();
    try {
      await createSecret(projectId, newKey, newValue, env);
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
    if (!window.confirm('Delete this secret?')) return;
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

  function maskValue(value: string | undefined): string {
    if (!value) return '••••••••';
    return '••••••••';
  }

  return (
    <div>
      {/* Environment Tabs */}
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        {ENVIRONMENTS.map((e) => (
          <button
            key={e}
            onClick={() => setEnv(e)}
            style={{
              padding: '6px 16px',
              borderRadius: 'var(--radius)',
              border: '1px solid',
              borderColor: env === e ? 'var(--color-primary)' : 'var(--color-border)',
              background: env === e ? 'var(--color-primary)' : 'var(--color-surface)',
              color: env === e ? '#fff' : 'var(--color-text)',
              fontSize: 13,
              fontWeight: 600,
              textTransform: 'uppercase',
            }}
          >
            {e}
          </button>
        ))}
        <div style={{ flex: 1 }} />
        <button onClick={() => setShowAdd(!showAdd)} style={btnPrimary}>
          {showAdd ? 'Cancel' : 'Add Secret'}
        </button>
      </div>

      {error && <div style={errorStyle}>{error}</div>}

      {showAdd && (
        <form onSubmit={handleAdd} style={formRow}>
          <input
            placeholder="KEY_NAME"
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            required
            style={inputStyle}
          />
          <input
            placeholder="Secret value"
            value={newValue}
            onChange={(e) => setNewValue(e.target.value)}
            required
            style={{ ...inputStyle, flex: 2 }}
          />
          <button type="submit" style={btnPrimary}>Save</button>
        </form>
      )}

      {loading ? (
        <p>Loading secrets...</p>
      ) : secrets.length === 0 ? (
        <p style={{ color: 'var(--color-text-secondary)', padding: 20 }}>
          No secrets in {env.toUpperCase()} environment.
        </p>
      ) : (
        <table style={tableStyle}>
          <thead>
            <tr>
              <th style={thStyle}>Key</th>
              <th style={thStyle}>Value</th>
              <th style={{ ...thStyle, width: 180 }}>Updated</th>
              <th style={{ ...thStyle, width: 200 }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {secrets.map((s) => (
              <tr key={s.id}>
                <td style={tdStyle}>
                  <code style={{ fontSize: 13, fontWeight: 600 }}>{s.key}</code>
                </td>
                <td style={tdStyle}>
                  {editing === s.id ? (
                    <div style={{ display: 'flex', gap: 8 }}>
                      <input
                        value={editValue}
                        onChange={(e) => setEditValue(e.target.value)}
                        style={{ ...inputStyle, flex: 1 }}
                        autoFocus
                      />
                      <button onClick={() => handleUpdate(s.id)} style={btnSmall}>Save</button>
                      <button onClick={() => setEditing(null)} style={btnSmallOutline}>Cancel</button>
                    </div>
                  ) : (
                    <code style={{ fontSize: 13 }}>
                      {revealed.has(s.id) ? s.value : maskValue(s.value)}
                    </code>
                  )}
                </td>
                <td style={tdStyle}>
                  <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                    {new Date(s.updated_at).toLocaleString()}
                  </span>
                </td>
                <td style={tdStyle}>
                  <div style={{ display: 'flex', gap: 6 }}>
                    <button onClick={() => toggleReveal(s.id)} style={btnSmallOutline}>
                      {revealed.has(s.id) ? 'Hide' : 'Reveal'}
                    </button>
                    <button
                      onClick={() => { setEditing(s.id); setEditValue(s.value || ''); }}
                      style={btnSmallOutline}
                    >
                      Edit
                    </button>
                    <button onClick={() => handleDelete(s.id)} style={btnSmallDanger}>
                      Delete
                    </button>
                  </div>
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
  padding: '6px 14px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 600,
};

const btnSmall: React.CSSProperties = {
  padding: '4px 10px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 4,
  fontSize: 12,
};

const btnSmallOutline: React.CSSProperties = {
  padding: '4px 10px',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  fontSize: 12,
};

const btnSmallDanger: React.CSSProperties = {
  padding: '4px 10px',
  background: 'transparent',
  color: 'var(--color-danger)',
  border: '1px solid var(--color-danger)',
  borderRadius: 4,
  fontSize: 12,
};

const formRow: React.CSSProperties = {
  display: 'flex',
  gap: 12,
  marginBottom: 16,
  background: 'var(--color-surface)',
  padding: 16,
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
};

const inputStyle: React.CSSProperties = {
  padding: '6px 10px',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  fontSize: 13,
  flex: 1,
};

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  overflow: 'hidden',
};

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '10px 16px',
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  borderBottom: '1px solid var(--color-border)',
  textTransform: 'uppercase',
};

const tdStyle: React.CSSProperties = {
  padding: '10px 16px',
  borderBottom: '1px solid var(--color-border)',
};

const errorStyle: React.CSSProperties = {
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
};
