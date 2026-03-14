import { useState, useEffect, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { listProjects, createProject, deleteProject } from '../api/client';
import type { Project } from '../types';

export function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    loadProjects();
  }, []);

  async function loadProjects() {
    try {
      const data = await listProjects();
      setProjects(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load projects');
    } finally {
      setLoading(false);
    }
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    try {
      await createProject(name, description);
      setName('');
      setDescription('');
      setShowCreate(false);
      loadProjects();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project');
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm('Delete this project? This cannot be undone.')) return;
    try {
      await deleteProject(id);
      loadProjects();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete project');
    }
  }

  if (loading) {
    return <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-secondary)' }}>Loading projects...</div>;
  }

  return (
    <div>
      <div style={headerRow}>
        <div>
          <h1 style={{ fontSize: 24, fontWeight: 700 }}>Projects</h1>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginTop: 2 }}>
            {projects.length} project{projects.length !== 1 ? 's' : ''}
          </p>
        </div>
        <button onClick={() => setShowCreate(!showCreate)} style={btnPrimary}>
          {showCreate ? 'Cancel' : '+ New Project'}
        </button>
      </div>

      {error && <div style={errorStyle}>{error}</div>}

      {showCreate && (
        <form onSubmit={handleCreate} style={createForm}>
          <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 12 }}>Create Project</h3>
          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
            <input
              placeholder="Project name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              style={inputStyle}
            />
            <input
              placeholder="Description (optional)"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              style={{ ...inputStyle, flex: 2 }}
            />
            <button type="submit" style={btnPrimary}>Create</button>
          </div>
        </form>
      )}

      {projects.length === 0 ? (
        <div style={emptyState}>
          <p style={{ fontSize: 16, fontWeight: 500, marginBottom: 4 }}>No projects yet</p>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>Create your first project to start managing secrets securely.</p>
        </div>
      ) : (
        <div style={{ display: 'grid', gap: 12 }}>
          {projects.map((p) => (
            <div key={p.id} style={projectCard}>
              <div
                style={{ flex: 1, cursor: 'pointer' }}
                onClick={() => navigate(`/projects/${p.id}`)}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <div style={projectIcon}>{p.name.charAt(0).toUpperCase()}</div>
                  <div>
                    <h3 style={{ fontSize: 15, fontWeight: 600 }}>{p.name}</h3>
                    {p.description && (
                      <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginTop: 2 }}>
                        {p.description}
                      </p>
                    )}
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 16, marginTop: 12 }}>
                  <span style={metaTag}>
                    Created {new Date(p.created_at).toLocaleDateString()}
                  </span>
                  <span style={metaTag}>3 environments</span>
                </div>
              </div>
              <button
                onClick={(e) => { e.stopPropagation(); handleDelete(p.id); }}
                style={btnDanger}
              >
                Delete
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

const headerRow: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  marginBottom: 24,
};

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 600,
};

const btnDanger: React.CSSProperties = {
  padding: '6px 12px',
  background: 'transparent',
  color: 'var(--color-danger)',
  border: '1px solid rgba(239, 68, 68, 0.3)',
  borderRadius: 'var(--radius)',
  fontSize: 12,
  alignSelf: 'flex-start',
};

const projectCard: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: 20,
  display: 'flex',
  alignItems: 'flex-start',
  gap: 16,
};

const projectIcon: React.CSSProperties = {
  width: 36,
  height: 36,
  borderRadius: 8,
  background: 'rgba(99, 102, 241, 0.15)',
  color: 'var(--color-primary)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontWeight: 700,
  fontSize: 16,
  flexShrink: 0,
};

const metaTag: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--color-text-secondary)',
  padding: '2px 8px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
};

const createForm: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: 20,
  marginBottom: 24,
};

const inputStyle: React.CSSProperties = {
  padding: '8px 12px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 14,
  color: 'var(--color-text)',
  flex: 1,
  minWidth: 200,
};

const emptyState: React.CSSProperties = {
  textAlign: 'center',
  padding: 60,
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
};

const errorStyle: React.CSSProperties = {
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '10px 14px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
  border: '1px solid rgba(239, 68, 68, 0.2)',
};
