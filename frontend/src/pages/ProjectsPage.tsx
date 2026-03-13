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

  if (loading) return <p>Loading projects...</p>;

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700 }}>Projects</h1>
        <button onClick={() => setShowCreate(!showCreate)} style={btnPrimary}>
          {showCreate ? 'Cancel' : 'New Project'}
        </button>
      </div>

      {error && <div style={errorStyle}>{error}</div>}

      {showCreate && (
        <form onSubmit={handleCreate} style={formCard}>
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
            style={inputStyle}
          />
          <button type="submit" style={btnPrimary}>Create</button>
        </form>
      )}

      {projects.length === 0 ? (
        <p style={{ color: 'var(--color-text-secondary)' }}>No projects yet. Create your first project to get started.</p>
      ) : (
        <div style={{ display: 'grid', gap: 12 }}>
          {projects.map((p) => (
            <div
              key={p.id}
              style={card}
            >
              <div
                style={{ flex: 1, cursor: 'pointer' }}
                onClick={() => navigate(`/projects/${p.id}`)}
              >
                <h3 style={{ fontSize: 16, fontWeight: 600 }}>{p.name}</h3>
                {p.description && (
                  <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginTop: 4 }}>
                    {p.description}
                  </p>
                )}
                <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 8 }}>
                  Created {new Date(p.created_at).toLocaleDateString()}
                </p>
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
  border: '1px solid var(--color-danger)',
  borderRadius: 'var(--radius)',
  fontSize: 12,
};

const card: React.CSSProperties = {
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  padding: 20,
  display: 'flex',
  alignItems: 'center',
  gap: 16,
};

const formCard: React.CSSProperties = {
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  padding: 20,
  display: 'flex',
  gap: 12,
  marginBottom: 24,
  flexWrap: 'wrap',
};

const inputStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 14,
  flex: 1,
  minWidth: 200,
};

const errorStyle: React.CSSProperties = {
  background: '#fef2f2',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
};
