import { useState, useEffect, useCallback } from 'react';
import type { MCPServer, MCPInstallation } from '../types/mcp';
import * as api from '../api/client';

export function MCPHubPage() {
  const [tab, setTab] = useState<'marketplace' | 'installed' | 'my-servers'>('marketplace');
  const [publicServers, setPublicServers] = useState<MCPServer[]>([]);
  const [myServers, setMyServers] = useState<MCPServer[]>([]);
  const [installations, setInstallations] = useState<MCPInstallation[]>([]);
  const [showRegister, setShowRegister] = useState(false);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [pub, mine, installs] = await Promise.all([
        api.listPublicMCPServers(),
        api.listMyMCPServers(),
        api.listMCPInstallations(),
      ]);
      setPublicServers(pub);
      setMyServers(mine);
      setInstallations(installs);
    } catch {
      // ignore
    }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: 'var(--color-text)' }}>MCP Server Hub</h1>
        <button onClick={() => setShowRegister(true)} style={primaryBtn}>
          + Add MCP Server
        </button>
      </div>

      <div style={{ display: 'flex', gap: 4, marginBottom: 20 }}>
        {(['marketplace', 'installed', 'my-servers'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            style={{
              ...tabBtn,
              background: tab === t ? 'var(--color-primary)' : 'transparent',
              color: tab === t ? '#fff' : 'var(--color-text-secondary)',
            }}
          >
            {t === 'marketplace' ? 'Marketplace' : t === 'installed' ? 'Installed' : 'My Servers'}
          </button>
        ))}
      </div>

      {loading ? (
        <p style={{ color: 'var(--color-text-secondary)' }}>Loading...</p>
      ) : (
        <>
          {tab === 'marketplace' && (
            <ServerGrid servers={publicServers} installations={installations} onInstall={async (id) => {
              await api.installMCPServer(id);
              load();
            }} />
          )}
          {tab === 'installed' && (
            <InstalledList installations={installations} servers={[...publicServers, ...myServers]} onUninstall={async (id) => {
              await api.uninstallMCPServer(id);
              load();
            }} />
          )}
          {tab === 'my-servers' && (
            <MyServersList servers={myServers} onDelete={async (id) => {
              await api.deleteMCPServer(id);
              load();
            }} onRebuild={async (id) => {
              await api.rebuildMCPServer(id);
              load();
            }} />
          )}
        </>
      )}

      {showRegister && (
        <RegisterServerModal onClose={() => setShowRegister(false)} onCreated={() => { setShowRegister(false); load(); }} />
      )}
    </div>
  );
}

function ServerGrid({ servers, installations, onInstall }: {
  servers: MCPServer[];
  installations: MCPInstallation[];
  onInstall: (id: string) => Promise<void>;
}) {
  const installedIds = new Set(installations.map(i => i.mcp_server_id));

  if (servers.length === 0) {
    return <p style={{ color: 'var(--color-text-secondary)' }}>No public MCP servers available yet.</p>;
  }

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 16 }}>
      {servers.map(server => (
        <div key={server.id} style={cardStyle}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <div>
              <h3 style={{ margin: '0 0 4px', fontSize: 15, fontWeight: 600, color: 'var(--color-text)' }}>{server.name}</h3>
              <p style={{ margin: '0 0 8px', fontSize: 12, color: 'var(--color-text-secondary)' }}>{server.description}</p>
            </div>
            <StatusBadge status={server.status} />
          </div>
          <div style={{ fontSize: 11, color: 'var(--color-text-secondary)', marginBottom: 8 }}>
            v{server.version} &middot; {server.install_count} installs &middot; {server.transport}
          </div>
          <div style={{ fontSize: 11, color: 'var(--color-text-secondary)', marginBottom: 12, wordBreak: 'break-all' }}>
            {server.github_url}
          </div>
          {installedIds.has(server.id) ? (
            <span style={{ fontSize: 12, color: 'var(--color-success)', fontWeight: 600 }}>Installed</span>
          ) : (
            <button onClick={() => onInstall(server.id)} style={smallBtn}>Install</button>
          )}
        </div>
      ))}
    </div>
  );
}

function InstalledList({ installations, servers, onUninstall }: {
  installations: MCPInstallation[];
  servers: MCPServer[];
  onUninstall: (id: string) => Promise<void>;
}) {
  const serverMap = new Map(servers.map(s => [s.id, s]));

  if (installations.length === 0) {
    return <p style={{ color: 'var(--color-text-secondary)' }}>No MCP servers installed. Browse the marketplace to get started.</p>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {installations.map(inst => {
        const server = serverMap.get(inst.mcp_server_id);
        return (
          <div key={inst.id} style={listItemStyle}>
            <div style={{ flex: 1 }}>
              <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--color-text)' }}>
                {server?.name || inst.mcp_server_id}
              </div>
              <div style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                {server?.description || ''} &middot; {inst.enabled ? 'Enabled' : 'Disabled'}
              </div>
            </div>
            <button onClick={() => onUninstall(inst.id)} style={dangerSmallBtn}>Uninstall</button>
          </div>
        );
      })}
    </div>
  );
}

