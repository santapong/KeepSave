import { useState, useEffect } from 'react';
import { listAuditLog } from '../api/client';
import type { AuditEntry } from '../types';

interface AuditLogViewerProps {
  projectId: string;
}

export function AuditLogViewer({ projectId }: AuditLogViewerProps) {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(true);
    listAuditLog(projectId)
      .then(setEntries)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load audit log'))
      .finally(() => setLoading(false));
  }, [projectId]);

  if (loading) return <p>Loading audit log...</p>;
  if (error) return <div style={errorStyle}>{error}</div>;

  if (entries.length === 0) {
    return <p style={{ color: 'var(--color-text-secondary)' }}>No audit entries yet.</p>;
  }

  return (
    <table style={tableStyle}>
      <thead>
        <tr>
          <th style={thStyle}>Timestamp</th>
          <th style={thStyle}>Action</th>
          <th style={thStyle}>Environment</th>
          <th style={thStyle}>Details</th>
          <th style={thStyle}>IP Address</th>
        </tr>
      </thead>
      <tbody>
        {entries.map((entry) => (
          <tr key={entry.id}>
            <td style={tdStyle}>
              <span style={{ fontSize: 12 }}>{new Date(entry.created_at).toLocaleString()}</span>
            </td>
            <td style={tdStyle}>
              <code style={{ fontSize: 12, fontWeight: 600 }}>{entry.action}</code>
            </td>
            <td style={tdStyle}>
              {entry.environment && (
                <span style={envBadge}>{entry.environment.toUpperCase()}</span>
              )}
            </td>
            <td style={tdStyle}>
              <pre style={{ fontSize: 11, margin: 0, whiteSpace: 'pre-wrap', maxWidth: 400 }}>
                {JSON.stringify(entry.details, null, 2)}
              </pre>
            </td>
            <td style={tdStyle}>
              <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                {entry.ip_address}
              </span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
};

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '10px 12px',
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  borderBottom: '1px solid var(--color-border)',
  textTransform: 'uppercase',
};

const tdStyle: React.CSSProperties = {
  padding: '8px 12px',
  borderBottom: '1px solid var(--color-border)',
  verticalAlign: 'top',
};

const envBadge: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 600,
  background: 'rgba(99, 102, 241, 0.15)',
  color: '#818cf8',
  textTransform: 'uppercase',
};

const errorStyle: React.CSSProperties = {
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
};
