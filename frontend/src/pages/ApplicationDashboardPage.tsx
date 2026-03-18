import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import type { DashboardApplication } from '../types';
import { AppChatbot } from '../components/AppChatbot';
import * as api from '../api/client';

export function ApplicationDashboardPage() {
  const [apps, setApps] = useState<DashboardApplication[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [activeCategory, setActiveCategory] = useState('All');
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingApp, setEditingApp] = useState<DashboardApplication | null>(null);
  const [page, setPage] = useState(0);
  const pageSize = 50;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.listApplications(search, activeCategory, pageSize, page * pageSize);
      setApps(data.applications || []);
      setCategories(data.categories || []);
      setTotal(data.total || 0);
    } catch {
      // ignore
    }
    setLoading(false);
  }, [search, activeCategory, page]);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete application "${name}"?`)) return;
    try {
      await api.deleteApplication(id);
      load();
    } catch {
      // ignore
    }
  };

  const handleToggleFavorite = async (id: string) => {
    try {
      await api.toggleApplicationFavorite(id);
      load();
    } catch {
      // ignore
    }
  };

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: 'var(--color-text)' }}>
            Application Dashboard
          </h1>
          <p style={{ margin: '4px 0 0', fontSize: 13, color: 'var(--color-text-secondary)' }}>
            Manage your services and applications &middot; {total} registered
          </p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <Link to="/applications/settings" style={{ ...secondaryBtn, textDecoration: 'none', display: 'flex', alignItems: 'center' }}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ marginRight: 4 }}>
              <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" /><circle cx="12" cy="12" r="3" />
            </svg>
            Settings & API
          </Link>
          <button onClick={() => { setEditingApp(null); setShowAddForm(true); }} style={primaryBtn}>
            + Register Service
          </button>
        </div>
      </div>

      {/* Search & Filter */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 20, flexWrap: 'wrap', alignItems: 'center' }}>
        <input
          type="text"
          placeholder="Search applications..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={searchInput}
        />
        <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
          {['All', ...categories].map((cat) => (
            <button
              key={cat}
              onClick={() => setActiveCategory(cat)}
              style={{
                ...chipBtn,
                background: activeCategory === cat ? 'var(--color-primary)' : 'var(--color-surface)',
                color: activeCategory === cat ? '#fff' : 'var(--color-text-secondary)',
                borderColor: activeCategory === cat ? 'var(--color-primary)' : 'var(--color-border)',
              }}
            >
              {cat}
            </button>
          ))}
        </div>
      </div>

      {/* App Grid */}
      {loading ? (
        <p style={{ color: 'var(--color-text-secondary)' }}>Loading...</p>
      ) : apps.length === 0 ? (
        <div style={emptyState}>
          <div style={{ fontSize: 48, marginBottom: 12 }}>🚀</div>
          <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: 'var(--color-text)' }}>No Applications Registered</h3>
          <p style={{ margin: '8px 0 0', fontSize: 13, color: 'var(--color-text-secondary)' }}>
            Register your first service to get started
          </p>
        </div>
      ) : (
        <div style={gridStyle}>
          {apps.map((app) => (
            <AppCard
              key={app.id}
              app={app}
              onEdit={() => { setEditingApp(app); setShowAddForm(true); }}
              onDelete={() => handleDelete(app.id, app.name)}
              onToggleFavorite={() => handleToggleFavorite(app.id)}
            />
          ))}
        </div>
      )}

      {/* Pagination */}
      {total > pageSize && (
        <div style={{ display: 'flex', justifyContent: 'center', gap: 8, marginTop: 24 }}>
          <button
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
            style={{ ...paginationBtn, opacity: page === 0 ? 0.5 : 1 }}
          >
            Previous
          </button>
          <span style={{ display: 'flex', alignItems: 'center', fontSize: 13, color: 'var(--color-text-secondary)' }}>
            Page {page + 1} of {Math.ceil(total / pageSize)}
          </span>
          <button
            onClick={() => setPage((p) => p + 1)}
            disabled={(page + 1) * pageSize >= total}
            style={{ ...paginationBtn, opacity: (page + 1) * pageSize >= total ? 0.5 : 1 }}
          >
            Next
          </button>
        </div>
      )}

      {/* Add/Edit Form Modal */}
      {showAddForm && (
        <AddEditForm
          app={editingApp}
          onClose={() => { setShowAddForm(false); setEditingApp(null); }}
          onSaved={load}
        />
      )}

      {/* AI Chatbot */}
      <AppChatbot applications={apps} />
    </div>
  );
}

// --- AppCard Component ---

function AppCard({
  app,
  onEdit,
  onDelete,
  onToggleFavorite,
}: {
  app: DashboardApplication;
  onEdit: () => void;
  onDelete: () => void;
  onToggleFavorite: () => void;
}) {
  const isImage = app.icon?.startsWith('data:image');

  return (
    <div style={cardStyle}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 12 }}>
        <div style={iconContainer}>
          {isImage ? (
            <img src={app.icon} alt={app.name} style={{ width: '100%', height: '100%', objectFit: 'cover', borderRadius: 12 }} />
          ) : (
            <span style={{ fontSize: 28 }}>{app.icon || '🌐'}</span>
          )}
        </div>
        <div style={{ display: 'flex', gap: 4 }}>
          <button onClick={onToggleFavorite} style={{ ...iconBtn, color: app.is_favorite ? '#f59e0b' : 'var(--color-text-secondary)' }} title="Favorite">
            {app.is_favorite ? '\u2605' : '\u2606'}
          </button>
          <button onClick={onEdit} style={iconBtn} title="Edit">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
            </svg>
          </button>
          <button onClick={onDelete} style={{ ...iconBtn, color: 'var(--color-text-secondary)' }} title="Delete">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 6h18" /><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" /><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
            </svg>
          </button>
        </div>
      </div>

      <a href={app.url} target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none', flex: 1 }}>
        <h3 style={{ margin: '0 0 6px', fontSize: 16, fontWeight: 700, color: 'var(--color-text)' }}>
          {app.name}
        </h3>
        <span style={categoryBadge}>{app.category || 'General'}</span>
        <p style={{ margin: '10px 0 0', fontSize: 13, color: 'var(--color-text-secondary)', lineHeight: 1.5, overflow: 'hidden', display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical' as const }}>
          {app.description || 'No description provided.'}
        </p>
      </a>

      <div style={{ marginTop: 16, paddingTop: 12, borderTop: '1px solid var(--color-border)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <div style={{ width: 6, height: 6, borderRadius: '50%', background: '#22c55e' }} />
          <span style={{ fontSize: 11, color: 'var(--color-text-secondary)', fontWeight: 500 }}>Active</span>
        </div>
        <span style={{ fontSize: 11, color: 'var(--color-text-secondary)' }}>
          {new Date(app.created_at).toLocaleDateString()}
        </span>
      </div>
    </div>
  );
}

// --- Add/Edit Form ---

function AddEditForm({
  app,
  onClose,
  onSaved,
}: {
  app: DashboardApplication | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(app?.name || '');
  const [url, setUrl] = useState(app?.url || '');
  const [description, setDescription] = useState(app?.description || '');
  const [icon, setIcon] = useState(app?.icon || '🚀');
  const [category, setCategory] = useState(app?.category || 'General');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const isEdit = !!app;

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      const reader = new FileReader();
      reader.onloadend = () => setIcon(reader.result as string);
      reader.readAsDataURL(file);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError('');
    try {
      if (isEdit) {
        await api.updateApplication(app.id, name, url, description, icon, category);
      } else {
        await api.createApplication(name, url, description, icon, category);
      }
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save');
    }
    setSaving(false);
  };

  return (
    <>
      <div onClick={onClose} style={overlayStyle} />
      <div style={panelStyle}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
          <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--color-text)' }}>
            {isEdit ? 'Edit Application' : 'Register New Service'}
          </h2>
          <button onClick={onClose} style={iconBtn}>&times;</button>
        </div>

        {error && <div style={{ padding: '8px 12px', borderRadius: 8, background: '#fef2f2', color: '#dc2626', fontSize: 13, marginBottom: 16 }}>{error}</div>}

        <form onSubmit={handleSubmit}>
          <div style={fieldGroup}>
            <label style={labelStyle}>URL</label>
            <input value={url} onChange={(e) => setUrl(e.target.value)} required placeholder="https://..." style={inputStyle} />
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 16, padding: 16, background: 'var(--color-surface)', borderRadius: 12, border: '1px solid var(--color-border)' }}>
            <div style={{ position: 'relative', width: 56, height: 56, borderRadius: 12, background: 'var(--color-bg)', border: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', justifyContent: 'center', overflow: 'hidden', cursor: 'pointer', flexShrink: 0 }}>
              {icon.startsWith('data:image') ? (
                <img src={icon} alt="Icon" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
              ) : (
                <span style={{ fontSize: 28 }}>{icon}</span>
              )}
              <input type="file" accept="image/*" onChange={handleFileUpload} style={{ position: 'absolute', inset: 0, opacity: 0, cursor: 'pointer' }} />
            </div>
            <div style={{ flex: 1 }}>
              <label style={labelStyle}>Icon (emoji or upload image)</label>
              <input value={icon} onChange={(e) => setIcon(e.target.value || '🚀')} maxLength={2} placeholder="Emoji" style={{ ...inputStyle, marginTop: 4 }} />
            </div>
          </div>

          <div style={fieldGroup}>
            <label style={labelStyle}>Name</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required placeholder="Service name" style={inputStyle} />
          </div>

          <div style={fieldGroup}>
            <label style={labelStyle}>Description</label>
            <textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What does this service do?" rows={3} style={{ ...inputStyle, resize: 'vertical' }} />
          </div>

          <div style={fieldGroup}>
            <label style={labelStyle}>Category</label>
            <select value={category} onChange={(e) => setCategory(e.target.value)} style={inputStyle}>
              <option value="General">General</option>
              <option value="Automation">Automation</option>
              <option value="Dev Tools">Dev Tools</option>
              <option value="Infrastructure">Infrastructure</option>
              <option value="Personal">Personal</option>
              <option value="AI & ML">AI & ML</option>
              <option value="Monitoring">Monitoring</option>
              <option value="Security">Security</option>
            </select>
          </div>

          <button type="submit" disabled={saving} style={{ ...primaryBtn, width: '100%', marginTop: 8, opacity: saving ? 0.7 : 1 }}>
            {saving ? 'Saving...' : isEdit ? 'Update Application' : 'Register Service'}
          </button>
        </form>
      </div>
    </>
  );
}

// --- Styles ---

const primaryBtn: React.CSSProperties = {
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  padding: '8px 16px',
  borderRadius: 8,
  fontSize: 13,
  fontWeight: 600,
  cursor: 'pointer',
  whiteSpace: 'nowrap',
};

const secondaryBtn: React.CSSProperties = {
  background: 'var(--color-surface)',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  padding: '8px 14px',
  borderRadius: 8,
  fontSize: 13,
  fontWeight: 500,
  cursor: 'pointer',
  whiteSpace: 'nowrap',
};

const paginationBtn: React.CSSProperties = {
  background: 'var(--color-surface)',
  color: 'var(--color-text)',
  border: '1px solid var(--color-border)',
  padding: '6px 14px',
  borderRadius: 8,
  fontSize: 13,
  cursor: 'pointer',
};

const searchInput: React.CSSProperties = {
  padding: '8px 14px',
  borderRadius: 8,
  border: '1px solid var(--color-border)',
  background: 'var(--color-surface)',
  color: 'var(--color-text)',
  fontSize: 13,
  outline: 'none',
  minWidth: 220,
  flex: 1,
  maxWidth: 360,
};

const chipBtn: React.CSSProperties = {
  padding: '5px 12px',
  borderRadius: 16,
  border: '1px solid var(--color-border)',
  fontSize: 12,
  fontWeight: 500,
  cursor: 'pointer',
  transition: 'all 0.15s',
};

const gridStyle: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
  gap: 16,
};

const cardStyle: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 12,
  padding: 20,
  display: 'flex',
  flexDirection: 'column',
  transition: 'box-shadow 0.2s, border-color 0.2s',
};

const iconContainer: React.CSSProperties = {
  width: 48,
  height: 48,
  borderRadius: 12,
  background: 'var(--color-bg)',
  border: '1px solid var(--color-border)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  overflow: 'hidden',
  flexShrink: 0,
};

const iconBtn: React.CSSProperties = {
  background: 'transparent',
  border: 'none',
  cursor: 'pointer',
  padding: 4,
  borderRadius: 6,
  color: 'var(--color-text-secondary)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontSize: 16,
};

const categoryBadge: React.CSSProperties = {
  display: 'inline-block',
  padding: '2px 10px',
  borderRadius: 12,
  background: 'var(--color-primary-glow)',
  color: 'var(--color-primary)',
  fontSize: 11,
  fontWeight: 600,
};

const emptyState: React.CSSProperties = {
  textAlign: 'center',
  padding: '60px 20px',
};

const overlayStyle: React.CSSProperties = {
  position: 'fixed',
  inset: 0,
  background: 'rgba(0,0,0,0.4)',
  zIndex: 100,
};

const panelStyle: React.CSSProperties = {
  position: 'fixed',
  top: 0,
  right: 0,
  width: 420,
  maxWidth: '100vw',
  height: '100vh',
  background: 'var(--color-nav)',
  borderLeft: '1px solid var(--color-border)',
  zIndex: 101,
  padding: 24,
  overflowY: 'auto',
};

const fieldGroup: React.CSSProperties = {
  marginBottom: 16,
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
  background: 'var(--color-bg)',
  color: 'var(--color-text)',
  fontSize: 13,
  outline: 'none',
  boxSizing: 'border-box',
};
