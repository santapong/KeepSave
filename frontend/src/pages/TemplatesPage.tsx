import { useState, useEffect } from 'react';
import type { SecretTemplate } from '../types';
import * as api from '../api/client';

export function TemplatesPage() {
  const [templates, setTemplates] = useState<SecretTemplate[]>([]);
  const [builtinTemplates, setBuiltinTemplates] = useState<SecretTemplate[]>([]);
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
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    loadTemplates();
  }, []);

  async function loadTemplates() {
    try {
      setLoading(true);
      const [custom, builtin] = await Promise.all([
        api.listTemplates(),
        api.listBuiltinTemplates(),
      ]);
      setTemplates(custom);
      setBuiltinTemplates(builtin);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load templates');
    } finally {
      setLoading(false);
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    try {
      const keys = parseKeysInput(keysText);
      await api.createTemplate(name, description, stack, { keys });
      setShowCreate(false);
      setName('');
      setDescription('');
      setStack('custom');
      setKeysText('');
      loadTemplates();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create template');
    }
  }

  async function handleDelete(templateId: string) {
    if (!confirm('Delete this template?')) return;
    try {
      await api.deleteTemplate(templateId);
      loadTemplates();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete template');
    }
  }

  async function handleApply(templateId: string) {
    if (!applyProjectId.trim()) {
      setError('Please enter a project ID');
      return;
    }
    try {
      const secrets = await api.applyTemplate(templateId, applyProjectId.trim(), applyEnv);
      setSuccess(`Applied template: ${secrets.length} secrets created`);
      setApplyingId(null);
      setApplyProjectId('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to apply template');
    }
  }

  function parseKeysInput(text: string): Array<Record<string, unknown>> {
    return text
      .split('\n')
      .filter((line) => line.trim())
      .map((line) => {
        const parts = line.split('=');
        const key = parts[0].trim();
        const defaultValue = parts.length > 1 ? parts.slice(1).join('=').trim() : '';
        return { key, default_value: defaultValue, required: true };
      });
  }

  function renderTemplateKeys(tmpl: SecretTemplate): string[] {
    const keysData = tmpl.keys?.keys;
    if (!Array.isArray(keysData)) return [];
    return keysData.map((k: Record<string, unknown>) => k.key as string).filter(Boolean);
  }

  const stackColors: Record<string, string> = {
    nodejs: '#68a063',
    python: '#3776ab',
    go: '#00add8',
    aws: '#ff9900',
    custom: '#6b7280',
  };

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1>Secret Templates</h1>
        <button
          onClick={() => setShowCreate(!showCreate)}
          style={{ padding: '8px 16px', background: 'var(--color-primary)', color: '#fff', border: 'none', borderRadius: 'var(--radius)', cursor: 'pointer' }}
        >
          {showCreate ? 'Cancel' : 'Create Template'}
        </button>
      </div>

      {error && (
        <div style={{ padding: 12, background: '#fef2f2', color: '#dc2626', borderRadius: 8, marginBottom: 16 }}>
          {error}
          <button onClick={() => setError('')} style={{ float: 'right', background: 'none', border: 'none', cursor: 'pointer' }}>x</button>
        </div>
      )}

      {success && (
        <div style={{ padding: 12, background: '#f0fdf4', color: '#16a34a', borderRadius: 8, marginBottom: 16 }}>
          {success}
          <button onClick={() => setSuccess('')} style={{ float: 'right', background: 'none', border: 'none', cursor: 'pointer' }}>x</button>
        </div>
      )}

      {showCreate && (
        <form onSubmit={handleCreate} style={{ padding: 16, border: '1px solid var(--color-border)', borderRadius: 'var(--radius)', marginBottom: 24, background: 'var(--color-surface)' }}>
          <h3 style={{ marginBottom: 12 }}>New Template</h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Template name"
              required
              style={{ padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 4 }}
            />
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Description"
              style={{ padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 4 }}
            />
            <select
              value={stack}
              onChange={(e) => setStack(e.target.value)}
              style={{ padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 4 }}
            >
              <option value="custom">Custom</option>
              <option value="nodejs">Node.js</option>
              <option value="python">Python</option>
              <option value="go">Go</option>
              <option value="aws">AWS</option>
            </select>
            <textarea
              value={keysText}
              onChange={(e) => setKeysText(e.target.value)}
              placeholder={"Keys (one per line, KEY=default_value):\nDATABASE_URL=\nPORT=3000\nLOG_LEVEL=info"}
              rows={5}
              style={{ padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 4, fontFamily: 'monospace', fontSize: 13 }}
            />
            <button
              type="submit"
              style={{ padding: '8px 16px', background: 'var(--color-primary)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', alignSelf: 'flex-start' }}
            >
              Create Template
            </button>
          </div>
        </form>
      )}

      {loading ? (
        <p>Loading...</p>
      ) : (
        <>
          <h2 style={{ marginBottom: 16, fontSize: 18 }}>Builtin Templates</h2>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 16, marginBottom: 32 }}>
            {builtinTemplates.map((tmpl, idx) => (
              <TemplateCard
                key={`builtin-${idx}`}
                template={tmpl}
                keys={renderTemplateKeys(tmpl)}
                stackColor={stackColors[tmpl.stack] || '#6b7280'}
                applyingId={applyingId}
                applyProjectId={applyProjectId}
                applyEnv={applyEnv}
                onApplyStart={() => setApplyingId(`builtin-${idx}`)}
                onApplyCancel={() => setApplyingId(null)}
                onProjectIdChange={setApplyProjectId}
                onEnvChange={setApplyEnv}
                onApply={() => handleApply(tmpl.id)}
                isBuiltin
              />
            ))}
          </div>

          <h2 style={{ marginBottom: 16, fontSize: 18 }}>Custom Templates</h2>
          {templates.length === 0 ? (
            <p style={{ color: 'var(--color-text-secondary)' }}>No custom templates yet.</p>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 16 }}>
              {templates.map((tmpl) => (
                <TemplateCard
                  key={tmpl.id}
                  template={tmpl}
                  keys={renderTemplateKeys(tmpl)}
                  stackColor={stackColors[tmpl.stack] || '#6b7280'}
                  applyingId={applyingId}
                  applyProjectId={applyProjectId}
                  applyEnv={applyEnv}
                  onApplyStart={() => setApplyingId(tmpl.id)}
                  onApplyCancel={() => setApplyingId(null)}
                  onProjectIdChange={setApplyProjectId}
                  onEnvChange={setApplyEnv}
                  onApply={() => handleApply(tmpl.id)}
                  onDelete={() => handleDelete(tmpl.id)}
                />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}

function TemplateCard({
  template,
  keys,
  stackColor,
  applyingId,
  applyProjectId,
  applyEnv,
  onApplyStart,
  onApplyCancel,
  onProjectIdChange,
  onEnvChange,
  onApply,
  onDelete,
  isBuiltin,
}: {
  template: SecretTemplate;
  keys: string[];
  stackColor: string;
  applyingId: string | null;
  applyProjectId: string;
  applyEnv: string;
  onApplyStart: () => void;
  onApplyCancel: () => void;
  onProjectIdChange: (v: string) => void;
  onEnvChange: (v: string) => void;
  onApply: () => void;
  onDelete?: () => void;
  isBuiltin?: boolean;
}) {
  const isApplying = applyingId === (isBuiltin ? `builtin-${template.name}` : template.id);

  return (
    <div style={{ padding: 16, border: '1px solid var(--color-border)', borderRadius: 'var(--radius)', background: 'var(--color-surface)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 8 }}>
        <div>
          <strong>{template.name}</strong>
          <span style={{ marginLeft: 8, padding: '2px 8px', background: stackColor, color: '#fff', borderRadius: 12, fontSize: 11 }}>
            {template.stack}
          </span>
        </div>
      </div>
      <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginBottom: 12 }}>{template.description}</p>
      <div style={{ fontSize: 12, fontFamily: 'monospace', color: 'var(--color-text-secondary)', marginBottom: 12 }}>
        {keys.slice(0, 5).map((k) => (
          <div key={k}>{k}</div>
        ))}
        {keys.length > 5 && <div>... +{keys.length - 5} more</div>}
      </div>
      <div style={{ display: 'flex', gap: 8 }}>
        {isApplying ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, width: '100%' }}>
            <input
              type="text"
              value={applyProjectId}
              onChange={(e) => onProjectIdChange(e.target.value)}
              placeholder="Project ID"
              style={{ padding: '4px 8px', border: '1px solid var(--color-border)', borderRadius: 4, fontSize: 12 }}
            />
            <select
              value={applyEnv}
              onChange={(e) => onEnvChange(e.target.value)}
              style={{ padding: '4px 8px', border: '1px solid var(--color-border)', borderRadius: 4, fontSize: 12 }}
            >
              <option value="alpha">Alpha</option>
              <option value="uat">UAT</option>
              <option value="prod">Prod</option>
            </select>
            <div style={{ display: 'flex', gap: 4 }}>
              <button onClick={onApply} style={{ flex: 1, padding: '4px 8px', background: 'var(--color-success)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>
                Confirm
              </button>
              <button onClick={onApplyCancel} style={{ flex: 1, padding: '4px 8px', background: 'var(--color-text-secondary)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <>
            <button onClick={onApplyStart} style={{ padding: '4px 12px', background: 'var(--color-primary)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>
              Apply
            </button>
            {onDelete && (
              <button onClick={onDelete} style={{ padding: '4px 12px', background: 'var(--color-danger)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>
                Delete
              </button>
            )}
          </>
        )}
      </div>
    </div>
  );
}
