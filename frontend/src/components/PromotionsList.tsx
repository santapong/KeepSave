import { useState, useEffect } from 'react';
import {
  listPromotions,
  approvePromotion,
  rejectPromotion,
  rollbackPromotion,
} from '../api/client';
import type { PromotionRequest } from '../types';

interface PromotionsListProps {
  projectId: string;
}

export function PromotionsList({ projectId }: PromotionsListProps) {
  const [promotions, setPromotions] = useState<PromotionRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

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

  if (loading) return <p>Loading promotion history...</p>;

  return (
    <div>
      {error && <div style={errorStyle}>{error}</div>}
      {promotions.length === 0 ? (
        <p style={{ color: 'var(--color-text-secondary)' }}>No promotions yet.</p>
      ) : (
        <div style={{ display: 'grid', gap: 12 }}>
          {promotions.map((p) => (
            <div key={p.id} style={cardStyle}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ fontSize: 14, fontWeight: 600 }}>
                      {p.source_environment.toUpperCase()} → {p.target_environment.toUpperCase()}
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
                    Policy: {p.override_policy} | {new Date(p.created_at).toLocaleString()}
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

const statusColors: Record<string, { bg: string; text: string }> = {
  pending: { bg: '#fef3c7', text: '#92400e' },
  approved: { bg: '#dbeafe', text: '#1e40af' },
  completed: { bg: '#dcfce7', text: '#166534' },
  rejected: { bg: '#fef2f2', text: '#991b1b' },
};

const cardStyle: React.CSSProperties = {
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  padding: 16,
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
};

const btnDanger: React.CSSProperties = {
  padding: '4px 12px',
  background: 'var(--color-danger)',
  color: '#fff',
  border: 'none',
  borderRadius: 4,
  fontSize: 12,
  fontWeight: 600,
};

const btnOutline: React.CSSProperties = {
  padding: '4px 12px',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  fontSize: 12,
};

const errorStyle: React.CSSProperties = {
  background: '#fef2f2',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
};
