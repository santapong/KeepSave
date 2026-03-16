import { useState, useEffect, useMemo } from 'react';
import { listAuditLog } from '../api/client';
import { formatDate } from '../utils/formatDate';
import type { AuditEntry } from '../types';

interface AuditLogViewerProps {
  projectId: string;
}

const EXAMPLE_ENTRIES: AuditEntry[] = [
  {
    id: 'ex-1',
    action: 'secret.create',
    environment: 'alpha',
    details: { key: 'DATABASE_URL' },
    ip_address: '127.0.0.1',
    created_at: new Date(Date.now() - 1 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: 'ex-2',
    action: 'secret.update',
    environment: 'alpha',
    details: { key: 'API_KEY', version: 2 },
    ip_address: '127.0.0.1',
    created_at: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: 'ex-3',
    action: 'promotion.create',
    environment: 'uat',
    details: { source: 'alpha', target: 'uat', keys_count: 5 },
    ip_address: '127.0.0.1',
    created_at: new Date(Date.now() - 1 * 24 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: 'ex-4',
    action: 'promotion.approve',
    environment: 'uat',
    details: { promotion_id: 'abc-123' },
    ip_address: '10.0.0.1',
    created_at: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: 'ex-5',
    action: 'apikey.create',
    environment: '',
    details: { name: 'ci-pipeline', scopes: ['read'] },
    ip_address: '192.168.1.1',
    created_at: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString(),
  },
];

function getActionBadgeColor(action: string): { bg: string; text: string } {
  if (action.startsWith('secret.create') || action.startsWith('secret.update')) {
    return { bg: 'rgba(34, 197, 94, 0.15)', text: '#22c55e' };
  }
  if (action.startsWith('secret.delete')) {
    return { bg: 'rgba(239, 68, 68, 0.15)', text: 'var(--color-danger)' };
  }
  if (action.startsWith('promotion.')) {
    return { bg: 'rgba(99, 102, 241, 0.15)', text: 'var(--color-primary)' };
  }
  if (action.startsWith('apikey.')) {
    return { bg: 'rgba(59, 130, 246, 0.15)', text: '#3b82f6' };
  }
  if (action.startsWith('auth.')) {
    return { bg: 'rgba(107, 114, 128, 0.15)', text: 'var(--color-text-secondary)' };
  }
  return { bg: 'rgba(107, 114, 128, 0.15)', text: 'var(--color-text-secondary)' };
}

function getEnvBadgeColor(env: string): { bg: string; text: string } {
  switch (env.toLowerCase()) {
    case 'alpha':
      return { bg: 'rgba(34, 197, 94, 0.15)', text: '#22c55e' };
    case 'uat':
      return { bg: 'rgba(99, 102, 241, 0.15)', text: 'var(--color-primary)' };
    case 'prod':
      return { bg: 'rgba(245, 158, 11, 0.15)', text: '#f59e0b' };
    default:
      return { bg: 'rgba(107, 114, 128, 0.15)', text: 'var(--color-text-secondary)' };
  }
}

export function AuditLogViewer({ projectId }: AuditLogViewerProps) {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());

  useEffect(() => {
    setLoading(true);
    setError('');
    listAuditLog(projectId)
      .then(setEntries)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load audit log'))
      .finally(() => setLoading(false));
  }, [projectId]);

  const isShowingExamples = !loading && entries.length === 0;
  const displayEntries = isShowingExamples ? EXAMPLE_ENTRIES : entries;

  const filteredEntries = useMemo(() => {
    if (!search.trim()) return displayEntries;
    const query = search.toLowerCase();
    return displayEntries.filter(
      (entry) =>
        entry.action.toLowerCase().includes(query) ||
        (entry.environment && entry.environment.toLowerCase().includes(query))
    );
  }, [displayEntries, search]);

  function toggleRow(id: string) {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  if (loading) {
    return (
      <div style={loadingContainer}>
        <div style={loadingSpinner} />
        <span style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
          Loading audit log...
        </span>
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div style={headerRow}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <h3 style={titleStyle}>Audit Log</h3>
            <span style={countBadge}>{filteredEntries.length}</span>
          </div>
          <p style={subtitleStyle}>
            Track all changes, promotions, and access events for this project.
          </p>
        </div>
      </div>

      {/* Error Banner */}
      {error && (
        <div style={errorBanner}>
          <span style={{ marginRight: 8, fontWeight: 700 }}>!</span>
          {error}
          <button onClick={() => setError('')} style={errorDismissBtn}>
            x
          </button>
        </div>
      )}

      {/* Example entries info banner */}
      {isShowingExamples && (
        <div style={infoBanner}>
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="#3b82f6"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            style={{ flexShrink: 0 }}
          >
            <circle cx="12" cy="12" r="10" />
            <path d="M12 16v-4" />
            <path d="M12 8h.01" />
          </svg>
          <span>
            These are example entries showing what audit activity looks like.
            Create secrets or promote to see real entries.
          </span>
        </div>
      )}

      {/* Search */}
      <div style={searchRow}>
        <div style={{ position: 'relative', display: 'flex', alignItems: 'center', flex: 1, maxWidth: 360 }}>
          <svg
            style={{ position: 'absolute', left: 10, width: 14, height: 14, pointerEvents: 'none' }}
            viewBox="0 0 24 24"
            fill="none"
            stroke="var(--color-text-secondary)"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.35-4.35" />
          </svg>
          <input
            type="text"
            placeholder="Filter by action or environment..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={searchInput}
          />
          {search && (
            <button onClick={() => setSearch('')} style={searchClearBtn}>
              x
            </button>
          )}
        </div>
        {search && (
          <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
            Showing {filteredEntries.length} of {displayEntries.length} entries
          </span>
        )}
      </div>

      {/* Table */}
      {filteredEntries.length === 0 ? (
        <div style={emptyCard}>
          <p style={{ fontSize: 14, fontWeight: 600, margin: '0 0 4px 0' }}>
            No matching entries
          </p>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', margin: 0 }}>
            Try a different search term or clear the filter.
          </p>
        </div>
      ) : (
        <div style={tableWrapper}>
          <table style={tableStyle}>
            <thead>
              <tr>
                <th style={thStyle}>Time</th>
                <th style={thStyle}>Action</th>
                <th style={thStyle}>Environment</th>
                <th style={thStyle}>Details</th>
                <th style={thStyle}>IP Address</th>
              </tr>
            </thead>
            <tbody>
              {filteredEntries.map((entry) => {
                const actionColor = getActionBadgeColor(entry.action);
                const isExpanded = expandedRows.has(entry.id);
                return (
                  <tr
                    key={entry.id}
                    style={{ transition: 'background 0.1s ease' }}
                    onMouseEnter={(e) => {
                      (e.currentTarget as HTMLTableRowElement).style.background =
                        'var(--color-surface-hover)';
                    }}
                    onMouseLeave={(e) => {
                      (e.currentTarget as HTMLTableRowElement).style.background = 'transparent';
                    }}
                  >
                    {/* Time */}
                    <td style={tdStyle}>
                      <span style={timeText} title={new Date(entry.created_at).toLocaleString()}>
                        {formatDate(entry.created_at)}
                      </span>
                    </td>

                    {/* Action */}
                    <td style={tdStyle}>
                      <span
                        style={{
                          ...actionBadge,
                          background: actionColor.bg,
                          color: actionColor.text,
                        }}
                      >
                        {entry.action}
                      </span>
                    </td>

                    {/* Environment */}
                    <td style={tdStyle}>
                      {entry.environment ? (
                        <span
                          style={{
                            ...envBadge,
                            background: getEnvBadgeColor(entry.environment).bg,
                            color: getEnvBadgeColor(entry.environment).text,
                          }}
                        >
                          {entry.environment.toUpperCase()}
                        </span>
                      ) : (
                        <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                          --
                        </span>
                      )}
                    </td>

                    {/* Details */}
                    <td style={tdStyle}>
                      <button onClick={() => toggleRow(entry.id)} style={detailsToggleBtn}>
                        {isExpanded ? 'Hide' : 'Details'}
                      </button>
                      {isExpanded && (
                        <pre style={detailsPre}>
                          {JSON.stringify(entry.details, null, 2)}
                        </pre>
                      )}
                    </td>

                    {/* IP Address */}
                    <td style={tdStyle}>
                      <span style={ipText}>{entry.ip_address}</span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

/* ---------- Style Constants ---------- */

const headerRow: React.CSSProperties = {
  marginBottom: 16,
};

const titleStyle: React.CSSProperties = {
  fontSize: 18,
  fontWeight: 700,
  margin: 0,
};

const countBadge: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  minWidth: 24,
  height: 22,
  borderRadius: 12,
  background: 'var(--color-primary)',
  color: '#fff',
  fontSize: 11,
  fontWeight: 700,
  padding: '0 7px',
};

const subtitleStyle: React.CSSProperties = {
  fontSize: 13,
  color: 'var(--color-text-secondary)',
  margin: '4px 0 0 0',
};

const errorBanner: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '10px 14px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
  border: '1px solid var(--color-danger)',
  fontWeight: 500,
};

const errorDismissBtn: React.CSSProperties = {
  marginLeft: 'auto',
  background: 'none',
  border: 'none',
  color: 'var(--color-danger)',
  cursor: 'pointer',
  fontSize: 14,
  fontWeight: 700,
  padding: '0 4px',
};

const infoBanner: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 10,
  background: 'rgba(59, 130, 246, 0.08)',
  border: '1px solid rgba(59, 130, 246, 0.25)',
  color: '#3b82f6',
  padding: '10px 14px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
  lineHeight: 1.5,
};

const searchRow: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
  marginBottom: 12,
};

const searchInput: React.CSSProperties = {
  width: '100%',
  padding: '8px 32px 8px 32px',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  background: 'var(--color-input-bg)',
  color: 'inherit',
  outline: 'none',
};

const searchClearBtn: React.CSSProperties = {
  position: 'absolute',
  right: 8,
  background: 'none',
  border: 'none',
  color: 'var(--color-text-secondary)',
  cursor: 'pointer',
  fontSize: 14,
  padding: '0 4px',
};

const loadingContainer: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  padding: 48,
  gap: 16,
};

const loadingSpinner: React.CSSProperties = {
  width: 24,
  height: 24,
  border: '3px solid var(--color-border)',
  borderTopColor: 'var(--color-primary)',
  borderRadius: '50%',
  animation: 'spin 0.8s linear infinite',
};

const emptyCard: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  padding: '40px 24px',
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  border: '1px solid var(--color-border)',
  textAlign: 'center',
};

const tableWrapper: React.CSSProperties = {
  borderRadius: 'var(--radius)',
  overflow: 'auto',
  boxShadow: 'var(--shadow)',
  border: '1px solid var(--color-border)',
};

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  background: 'var(--color-surface)',
};

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '10px 16px',
  fontSize: 11,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  borderBottom: '1px solid var(--color-border)',
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
  whiteSpace: 'nowrap',
};

const tdStyle: React.CSSProperties = {
  padding: '10px 16px',
  borderBottom: '1px solid var(--color-border)',
  verticalAlign: 'top',
};

const timeText: React.CSSProperties = {
  fontSize: 12,
  color: 'var(--color-text-secondary)',
  whiteSpace: 'nowrap',
};

const actionBadge: React.CSSProperties = {
  display: 'inline-block',
  padding: '2px 8px',
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 600,
  fontFamily: 'monospace',
  whiteSpace: 'nowrap',
};

const envBadge: React.CSSProperties = {
  display: 'inline-block',
  padding: '2px 8px',
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 600,
  textTransform: 'uppercase',
  letterSpacing: '0.03em',
};

const detailsToggleBtn: React.CSSProperties = {
  padding: '3px 10px',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  fontSize: 11,
  cursor: 'pointer',
  fontWeight: 500,
  transition: 'all 0.1s ease',
};

const detailsPre: React.CSSProperties = {
  marginTop: 8,
  marginBottom: 0,
  padding: '8px 12px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 12,
  fontFamily: 'monospace',
  overflow: 'auto',
  maxWidth: 400,
  whiteSpace: 'pre-wrap',
  lineHeight: 1.5,
};

const ipText: React.CSSProperties = {
  fontSize: 12,
  fontFamily: 'monospace',
  color: 'var(--color-text-secondary)',
};
