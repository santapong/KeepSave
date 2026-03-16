import { useState, useEffect, useCallback } from 'react';
import type { OAuthClient } from '../types/mcp';
import * as api from '../api/client';

export function OAuthClientsPage() {
  const [clients, setClients] = useState<OAuthClient[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [newSecret, setNewSecret] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setClients(await api.listOAuthClients());
    } catch {
      // ignore
    }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: 'var(--color-text)' }}>OAuth Clients</h1>
          <p style={{ margin: '4px 0 0', fontSize: 13, color: 'var(--color-text-secondary)' }}>
            Manage OAuth 2.0 client applications that authenticate via KeepSave
          </p>
        </div>
        <button onClick={() => { setShowCreate(true); setNewSecret(null); }} style={primaryBtn}>
          + Register Client
        </button>
      </div>

      {newSecret && (
        <div style={secretBanner}>
          <strong>Client Secret (copy now, shown only once):</strong>
          <code style={secretCode}>{newSecret}</code>
          <button onClick={() => { navigator.clipboard.writeText(newSecret); }} style={copyBtn}>Copy</button>
        </div>
      )}

      {loading ? (
        <p style={{ color: 'var(--color-text-secondary)' }}>Loading...</p>
      ) : clients.length === 0 ? (
        <p style={{ color: 'var(--color-text-secondary)' }}>No OAuth clients registered yet.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {clients.map(client => (
            <div key={client.id} style={clientCard}>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                  <span style={{ fontWeight: 600, fontSize: 14, color: 'var(--color-text)' }}>{client.name}</span>
                  {client.is_public && <span style={publicBadge}>Public</span>}
                </div>
                <div style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 4 }}>
                  {client.description}
                </div>
                <div style={{ fontSize: 11, color: 'var(--color-text-secondary)' }}>
                  <strong>Client ID:</strong> <code>{client.client_id}</code>
                </div>
                <div style={{ fontSize: 11, color: 'var(--color-text-secondary)', marginTop: 2 }}>
                  <strong>Scopes:</strong> {client.scopes.join(', ')} &middot; <strong>Grants:</strong> {client.grant_types.join(', ')}
                </div>
                <div style={{ fontSize: 11, color: 'var(--color-text-secondary)', marginTop: 2 }}>
                  <strong>Redirect URIs:</strong> {client.redirect_uris.join(', ') || 'none'}
                </div>
              </div>
              <button onClick={async () => { await api.deleteOAuthClient(client.id); load(); }} style={dangerBtn}>
                Delete
              </button>
            </div>
          ))}
        </div>
      )}

      {showCreate && (
        <CreateClientModal
          onClose={() => setShowCreate(false)}
          onCreated={(secret) => { setShowCreate(false); setNewSecret(secret); load(); }}
        />
      )}
    </div>
  );
}

function CreateClientModal({ onClose, onCreated }: { onClose: () => void; onCreated: (secret: string) => void }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [redirectURIs, setRedirectURIs] = useState('');
  const [scopes, setScopes] = useState('read');
  const [grantTypes, setGrantTypes] = useState('authorization_code');
  const [isPublic, setIsPublic] = useState(false);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!name) {
      setError('Name is required');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      const result = await api.registerOAuthClient(
        name,
        description,
        redirectURIs.split('\n').map(s => s.trim()).filter(Boolean),
        scopes.split(',').map(s => s.trim()).filter(Boolean),
        grantTypes.split(',').map(s => s.trim()).filter(Boolean),
        isPublic,
      );
      onCreated(result.client_secret);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to register client');
    }
    setSubmitting(false);
  };

  return (
    <div style={overlayStyle}>
      <div style={modalStyle}>
        <h2 style={{ margin: '0 0 16px', fontSize: 18, fontWeight: 700, color: 'var(--color-text)' }}>Register OAuth Client</h2>

        <label style={labelStyle}>Application Name *</label>
        <input value={name} onChange={e => setName(e.target.value)} style={inputStyle} placeholder="My Application" />

        <label style={labelStyle}>Description</label>
        <input value={description} onChange={e => setDescription(e.target.value)} style={inputStyle} placeholder="What does this app do?" />

        <label style={labelStyle}>Redirect URIs (one per line)</label>
        <textarea value={redirectURIs} onChange={e => setRedirectURIs(e.target.value)} style={{ ...inputStyle, minHeight: 60 }} placeholder="https://myapp.com/callback" />

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
          <div>
            <label style={labelStyle}>Scopes (comma-separated)</label>
            <input value={scopes} onChange={e => setScopes(e.target.value)} style={inputStyle} placeholder="read,write" />
          </div>
          <div>
            <label style={labelStyle}>Grant Types (comma-separated)</label>
            <input value={grantTypes} onChange={e => setGrantTypes(e.target.value)} style={inputStyle} placeholder="authorization_code,client_credentials" />
          </div>
        </div>

        <label style={{ ...labelStyle, display: 'flex', alignItems: 'center', gap: 8, marginTop: 16 }}>
          <input type="checkbox" checked={isPublic} onChange={e => setIsPublic(e.target.checked)} />
          Public client (no client_secret required for token exchange)
        </label>

        {error && <p style={{ color: '#ef4444', fontSize: 13, margin: '8px 0' }}>{error}</p>}

        <div style={{ display: 'flex', gap: 8, marginTop: 16, justifyContent: 'flex-end' }}>
          <button onClick={onClose} style={cancelBtn}>Cancel</button>
          <button onClick={handleSubmit} disabled={submitting} style={primaryBtn}>
            {submitting ? 'Registering...' : 'Register Client'}
          </button>
        </div>
      </div>
    </div>
  );
}

