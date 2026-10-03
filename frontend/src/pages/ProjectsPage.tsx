import { useState, useEffect, useRef, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { listProjects, createProject, deleteProject, importEnv } from '../api/client';
import type { Project } from '../types';
import { useToast } from '@/hooks/useToast';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Page, PageHeader, Chip } from '../components/cosmic/primitives';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../components/ui/dialog';
import { ArrowRight, ArrowUpRight, FolderClosed, LayoutGrid, List, Plus, Search, Upload, KeyRound, Cable, GitPullRequest, Trash2, ShieldCheck, X } from '@/components/icons';

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
  const [view, setView] = useState<'grid' | 'list'>('grid');
  const [sort, setSort] = useState('updated');
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

  const filtered = projects.filter((p) => `${p.name} ${p.description}`.toLowerCase().includes(filter.toLowerCase())).sort((a, b) => {
    if (sort === 'name') return a.name.localeCompare(b.name);
    const field = sort === 'created' ? 'created_at' : 'updated_at';
    return new Date(b[field]).getTime() - new Date(a[field]).getTime();
  });
  const formatUpdated = (date: string) => new Date(date).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });

  return (
    <Page>
      <PageHeader
        eyebrow="YOUR WORKSPACE"
        title="Projects"
        sub="Your applications. Their secrets. One place to keep them together."
        actions={<>
          <button className="cz-btn" onClick={openImport} disabled={importing || loading || projects.length === 0}><Upload size={15} />{importing ? 'Importing…' : 'Import .env'}</button>
          <button className="cz-btn cz-btn-primary" onClick={() => setShowCreate(true)}><Plus size={16} /> New project</button>
        </>}
      />

      <section className="ks-workspace-welcome" aria-label="KeepSave workflow">
        <div className="ks-welcome-copy"><span className="ks-section-kicker">A LITTLE ORDER. A LOT LESS WORRY.</span><h2>Keep your secrets<br /><em>in their own orbit.</em></h2><p>Separate every environment. Give each tool the access it needs. Review changes before they go live.</p><Link to="/help">Explore the guide <ArrowUpRight size={14} /></Link></div>
        <div className="ks-welcome-art" aria-hidden="true"><img src="/images/event-horizon.webp" alt="" /></div>
        <div className="ks-workflow-steps">
          <div><span><KeyRound size={17} /></span><section><h3>01 <b>Store</b></h3><p>A separate vault for every project.</p></section></div>
          <div><span><Cable size={17} /></span><section><h3>02 <b>Connect</b></h3><p>Scoped access for your apps and agents.</p></section></div>
          <div><span><GitPullRequest size={17} /></span><section><h3>03 <b>Promote</b></h3><p>Review changes from Alpha to Production.</p></section></div>
        </div>
      </section>

      <div className="ks-projects-heading"><h2>All projects <span>{loading || error ? '—' : projects.length}</span></h2><span className="ks-protection-label"><ShieldCheck size={14} /> Encrypted at rest</span></div>
      <div className="ks-project-toolbar">
        <label className="ks-project-search"><Search size={16} /><input aria-label="Filter projects" placeholder="Find a project…" value={filter} onChange={(event) => setFilter(event.target.value)} />{filter && <button type="button" onClick={() => setFilter('')} aria-label="Clear project search"><X size={14} /></button>}</label>
        <div className="ks-project-display"><select aria-label="Sort projects" value={sort} onChange={(event) => setSort(event.target.value)}><option value="updated">Recently updated</option><option value="created">Recently created</option><option value="name">Name A–Z</option></select><div className="ks-view-switch" aria-label="Project view"><button type="button" aria-label="Grid view" aria-pressed={view === 'grid'} onClick={() => setView('grid')}><LayoutGrid size={16} /></button><button type="button" aria-label="List view" aria-pressed={view === 'list'} onClick={() => setView('list')}><List size={17} /></button></div></div>
      </div>
      {error && <div role="alert" className="cz-login-error">{error} <button className="cz-btn" onClick={loadProjects}>Retry</button></div>}
      {loading ? <div className="ks-project-loading" role="status"><span /> Loading your projects…</div> : error ? null : filtered.length === 0 ?
        <div className="ks-project-empty"><span className="ks-empty-icon"><FolderClosed size={28} strokeWidth={1.4} /></span><h3>{filter ? 'No matching projects' : 'Your first project starts here'}</h3><p>{filter ? 'Try another name or clear your search.' : 'Create a project, then add a secret or bring your existing .env file.'}</p>{filter ? <button className="cz-btn" onClick={() => setFilter('')}>Clear search</button> : <button className="cz-btn cz-btn-primary" onClick={() => setShowCreate(true)}><Plus size={16} /> Create your first project</button>}</div> :
        <div className={`ks-project-grid ${view === 'list' ? 'ks-project-list' : ''}`}>
          {filtered.map((project) => <article className="ks-project-card" key={project.id}>
            <Link to={`/projects/${project.id}`} className="ks-project-card-main" aria-label={`Open ${project.name}`}>
              <div className="ks-project-card-top"><span className="ks-project-symbol"><FolderClosed size={21} strokeWidth={1.5} /></span><ArrowUpRight className="ks-project-open" size={18} /></div>
              <div className="ks-project-card-title"><h3>{project.name}</h3><p>{project.description || 'A dedicated vault for your environment secrets.'}</p></div>
              <div className="ks-project-environments"><Chip>Alpha</Chip><Chip>UAT</Chip><Chip variant="prod">Production</Chip></div>
            </Link>
            <footer><span>Updated {formatUpdated(project.updated_at)}</span><button type="button" className="ks-project-delete" aria-label={`Delete ${project.name}`} title="Delete project" onClick={() => setDeleteTarget(project)}><Trash2 size={14} /></button></footer>
          </article>)}
        </div>
      }
      <div className="ks-projects-footnote"><span>Alpha <ArrowRight size={12} /> UAT <ArrowRight size={12} /> Production</span><p>One project. Three environments. Clear boundaries.</p></div>

      {/* Create dialog */}
      <Dialog open={showCreate} onOpenChange={(open) => !creating && setShowCreate(open)}>
          <DialogContent className="cz-card" style={{ width: 'min(520px, 92vw)', padding: 28 }}>
            <DialogTitle>New project</DialogTitle>
            <DialogDescription>Create a vault with Alpha, UAT, and Production environments.</DialogDescription>
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
                  placeholder="e.g. storefront-api"
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
                  {creating ? 'Creating…' : 'Create project'}
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

      <Dialog open={importOpen} onOpenChange={(open) => !importing && setImportOpen(open)}>
          <DialogContent className="cz-card" style={{ width: 'min(520px, 92vw)', padding: 28 }}>
            <DialogTitle>Import .env</DialogTitle><DialogDescription>Choose a project and environment, then select your .env file.</DialogDescription>
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
                {importing ? 'Importing…' : <>Select file <ArrowRight size={14} /></>}
              </button>
            </div>
          </DialogContent>
      </Dialog>
    </Page>
  );
}
