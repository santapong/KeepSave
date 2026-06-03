import { useState, useEffect } from 'react';
import { useParams, Link, Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import { getProject, exportEnv, rotateProjectKeys } from '../api/client';
import { SecretsPanel } from '../components/SecretsPanel';
import { PromotionWizard } from '../components/PromotionWizard';
import { PromotionsList } from '../components/PromotionsList';
import { AuditLogViewer } from '../components/AuditLogViewer';
import { ProjectAPIKeysPanel } from '../components/ProjectAPIKeysPanel';
import { TypedConfirmModal } from '../components/TypedConfirmModal';
import { useToast } from '@/hooks/useToast';
import { OrbitalPipeline } from '../components/cosmic/OrbitalPipeline';
import { Page, PageHeader, KpiStrip, Kpi, SectionHead } from '../components/cosmic/primitives';
import type { Project } from '../types';

const ENVIRONMENTS = ['alpha', 'uat', 'prod'] as const;
type Env = (typeof ENVIRONMENTS)[number];

type Tab = 'secrets' | 'promote' | 'promotions' | 'audit' | 'api-keys';

export function ProjectDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [project, setProject] = useState<Project | null>(null);
  const [error, setError] = useState('');
  const [exporting, setExporting] = useState(false);
  const [exportPickerOpen, setExportPickerOpen] = useState(false);
  const [rotating, setRotating] = useState(false);
  const [rotateModalOpen, setRotateModalOpen] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const { toast } = useToast();

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

  // Task A.2: Export .env for the selected environment, triggers a download.
  async function handleExport(env: Env) {
    if (!id || !project) return;
    setExportPickerOpen(false);
    setExporting(true);
    try {
      const content = await exportEnv(id, env);
      const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${project.name}.${env}.env`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      toast({ title: 'Exported', description: `Downloaded ${env.toUpperCase()} secrets as .env file.` });
    } catch (err) {
      toast({
        title: 'Export failed',
        description: err instanceof Error ? err.message : 'Failed to export .env file.',
        variant: 'destructive',
      });
    } finally {
      setExporting(false);
    }
  }

  // Task A.1: Rotate all keys, gated by TypedConfirmModal (destructive).
  async function handleRotateAll() {
    if (!id) return;
    setRotating(true);
    try {
      await rotateProjectKeys(id);
      toast({ title: 'Keys rotated', description: 'All project keys have been rotated successfully.' });
    } catch (err) {
      toast({
        title: 'Rotation failed',
        description: err instanceof Error ? err.message : 'Failed to rotate project keys.',
        variant: 'destructive',
      });
    } finally {
      setRotating(false);
    }
  }

  if (error) {
    return (
      <Page>
        <div className="cz-login-error">{error}</div>
      </Page>
    );
  }

  if (!project) {
    return (
      <Page>
        <div className="cz-faint" style={{ padding: '24px 0', fontSize: 11, letterSpacing: '0.14em', textTransform: 'uppercase' }}>
          Loading project…
        </div>
      </Page>
    );
  }

  const tabs: { key: Tab; label: string; path: string }[] = [
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
    <Page>
      <Link to="/" className="cz-btn cz-btn-ghost" style={{ marginBottom: 18, display: 'inline-flex' }}>
        ← All projects
      </Link>

      <PageHeader
        eyebrow={`Project · prj_${project.id.slice(0, 6)}`}
        title={<>{head} <em>/ {tail}</em></>}
        sub={project.description || undefined}
        actions={
          <>
            <div style={{ position: 'relative' }}>
              <button className="cz-btn" disabled={exporting} onClick={() => setExportPickerOpen((v) => !v)}>
                {exporting ? 'Exporting…' : 'Export .env'}
              </button>
              {exportPickerOpen && (
                <div
                  role="menu"
                  className="cz-card"
                  style={{
                    position: 'absolute',
                    top: 'calc(100% + 6px)',
                    right: 0,
                    zIndex: 50,
                    minWidth: 170,
                    display: 'flex',
                    flexDirection: 'column',
                    padding: 6,
                    gap: 2,
                  }}
                  onMouseLeave={() => setExportPickerOpen(false)}
                >
                  <div className="cz-eyebrow" style={{ padding: '6px 10px' }}>Environment</div>
                  {ENVIRONMENTS.map((e) => (
                    <button
                      key={e}
                      role="menuitem"
                      className="cz-btn cz-btn-ghost"
                      style={{ justifyContent: 'flex-start', textTransform: 'uppercase' }}
                      onClick={() => handleExport(e)}
                    >
                      {e}
                    </button>
                  ))}
                </div>
              )}
            </div>
            <button className="cz-btn" disabled={rotating} onClick={() => setRotateModalOpen(true)}>
              {rotating ? 'Rotating…' : 'Rotate all'}
            </button>
            <button className="cz-btn cz-btn-primary" onClick={() => navigate(`/projects/${id}/promote`)}>
              + Promote
            </button>
          </>
        }
      />

      <KpiStrip>
        <Kpi label="Secrets" value="—" hint="this project" bars={[2, 3, 2, 4, 3, 5, 4, 5]} />
        <Kpi label="Environments" value="03" hint="alpha · uat · prod" bars={[1, 2, 2, 3, 3, 3, 3, 3]} />
        <Kpi label="Reads / hr" value="1,204" hint="+4% h/h" trend="up" bars={[6, 7, 8, 9, 8, 10, 11, 12]} flux />
        <Kpi label="Drift" value="Δ0" hint="envs aligned" />
      </KpiStrip>

      {/* Tabs */}
      <div className="cz-tabs">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            onClick={() => navigate(tab.path)}
            className={`cz-tab ${currentTab === tab.key ? 'cz-on' : ''}`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <div style={{ marginTop: 8 }}>
        <Routes>
          <Route index element={<SecretsPanel projectId={id!} />} />
          <Route
            path="promote"
            element={
              <div>
                <SectionHead index="01" title="Promotion pipeline" meta="alpha · uat · prod" />
                <OrbitalPipeline
                  stages={[
                    { id: 'alpha', label: 'alpha', sub: 'Development' },
                    { id: 'uat', label: 'uat', sub: 'Staging' },
                    { id: 'prod', label: 'prod', sub: 'Production' },
                  ]}
                />
                <SectionHead index="02" title="Promote changes" meta="diff · approve · apply" style={{ marginTop: 38 }} />
                <PromotionWizard projectId={id!} />
              </div>
            }
          />
          <Route path="promotions" element={<PromotionsList projectId={id!} />} />
          <Route path="audit" element={<AuditLogViewer projectId={id!} />} />
          <Route path="api-keys" element={<ProjectAPIKeysPanel projectId={id!} />} />
        </Routes>
      </div>

      <TypedConfirmModal
        open={rotateModalOpen}
        onOpenChange={setRotateModalOpen}
        title="Rotate all project keys"
        description={`Rotate the data encryption keys for "${project.name}". All secrets will be re-encrypted under fresh DEKs. This operation cannot be undone and will take a few seconds.`}
        confirmPhrase={project.name}
        confirmLabel="Rotate keys"
        onConfirm={handleRotateAll}
      />
    </Page>
  );
}
