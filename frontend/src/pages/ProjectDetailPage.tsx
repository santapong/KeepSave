import { useState, useEffect } from 'react';
import { useParams, Link, Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import { getProject } from '../api/client';
import { SecretsPanel } from '../components/SecretsPanel';
import { PromotionWizard } from '../components/PromotionWizard';
import { PromotionsList } from '../components/PromotionsList';
import { AuditLogViewer } from '../components/AuditLogViewer';
import { ProjectAPIKeysPanel } from '../components/ProjectAPIKeysPanel';
import type { Project } from '../types';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';

import { ArrowLeft } from 'lucide-react';

type Tab = 'secrets' | 'promote' | 'promotions' | 'audit' | 'api-keys';

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
      <div className="bg-destructive/10 text-destructive px-3.5 py-2.5 rounded-md text-sm border border-destructive/20">
        {error}
      </div>
    );
  }

  if (!project) {
    return (
      <div className="space-y-3 p-10">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-64" />
        <Skeleton className="h-10 w-full mt-6 rounded-md" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    );
  }

  const tabs: { key: Tab; label: string; path: string }[] = [
    { key: 'secrets', label: 'Secrets', path: `/projects/${id}` },
    { key: 'promote', label: 'Promote', path: `/projects/${id}/promote` },
    { key: 'promotions', label: 'History', path: `/projects/${id}/promotions` },
    { key: 'audit', label: 'Audit Log', path: `/projects/${id}/audit` },
    { key: 'api-keys', label: 'API Keys', path: `/projects/${id}/api-keys` },
  ];

  return (
    <div>
      <div className="mb-6">
        <Link to="/" className="text-sm text-muted-foreground hover:text-foreground inline-flex items-center gap-1 transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" />
          Projects
        </Link>
        <h1 className="text-2xl font-bold mt-2">{project.name}</h1>
        {project.description && (
          <p className="text-sm text-muted-foreground">{project.description}</p>
        )}
      </div>

      <Tabs value={currentTab} onValueChange={(value) => {
        const tab = tabs.find((t) => t.key === value);
        if (tab) navigate(tab.path);
      }}>
        <TabsList className="mb-6">
          {tabs.map((tab) => (
            <TabsTrigger key={tab.key} value={tab.key}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      <Routes>
        <Route index element={<SecretsPanel projectId={id!} />} />
        <Route path="promote" element={<PromotionWizard projectId={id!} />} />
        <Route path="promotions" element={<PromotionsList projectId={id!} />} />
        <Route path="audit" element={<AuditLogViewer projectId={id!} />} />
        <Route path="api-keys" element={<ProjectAPIKeysPanel projectId={id!} />} />
      </Routes>
    </div>
  );
}
