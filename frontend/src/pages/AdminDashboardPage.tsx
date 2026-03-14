import { useState, useEffect, useMemo } from 'react';
import * as api from '../api/client';

export function AdminDashboardPage() {
  const [dashboard, setDashboard] = useState<Record<string, unknown> | null>(null);
  const [traces, setTraces] = useState<Record<string, unknown>[]>([]);
  const [events, setEvents] = useState<Record<string, unknown>[]>([]);
  const [plugins, setPlugins] = useState<Record<string, unknown>[]>([]);
  const [securityEvents, _setSecurityEvents] = useState<Record<string, unknown>[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'overview' | 'traces' | 'events' | 'plugins' | 'security'>('overview');
  const [traceSearch, setTraceSearch] = useState('');
  const [eventSearch, setEventSearch] = useState('');
  const [securitySearch, setSecuritySearch] = useState('');

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

  const filteredTraces = useMemo(() => {
    if (!traceSearch) return traces;
    const q = traceSearch.toLowerCase();
    return traces.filter(t =>
      (t.operation as string)?.toLowerCase().includes(q) ||
      (t.status as string)?.toLowerCase().includes(q) ||
      (t.trace_id as string)?.toLowerCase().includes(q)
    );
  }, [traces, traceSearch]);

  const filteredEvents = useMemo(() => {
    if (!eventSearch) return events;
    const q = eventSearch.toLowerCase();
    return events.filter(e =>
      (e.event_type as string)?.toLowerCase().includes(q) ||
      (e.aggregate_id as string)?.toLowerCase().includes(q)
    );
  }, [events, eventSearch]);

  const filteredSecurity = useMemo(() => {
    if (!securitySearch) return securityEvents;
    const q = securitySearch.toLowerCase();
    return securityEvents.filter(e =>
      (e.event_type as string)?.toLowerCase().includes(q) ||
      (e.severity as string)?.toLowerCase().includes(q) ||
      (e.ip_address as string)?.toLowerCase().includes(q)
    );
  }, [securityEvents, securitySearch]);

  const tabs = [
    { key: 'overview' as const, label: 'Overview', icon: '\u25A0' },
    { key: 'traces' as const, label: 'Traces', count: traces.length },
    { key: 'events' as const, label: 'Events', count: events.length },
    { key: 'plugins' as const, label: 'Plugins', count: plugins.length },
    { key: 'security' as const, label: 'Security', count: securityEvents.length },
  ];

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 80 }}>
        <div style={{ textAlign: 'center' }}>
          <div style={{ fontSize: 14, color: 'var(--color-text-secondary)' }}>Loading admin dashboard...</div>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700 }}>Admin Dashboard</h1>
        <button onClick={loadData} style={refreshBtn}>Refresh</button>
      </div>

      {/* Tabs */}
      <div style={tabBar}>
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            style={{
              ...tabButton,
              borderBottomColor: activeTab === tab.key ? 'var(--color-primary)' : 'transparent',
              color: activeTab === tab.key ? 'var(--color-primary)' : 'var(--color-text-secondary)',
              fontWeight: activeTab === tab.key ? 600 : 400,
            }}
          >
            {tab.label}
            {'count' in tab && tab.count !== undefined && (
              <span style={countBadge}>{tab.count}</span>
            )}
          </button>
        ))}
      </div>

      {/* Overview Tab */}
      {activeTab === 'overview' && (
        <div>
          <div style={statsGrid}>
            <StatCard
              label="API Status"
              value={dashboard?.status as string || 'unknown'}
              color={dashboard?.status === 'ok' ? 'var(--color-success)' : 'var(--color-warning)'}
            />
            <StatCard label="Total Traces" value={String(traces.length)} color="var(--color-primary)" />
            <StatCard label="Total Events" value={String(events.length)} color="#818cf8" />
            <StatCard label="Plugins" value={String(plugins.length)} color="var(--color-warning)" />
          </div>

          <div style={{ ...card, marginTop: 16 }}>
            <h3 style={sectionTitle}>Endpoints</h3>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <EndpointRow label="Health" path={dashboard?.health_url as string || '/healthz'} />
              <EndpointRow label="Readiness" path={dashboard?.ready_url as string || '/readyz'} />
              <EndpointRow label="Metrics" path={dashboard?.metrics_url as string || '/metrics'} />
              <EndpointRow label="OpenAPI" path="/api/v1/openapi.json" />
            </div>
          </div>
        </div>
      )}

      {/* Traces Tab */}
      {activeTab === 'traces' && (
        <div>
          <div style={searchBar}>
            <input
              type="text"
              value={traceSearch}
              onChange={(e) => setTraceSearch(e.target.value)}
              placeholder="Search traces by operation, status, or trace ID..."
              style={searchInput}
            />
            <span style={resultCount}>{filteredTraces.length} result{filteredTraces.length !== 1 ? 's' : ''}</span>
          </div>

          <div style={tableCard}>
            <table style={tableStyle}>
              <thead>
                <tr>
                  <th style={thStyle}>Operation</th>
                  <th style={thStyle}>Status</th>
                  <th style={thStyle}>Duration</th>
                  <th style={thStyle}>Trace ID</th>
                </tr>
              </thead>
              <tbody>
                {filteredTraces.map((trace, i) => (
                  <tr key={i} style={rowStyle}>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 13, fontWeight: 500 }}>{trace.operation as string}</code>
                    </td>
                    <td style={tdStyle}>
                      <span style={{
                        ...statusBadge,
                        background: trace.status === 'ok' ? 'rgba(34, 197, 94, 0.15)' : 'rgba(239, 68, 68, 0.15)',
                        color: trace.status === 'ok' ? '#22c55e' : '#ef4444',
                      }}>
                        {trace.status as string}
                      </span>
                    </td>
                    <td style={tdStyle}>
                      <span style={{ fontFamily: 'monospace', fontSize: 13 }}>{trace.duration as string}</span>
                    </td>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                        {(trace.trace_id as string)?.slice(0, 16)}...
                      </code>
                    </td>
                  </tr>
                ))}
                {filteredTraces.length === 0 && (
                  <tr>
                    <td colSpan={4} style={emptyCell}>
                      {traceSearch ? `No traces matching "${traceSearch}"` : 'No traces available'}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Events Tab */}
      {activeTab === 'events' && (
        <div>
          <div style={searchBar}>
            <input
              type="text"
              value={eventSearch}
              onChange={(e) => setEventSearch(e.target.value)}
              placeholder="Search events by type or aggregate ID..."
              style={searchInput}
            />
            <span style={resultCount}>{filteredEvents.length} result{filteredEvents.length !== 1 ? 's' : ''}</span>
          </div>

          <div style={tableCard}>
            <table style={tableStyle}>
              <thead>
                <tr>
                  <th style={thStyle}>Event Type</th>
                  <th style={thStyle}>Aggregate ID</th>
                  <th style={thStyle}>Published</th>
                  <th style={thStyle}>Time</th>
                </tr>
              </thead>
              <tbody>
                {filteredEvents.map((evt, i) => (
                  <tr key={i} style={rowStyle}>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 13, fontWeight: 500 }}>{evt.event_type as string}</code>
                    </td>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                        {(evt.aggregate_id as string)?.slice(0, 12)}...
                      </code>
                    </td>
                    <td style={tdStyle}>
                      <span style={{
                        ...statusBadge,
                        background: evt.published ? 'rgba(34, 197, 94, 0.15)' : 'rgba(245, 158, 11, 0.15)',
                        color: evt.published ? '#22c55e' : '#f59e0b',
                      }}>
                        {evt.published ? 'Published' : 'Pending'}
                      </span>
                    </td>
                    <td style={tdStyle}>
                      <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                        {new Date(evt.created_at as string).toLocaleString()}
                      </span>
                    </td>
                  </tr>
                ))}
                {filteredEvents.length === 0 && (
                  <tr>
                    <td colSpan={4} style={emptyCell}>
                      {eventSearch ? `No events matching "${eventSearch}"` : 'No events recorded'}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Plugins Tab */}
      {activeTab === 'plugins' && (
        <div>
          {plugins.length === 0 ? (
            <div style={emptyState}>
              <p style={{ fontSize: 16, fontWeight: 500, marginBottom: 4 }}>No plugins registered</p>
              <p style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
                Plugins extend KeepSave with custom secret providers, notifications, and validation.
              </p>
            </div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 16 }}>
              {plugins.map((plugin, i) => (
                <div key={i} style={pluginCard}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                    <h3 style={{ fontSize: 15, fontWeight: 600 }}>{plugin.name as string}</h3>
                    <span style={{
                      ...statusBadge,
                      background: plugin.enabled ? 'rgba(34, 197, 94, 0.15)' : 'rgba(239, 68, 68, 0.15)',
                      color: plugin.enabled ? '#22c55e' : '#ef4444',
                    }}>
                      {plugin.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </div>
                  <div style={{ display: 'flex', gap: 8 }}>
                    <span style={metaBadge}>{plugin.plugin_type as string}</span>
                    <span style={metaBadge}>v{plugin.version as string}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Security Tab */}
      {activeTab === 'security' && (
        <div>
          <div style={searchBar}>
            <input
              type="text"
              value={securitySearch}
              onChange={(e) => setSecuritySearch(e.target.value)}
              placeholder="Search security events by type, severity, or IP..."
              style={searchInput}
            />
            <span style={resultCount}>{filteredSecurity.length} result{filteredSecurity.length !== 1 ? 's' : ''}</span>
          </div>

          <div style={tableCard}>
            <table style={tableStyle}>
              <thead>
                <tr>
                  <th style={thStyle}>Event</th>
                  <th style={thStyle}>Severity</th>
                  <th style={thStyle}>IP Address</th>
                  <th style={thStyle}>Time</th>
                </tr>
              </thead>
              <tbody>
                {filteredSecurity.map((evt, i) => (
                  <tr key={i} style={rowStyle}>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 13, fontWeight: 500 }}>{evt.event_type as string}</code>
                    </td>
                    <td style={tdStyle}>
                      <span style={{
                        ...statusBadge,
                        background: evt.severity === 'critical' ? 'rgba(239, 68, 68, 0.15)' : evt.severity === 'warning' ? 'rgba(245, 158, 11, 0.15)' : 'rgba(99, 102, 241, 0.15)',
                        color: evt.severity === 'critical' ? '#ef4444' : evt.severity === 'warning' ? '#f59e0b' : '#818cf8',
                      }}>
                        {evt.severity as string}
                      </span>
                    </td>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>{evt.ip_address as string}</code>
                    </td>
                    <td style={tdStyle}>
                      <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                        {new Date(evt.created_at as string).toLocaleString()}
                      </span>
                    </td>
                  </tr>
                ))}
                {filteredSecurity.length === 0 && (
                  <tr>
                    <td colSpan={4} style={emptyCell}>
                      {securitySearch ? `No events matching "${securitySearch}"` : 'No security events'}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

/* Sub-components */

function StatCard({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div style={card}>
      <p style={{ fontSize: 12, fontWeight: 500, color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>{label}</p>
      <p style={{ fontSize: 28, fontWeight: 700, color, marginTop: 4, fontVariantNumeric: 'tabular-nums' }}>{value}</p>
    </div>
  );
}

function EndpointRow({ label, path }: { label: string; path: string }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 0' }}>
      <span style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>{label}</span>
      <code style={{ fontSize: 12, padding: '3px 8px', background: 'var(--color-input-bg)', borderRadius: 4, border: '1px solid var(--color-border)' }}>{path}</code>
    </div>
  );
}

/* Styles */

const tabBar: React.CSSProperties = {
  display: 'flex',
  gap: 0,
  borderBottom: '1px solid var(--color-border)',
  marginBottom: 24,
};

const tabButton: React.CSSProperties = {
  background: 'transparent',
  border: 'none',
  borderBottom: '2px solid transparent',
  padding: '10px 16px',
  fontSize: 14,
  cursor: 'pointer',
  display: 'flex',
  alignItems: 'center',
  gap: 6,
  transition: 'color 0.15s',
};

const countBadge: React.CSSProperties = {
  fontSize: 11,
  padding: '1px 6px',
  borderRadius: 10,
  background: 'var(--color-border)',
  color: 'var(--color-text-secondary)',
  fontWeight: 600,
};

const statsGrid: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
  gap: 16,
};

const card: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: 20,
};

const sectionTitle: React.CSSProperties = {
  fontSize: 14,
  fontWeight: 600,
  marginBottom: 12,
  color: 'var(--color-text-secondary)',
  textTransform: 'uppercase',
  letterSpacing: '0.04em',
};

const searchBar: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
  marginBottom: 16,
};

const searchInput: React.CSSProperties = {
  flex: 1,
  padding: '10px 14px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 14,
  color: 'var(--color-text)',
};

const resultCount: React.CSSProperties = {
  fontSize: 12,
  color: 'var(--color-text-secondary)',
  whiteSpace: 'nowrap',
};

const tableCard: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  overflow: 'hidden',
};

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
};

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '12px 16px',
  fontSize: 11,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  borderBottom: '1px solid var(--color-border)',
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
  background: 'var(--color-input-bg)',
};

const tdStyle: React.CSSProperties = {
  padding: '12px 16px',
  borderBottom: '1px solid var(--color-border)',
};

const rowStyle: React.CSSProperties = {
  transition: 'background 0.1s',
};

const statusBadge: React.CSSProperties = {
  padding: '3px 10px',
  borderRadius: 12,
  fontSize: 11,
  fontWeight: 600,
  textTransform: 'uppercase',
  letterSpacing: '0.03em',
};

const metaBadge: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 4,
  fontSize: 11,
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  color: 'var(--color-text-secondary)',
};

const pluginCard: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: 20,
};

const emptyCell: React.CSSProperties = {
  padding: 40,
  textAlign: 'center',
  color: 'var(--color-text-secondary)',
  fontSize: 14,
};

const emptyState: React.CSSProperties = {
  textAlign: 'center',
  padding: 60,
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  border: '1px solid var(--color-border)',
};

const refreshBtn: React.CSSProperties = {
  padding: '6px 14px',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 500,
};