const primaryBtn: React.CSSProperties = {
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  padding: '8px 16px',
  borderRadius: 8,
  fontSize: 13,
  fontWeight: 600,
  cursor: 'pointer',
};

const cancelBtn: React.CSSProperties = {
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  padding: '8px 16px',
  borderRadius: 8,
  fontSize: 13,
  cursor: 'pointer',
};

const dangerBtn: React.CSSProperties = {
  background: 'transparent',
  color: '#ef4444',
  border: '1px solid #ef4444',
  padding: '4px 12px',
  borderRadius: 6,
  fontSize: 12,
  fontWeight: 500,
  cursor: 'pointer',
};

const clientCard: React.CSSProperties = {
  display: 'flex',
  alignItems: 'flex-start',
  background: 'var(--color-card)',
  border: '1px solid var(--color-border)',
  borderRadius: 10,
  padding: '14px 16px',
};

const publicBadge: React.CSSProperties = {
  fontSize: 10,
  fontWeight: 600,
  padding: '2px 8px',
  borderRadius: 10,
  background: '#6366f120',
  color: '#6366f1',
};

const secretBanner: React.CSSProperties = {
  background: '#fef3c7',
  border: '1px solid #f59e0b',
  borderRadius: 10,
  padding: '12px 16px',
  marginBottom: 16,
  fontSize: 13,
  color: '#92400e',
};

const secretCode: React.CSSProperties = {
  display: 'block',
  marginTop: 6,
  padding: '6px 10px',
  background: '#fff',
  border: '1px solid #e5e7eb',
  borderRadius: 6,
  fontFamily: 'monospace',
  fontSize: 12,
  wordBreak: 'break-all',
};

const copyBtn: React.CSSProperties = {
  marginTop: 8,
  background: '#f59e0b',
  color: '#fff',
  border: 'none',
  padding: '4px 12px',
  borderRadius: 6,
  fontSize: 12,
  fontWeight: 600,
  cursor: 'pointer',
};

const overlayStyle: React.CSSProperties = {
  position: 'fixed',
  top: 0,
  left: 0,
  right: 0,
  bottom: 0,
  background: 'rgba(0,0,0,0.5)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  zIndex: 100,
};

const modalStyle: React.CSSProperties = {
  background: 'var(--color-card)',
  borderRadius: 16,
  padding: 24,
  width: '100%',
  maxWidth: 520,
  maxHeight: '90vh',
  overflow: 'auto',
};

const labelStyle: React.CSSProperties = {
  display: 'block',
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  marginBottom: 4,
  marginTop: 12,
};

const inputStyle: React.CSSProperties = {
  width: '100%',
  padding: '8px 12px',
  border: '1px solid var(--color-border)',
  borderRadius: 8,
  background: 'var(--color-bg)',
  color: 'var(--color-text)',
  fontSize: 13,
  boxSizing: 'border-box',
};
