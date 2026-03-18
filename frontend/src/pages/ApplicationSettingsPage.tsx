import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import type { APIKey } from '../types';
import * as api from '../api/client';

export function ApplicationSettingsPage() {
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [newKeyScopes, setNewKeyScopes] = useState<string[]>(['read']);
  const [createdKey, setCreatedKey] = useState('');
  const [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const keys = await api.listAPIKeys();
      setApiKeys(keys || []);
    } catch {
      // ignore
    }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const result = await api.createAPIKey(newKeyName, '', newKeyScopes);
      setCreatedKey(result.raw_key);
      setNewKeyName('');
      setNewKeyScopes(['read']);
      setShowCreate(false);
      load();
    } catch {
      // ignore
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Revoke this API key? This cannot be undone.')) return;
    try {
      await api.deleteAPIKey(id);
      load();
    } catch {
      // ignore
    }
  };

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const toggleScope = (scope: string) => {
    setNewKeyScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
    );
  };

  return (
    <div>
      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <Link to="/applications" style={{ fontSize: 13, color: 'var(--color-primary)', textDecoration: 'none', marginBottom: 8, display: 'inline-block' }}>
          &larr; Back to Applications
        </Link>
        <h1 style={{ margin: '8px 0 0', fontSize: 22, fontWeight: 700, color: 'var(--color-text)' }}>
          Application Dashboard Settings
        </h1>
        <p style={{ margin: '4px 0 0', fontSize: 13, color: 'var(--color-text-secondary)' }}>
          Manage API keys and view integration documentation
        </p>
      </div>

      {/* Created Key Banner */}
      {createdKey && (
        <div style={{ padding: 16, borderRadius: 12, background: '#ecfdf5', border: '1px solid #6ee7b7', marginBottom: 24 }}>
          <p style={{ margin: '0 0 8px', fontSize: 13, fontWeight: 600, color: '#065f46' }}>
            API Key Created! Copy it now — it won't be shown again.
          </p>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <code style={{ flex: 1, padding: '8px 12px', background: '#fff', borderRadius: 8, fontSize: 12, fontFamily: 'monospace', wordBreak: 'break-all', color: '#065f46' }}>
              {createdKey}
            </code>
            <button onClick={() => handleCopy(createdKey)} style={primaryBtn}>
              {copied ? 'Copied!' : 'Copy'}
            </button>
          </div>
        </div>
      )}

      {/* Two Column Layout */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
        {/* Left: API Keys */}
        <div>
          <div style={{ ...sectionCard, marginBottom: 16 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <h2 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: 'var(--color-text)' }}>API Keys</h2>
              <button onClick={() => setShowCreate(!showCreate)} style={primaryBtn}>
                {showCreate ? 'Cancel' : '+ New Key'}
              </button>
            </div>

            {showCreate && (
              <form onSubmit={handleCreate} style={{ padding: 16, background: 'var(--color-bg)', borderRadius: 10, marginBottom: 16, border: '1px solid var(--color-border)' }}>
                <div style={{ marginBottom: 12 }}>
                  <label style={labelStyle}>Key Name</label>
                  <input value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} required placeholder="e.g. ci-pipeline" style={inputStyle} />
                </div>
                <div style={{ marginBottom: 12 }}>
                  <label style={labelStyle}>Scopes</label>
                  <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    {['read', 'write', 'delete'].map((scope) => (
                      <label key={scope} style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 13, color: 'var(--color-text)', cursor: 'pointer' }}>
                        <input type="checkbox" checked={newKeyScopes.includes(scope)} onChange={() => toggleScope(scope)} />
                        {scope}
                      </label>
                    ))}
                  </div>
                </div>
                <button type="submit" style={{ ...primaryBtn, width: '100%' }}>Create API Key</button>
              </form>
            )}

            {loading ? (
              <p style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>Loading...</p>
            ) : apiKeys.length === 0 ? (
              <p style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>No API keys yet. Create one to get started.</p>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {apiKeys.map((key) => (
                  <div key={key.id} style={{ padding: 12, background: 'var(--color-bg)', borderRadius: 10, border: '1px solid var(--color-border)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div>
                      <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text)' }}>{key.name}</div>
                      <div style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 2 }}>
                        Scopes: {key.scopes?.join(', ') || 'read'} &middot; Created {new Date(key.created_at).toLocaleDateString()}
                      </div>
                    </div>
                    <button onClick={() => handleDelete(key.id)} style={{ ...dangerBtn }}>Revoke</button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right: API Reference */}
        <div style={sectionCard}>
          <h2 style={{ margin: '0 0 16px', fontSize: 16, fontWeight: 700, color: 'var(--color-text)' }}>API Reference</h2>
          <p style={{ margin: '0 0 16px', fontSize: 13, color: 'var(--color-text-secondary)' }}>
            Use these endpoints to manage applications programmatically. Authenticate with a Bearer token (JWT or API key).
          </p>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <EndpointDoc method="GET" path="/api/v1/applications" description="List applications with search, category filter, and pagination" params="?search=&category=&limit=50&offset=0" />
            <EndpointDoc method="POST" path="/api/v1/applications" description="Create a new application" body='{"name": "My App", "url": "https://...", "description": "...", "icon": "🚀", "category": "General"}' />
            <EndpointDoc method="GET" path="/api/v1/applications/:id" description="Get a single application by ID" />
            <EndpointDoc method="PUT" path="/api/v1/applications/:id" description="Update an application" body='{"name": "...", "url": "...", ...}' />
            <EndpointDoc method="DELETE" path="/api/v1/applications/:id" description="Delete an application" />
            <EndpointDoc method="POST" path="/api/v1/applications/:id/favorite" description="Toggle favorite status" />
          </div>

          <h3 style={{ margin: '24px 0 8px', fontSize: 14, fontWeight: 700, color: 'var(--color-text)' }}>Example Usage</h3>
          <pre style={codeBlock}>{`curl -X GET \\
  http://localhost:8080/api/v1/applications \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json"

curl -X POST \\
  http://localhost:8080/api/v1/applications \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"MedQCNN","url":"http://localhost:8000","icon":"🧬","category":"AI & ML"}'`}</pre>
        </div>
      </div>
    </div>
  );
}

function EndpointDoc({ method, path, description, params, body }: { method: string; path: string; description: string; params?: string; body?: string }) {
  const methodColors: Record<string, string> = {
    GET: '#22c55e', POST: '#3b82f6', PUT: '#f59e0b', DELETE: '#ef4444',
  };
  return (
    <div style={{ padding: 10, background: 'var(--color-bg)', borderRadius: 8, border: '1px solid var(--color-border)' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
        <span style={{ padding: '2px 8px', borderRadius: 4, background: methodColors[method] || '#6b7280', color: '#fff', fontSize: 11, fontWeight: 700, fontFamily: 'monospace' }}>{method}</span>
        <code style={{ fontSize: 12, color: 'var(--color-text)', fontFamily: 'monospace' }}>{path}{params || ''}</code>
      </div>
      <p style={{ margin: 0, fontSize: 12, color: 'var(--color-text-secondary)' }}>{description}</p>
      {body && <pre style={{ margin: '6px 0 0', fontSize: 11, color: 'var(--color-text-secondary)', fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>{body}</pre>}
    </div>
  );
}

// --- Styles ---

const primaryBtn: React.CSSProperties = {
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  padding: '6px 14px',
  borderRadius: 8,
  fontSize: 12,
  fontWeight: 600,
  cursor: 'pointer',
};

const dangerBtn: React.CSSProperties = {
  background: 'transparent',
  color: '#ef4444',
  border: '1px solid #fca5a5',
  padding: '4px 10px',
  borderRadius: 6,
  fontSize: 11,
  fontWeight: 600,
  cursor: 'pointer',
};

const sectionCard: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 12,
  padding: 20,
};

const labelStyle: React.CSSProperties = {
  display: 'block',
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  marginBottom: 6,
};

const inputStyle: React.CSSProperties = {
  width: '100%',
  padding: '8px 12px',
  borderRadius: 8,
  border: '1px solid var(--color-border)',
  background: 'var(--color-surface)',
  color: 'var(--color-text)',
  fontSize: 13,
  outline: 'none',
  boxSizing: 'border-box',
};

const codeBlock: React.CSSProperties = {
  padding: 14,
  background: 'var(--color-bg)',
  borderRadius: 8,
  border: '1px solid var(--color-border)',
  fontSize: 12,
  fontFamily: 'monospace',
  color: 'var(--color-text)',
  overflow: 'auto',
  whiteSpace: 'pre-wrap',
  lineHeight: 1.6,
};
