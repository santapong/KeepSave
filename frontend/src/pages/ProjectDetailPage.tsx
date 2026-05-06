import { useState, useEffect } from 'react';
import { useParams, Link, Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import { getProject } from '../api/client';
import { SecretsPanel } from '../components/SecretsPanel';
import { PromotionWizard } from '../components/PromotionWizard';
import { PromotionsList } from '../components/PromotionsList';
import { AuditLogViewer } from '../components/AuditLogViewer';
import { ProjectAPIKeysPanel } from '../components/ProjectAPIKeysPanel';
import type { Project } from '../types';

type Tab = 'secrets' | 'promote' | 'promotions' | 'audit' | 'api-keys';

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

export function ProjectDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [project, setProject] = useState<Project | null>(null);
  const [error, setError] = useState('');
  const navigate = useNavigate();
  const location = useLocation();

  const currentTab: Tab = (() => {
    const path = location.pathname;
    if (path.includes('/promote')) return 'promote';
    if (path.includes('/promotions')) return 'promotions';
    if (path.includes('/audit')) return 'audit';
    if (path.includes('/api-keys')) return 'api-keys';
    return 'secrets';
  })();

  useEffect(() => {
    if (!id) return;
    getProject(id)
      .then(setProject)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load project'));
  }, [id]);

  if (error) {
    return (
      <div className="ks-page">
        <div className="ks-error">{error}</div>
      </div>
    );
  }

  if (!project) {
    return (
      <div className="ks-page">
        <div className="ks-faint" style={{ padding: '24px 0', fontSize: 11, letterSpacing: '0.14em', textTransform: 'uppercase' }}>
          Loading project…
        </div>
      </div>
    );
  }

  const tabs: { key: Tab; label: string; path: string; count?: string }[] = [
    { key: 'secrets', label: 'Secrets', path: `/projects/${id}` },
    { key: 'promote', label: 'Pipeline', path: `/projects/${id}/promote` },
    { key: 'promotions', label: 'History', path: `/projects/${id}/promotions` },
    { key: 'audit', label: 'Audit', path: `/projects/${id}/audit` },
    { key: 'api-keys', label: 'API Keys', path: `/projects/${id}/api-keys` },
  ];

  const nameParts = project.name.split('-');
  const head = nameParts[0];
  const tail = nameParts.slice(1).join('-') || 'vault';

  return (
    <div className="ks-page">
      <Link
        to="/"
        className="ks-btn ks-btn-ghost"
        style={{ marginBottom: 18, display: 'inline-flex', textDecoration: 'none' }}
      >
        ← All projects
      </Link>

      <div className="ks-page-head">
        <div>
          <div className="ks-eyebrow ks-amber">Project № prj_{project.id.slice(0, 6)}</div>
          <h1 className="ks-page-title">
            {head} <em>/ {tail}</em>
          </h1>
          {project.description && (
            <p className="ks-page-sub">{project.description}</p>
          )}
        </div>
        <div style={{ display: 'flex', gap: 10 }}>
          <button className="ks-btn">Export .env</button>
          <button className="ks-btn">Rotate all</button>
          <button
            className="ks-btn ks-btn-primary"
            onClick={() => navigate(`/projects/${id}/promote`)}
          >
            + Promote
          </button>
        </div>
      </div>

      <div className="ks-kpi-strip">
        <Kpi label="SECRETS" value="—" hint="this project" bars={[2, 3, 2, 4, 3, 5, 4, 5]} />
        <Kpi label="ENVIRONMENTS" value="03" hint="alpha · uat · prod" bars={[1, 2, 2, 3, 3, 3, 3, 3]} />
        <Kpi label="READS / HR" value="1,204" hint="+4% h/h" trend="up" bars={[6, 7, 8, 9, 8, 10, 11, 12]} />
        <Kpi label="DRIFT" value="Δ0" hint="envs aligned" bars={[0, 1, 0, 0, 0, 0, 1, 0]} />
      </div>

      {/* Tabs */}
      <div className="ks-pd-tabs">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            onClick={() => navigate(tab.path)}
            className={`ks-pd-tab ${currentTab === tab.key ? 'on' : ''}`}
          >
            {tab.label}
            {tab.count && <span className="ks-faint" style={{ fontSize: 10, marginLeft: 6 }}>{tab.count}</span>}
          </button>
        ))}
      </div>

      <div style={{ marginTop: 20 }}>
        <Routes>
          <Route index element={<SecretsPanel projectId={id!} />} />
          <Route path="promote" element={<PromotionWizard projectId={id!} />} />
          <Route path="promotions" element={<PromotionsList projectId={id!} />} />
          <Route path="audit" element={<AuditLogViewer projectId={id!} />} />
          <Route path="api-keys" element={<ProjectAPIKeysPanel projectId={id!} />} />
        </Routes>
      </div>
    </div>
  );
}
