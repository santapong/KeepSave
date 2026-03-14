import { useState } from 'react';
import { promoteDiff, promote } from '../api/client';
import type { DiffEntry } from '../types';

interface PromotionWizardProps {
  projectId: string;
}

type Step = 'configure' | 'review' | 'done';

const PATHS = [
  { source: 'alpha', target: 'uat', label: 'Alpha → UAT' },
  { source: 'uat', target: 'prod', label: 'UAT → PROD' },
];

export function PromotionWizard({ projectId }: PromotionWizardProps) {
  const [step, setStep] = useState<Step>('configure');
  const [pathIdx, setPathIdx] = useState(0);
  const [overridePolicy, setOverridePolicy] = useState('skip');
  const [notes, setNotes] = useState('');
  const [diff, setDiff] = useState<DiffEntry[]>([]);
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<string>('');

  const path = PATHS[pathIdx];

  async function handlePreview() {
    setLoading(true);
    setError('');
    try {
      const entries = await promoteDiff(projectId, path.source, path.target);
      setDiff(entries);
      setSelectedKeys(new Set(entries.filter((e) => e.action !== 'no_change').map((e) => e.key)));
      setStep('review');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load diff');
    } finally {
      setLoading(false);
    }
  }

  async function handlePromote() {
    setLoading(true);
    setError('');
    try {
      const keys = selectedKeys.size > 0 ? Array.from(selectedKeys) : undefined;
      const promotion = await promote(
        projectId,
        path.source,
        path.target,
        overridePolicy,
        keys,
        notes || undefined
      );
      setResult(
        promotion.status === 'pending'
          ? 'Promotion request created. PROD promotions require approval.'
          : 'Promotion completed successfully!'
      );
      setStep('done');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Promotion failed');
    } finally {
      setLoading(false);
    }
  }

  function toggleKey(key: string) {
    setSelectedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  if (step === 'done') {
    return (
      <div style={cardStyle}>
        <div style={{ textAlign: 'center', padding: 40 }}>
          <h3 style={{ fontSize: 18, color: 'var(--color-success)', marginBottom: 8 }}>
            {result}
          </h3>
          <button
            onClick={() => { setStep('configure'); setResult(''); setDiff([]); }}
            style={btnPrimary}
          >
            New Promotion
          </button>
        </div>
      </div>
    );
  }

  return (
    <div>
      {error && <div style={errorStyle}>{error}</div>}

      {step === 'configure' && (
        <div style={cardStyle}>
          <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>Configure Promotion</h3>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Promotion Path</label>
            <div style={{ display: 'flex', gap: 8 }}>
              {PATHS.map((p, idx) => (
                <button
                  key={idx}
                  onClick={() => setPathIdx(idx)}
                  style={{
                    padding: '8px 16px',
                    borderRadius: 'var(--radius)',
                    border: '1px solid',
                    borderColor: pathIdx === idx ? 'var(--color-primary)' : 'var(--color-border)',
                    background: pathIdx === idx ? 'var(--color-primary)' : 'var(--color-surface)',
                    color: pathIdx === idx ? '#fff' : 'var(--color-text)',
                    fontSize: 13,
                    fontWeight: 600,
                  }}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Override Policy</label>
            <select
              value={overridePolicy}
              onChange={(e) => setOverridePolicy(e.target.value)}
              style={selectStyle}
            >
              <option value="skip">Skip - Don&apos;t overwrite existing keys</option>
              <option value="overwrite">Overwrite - Replace all values</option>
            </select>
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Notes (optional)</label>
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="Reason for promotion..."
              style={{ ...selectStyle, minHeight: 60, resize: 'vertical' }}
            />
          </div>

          <button onClick={handlePreview} disabled={loading} style={btnPrimary}>
            {loading ? 'Loading diff...' : 'Preview Changes'}
          </button>
        </div>
      )}

      {step === 'review' && (
        <div style={cardStyle}>
          <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
            Review Changes: {path.label}
          </h3>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginBottom: 16 }}>
            Select which keys to promote. Override policy: <strong>{overridePolicy}</strong>
          </p>

          {diff.length === 0 ? (
            <p style={{ color: 'var(--color-text-secondary)' }}>No differences found.</p>
          ) : (
            <table style={tableStyle}>
              <thead>
                <tr>
                  <th style={{ ...thStyle, width: 40 }}></th>
                  <th style={thStyle}>Key</th>
                  <th style={thStyle}>Action</th>
                  <th style={thStyle}>Source</th>
                  <th style={thStyle}>Target</th>
                </tr>
              </thead>
              <tbody>
                {diff.map((entry) => (
                  <tr key={entry.key} style={{ opacity: entry.action === 'no_change' ? 0.5 : 1 }}>
                    <td style={tdStyle}>
                      <input
                        type="checkbox"
                        checked={selectedKeys.has(entry.key)}
                        onChange={() => toggleKey(entry.key)}
                        disabled={entry.action === 'no_change'}
                      />
                    </td>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 13, fontWeight: 600 }}>{entry.key}</code>
                    </td>
                    <td style={tdStyle}>
                      <span style={{
                        ...badgeBase,
                        background: entry.action === 'add' ? 'rgba(34, 197, 94, 0.15)' : entry.action === 'update' ? 'rgba(245, 158, 11, 0.15)' : 'var(--color-input-bg)',
                        color: entry.action === 'add' ? 'var(--color-success)' : entry.action === 'update' ? 'var(--color-warning)' : 'var(--color-text-secondary)',
                      }}>
                        {entry.action}
                      </span>
                    </td>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 12 }}>{entry.source_exists ? '••••••' : '-'}</code>
                    </td>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 12 }}>{entry.target_exists ? '••••••' : '-'}</code>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
            <button onClick={() => setStep('configure')} style={btnOutline}>
              Back
            </button>
            <button
              onClick={handlePromote}
              disabled={loading || selectedKeys.size === 0}
              style={{
                ...btnPrimary,
                background: path.target === 'prod' ? 'var(--color-warning)' : 'var(--color-primary)',
              }}
            >
              {loading
                ? 'Promoting...'
                : path.target === 'prod'
                  ? 'Request PROD Promotion'
                  : `Promote to ${path.target.toUpperCase()}`}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

const cardStyle: React.CSSProperties = {
  background: 'var(--color-surface)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
  padding: 24,
};

const labelStyle: React.CSSProperties = {
  display: 'block',
  fontSize: 13,
  fontWeight: 600,
  marginBottom: 6,
};

const selectStyle: React.CSSProperties = {
  width: '100%',
  padding: '8px 12px',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 14,
};

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 600,
};

const btnOutline: React.CSSProperties = {
  padding: '8px 16px',
  background: 'transparent',
  color: 'var(--color-text)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 13,
};

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
};

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '8px 12px',
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  borderBottom: '1px solid var(--color-border)',
  textTransform: 'uppercase',
};

const tdStyle: React.CSSProperties = {
  padding: '8px 12px',
  borderBottom: '1px solid var(--color-border)',
};

const badgeBase: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 600,
  textTransform: 'uppercase',
};

const errorStyle: React.CSSProperties = {
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
};
