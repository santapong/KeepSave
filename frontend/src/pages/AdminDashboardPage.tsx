import { useState } from 'react';
import { Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import {
  LayoutDashboard,
  BarChart3,
  Bot,
  Shield,
  Activity,
  Server,
  Calendar,
  Puzzle,
  RefreshCw,
} from 'lucide-react';
import { TimeRangeSelector } from '@/components/dashboard/TimeRangeSelector';
import { OverviewTab } from '@/components/dashboard/OverviewTab';
import { MetricsTab } from '@/components/dashboard/MetricsTab';
import { AgentsTab } from '@/components/dashboard/AgentsTab';
import { SecurityTab } from '@/components/dashboard/SecurityTab';
import { TracesTab } from '@/components/dashboard/TracesTab';
import { MCPTab } from '@/components/dashboard/MCPTab';
import { EventsTab } from '@/components/dashboard/EventsTab';
import { PluginsTab } from '@/components/dashboard/PluginsTab';

const TABS = [
  { key: '', label: 'Overview', icon: LayoutDashboard },
  { key: 'metrics', label: 'Metrics', icon: BarChart3 },
  { key: 'agents', label: 'Agents', icon: Bot },
  { key: 'security', label: 'Security', icon: Shield },
  { key: 'traces', label: 'Traces', icon: Activity },
  { key: 'mcp', label: 'MCP Hub', icon: Server },
  { key: 'events', label: 'Events', icon: Calendar },
  { key: 'plugins', label: 'Plugins', icon: Puzzle },
] as const;

export function AdminDashboardPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [timeRange, setTimeRange] = useState('24h');
  const [refreshKey, setRefreshKey] = useState(0);

  const currentTab = location.pathname.replace('/admin', '').replace('/', '') || '';

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Admin Dashboard</h1>
          <p className="text-sm text-muted-foreground">
            Monitor system health, metrics, agents, and security
          </p>
        </div>
        <div className="flex items-center gap-3">
          <TimeRangeSelector value={timeRange} onChange={setTimeRange} />
          <Button
            variant="outline"
            size="sm"
            onClick={() => setRefreshKey((k) => k + 1)}
          >
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      {/* Tab Navigation */}
      <div className="flex gap-1 overflow-x-auto border-b border-border pb-px">
        {TABS.map((tab) => {
          const isActive = currentTab === tab.key;
          const Icon = tab.icon;
          return (
            <button
              key={tab.key}
              onClick={() => navigate(tab.key ? `/admin/${tab.key}` : '/admin')}
              className={cn(
                'inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium whitespace-nowrap border-b-2 transition-colors',
                isActive
                  ? 'border-primary text-primary'
                  : 'border-transparent text-muted-foreground hover:text-foreground hover:border-border'
              )}
            >
              <Icon className="h-4 w-4" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* Tab Content */}
      <Routes>
        <Route index element={<OverviewTab key={refreshKey} />} />
        <Route path="metrics" element={<MetricsTab key={refreshKey} />} />
        <Route path="agents" element={<AgentsTab key={refreshKey} />} />
        <Route path="security" element={<SecurityTab key={refreshKey} />} />
        <Route path="traces" element={<TracesTab key={refreshKey} />} />
        <Route path="mcp" element={<MCPTab key={refreshKey} />} />
        <Route path="events" element={<EventsTab key={refreshKey} />} />
        <Route path="plugins" element={<PluginsTab key={refreshKey} />} />
      </Routes>
    </div>
  );
}
