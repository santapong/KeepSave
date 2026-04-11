import { useState, useEffect } from 'react';
import { Users, Activity, KeyRound } from 'lucide-react';
import * as api from '@/api/client';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '@/components/ui/table';
import { StatCard } from '@/components/dashboard/StatCard';
import { HeatmapGrid } from '@/components/dashboard/HeatmapGrid';

interface AgentActivityRecord {
  id?: string;
  timestamp?: string;
  api_key_name?: string;
  action?: string;
  secret_key?: string;
  environment?: string;
  ip_address?: string;
  project_id?: string;
}

interface HeatmapCell {
  hour: number;
  day: number;
  count: number;
}

interface ProjectOption {
  id: string;
  name: string;
}

export function AgentsTab() {
  const [activities, setActivities] = useState<AgentActivityRecord[]>([]);
  const [heatmapData, setHeatmapData] = useState<HeatmapCell[]>([]);
  const [projects, setProjects] = useState<ProjectOption[]>([]);
  const [selectedProject, setSelectedProject] = useState<string>('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch projects list on mount
  useEffect(() => {
    let cancelled = false;

    async function fetchProjects() {
      try {
        const projectList = await api.listProjects();
        if (!cancelled) {
          setProjects(
            projectList.map((p) => ({ id: p.id, name: p.name })),
          );
        }
      } catch {
        // Projects list is optional; continue without it
      }
    }

    fetchProjects();
    return () => { cancelled = true; };
  }, []);

  // Fetch activity and heatmap data when project selection changes
  useEffect(() => {
    let cancelled = false;

    async function fetchData() {
      setLoading(true);
      setError(null);
      try {
        // Fetch global agent activity
        const activityData = await api.getGlobalAgentActivity();
        if (cancelled) return;

        const parsed: AgentActivityRecord[] = activityData.map((a) => ({
          id: String(a.id ?? ''),
          timestamp: String(a.timestamp ?? a.created_at ?? ''),
          api_key_name: String(a.api_key_name ?? a.agent ?? 'Unknown'),
          action: String(a.action ?? a.event_type ?? ''),
          secret_key: String(a.secret_key ?? a.key ?? ''),
          environment: String(a.environment ?? ''),
          ip_address: String(a.ip_address ?? a.ip ?? ''),
          project_id: String(a.project_id ?? ''),
        }));

        // Filter by project if selected
        const filtered =
          selectedProject === 'all'
            ? parsed
            : parsed.filter((a) => a.project_id === selectedProject);

        setActivities(filtered);

        // Try fetching heatmap data for selected project
        if (selectedProject !== 'all') {
          try {
            const heatmapRaw = await api.getAgentHeatmap(selectedProject);
            if (!cancelled) {
              setHeatmapData(
                heatmapRaw.map((h) => ({
                  hour: Number(h.hour ?? 0),
                  day: Number(h.day ?? 0),
                  count: Number(h.count ?? 0),
                })),
              );
            }
          } catch {
            if (!cancelled) setHeatmapData([]);
          }
        } else {
          // Build aggregate heatmap from activity timestamps
          const heatmap = buildHeatmapFromActivities(parsed);
          if (!cancelled) setHeatmapData(heatmap);
        }
      } catch (err) {
        if (!cancelled)
          setError(err instanceof Error ? err.message : 'Failed to load agent activity');
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchData();
    return () => { cancelled = true; };
  }, [selectedProject]);

  // Compute stats
  const totalActivities = activities.length;
  const activeAgents = new Set(activities.map((a) => a.api_key_name)).size;
  const uniqueSecrets = new Set(
    activities.filter((a) => a.secret_key).map((a) => a.secret_key),
  ).size;

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64 rounded-md" />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-[100px] rounded-lg" />
          ))}
        </div>
        <Skeleton className="h-[180px] rounded-lg" />
        <Skeleton className="h-[300px] rounded-lg" />
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

  return (
    <div className="space-y-6">
      {/* Project selector */}
      <div className="flex items-center gap-3">
        <label className="text-sm font-medium text-muted-foreground">Project:</label>
        <Select value={selectedProject} onValueChange={setSelectedProject}>
          <SelectTrigger className="w-64">
            <SelectValue placeholder="All Projects" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Projects</SelectItem>
            {projects.map((p) => (
              <SelectItem key={p.id} value={p.id}>
                {p.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard
          label="Total Activities"
          value={totalActivities}
          icon={<Activity className="h-5 w-5" />}
        />
        <StatCard
          label="Active Agents"
          value={activeAgents}
          icon={<Users className="h-5 w-5" />}
        />
        <StatCard
          label="Unique Secrets Accessed"
          value={uniqueSecrets}
          icon={<KeyRound className="h-5 w-5" />}
        />
      </div>

      {/* Heatmap */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Activity Heatmap</CardTitle>
        </CardHeader>
        <CardContent>
          {heatmapData.length > 0 ? (
            <HeatmapGrid data={heatmapData} />
          ) : (
            <p className="text-sm text-muted-foreground">
              No heatmap data available for the selected project.
            </p>
          )}
        </CardContent>
      </Card>

      {/* Activity log table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Activity Log</CardTitle>
        </CardHeader>
        <CardContent>
          {activities.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <Activity className="mb-3 h-10 w-10 text-muted-foreground/50" />
              <p className="text-sm font-medium text-muted-foreground">
                No agent activity recorded yet
              </p>
              <p className="mt-1 text-xs text-muted-foreground/70">
                Agent access events will appear here once API keys are used.
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>Agent (API Key)</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Secret Key</TableHead>
                  <TableHead>Environment</TableHead>
                  <TableHead>IP</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {activities.slice(0, 100).map((activity, idx) => (
                  <TableRow key={activity.id || idx}>
                    <TableCell className="whitespace-nowrap font-mono text-xs">
                      {formatTimestamp(activity.timestamp)}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-mono text-xs">
                        {activity.api_key_name}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <span className="text-sm">{activity.action}</span>
                    </TableCell>
                    <TableCell>
                      <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
                        {activity.secret_key || '--'}
                      </code>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary" className="text-xs">
                        {activity.environment || '--'}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {activity.ip_address || '--'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function formatTimestamp(ts?: string): string {
  if (!ts) return '--';
  try {
    const date = new Date(ts);
    if (isNaN(date.getTime())) return ts;
    return date.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  } catch {
    return ts;
  }
}

function buildHeatmapFromActivities(
  activities: AgentActivityRecord[],
): { hour: number; day: number; count: number }[] {
  const counts = new Map<string, number>();

  for (const a of activities) {
    if (!a.timestamp) continue;
    try {
      const date = new Date(a.timestamp);
      if (isNaN(date.getTime())) continue;
      const hour = date.getHours();
      // getDay() returns 0=Sunday, shift to 0=Monday
      const rawDay = date.getDay();
      const day = rawDay === 0 ? 6 : rawDay - 1;
      const key = `${day}-${hour}`;
      counts.set(key, (counts.get(key) ?? 0) + 1);
    } catch {
      // skip malformed timestamps
    }
  }

  const result: { hour: number; day: number; count: number }[] = [];
  for (const [key, count] of counts) {
    const [day, hour] = key.split('-').map(Number);
    result.push({ day, hour, count });
  }

  return result;
}
