import { useState, useEffect } from 'react';
import * as api from '../api/client';

export function AdminDashboardPage() {
  const [dashboard, setDashboard] = useState<Record<string, unknown> | null>(null);
  const [traces, setTraces] = useState<Record<string, unknown>[]>([]);
  const [events, setEvents] = useState<Record<string, unknown>[]>([]);
  const [plugins, setPlugins] = useState<Record<string, unknown>[]>([]);
  const [securityEvents, setSecurityEvents] = useState<Record<string, unknown>[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'overview' | 'traces' | 'events' | 'plugins' | 'security'>('overview');

  useEffect(() => {
    loadData();
  }, []);

  async function loadData() {
    setLoading(true);
    try {
      const [dashData, traceData, eventData, pluginData] = await Promise.all([
        api.getAdminDashboard(),
        api.getTraces(),
        api.getEvents(),
        api.getPlugins(),
      ]);
      setDashboard(dashData);
      setTraces(traceData);
      setEvents(eventData);
      setPlugins(pluginData);
    } catch {
      // Silently handle errors for optional data
    } finally {
      setLoading(false);
    }
  }

  const tabs = [
    { key: 'overview' as const, label: 'Overview' },
    { key: 'traces' as const, label: 'Traces' },
    { key: 'events' as const, label: 'Events' },
    { key: 'plugins' as const, label: 'Plugins' },
    { key: 'security' as const, label: 'Security' },
  ];

  if (loading) {
    return <div style={{ padding: 24 }}>Loading admin dashboard...</div>;
  }

  return (
    <div>
      <h1 style={{ marginBottom: 16 }}>Admin Dashboard</h1>

      <div style={{ display: 'flex', gap: 8, marginBottom: 24 }}>
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            style={{
              padding: '8px 16px',
              border: 'none',
              borderRadius: 4,
              background: activeTab === tab.key ? 'var(--color-primary, #3b82f6)' : '#e5e7eb',
              color: activeTab === tab.key ? '#fff' : '#374151',
              cursor: 'pointer',
              fontWeight: activeTab === tab.key ? 600 : 400,
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'overview' && (
        <div>
          <h2>System Status</h2>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(250px, 1fr))', gap: 16, marginTop: 16 }}>
            <StatusCard title="API Status" value={dashboard?.status as string || 'unknown'} />
            <StatusCard title="Metrics" value={dashboard?.metrics_url as string || '/metrics'} />
            <StatusCard title="Health" value={dashboard?.health_url as string || '/healthz'} />
            <StatusCard title="Readiness" value={dashboard?.ready_url as string || '/readyz'} />
          </div>
        </div>
      )}

      {activeTab === 'traces' && (
        <div>
          <h2>Recent Traces</h2>
          <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: 16 }}>
            <thead>
              <tr style={{ borderBottom: '2px solid #e5e7eb' }}>
                <th style={{ textAlign: 'left', padding: 8 }}>Operation</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Status</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Duration</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Trace ID</th>
              </tr>
            </thead>
            <tbody>
              {traces.map((trace, i) => (
                <tr key={i} style={{ borderBottom: '1px solid #f3f4f6' }}>
                  <td style={{ padding: 8 }}>{trace.operation as string}</td>
                  <td style={{ padding: 8 }}>
                    <span style={{
                      background: trace.status === 'ok' ? '#dcfce7' : '#fee2e2',
                      color: trace.status === 'ok' ? '#166534' : '#991b1b',
                      padding: '2px 8px',
                      borderRadius: 4,
                      fontSize: 12,
                    }}>
                      {trace.status as string}
                    </span>
                  </td>
                  <td style={{ padding: 8, fontFamily: 'monospace' }}>{trace.duration as string}</td>
                  <td style={{ padding: 8, fontFamily: 'monospace', fontSize: 12 }}>{(trace.trace_id as string)?.slice(0, 16)}</td>
                </tr>
              ))}
              {traces.length === 0 && (
                <tr><td colSpan={4} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>No traces available</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {activeTab === 'events' && (
        <div>
          <h2>Event Log</h2>
          <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: 16 }}>
            <thead>
              <tr style={{ borderBottom: '2px solid #e5e7eb' }}>
                <th style={{ textAlign: 'left', padding: 8 }}>Event Type</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Aggregate</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Published</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Time</th>
              </tr>
            </thead>
            <tbody>
              {events.map((evt, i) => (
                <tr key={i} style={{ borderBottom: '1px solid #f3f4f6' }}>
                  <td style={{ padding: 8, fontFamily: 'monospace' }}>{evt.event_type as string}</td>
                  <td style={{ padding: 8, fontFamily: 'monospace', fontSize: 12 }}>{(evt.aggregate_id as string)?.slice(0, 8)}</td>
                  <td style={{ padding: 8 }}>{evt.published ? 'Yes' : 'No'}</td>
                  <td style={{ padding: 8, fontSize: 12 }}>{new Date(evt.created_at as string).toLocaleString()}</td>
                </tr>
              ))}
              {events.length === 0 && (
                <tr><td colSpan={4} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>No events recorded</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {activeTab === 'plugins' && (
        <div>
          <h2>Registered Plugins</h2>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 16, marginTop: 16 }}>
            {plugins.map((plugin, i) => (
              <div key={i} style={{ border: '1px solid #e5e7eb', borderRadius: 8, padding: 16 }}>
                <h3 style={{ margin: 0 }}>{plugin.name as string}</h3>
                <p style={{ margin: '4px 0', color: '#6b7280', fontSize: 14 }}>
                  Type: {plugin.plugin_type as string} | v{plugin.version as string}
                </p>
                <span style={{
                  background: plugin.enabled ? '#dcfce7' : '#fee2e2',
                  color: plugin.enabled ? '#166534' : '#991b1b',
                  padding: '2px 8px',
                  borderRadius: 4,
                  fontSize: 12,
                }}>
                  {plugin.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
            ))}
            {plugins.length === 0 && (
              <p style={{ color: '#9ca3af' }}>No plugins registered</p>
            )}
          </div>
        </div>
      )}

      {activeTab === 'security' && (
        <div>
          <h2>Security Events</h2>
          <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: 16 }}>
            <thead>
              <tr style={{ borderBottom: '2px solid #e5e7eb' }}>
                <th style={{ textAlign: 'left', padding: 8 }}>Event</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Severity</th>
                <th style={{ textAlign: 'left', padding: 8 }}>IP Address</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Time</th>
              </tr>
            </thead>
            <tbody>
              {securityEvents.map((evt, i) => (
                <tr key={i} style={{ borderBottom: '1px solid #f3f4f6' }}>
                  <td style={{ padding: 8 }}>{evt.event_type as string}</td>
                  <td style={{ padding: 8 }}>
                    <span style={{
                      background: evt.severity === 'critical' ? '#fee2e2' : evt.severity === 'warning' ? '#fef3c7' : '#e0f2fe',
                      padding: '2px 8px',
                      borderRadius: 4,
                      fontSize: 12,
                    }}>
                      {evt.severity as string}
                    </span>
                  </td>
                  <td style={{ padding: 8, fontFamily: 'monospace' }}>{evt.ip_address as string}</td>
                  <td style={{ padding: 8, fontSize: 12 }}>{new Date(evt.created_at as string).toLocaleString()}</td>
                </tr>
              ))}
              {securityEvents.length === 0 && (
                <tr><td colSpan={4} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>No security events</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function StatusCard({ title, value }: { title: string; value: string }) {
  return (
    <div style={{ border: '1px solid #e5e7eb', borderRadius: 8, padding: 16 }}>
      <p style={{ margin: 0, fontSize: 13, color: '#6b7280' }}>{title}</p>
      <p style={{ margin: '4px 0 0', fontSize: 18, fontWeight: 600 }}>{value}</p>
    </div>
  );
}
