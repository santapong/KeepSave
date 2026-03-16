import { useState, useEffect, type FormEvent } from 'react';
import type { SecretTemplate, Project } from '../types';
import * as api from '../api/client';

export function TemplatesPage() {
  const [templates, setTemplates] = useState<SecretTemplate[]>([]);
  const [builtinTemplates, setBuiltinTemplates] = useState<SecretTemplate[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [stack, setStack] = useState('custom');
  const [keysText, setKeysText] = useState('');
  const [applyProjectId, setApplyProjectId] = useState('');
  const [applyEnv, setApplyEnv] = useState('alpha');
  const [applyingId, setApplyingId] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, []);

  async function loadData() {
    try {
      setLoading(true);
      const [custom, builtin, projectList] = await Promise.all([
        api.listTemplates(),
        api.listBuiltinTemplates(),
        api.listProjects(),
      ]);
      setTemplates(custom);
      setBuiltinTemplates(builtin);
      setProjects(projectList);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load templates');
    } finally {
      setLoading(false);
    }
  }

  function parseKeysInput(text: string): Record<string, unknown> {
    const result: Record<string, unknown> = {};
    text
      .split('\n')
      .filter((line) => line.trim())
      .forEach((line) => {
        const eqIndex = line.indexOf('=');
        let key: string;
        let defaultValue: string;
        if (eqIndex === -1) {
          key = line.trim();
          defaultValue = '';
        } else {
          key = line.substring(0, eqIndex).trim();
          defaultValue = line.substring(eqIndex + 1).trim();
        }
        if (key) {
          result[key] = { default_value: defaultValue, required: true };
        }
      });
    return result;
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    try {
      const keys = parseKeysInput(keysText);
      await api.createTemplate(name, description, stack, keys);
      setShowCreate(false);
      setName('');
      setDescription('');
      setStack('custom');
      setKeysText('');
      setSuccess('Template created successfully');
      loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create template');
    }
  }

  async function handleDelete(templateId: string) {
    if (!window.confirm('Delete this template? This cannot be undone.')) return;
    try {
      await api.deleteTemplate(templateId);
      setSuccess('Template deleted');
      loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete template');
    }
  }

  async function handleApply(templateId: string) {
    if (!applyProjectId) {
      setError('Please select a project');
      return;
    }
    try {
      const secrets = await api.applyTemplate(templateId, applyProjectId, applyEnv);
      setSuccess(`Applied template: ${secrets.length} secret${secrets.length !== 1 ? 's' : ''} created`);
      setApplyingId(null);
      setApplyProjectId('');
      setApplyEnv('alpha');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to apply template');
    }
  }

  function getTemplateKeyNames(tmpl: SecretTemplate): string[] {
    return Object.keys(tmpl.keys || {});
  }

  const totalCount = builtinTemplates.length + templates.length;

  if (loading) {
    return <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-secondary)' }}>Loading templates...</div>;
  }

  return (
    <div>
      {/* Header */}
      <div style={headerRow}>
        <div>
          <h1 style={{ fontSize: 24, fontWeight: 700 }}>Secret Templates</h1>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginTop: 2 }}>
            {totalCount} template{totalCount !== 1 ? 's' : ''}
          </p>
        </div>
        <button onClick={() => setShowCreate(!showCreate)} style={btnPrimary}>
          {showCreate ? 'Cancel' : '+ New Template'}
        </button>
      </div>

      {/* Error alert */}
      {error && (
        <div style={errorStyle}>
          {error}
          <button
            onClick={() => setError('')}
            style={{ float: 'right', background: 'none', border: 'none', color: 'inherit', cursor: 'pointer', fontWeight: 600, fontSize: 14 }}
          >
            x
          </button>
        </div>
      )}

      {/* Success alert */}
      {success && (
        <div style={successStyle}>
          {success}
          <button
            onClick={() => setSuccess('')}
            style={{ float: 'right', background: 'none', border: 'none', color: 'inherit', cursor: 'pointer', fontWeight: 600, fontSize: 14 }}
          >
            x
          </button>
        </div>
      )}

      {/* Create form */}
      {showCreate && (
        <form onSubmit={handleCreate} style={createForm}>
          <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 12 }}>Create Template</h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Template name"
                required
                style={{ ...inputStyle, flex: 1, minWidth: 200 }}
              />
              <select
                value={stack}
                onChange={(e) => setStack(e.target.value)}
                style={{ ...inputStyle, flex: 0, minWidth: 140 }}
              >
                <option value="custom">Custom</option>
                <option value="nodejs">Node.js</option>
                <option value="python">Python</option>
                <option value="go">Go</option>
                <option value="aws">AWS</option>
              </select>
            </div>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Description (optional)"
              style={inputStyle}
            />
            <textarea
              value={keysText}
              onChange={(e) => setKeysText(e.target.value)}
              placeholder={"Keys (one per line, KEY=default_value):\nDATABASE_URL=\nPORT=3000\nLOG_LEVEL=info"}
              rows={5}
              style={{ ...inputStyle, fontFamily: 'monospace', fontSize: 13, resize: 'vertical' }}
            />
            <button type="submit" style={{ ...btnPrimary, alignSelf: 'flex-start' }}>
              Create Template
            </button>
          </div>
        </form>
      )}

      {/* Builtin Templates */}
      <h2 style={sectionTitle}>Builtin Templates</h2>
      {builtinTemplates.length === 0 ? (
        <p style={{ color: 'var(--color-text-secondary)', fontSize: 13, marginBottom: 24 }}>No builtin templates available.</p>
      ) : (
        <div style={cardGrid}>
          {builtinTemplates.map((tmpl, idx) => {
            const cardId = `builtin-${idx}`;
            return (
              <TemplateCard
                key={cardId}
                template={tmpl}
                cardId={cardId}
                keys={getTemplateKeyNames(tmpl)}
                stackColor={stackColors[tmpl.stack] || stackColors.custom}
                applyingId={applyingId}
                applyProjectId={applyProjectId}
                applyEnv={applyEnv}
                projects={projects}
                onApplyStart={() => { setApplyingId(cardId); setApplyProjectId(''); setApplyEnv('alpha'); }}
                onApplyCancel={() => setApplyingId(null)}
                onProjectIdChange={setApplyProjectId}
                onEnvChange={setApplyEnv}
                onApply={() => handleApply(tmpl.id)}
                isBuiltin
              />
            );
          })}
        </div>
      )}

      {/* Custom Templates */}
      <h2 style={sectionTitle}>Custom Templates</h2>
      {templates.length === 0 ? (
        <div style={emptyState}>
          <p style={{ fontSize: 16, fontWeight: 500, marginBottom: 4 }}>No custom templates yet</p>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
            Create a template to quickly apply a set of secret keys to any project.
          </p>
        </div>
      ) : (
        <div style={cardGrid}>
          {templates.map((tmpl) => (
            <TemplateCard
              key={tmpl.id}
              template={tmpl}
              cardId={tmpl.id}
              keys={getTemplateKeyNames(tmpl)}
              stackColor={stackColors[tmpl.stack] || stackColors.custom}
              applyingId={applyingId}
              applyProjectId={applyProjectId}
              applyEnv={applyEnv}
              projects={projects}
              onApplyStart={() => { setApplyingId(tmpl.id); setApplyProjectId(''); setApplyEnv('alpha'); }}
              onApplyCancel={() => setApplyingId(null)}
              onProjectIdChange={setApplyProjectId}
              onEnvChange={setApplyEnv}
              onApply={() => handleApply(tmpl.id)}
              onDelete={() => handleDelete(tmpl.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* ───────────────────── TemplateCard Component ───────────────────── */

function TemplateCard({
  template,
  cardId,
  keys,
  stackColor,
  applyingId,
  applyProjectId,
  applyEnv,
  projects,
  onApplyStart,
  onApplyCancel,
  onProjectIdChange,
  onEnvChange,
  onApply,
  onDelete,
  isBuiltin,
}: {
  template: SecretTemplate;
  cardId: string;
  keys: string[];
  stackColor: string;
  applyingId: string | null;
  applyProjectId: string;
  applyEnv: string;
  projects: Project[];
  onApplyStart: () => void;
  onApplyCancel: () => void;
  onProjectIdChange: (v: string) => void;
  onEnvChange: (v: string) => void;
  onApply: () => void;
  onDelete?: () => void;
  isBuiltin?: boolean;
}) {
  const isApplying = applyingId === cardId;

  return (
    <div style={templateCard}>
      {/* Header: name + stack badge */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 8 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <strong style={{ fontSize: 15 }}>{template.name}</strong>
          <span style={{ ...stackBadge, background: stackColor }}>{template.stack}</span>
          {isBuiltin && <span style={builtinBadge}>builtin</span>}
        </div>
      </div>

      {/* Description */}
      {template.description && (
        <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginBottom: 12, lineHeight: 1.4 }}>
          {template.description}
        </p>
      )}

      {/* Keys list */}
      <div style={{ fontSize: 12, fontFamily: 'monospace', color: 'var(--color-text-secondary)', marginBottom: 12 }}>
        {keys.length === 0 ? (
          <span style={{ fontStyle: 'italic', fontFamily: 'inherit' }}>No keys defined</span>
        ) : (
          <>
            {keys.slice(0, 5).map((k) => (
              <div key={k} style={keyTag}>{k}</div>
            ))}
            {keys.length > 5 && (
              <div style={{ marginTop: 4, fontSize: 11, fontFamily: 'inherit' }}>
                +{keys.length - 5} more key{keys.length - 5 !== 1 ? 's' : ''}
              </div>
            )}
          </>
        )}
      </div>

      {/* Actions */}
      <div style={{ marginTop: 'auto' }}>
        {isApplying ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <select
              value={applyProjectId}
              onChange={(e) => onProjectIdChange(e.target.value)}
              style={selectSmall}
            >
              <option value="">-- Select project --</option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
            <select
              value={applyEnv}
              onChange={(e) => onEnvChange(e.target.value)}
              style={selectSmall}
            >
              <option value="alpha">Alpha</option>
              <option value="uat">UAT</option>
              <option value="prod">Prod</option>
            </select>
            <div style={{ display: 'flex', gap: 6 }}>
              <button onClick={onApply} style={btnConfirm}>
                Confirm
              </button>
              <button onClick={onApplyCancel} style={btnCancel}>
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <div style={{ display: 'flex', gap: 8 }}>
            <button onClick={onApplyStart} style={btnApply}>
              Apply
            </button>
            {onDelete && (
              <button onClick={onDelete} style={btnDanger}>
                Delete
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

/* ───────────────────── Style Constants ───────────────────── */

const stackColors: Record<string, string> = {
  nodejs: '#68a063',
  python: '#3776ab',
  go: '#00add8',
  aws: '#ff9900',
  custom: '#6b7280',
};

const headerRow: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  marginBottom: 24,
};

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 600,
  cursor: 'pointer',
};

const btnApply: React.CSSProperties = {
  padding: '6px 14px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 12,
  fontWeight: 600,
  cursor: 'pointer',
};

const btnDanger: React.CSSProperties = {
  padding: '6px 12px',
  background: 'transparent',
  color: 'var(--color-danger)',
  border: '1px solid rgba(239, 68, 68, 0.3)',
  borderRadius: 'var(--radius)',
  fontSize: 12,
  cursor: 'pointer',
};

const btnConfirm: React.CSSProperties = {
  flex: 1,
  padding: '6px 10px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 12,
  fontWeight: 600,
  cursor: 'pointer',
};

const btnCancel: React.CSSProperties = {
  flex: 1,
  padding: '6px 10px',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 12,
  cursor: 'pointer',
};

const templateCard: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: 20,
  display: 'flex',
  flexDirection: 'column',
  boxShadow: 'var(--shadow)',
};

const cardGrid: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
  gap: 16,
  marginBottom: 32,
};

const sectionTitle: React.CSSProperties = {
  fontSize: 16,
  fontWeight: 600,
  marginBottom: 12,
};

const stackBadge: React.CSSProperties = {
  padding: '2px 10px',
  color: '#fff',
  borderRadius: 12,
  fontSize: 11,
  fontWeight: 600,
  whiteSpace: 'nowrap',
};

const builtinBadge: React.CSSProperties = {
  padding: '2px 8px',
  background: 'var(--color-input-bg)',
  color: 'var(--color-text-secondary)',
  border: '1px solid var(--color-border)',
  borderRadius: 12,
  fontSize: 10,
  fontWeight: 500,
};

const keyTag: React.CSSProperties = {
  display: 'inline-block',
  padding: '1px 6px',
  margin: '2px 4px 2px 0',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  fontSize: 11,
};

const selectSmall: React.CSSProperties = {
  padding: '6px 10px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 12,
  color: 'var(--color-text)',
};

const createForm: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: 20,
  marginBottom: 24,
};

const inputStyle: React.CSSProperties = {
  padding: '8px 12px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 14,
  color: 'var(--color-text)',
};

const emptyState: React.CSSProperties = {
  textAlign: 'center',
  padding: 60,
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
};

const errorStyle: React.CSSProperties = {
  background: 'var(--color-error-bg)',
  color: 'var(--color-danger)',
  padding: '10px 14px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
  border: '1px solid rgba(239, 68, 68, 0.2)',
};

const successStyle: React.CSSProperties = {
  background: 'rgba(34, 197, 94, 0.1)',
  color: '#22c55e',
  padding: '10px 14px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
  border: '1px solid rgba(34, 197, 94, 0.2)',
};
