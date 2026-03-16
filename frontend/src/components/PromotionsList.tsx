import { useState, useEffect, useMemo } from 'react';
import {
  listPromotions,
  approvePromotion,
  rejectPromotion,
  rollbackPromotion,
} from '../api/client';
import { formatDate } from '../utils/formatDate';
import type { PromotionRequest } from '../types';

interface PromotionsListProps {
  projectId: string;
}

type StatusFilter = 'all' | 'pending' | 'completed' | 'rejected';

const FILTERS: { key: StatusFilter; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'pending', label: 'Pending' },
  { key: 'completed', label: 'Completed' },
  { key: 'rejected', label: 'Rejected' },
];

function getLeftBorderColor(source: string, target: string): string {
  if (source === 'uat' && target === 'prod') return 'var(--color-warning)';
  return 'var(--color-primary)';
}

export function PromotionsList({ projectId }: PromotionsListProps) {
  const [promotions, setPromotions] = useState<PromotionRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState<StatusFilter>('all');

  useEffect(() => {
    loadPromotions();
  }, [projectId]); // eslint-disable-line react-hooks/exhaustive-deps

  async function loadPromotions() {
    setLoading(true);
    try {
      const data = await listPromotions(projectId);
      setPromotions(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load promotions');
    } finally {
      setLoading(false);
    }
  }

  async function handleApprove(id: string) {
    try {
      await approvePromotion(projectId, id);
      loadPromotions();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to approve');
    }
  }

  async function handleReject(id: string) {
    try {
      await rejectPromotion(projectId, id);
      loadPromotions();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reject');
    }
  }

  async function handleRollback(id: string) {
    if (!window.confirm('Rollback this promotion? Target environment secrets will be restored.')) return;
    try {
      await rollbackPromotion(projectId, id);
      loadPromotions();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to rollback');
    }
  }

  /* ---------- Counts & Filtered List ---------- */
  const counts = useMemo(() => {
    const map: Record<StatusFilter, number> = { all: promotions.length, pending: 0, completed: 0, rejected: 0 };
    for (const p of promotions) {
      if (p.status === 'pending') map.pending++;
      else if (p.status === 'completed') map.completed++;
      else if (p.status === 'rejected') map.rejected++;
    }
    return map;
  }, [promotions]);

  const filtered = useMemo(() => {
    if (filter === 'all') return promotions;
    return promotions.filter((p) => p.status === filter);
  }, [promotions, filter]);

  if (loading) return <p>Loading promotion history...</p>;

  return (
    <div>
      {error && <div style={errorStyle}>{error}</div>}

      {/* Status filter bar */}
      <div style={{ display: 'flex', gap: 8, marginBottom: 16, flexWrap: 'wrap' }}>
        {FILTERS.map((f) => {
          const active = filter === f.key;
          return (
            <button
              key={f.key}
              onClick={() => setFilter(f.key)}
              style={{
                padding: '6px 14px',
                borderRadius: 999,
                border: active ? 'none' : '1px solid var(--color-border)',
                background: active ? 'var(--color-primary)' : 'transparent',
                color: active ? '#fff' : 'var(--color-text-secondary)',
                fontSize: 13,
                fontWeight: 600,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: 6,
              }}
            >
              {f.label}
              <span
                style={{
                  background: active ? 'rgba(255,255,255,0.25)' : 'var(--color-border)',
                  color: active ? '#fff' : 'var(--color-text-secondary)',
                  padding: '1px 7px',
                  borderRadius: 999,
                  fontSize: 11,
                  fontWeight: 700,
                  lineHeight: '16px',
                }}
              >
                {counts[f.key]}
              </span>
            </button>
          );
        })}
      </div>

      {/* Result count */}
      {filter !== 'all' && (
        <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 12 }}>
          Showing {filtered.length} of {promotions.length} promotions
        </p>
      )}

      {filtered.length === 0 ? (
        <div style={emptyCardStyle}>
          <div style={{ textAlign: 'center', padding: '32px 16px' }}>
            <svg
              width="40"
              height="40"
              viewBox="0 0 24 24"
              fill="none"
              stroke="var(--color-text-secondary)"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              style={{ marginBottom: 12, opacity: 0.5 }}
            >
              <path d="M12 2L2 7l10 5 10-5-10-5z" />
              <path d="M2 17l10 5 10-5" />
              <path d="M2 12l10 5 10-5" />
            </svg>
            <p style={{ fontSize: 14, fontWeight: 600, marginBottom: 4 }}>
              {filter === 'all' ? 'No promotions yet' : `No ${filter} promotions`}
            </p>
            <p style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
              {filter === 'all'
                ? 'Promote secrets between environments to see them here.'
                : 'Try a different filter to see more results.'}
            </p>
          </div>
        </div>
      ) : (
        <div style={{ display: 'grid', gap: 12 }}>
          {filtered.map((p) => (
            <div
              key={p.id}
              style={{
                ...cardStyle,
                borderLeft: `3px solid ${getLeftBorderColor(p.source_environment, p.target_environment)}`,
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ fontSize: 14, fontWeight: 600 }}>
                      {p.source_environment.toUpperCase()} &rarr; {p.target_environment.toUpperCase()}
                    </span>
                    <span style={{
                      ...statusBadge,
                      background: statusColors[p.status]?.bg || '#f3f4f6',
                      color: statusColors[p.status]?.text || '#6b7280',
                    }}>
                      {p.status}
                    </span>
                  </div>
                  {p.notes && (
                    <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginTop: 4 }}>
                      {p.notes}
                    </p>
                  )}
                  <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 4 }}>
                    Policy: {p.override_policy} | {formatDate(p.created_at)}
                  </p>
                  {p.keys_filter && p.keys_filter.length > 0 && (
                    <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 2 }}>
                      Keys: {p.keys_filter.join(', ')}
                    </p>
                  )}
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  {p.status === 'pending' && (
                    <>
                      <button onClick={() => handleApprove(p.id)} style={btnSuccess}>Approve</button>
                      <button onClick={() => handleReject(p.id)} style={btnDanger}>Reject</button>
                    </>
                  )}
                  {p.status === 'completed' && (
                    <button onClick={() => handleRollback(p.id)} style={btnOutline}>Rollback</button>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/* ---------- Styles ---------- */

const statusColors: Record<string, { bg: string; text: string }> = {
  pending: { bg: 'rgba(245, 158, 11, 0.15)', text: '#f59e0b' },
  approved: { bg: 'rgba(99, 102, 241, 0.15)', text: '#818cf8' },
  completed: { bg: 'rgba(34, 197, 94, 0.15)', text: '#22c55e' },
  rejected: { bg: 'rgba(239, 68, 68, 0.15)', text: '#ef4444' },
};

const cardStyle: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  padding: 16,
};

const emptyCardStyle: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
};

const statusBadge: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 600,
  textTransform: 'uppercase',
};

const btnSuccess: React.CSSProperties = {
  padding: '4px 12px',
  background: 'var(--color-success)',
  color: '#fff',
  border: 'none',
  borderRadius: 4,
  fontSize: 12,
  fontWeight: 600,
  cursor: 'pointer',
};

const btnDanger: React.CSSProperties = {
  padding: '4px 12px',
  background: 'var(--color-danger)',
  color: '#fff',
  border: 'none',
  borderRadius: 4,
  fontSize: 12,
  fontWeight: 600,
  cursor: 'pointer',
};

const btnOutline: React.CSSProperties = {
  padding: '4px 12px',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  fontSize: 12,
  cursor: 'pointer',
};

const errorStyle: React.CSSProperties = {
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
};