function MyServersList({ servers, onDelete, onRebuild }: {
  servers: MCPServer[];
  onDelete: (id: string) => Promise<void>;
  onRebuild: (id: string) => Promise<void>;
}) {
  if (servers.length === 0) {
    return <p style={{ color: 'var(--color-text-secondary)' }}>You haven't registered any MCP servers yet.</p>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {servers.map(server => (
        <div key={server.id} style={listItemStyle}>
          <div style={{ flex: 1 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontWeight: 600, fontSize: 14, color: 'var(--color-text)' }}>{server.name}</span>
              <StatusBadge status={server.status} />
              {server.is_public && <span style={publicBadge}>Public</span>}
            </div>
            <div style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 2 }}>
              {server.github_url} &middot; {server.github_branch} &middot; v{server.version}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 6 }}>
            <button onClick={() => onRebuild(server.id)} style={smallBtn}>Rebuild</button>
            <button onClick={() => onDelete(server.id)} style={dangerSmallBtn}>Delete</button>
          </div>
        </div>
      ))}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    ready: '#22c55e',
    building: '#f59e0b',
    pending: '#6b7280',
    error: '#ef4444',
  };
  return (
    <span style={{
      fontSize: 10,
      fontWeight: 600,
      padding: '2px 8px',
      borderRadius: 10,
      background: (colors[status] || '#6b7280') + '20',
      color: colors[status] || '#6b7280',
      textTransform: 'uppercase',
    }}>
      {status}
    </span>
  );
}

function RegisterServerModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [githubUrl, setGithubUrl] = useState('');
  const [branch, setBranch] = useState('main');
  const [entryCommand, setEntryCommand] = useState('');
  const [transport, setTransport] = useState('stdio');
  const [isPublic, setIsPublic] = useState(false);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!name || !githubUrl) {
      setError('Name and GitHub URL are required');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      await api.registerMCPServer(name, description, githubUrl, branch, entryCommand, transport, {}, isPublic);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to register server');
    }
    setSubmitting(false);
  };

  return (
    <div style={overlayStyle}>
      <div style={modalStyle}>
        <h2 style={{ margin: '0 0 16px', fontSize: 18, fontWeight: 700, color: 'var(--color-text)' }}>Register MCP Server</h2>

        <label style={labelStyle}>Name *</label>
        <input value={name} onChange={e => setName(e.target.value)} style={inputStyle} placeholder="my-mcp-server" />

        <label style={labelStyle}>GitHub URL *</label>
        <input value={githubUrl} onChange={e => setGithubUrl(e.target.value)} style={inputStyle} placeholder="https://github.com/user/mcp-server" />

        <label style={labelStyle}>Description</label>
        <input value={description} onChange={e => setDescription(e.target.value)} style={inputStyle} placeholder="What does this server do?" />

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
          <div>
            <label style={labelStyle}>Branch</label>
            <input value={branch} onChange={e => setBranch(e.target.value)} style={inputStyle} />
          </div>
          <div>
            <label style={labelStyle}>Transport</label>
            <select value={transport} onChange={e => setTransport(e.target.value)} style={inputStyle}>
              <option value="stdio">stdio</option>
              <option value="sse">SSE</option>
              <option value="streamable-http">Streamable HTTP</option>
            </select>
          </div>
        </div>

        <label style={labelStyle}>Entry Command (auto-detected if empty)</label>
        <input value={entryCommand} onChange={e => setEntryCommand(e.target.value)} style={inputStyle} placeholder="node dist/index.js" />

        <label style={{ ...labelStyle, display: 'flex', alignItems: 'center', gap: 8 }}>
          <input type="checkbox" checked={isPublic} onChange={e => setIsPublic(e.target.checked)} />
          Make this server public in the marketplace
        </label>

        {error && <p style={{ color: '#ef4444', fontSize: 13, margin: '8px 0' }}>{error}</p>}

        <div style={{ display: 'flex', gap: 8, marginTop: 16, justifyContent: 'flex-end' }}>
          <button onClick={onClose} style={cancelBtn}>Cancel</button>
          <button onClick={handleSubmit} disabled={submitting} style={primaryBtn}>
            {submitting ? 'Registering...' : 'Register Server'}
          </button>
        </div>
      </div>
    </div>
  );
}

// Styles
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

const tabBtn: React.CSSProperties = {
  border: 'none',
  padding: '6px 14px',
  borderRadius: 8,
  fontSize: 13,
  fontWeight: 500,
  cursor: 'pointer',
  transition: 'background 0.15s',
};

const cardStyle: React.CSSProperties = {
  background: 'var(--color-card)',
  border: '1px solid var(--color-border)',
  borderRadius: 12,
  padding: 16,
};

const listItemStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  background: 'var(--color-card)',
  border: '1px solid var(--color-border)',
  borderRadius: 10,
  padding: '12px 16px',
};

const smallBtn: React.CSSProperties = {
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  padding: '4px 12px',
  borderRadius: 6,
  fontSize: 12,
  fontWeight: 600,
  cursor: 'pointer',
};

const dangerSmallBtn: React.CSSProperties = {
  background: 'transparent',
  color: '#ef4444',
  border: '1px solid #ef4444',
  padding: '4px 12px',
  borderRadius: 6,
  fontSize: 12,
  fontWeight: 500,
  cursor: 'pointer',
};

const publicBadge: React.CSSProperties = {
  fontSize: 10,
  fontWeight: 600,
  padding: '2px 8px',
  borderRadius: 10,
  background: '#6366f120',
  color: '#6366f1',
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
