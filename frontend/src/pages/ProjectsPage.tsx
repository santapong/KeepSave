import { useState, useEffect, useRef, type FormEvent } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { listProjects, createProject, deleteProject, importEnv } from '../api/client';
import type { Project } from '../types';
import { useToast } from '@/hooks/useToast';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Page, PageHeader, KpiStrip, Kpi, Chip, Pill, Dot } from '../components/cosmic/primitives';
import { EventHorizon } from '../components/cosmic/EventHorizon';

const IMPORT_ENVIRONMENTS = ['alpha', 'uat', 'prod'] as const;
type ImportEnv = (typeof IMPORT_ENVIRONMENTS)[number];

type Health = 'go' | 'warn' | 'stop';

function projectHealth(p: Project): Health {
  // Deterministic pseudo-status from id char sum
  const sum = p.id.split('').reduce((a, c) => a + c.charCodeAt(0), 0);
  const m = sum % 7;
  if (m === 0) return 'stop';
  if (m === 1 || m === 2) return 'warn';
  return 'go';
}

function relTime(iso: string): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return '—';
  const diff = Math.max(0, Date.now() - t);
  const m = Math.floor(diff / 60000);
  if (m < 1) return 'just now';
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h`;
  const d = Math.floor(h / 24);
  return `${d}d`;
}

const SECRETS_BARS = [12, 18, 14, 22, 20, 26, 24, 28];
const READS_BARS = [8, 10, 12, 11, 14, 16, 15, 18];
const KPI_BARS = [3, 5, 4, 6, 5, 7, 6, 8];

const TICKER_EVENTS = [
  { who: 'agent:nexus-runtime', what: 'read', target: 'ANTHROPIC_API_KEY', env: 'prod', ago: '12s' },
  { who: 'santapong', what: 'promote', target: 'alpha → uat', env: 'medqcnn', ago: '1m' },
  { who: 'agent:ci-builder', what: 'read', target: 'STRIPE_SECRET', env: 'prod', ago: '2m' },
  { who: 'ops@acme', what: 'rotate', target: 'JWT_SECRET_KEY', env: 'prod', ago: '4m' },
  { who: 'agent:nexus-runtime', what: 'read', target: 'DATABASE_URL', env: 'prod', ago: '6m' },
  { who: 'santapong', what: 'create', target: 'OIDC_CLIENT_SECRET', env: 'prod', ago: '12m' },
  { who: 'security-bot', what: 'alert', target: 'rate limit spike', env: 'prod', ago: '18m' },
];

function Ticker() {
  const stream = [...TICKER_EVENTS, ...TICKER_EVENTS, ...TICKER_EVENTS];
  return (
    <div className="cz-ticker">
      <span className="cz-ticker-label">
        <Dot status="go" /> Live ledger
      </span>
      <div className="cz-ticker-mask">
        <div className="cz-ticker-stream">
          {stream.map((e, i) => (
            <span key={i}>
              <span className="cz-faint">[{e.ago}]</span> <b>{e.who}</b> <em>{e.what}</em> {e.target}{' '}
              <span className="cz-faint">· {e.env}</span>
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

export function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<Project | null>(null);
  const [filter, setFilter] = useState<'ALL' | 'SEALED' | 'DRIFT' | 'ATTN'>('ALL');
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
      toast({ title: 'Project created', description: `"${name}" has been created.` });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project');
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
          `Imported ${meaningfulLines.filter((l) => l.includes('=')).length} entries into ${projectName} / ${importEnvironment.toUpperCase()}.` +
          (result ? ` (${JSON.stringify(result)})` : ''),
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

  const filtered = projects.filter((p) => {
    if (filter === 'ALL') return true;
    const h = projectHealth(p);
    if (filter === 'SEALED') return h === 'go';
    if (filter === 'DRIFT') return h === 'warn';
    if (filter === 'ATTN') return h === 'stop';
    return true;
  });

  return (
    <Page>
      <PageHeader
        eyebrow="Vault · shelf"
        title={<>Projects on <em>the shelf.</em></>}
        sub="Every project has its own encryption key. Every environment has its own revision. Promote changes explicitly; nothing leaks sideways."
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

      <KpiStrip>
        <Kpi label="Projects" value={String(projects.length).padStart(2, '0')} hint="+1 this week" trend="up" bars={KPI_BARS} />
        <Kpi label="Secrets" value="251" hint="across 3 environments" bars={SECRETS_BARS} />
        <Kpi label="Reads / day" value="18,420" hint="+12.4% week over week" trend="up" bars={READS_BARS} flux />
        <Kpi label="Anomalies" value="00" hint="last 24h · healthy" trend="up" />
      </KpiStrip>

      <Ticker />

      <div style={{ height: 28 }} />

      <div className="cz-filter-bar">
        <div className="cz-seg">
          {(['ALL', 'SEALED', 'DRIFT', 'ATTN'] as const).map((f) => (
            <button key={f} className={filter === f ? 'cz-on' : ''} onClick={() => setFilter(f)}>
              {f}
            </button>
          ))}
        </div>
      </div>

      {error && <div className="cz-login-error" style={{ marginBottom: 16 }}>{error}</div>}

      {loading ? (
        <div className="cz-faint" style={{ padding: '24px 0', fontSize: 11, letterSpacing: '0.14em', textTransform: 'uppercase' }}>
          Loading projects…
        </div>
      ) : filtered.length === 0 ? (
        <div className="cz-card cz-empty-state">
          <EventHorizon size={150} />
          <div className="cz-eyebrow">Empty shelf</div>
          <p className="cz-mute" style={{ marginTop: -10 }}>
            Nothing on the shelf. Create your first project to begin.
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
                <th>Status</th>
                <th style={{ textAlign: 'right' }}>Updated</th>
                <th style={{ width: 50 }} />
              </tr>
            </thead>
            <tbody>
              {filtered.map((p, i) => {
                const h = projectHealth(p);
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
                    <td>
                      <Pill variant={h === 'go' ? 'go' : h === 'stop' ? 'stop' : undefined}>
                        <Dot status={h} />
                        {h === 'go' ? 'Sealed' : h === 'warn' ? 'Drift' : 'Attention'}
                      </Pill>
                    </td>
                    <td className="cz-faint" style={{ textAlign: 'right', fontFamily: 'var(--cz-mono)', fontSize: 12 }}>
                      {relTime(p.updated_at)} ago
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
      {showCreate && (
        <div className="cz-cmdk-back" onClick={() => setShowCreate(false)}>
          <div className="cz-card" style={{ width: 'min(520px, 92vw)', padding: 28 }} onClick={(e) => e.stopPropagation()}>
            <div className="cz-eyebrow">New project</div>
            <h2 style={{ fontFamily: 'var(--cz-sans)', fontWeight: 300, fontSize: 30, letterSpacing: '-0.02em', margin: '8px 0 18px', color: 'var(--cz-ink)' }}>
              Open a <em style={{ fontStyle: 'normal', color: 'var(--cz-accent-hi)' }}>shelf.</em>
            </h2>
            <form onSubmit={handleCreate} className="cz-login-form">
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
                <button type="submit" className="cz-btn cz-btn-primary" style={{ flex: 1, justifyContent: 'center' }}>
                  Create →
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

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
