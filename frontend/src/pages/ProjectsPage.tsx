import { useState, useEffect, useRef, type FormEvent } from 'react';
import { useNavigate, Link, useSearchParams } from 'react-router-dom';
import { listProjects, createProject, deleteProject, importEnv } from '../api/client';
import type { Project } from '../types';
import { useToast } from '@/hooks/useToast';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Page, PageHeader, Chip } from '../components/cosmic/primitives';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../components/ui/dialog';
import { EventHorizon } from '../components/cosmic/EventHorizon';

const IMPORT_ENVIRONMENTS = ['alpha', 'uat', 'prod'] as const;
type ImportEnv = (typeof IMPORT_ENVIRONMENTS)[number];

export function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchParams, setSearchParams] = useSearchParams();
  const showCreate = searchParams.get('new') === 'project';
  function setShowCreate(open: boolean) {
    setSearchParams((params) => {
      if (open) params.set('new', 'project');
      else params.delete('new');
      return params;
    });
  }
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<Project | null>(null);
  const [filter, setFilter] = useState('');
  const [importOpen, setImportOpen] = useState(false);
  const [importProjectId, setImportProjectId] = useState('');
  const [importEnvironment, setImportEnvironment] = useState<ImportEnv>('alpha');
  const [importOverwrite, setImportOverwrite] = useState(false);
  const [importing, setImporting] = useState(false);
  const importFileRef = useRef<HTMLInputElement | null>(null);
  const navigate = useNavigate();
  const { toast } = useToast();

  useEffect(() => {
    loadProjects();
  }, []);

  async function loadProjects() {
    setError('');
    setLoading(true);
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
    if (creating || !name.trim()) return;
    setCreating(true);
    setCreateError('');
    try {
      await createProject(name.trim(), description.trim());
      setName('');
      setDescription('');
      setShowCreate(false);
      loadProjects();
      toast({ title: 'Project created', description: `"${name}" has been created.` });
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'Failed to create project');
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(id: string) {
    try {
      await deleteProject(id);
      loadProjects();
      toast({ title: 'Project deleted', description: 'Removed.', variant: 'destructive' });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete project');
    }
  }

  // Task A.3: Import .env. Opens a dialog to pick target project + environment,
  // then a file picker. Parses key=value lines and POSTs to /env-import.
  function openImport() {
    if (projects.length === 0) {
      toast({
        title: 'No projects yet',
        description: 'Create a project before importing secrets.',
        variant: 'destructive',
      });
      return;
    }
    setImportProjectId(projects[0]?.id ?? '');
    setImportEnvironment('alpha');
    setImportOverwrite(false);
    setImportOpen(true);
  }

  async function handleImportFile(file: File) {
    if (!importProjectId) return;
    setImporting(true);
    try {
      const content = await file.text();
      const meaningfulLines = content
        .split(/\r?\n/)
        .map((l) => l.trim())
        .filter((l) => l.length > 0 && !l.startsWith('#'));
      if (meaningfulLines.length === 0) {
        throw new Error('The selected file has no key=value entries.');
      }
      if (!meaningfulLines.some((l) => l.includes('='))) {
        throw new Error('No KEY=VALUE pairs found. Make sure the file is a valid .env.');
      }
      const result = await importEnv(importProjectId, importEnvironment, content, importOverwrite);
      const projectName = projects.find((p) => p.id === importProjectId)?.name ?? 'project';
      toast({
        title: 'Import complete',
        description:
          `Saved ${(result.created?.length ?? 0) + (result.updated?.length ?? 0)} entries in ${projectName} / ${importEnvironment.toUpperCase()}. Skipped ${result.skipped?.length ?? 0}.`,
      });
      setImportOpen(false);
      loadProjects();
    } catch (err) {
      toast({
        title: 'Import failed',
        description: err instanceof Error ? err.message : 'Could not import the .env file.',
        variant: 'destructive',
      });
    } finally {
      setImporting(false);
      if (importFileRef.current) importFileRef.current.value = '';
    }
  }

  const filtered = projects.filter((p) => `${p.name} ${p.description}`.toLowerCase().includes(filter.toLowerCase()));

  return (
    <Page>
      <PageHeader
        eyebrow="Workspace / Vault"
        title={<>Your <em>projects.</em></>}
        sub="A separate vault for each project. Add secrets, connect your tools, and review changes between environments."
        actions={
          <>
            <button className="cz-btn" onClick={openImport} disabled={importing}>
              {importing ? 'Importing…' : 'Import .env'}
            </button>
            <button className="cz-btn cz-btn-primary" onClick={() => setShowCreate(true)}>
              + New project
            </button>
          </>
        }
      />

      <div className="ks-workspace-summary">
        <span><strong>{loading || error ? '—' : projects.length}</strong> {projects.length === 1 ? 'project' : 'projects'} in your workspace</span>
        <span>3 environments per project</span>
        <span>Encrypted at rest · AES-256-GCM</span>
      </div>

      <div className="cz-filter-bar" style={{ marginTop: 28, gap: 16, flexWrap: 'wrap' }}>
        <span className="cz-eyebrow">Project directory</span>
        <input className="cz-input" aria-label="Filter projects" placeholder="Search projects…" value={filter} onChange={(e) => setFilter(e.target.value)} style={{ width: 'min(320px, 100%)' }} />
      </div>

      {error && <div role="alert" className="cz-login-error" style={{ marginBottom: 16 }}>{error} <button className="cz-btn" onClick={loadProjects}>Retry</button></div>}

      {loading ? (
        <div className="cz-faint" style={{ padding: '24px 0', fontSize: 11, letterSpacing: '0.14em', textTransform: 'uppercase' }}>
          Loading projects…
        </div>
      ) : error ? null : filtered.length === 0 ? (
        <div className="cz-card cz-empty-state">
          <EventHorizon size={150} />
          <div className="cz-eyebrow">{filter ? "No matching projects" : "Empty shelf"}</div>
          <p className="cz-mute" style={{ marginTop: -10 }}>
            {filter ? "Try another project name or clear your search." : "Create your first project, then add a secret or import your .env file."}
          </p>
          <button className="cz-btn cz-btn-primary" onClick={() => setShowCreate(true)}>
            + New project
          </button>
        </div>
      ) : (
        <div className="cz-card cz-secrets">
          <table className="cz-dtable">
            <thead>
              <tr>
                <th style={{ width: 40 }}>№</th>
                <th>Project</th>
                <th>ID</th>
                <th>Environments</th>
                <th style={{ textAlign: 'right' }}>Created</th>
                <th style={{ textAlign: 'right' }}>Updated</th>
                <th style={{ width: 50 }} />
              </tr>
            </thead>
            <tbody>
              {filtered.map((p, i) => {
                return (
                  <tr key={p.id} onClick={() => navigate(`/projects/${p.id}`)} style={{ cursor: 'pointer' }}>
                    <td className="cz-faint" style={{ fontFamily: 'var(--cz-mono)', fontSize: 12 }}>
                      {String(i + 1).padStart(2, '0')}
                    </td>
                    <td>
                      <div className="cz-cell-name">
                        <Link
                          to={`/projects/${p.id}`}
                          className="cz-n"
                          style={{ textDecoration: 'none', color: 'inherit', width: 'fit-content' }}
                          onClick={(e) => e.stopPropagation()}
                        >
                          {p.name}
                        </Link>
                        <span className="cz-d">{p.description || '—'}</span>
                      </div>
                    </td>
                    <td className="cz-faint" style={{ fontFamily: 'var(--cz-mono)', fontSize: 11 }}>
                      prj_{p.id.slice(0, 6)}
                    </td>
                    <td>
                      <div className="cz-envs">
                        <Chip>alpha</Chip>
                        <Chip>uat</Chip>
                        <Chip variant="prod">prod</Chip>
                      </div>
                    </td>
                    <td className="cz-faint" style={{ textAlign: 'right', fontFamily: 'var(--cz-mono)', fontSize: 12 }}>
                      {new Date(p.created_at).toLocaleDateString()}
                    </td>
                    <td className="cz-faint" style={{ textAlign: 'right', fontFamily: 'var(--cz-mono)', fontSize: 12 }}>
                      {new Date(p.updated_at).toLocaleDateString()}
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <button
                        type="button"
                        className="cz-faint"
                        style={{ cursor: 'pointer', fontSize: 16, lineHeight: 1, background: 'transparent', border: 0, padding: '2px 6px', borderRadius: 6 }}
                        title="Delete project"
                        aria-label={`Delete ${p.name}`}
                        onClick={(e) => {
                          e.stopPropagation();
                          setDeleteTarget(p);
                        }}
                      >
                        ⋯
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Create dialog */}
      <Dialog open={showCreate} onOpenChange={(open) => !creating && setShowCreate(open)}>
          <DialogContent className="cz-card" style={{ width: 'min(520px, 92vw)', padding: 28 }}>
            <DialogTitle>New project</DialogTitle>
            <DialogDescription>Create a vault with Alpha, UAT, and Production environments.</DialogDescription>
            <h2 style={{ fontFamily: 'var(--cz-sans)', fontWeight: 300, fontSize: 30, letterSpacing: '-0.02em', margin: '8px 0 18px', color: 'var(--cz-ink)' }}>
              Open a <em style={{ fontStyle: 'normal', color: 'var(--cz-accent-hi)' }}>shelf.</em>
            </h2>
            <form onSubmit={handleCreate} className="cz-login-form">
              {createError && <div className="cz-login-error" role="alert">{createError}</div>}
              <div className="cz-login-field">
                <label htmlFor="project-name">Project name</label>
                <input
                  id="project-name"
                  className="cz-input"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                  autoFocus
                  placeholder="e.g. nexus-platform"
                />
              </div>
              <div className="cz-login-field">
                <label htmlFor="project-desc">Description</label>
                <input
                  id="project-desc"
                  className="cz-input"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="optional"
                />
              </div>
              <div style={{ display: 'flex', gap: 8, marginTop: 6 }}>
                <button type="button" className="cz-btn" style={{ flex: 1, justifyContent: 'center' }} onClick={() => setShowCreate(false)}>
                  Cancel
                </button>
                <button type="submit" disabled={creating || !name.trim()} className="cz-btn cz-btn-primary" style={{ flex: 1, justifyContent: 'center' }}>
                  {creating ? "Creating…" : "Create →"}
                </button>
              </div>
            </form>
          </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete Project"
        description={deleteTarget ? `Delete "${deleteTarget.name}"? This action cannot be undone.` : ''}
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleteTarget) handleDelete(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />

      {importOpen && (
        <div className="cz-cmdk-back" onClick={() => !importing && setImportOpen(false)}>
          <div className="cz-card" style={{ width: 'min(520px, 92vw)', padding: 28 }} onClick={(e) => e.stopPropagation()}>
            <div className="cz-eyebrow">Import .env</div>
            <h2 style={{ fontFamily: 'var(--cz-sans)', fontWeight: 300, fontSize: 28, letterSpacing: '-0.02em', margin: '8px 0 18px', color: 'var(--cz-ink)' }}>
              Restore <em style={{ fontStyle: 'normal', color: 'var(--cz-accent-hi)' }}>secrets.</em>
            </h2>
            <div className="cz-login-field" style={{ marginBottom: 14 }}>
              <label htmlFor="import-project">Project</label>
              <select id="import-project" className="cz-input" value={importProjectId} onChange={(e) => setImportProjectId(e.target.value)} disabled={importing}>
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            </div>
            <div className="cz-login-field" style={{ marginBottom: 14 }}>
              <label htmlFor="import-env">Environment</label>
              <select
                id="import-env"
                className="cz-input"
                value={importEnvironment}
                onChange={(e) => setImportEnvironment(e.target.value as ImportEnv)}
                disabled={importing}
              >
                {IMPORT_ENVIRONMENTS.map((e) => (
                  <option key={e} value={e}>{e.toUpperCase()}</option>
                ))}
              </select>
            </div>
            <label className="cz-login-check" style={{ marginBottom: 4 }}>
              <input type="checkbox" checked={importOverwrite} onChange={(e) => setImportOverwrite(e.target.checked)} disabled={importing} />
              Overwrite existing keys
            </label>
            <input
              ref={importFileRef}
              type="file"
              accept=".env,text/plain,application/octet-stream"
              style={{ display: 'none' }}
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) handleImportFile(file);
              }}
            />
            <div style={{ display: 'flex', gap: 8, marginTop: 18 }}>
              <button type="button" className="cz-btn" style={{ flex: 1, justifyContent: 'center' }} onClick={() => setImportOpen(false)} disabled={importing}>
                Cancel
              </button>
              <button
                type="button"
                className="cz-btn cz-btn-primary"
                style={{ flex: 1, justifyContent: 'center' }}
                onClick={() => importFileRef.current?.click()}
                disabled={importing || !importProjectId}
              >
                {importing ? 'Importing…' : 'Select file →'}
              </button>
            </div>
          </div>
        </div>
      )}
    </Page>
  );
}
