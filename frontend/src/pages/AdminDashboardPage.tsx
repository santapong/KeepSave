import { useState } from 'react';
import { Routes, Route, useNavigate, useLocation } from 'react-router-dom';
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
import { Page, PageHeader } from '@/components/cosmic/primitives';
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
    <Page>
      <PageHeader
        eyebrow="Platform · observability"
        title="Admin dashboard"
        sub="Monitor system health, metrics, agents, and security across the platform."
        actions={
          <>
            <TimeRangeSelector value={timeRange} onChange={setTimeRange} />
            <button type="button" className="cz-btn" onClick={() => setRefreshKey((k) => k + 1)}>
              <RefreshCw size={14} /> Refresh
            </button>
          </>
        }
      />

      {/* Tab Navigation */}
      <div className="cz-tabs">
        {TABS.map((tab) => {
          const isActive = currentTab === tab.key;
          const Icon = tab.icon;
          return (
            <button
              key={tab.key}
              onClick={() => navigate(tab.key ? `/admin/${tab.key}` : '/admin')}
              className={`cz-tab ${isActive ? 'cz-on' : ''}`}
              style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}
            >
              <Icon size={15} />
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
    </Page>
  );
}
