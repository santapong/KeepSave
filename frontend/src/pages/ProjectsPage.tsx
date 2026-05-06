import { useState, useEffect, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { listProjects, createProject, deleteProject } from '../api/client';
import type { Project } from '../types';
import { useToast } from '@/hooks/useToast';
import { ConfirmDialog } from '@/components/ConfirmDialog';

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

const KPI_BARS = [3, 5, 4, 6, 5, 7, 6, 8];
const SECRETS_BARS = [12, 18, 14, 22, 20, 26, 24, 28];
const READS_BARS = [8, 10, 12, 11, 14, 16, 15, 18];
const ANOM_BARS = [0, 1, 0, 0, 2, 0, 0, 0];

const TICKER_EVENTS = [
  { who: 'agent:nexus-runtime', what: 'read', target: 'ANTHROPIC_API_KEY', env: 'prod', ago: '12s' },
  { who: 'santapong', what: 'promote', target: 'alpha → uat', env: 'medqcnn', ago: '1m' },
  { who: 'agent:ci-builder', what: 'read', target: 'STRIPE_SECRET', env: 'prod', ago: '2m' },
  { who: 'ops@acme', what: 'rotate', target: 'JWT_SECRET_KEY', env: 'prod', ago: '4m' },
  { who: 'agent:nexus-runtime', what: 'read', target: 'DATABASE_URL', env: 'prod', ago: '6m' },
  { who: 'santapong', what: 'create', target: 'OIDC_CLIENT_SECRET', env: 'prod', ago: '12m' },
  { who: 'security-bot', what: 'alert', target: 'rate limit spike', env: 'prod', ago: '18m' },
];

function Kpi({
  label,
  value,
  hint,
  trend,
  bars,
}: {
  label: string;
  value: string;
  hint?: string;
  trend?: 'up' | 'down';
  bars: number[];
}) {
  const max = Math.max(...bars, 1);
  return (
    <div className="ks-kpi">
      <div className="ks-eyebrow">{label}</div>
      <div className="ks-v ks-num">
        {value.length > 3 ? value : <em>{value}</em>}
      </div>
      {hint && <div className={`ks-delta ${trend ?? ''}`}>{hint}</div>}
      <div className="ks-sparkbars">
        {bars.map((b, i) => (
          <div
            key={i}
            className={`ks-bar ${i === bars.length - 1 ? 'hi' : ''}`}
            style={{ height: `${4 + (b / max) * 20}px` }}
          />
        ))}
      </div>
    </div>
  );
}

function Ticker() {
  const stream = [...TICKER_EVENTS, ...TICKER_EVENTS, ...TICKER_EVENTS];
  return (
    <div className="ks-ticker">
      <span className="ks-ticker-label">LEDGER · LIVE</span>
      <div style={{ overflow: 'hidden', flex: 1 }}>
        <div className="ks-ticker-stream">
          {stream.map((e, i) => (
            <span key={i}>
              <span className="ks-faint">[{e.ago}]</span>
              <b>{e.who}</b>
              <em>{e.what}</em>
              <span>{e.target}</span>
              <span className="ks-faint">· {e.env}</span>
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

  const filtered = projects.filter((p) => {
    if (filter === 'ALL') return true;
    const h = projectHealth(p);
    if (filter === 'SEALED') return h === 'go';
    if (filter === 'DRIFT') return h === 'warn';
    if (filter === 'ATTN') return h === 'stop';
    return true;
  });

  return (
    <div className="ks-page">
      <div className="ks-page-head">
        <div>
          <div className="ks-eyebrow ks-amber">Section I · Shelf</div>
          <h1 className="ks-page-title">Projects on <em>the shelf.</em></h1>
          <p className="ks-page-sub">
            Every project has its own encryption key. Every environment has its own revision. Promote changes explicitly; nothing leaks sideways.
          </p>
        </div>
        <div style={{ display: 'flex', gap: 10 }}>
          <button className="ks-btn">Import .env</button>
          <button className="ks-btn ks-btn-primary" onClick={() => setShowCreate(true)}>
            + New project
          </button>
        </div>
      </div>

      <div className="ks-kpi-strip">
        <Kpi label="PROJECTS" value={String(projects.length).padStart(2, '0')} hint="+1 this week" trend="up" bars={KPI_BARS} />
        <Kpi label="SECRETS" value="251" hint="across 3 envs" bars={SECRETS_BARS} />
        <Kpi label="READS / DAY" value="18,420" hint="+12.4% w/w" trend="up" bars={READS_BARS} />
        <Kpi label="ANOMALIES" value="00" hint="last 24h · healthy" trend="up" bars={ANOM_BARS} />
      </div>

      <Ticker />

      <div style={{ height: 28 }} />

      {/* Filter bar */}
      <div className="ks-filter-bar">
        <div className="ks-filter-group">
          {(['ALL', 'SEALED', 'DRIFT', 'ATTN'] as const).map((f) => (
            <button
              key={f}
              className={`ks-filter-btn ${filter === f ? 'on' : ''}`}
              onClick={() => setFilter(f)}
            >
              {f}
            </button>
          ))}
        </div>
        <div style={{ flex: 1 }} />
        <div className="ks-filter-group">
          <button className="ks-filter-btn">SORT · ACTIVITY ▾</button>
          <button className="ks-filter-btn on">TABLE</button>
        </div>
      </div>

      {error && (
        <div className="ks-error" style={{ marginBottom: 16 }}>{error}</div>
      )}

      {loading ? (
        <div className="ks-faint" style={{ padding: '24px 0', fontSize: 11, letterSpacing: '0.14em', textTransform: 'uppercase' }}>
          Loading projects…
        </div>
      ) : filtered.length === 0 ? (
        <div
          className="ks-secrets"
          style={{ padding: 60, textAlign: 'center', borderStyle: 'dashed' }}
        >
          <div className="ks-eyebrow ks-amber" style={{ marginBottom: 14 }}>Empty shelf</div>
          <p className="ks-dim" style={{ fontSize: 13, marginBottom: 18 }}>
            Nothing on the shelf. Create your first project to begin.
          </p>
          <button className="ks-btn ks-btn-primary" onClick={() => setShowCreate(true)}>
            + New project
          </button>
        </div>
      ) : (
        <table className="ks-proj-table">
          <thead>
            <tr>
              <th style={{ width: 40 }}>№</th>
              <th>Project</th>
              <th>ID</th>
              <th>Environments</th>
              <th style={{ textAlign: 'right' }}>Created</th>
              <th>Status</th>
              <th style={{ textAlign: 'right' }}>Updated</th>
              <th style={{ width: 60 }} />
            </tr>
          </thead>
          <tbody>
            {filtered.map((p, i) => {
              const h = projectHealth(p);
              const statusBorder =
                h === 'go'
                  ? 'oklch(0.45 0.10 150)'
                  : h === 'warn'
                  ? 'oklch(0.55 0.10 85)'
                  : 'oklch(0.42 0.13 27)';
              return (
                <tr key={p.id} onClick={() => navigate(`/projects/${p.id}`)}>
                  <td className="ks-faint ks-num">{String(i + 1).padStart(2, '0')}</td>
                  <td>
                    <div className="ks-proj-name">
                      <span className="ks-n">{p.name}</span>
                      <span className="ks-d">{p.description || '—'}</span>
                    </div>
                  </td>
                  <td className="ks-faint ks-num" style={{ fontSize: 10 }}>
                    prj_{p.id.slice(0, 6)}
                  </td>
                  <td>
                    <div className="ks-env-row">
                      <span className="ks-env-chip">alpha</span>
                      <span className="ks-env-chip">uat</span>
                      <span className="ks-env-chip prod">prod</span>
                    </div>
                  </td>
                  <td className="ks-faint ks-num" style={{ textAlign: 'right' }}>
                    {new Date(p.created_at).toLocaleDateString()}
                  </td>
                  <td>
                    <span
                      className="ks-pill"
                      style={{ borderColor: statusBorder }}
                    >
                      <span className={`ks-dot ks-dot-${h}`} />
                      {h === 'go' ? 'SEALED' : h === 'warn' ? 'DRIFT' : 'ATTN'}
                    </span>
                  </td>
                  <td className="ks-faint ks-num" style={{ textAlign: 'right' }}>
                    {relTime(p.updated_at)} ago
                  </td>
                  <td
                    style={{ textAlign: 'right' }}
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleteTarget(p);
                    }}
                  >
                    <span
                      className="ks-faint"
                      style={{ cursor: 'pointer', fontSize: 16 }}
                      title="Delete"
                    >
                      ⋯
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      {/* Create dialog (simple inline modal) */}
      {showCreate && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'oklch(0.12 0.008 60 / 0.75)',
            backdropFilter: 'blur(6px)',
            zIndex: 300,
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'flex-start',
            paddingTop: '14vh',
          }}
          onClick={() => setShowCreate(false)}
        >
          <div
            style={{
              width: 'min(520px, 92vw)',
              background: 'var(--ks-bg)',
              border: '1px solid var(--ks-amber-dim)',
              boxShadow: '0 40px 80px oklch(0 0 0 / 0.6)',
              padding: 28,
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="ks-eyebrow ks-amber">— New project —</div>
            <h2
              className="ks-serif-display"
              style={{ fontSize: 32, marginTop: 8, marginBottom: 18 }}
            >
              Open a <em style={{ fontStyle: 'italic', color: 'var(--ks-amber)' }}>shelf</em>.
            </h2>
            <form onSubmit={handleCreate} className="ks-login-form">
              <div className="ks-tweak-row">
                <label>Project name</label>
                <input
                  className="ks-input"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                  autoFocus
                  placeholder="e.g. nexus-platform"
                />
              </div>
              <div className="ks-tweak-row">
                <label>Description</label>
                <input
                  className="ks-input"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="optional"
                />
              </div>
              <div style={{ display: 'flex', gap: 8, marginTop: 6 }}>
                <button
                  type="button"
                  className="ks-btn"
                  style={{ flex: 1, justifyContent: 'center' }}
                  onClick={() => setShowCreate(false)}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="ks-btn ks-btn-primary"
                  style={{ flex: 1, justifyContent: 'center' }}
                >
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
        description={
          deleteTarget
            ? `Delete "${deleteTarget.name}"? This action cannot be undone.`
            : ''
        }
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleteTarget) handleDelete(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}
