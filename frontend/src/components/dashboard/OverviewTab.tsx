import { useState, useEffect } from 'react';
import { Activity, FolderKey, KeyRound, Lock } from 'lucide-react';
import * as api from '@/api/client';
import { cn } from '@/lib/utils';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { StatCard } from '@/components/dashboard/StatCard';

interface HealthEndpoint {
  label: string;
  path: string;
  status: 'ok' | 'error' | 'unknown';
}

export function OverviewTab() {
  const [dashboard, setDashboard] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function fetchData() {
      setLoading(true);
      setError(null);
      try {
        const data = await api.getAdminDashboard();
        if (!cancelled) setDashboard(data);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load dashboard');
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchData();
    return () => { cancelled = true; };
  }, []);

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-[100px] rounded-lg" />
          ))}
        </div>
        <Skeleton className="h-[200px] rounded-lg" />
        <Skeleton className="h-[80px] rounded-lg" />
      </div>
    );
  }

  if (error) {
    return (
      <Card>
        <CardContent className="p-6">
          <p className="text-sm text-destructive">{error}</p>
        </CardContent>
      </Card>
    );
  }

  const status = (dashboard?.status as string) ?? 'unknown';
  const totalProjects = Number(dashboard?.total_projects ?? 0);
  const totalSecrets = Number(dashboard?.total_secrets ?? 0);
  const activeApiKeys = Number(dashboard?.active_api_keys ?? 0);
  const uptime = (dashboard?.uptime as string) ?? 'N/A';

  const endpoints: HealthEndpoint[] = [
    {
      label: 'Health Check',
      path: (dashboard?.health_url as string) ?? '/healthz',
      status: status === 'ok' ? 'ok' : 'unknown',
    },
    {
      label: 'Readiness',
      path: (dashboard?.ready_url as string) ?? '/readyz',
      status: status === 'ok' ? 'ok' : 'unknown',
    },
    {
      label: 'Metrics',
      path: (dashboard?.metrics_url as string) ?? '/metrics',
      status: status === 'ok' ? 'ok' : 'unknown',
    },
    {
      label: 'API Docs',
      path: '/api/v1/openapi.json',
      status: 'ok',
    },
  ];

  return (
    <div className="space-y-6">
      {/* Stat cards row */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="API Status"
          value={status}
          icon={<Activity className="h-5 w-5" />}
          color={
            status === 'ok'
              ? 'text-green-600'
              : status === 'error'
                ? 'text-destructive'
                : 'text-muted-foreground'
          }
        />
        <StatCard
          label="Total Projects"
          value={totalProjects}
          icon={<FolderKey className="h-5 w-5" />}
        />
        <StatCard
          label="Total Secrets"
          value={totalSecrets}
          icon={<Lock className="h-5 w-5" />}
        />
        <StatCard
          label="Active API Keys"
          value={activeApiKeys}
          icon={<KeyRound className="h-5 w-5" />}
        />
      </div>

      {/* Health check endpoints */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Health Checks</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {endpoints.map((ep) => (
              <div
                key={ep.path}
                className="flex items-center justify-between rounded-md border p-3"
              >
                <div className="flex items-center gap-2">
                  <span
                    className={cn(
                      'inline-block h-2.5 w-2.5 rounded-full',
                      ep.status === 'ok' ? 'bg-green-500' : 'bg-red-500',
                    )}
                  />
                  <span className="text-sm font-medium">{ep.label}</span>
                </div>
                <code className="rounded bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                  {ep.path}
                </code>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Uptime display */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">System Uptime</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-3">
            <Badge variant={status === 'ok' ? 'success' : 'destructive'}>
              {status === 'ok' ? 'Running' : 'Degraded'}
            </Badge>
            <span className="text-sm text-muted-foreground">
              Uptime: <span className="font-mono font-medium text-foreground">{uptime}</span>
            </span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
