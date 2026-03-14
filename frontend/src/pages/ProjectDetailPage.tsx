import { useState, useEffect } from 'react';
import { useParams, Link, Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import { getProject } from '../api/client';
import { SecretsPanel } from '../components/SecretsPanel';
import { PromotionWizard } from '../components/PromotionWizard';
import { PromotionsList } from '../components/PromotionsList';
import { AuditLogViewer } from '../components/AuditLogViewer';
import type { Project } from '../types';

type Tab = 'secrets' | 'promote' | 'promotions' | 'audit';

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
    return 'secrets';
  })();

  useEffect(() => {
    if (!id) return;
    getProject(id)
      .then(setProject)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load project'));
  }, [id]);

  if (error) return <div style={errorStyle}>{error}</div>;
  if (!project) return <p>Loading...</p>;

  const tabs: { key: Tab; label: string; path: string }[] = [
    { key: 'secrets', label: 'Secrets', path: `/projects/${id}` },
    { key: 'promote', label: 'Promote', path: `/projects/${id}/promote` },
    { key: 'promotions', label: 'History', path: `/projects/${id}/promotions` },
    { key: 'audit', label: 'Audit Log', path: `/projects/${id}/audit` },
  ];

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <Link to="/" style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
          &larr; Projects
        </Link>
        <h1 style={{ fontSize: 24, fontWeight: 700, marginTop: 8 }}>{project.name}</h1>
        {project.description && (
          <p style={{ fontSize: 14, color: 'var(--color-text-secondary)' }}>{project.description}</p>
        )}
      </div>

      <div style={tabBar}>
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => navigate(tab.path)}
            style={{
              ...tabBtn,
              borderBottomColor: currentTab === tab.key ? 'var(--color-primary)' : 'transparent',
              color: currentTab === tab.key ? 'var(--color-primary)' : 'var(--color-text-secondary)',
              fontWeight: currentTab === tab.key ? 600 : 400,
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <Routes>
        <Route index element={<SecretsPanel projectId={id!} />} />
        <Route path="promote" element={<PromotionWizard projectId={id!} />} />
        <Route path="promotions" element={<PromotionsList projectId={id!} />} />
        <Route path="audit" element={<AuditLogViewer projectId={id!} />} />
      </Routes>
    </div>
  );
}

const tabBar: React.CSSProperties = {
  display: 'flex',
  gap: 0,
  borderBottom: '1px solid var(--color-border)',
  marginBottom: 24,
};

const tabBtn: React.CSSProperties = {
  background: 'transparent',
  border: 'none',
  borderBottom: '2px solid transparent',
  padding: '8px 16px',
  fontSize: 14,
  cursor: 'pointer',
};

const errorStyle: React.CSSProperties = {
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
};
