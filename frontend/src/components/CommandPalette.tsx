import { useEffect, useRef, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';

interface CommandAction {
  id: string;
  label: string;
  hint?: string;
  path: string;
}

const ACTIONS: CommandAction[] = [
  { id: 'projects', label: 'Go to Projects', hint: 'shelf · the dossier of vaults', path: '/' },
  { id: 'organizations', label: 'Go to Organizations', hint: 'tenancy', path: '/organizations' },
  { id: 'templates', label: 'Go to Templates', hint: 'stack starter kits', path: '/templates' },
  { id: 'mcp-hub', label: 'Go to MCP Hub', hint: 'agent gateway', path: '/mcp-hub' },
  { id: 'oauth', label: 'Go to OAuth Clients', hint: 'integrations', path: '/oauth-clients' },
  { id: 'applications', label: 'Go to Applications', hint: 'app dashboard', path: '/applications' },
  { id: 'ai', label: 'Go to AI Intelligence', hint: 'drift · anomaly · usage', path: '/ai' },
  { id: 'admin', label: 'Go to Admin Dashboard', hint: 'observability · audit', path: '/admin' },
  { id: 'help', label: 'Go to Docs', hint: 'embed widget · integration', path: '/help' },
];

interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Minimum-viable Cmd+K / Ctrl+K command palette.
 *
 * Wires the previously-decorative chrome in Layout.tsx (the "⌘ K" badge) to an
 * actual searchable navigation menu. Esc or click-outside closes. Up/Down
 * arrows move selection; Enter activates.
 */
export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const navigate = useNavigate();

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return ACTIONS;
    return ACTIONS.filter(
      (a) =>
        a.label.toLowerCase().includes(q) ||
        (a.hint ?? '').toLowerCase().includes(q) ||
        a.id.toLowerCase().includes(q),
    );
  }, [query]);

  useEffect(() => {
    if (open) {
      setQuery('');
      setSelected(0);
      // defer focus so the input has mounted
      setTimeout(() => inputRef.current?.focus(), 30);
    }
  }, [open]);

  useEffect(() => {
    // Keep selection in range as the filter changes.
    if (selected >= filtered.length) setSelected(Math.max(0, filtered.length - 1));
  }, [filtered, selected]);

  if (!open) return null;

  function activate(action: CommandAction) {
    onClose();
    navigate(action.path);
  }

  function handleKey(e: React.KeyboardEvent<HTMLDivElement>) {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelected((s) => (filtered.length === 0 ? 0 : (s + 1) % filtered.length));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelected((s) => (filtered.length === 0 ? 0 : (s - 1 + filtered.length) % filtered.length));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const action = filtered[selected];
      if (action) activate(action);
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Command palette"
      onMouseDown={(e) => {
        // click-outside closes
        if (e.target === e.currentTarget) onClose();
      }}
      onKeyDown={handleKey}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'oklch(0.12 0.008 60 / 0.75)',
        backdropFilter: 'blur(6px)',
        zIndex: 400,
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'flex-start',
        paddingTop: '14vh',
      }}
    >
      <div
        style={{
          width: 'min(560px, 92vw)',
          background: 'var(--ks-bg, #1a1a1a)',
          border: '1px solid var(--ks-amber-dim, rgba(255,193,7,0.25))',
          boxShadow: '0 40px 80px oklch(0 0 0 / 0.6)',
          padding: 0,
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '14px 16px', borderBottom: '1px solid var(--ks-border, #2a2a2a)' }}>
          <span className="ks-faint" style={{ fontSize: 14 }}>⌕</span>
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Type to filter actions…"
            spellCheck={false}
            autoComplete="off"
            style={{
              flex: 1,
              background: 'transparent',
              border: 'none',
              outline: 'none',
              color: 'inherit',
              fontFamily: 'var(--ks-mono, monospace)',
              fontSize: 13,
            }}
          />
          <span className="ks-faint" style={{ fontSize: 10, letterSpacing: '0.14em', textTransform: 'uppercase' }}>
            esc to close
          </span>
        </div>
        <div role="listbox" style={{ maxHeight: '50vh', overflowY: 'auto' }}>
          {filtered.length === 0 ? (
            <div className="ks-faint" style={{ padding: '20px 16px', fontSize: 12 }}>
              No actions match &quot;{query}&quot;.
            </div>
          ) : (
            filtered.map((action, i) => {
              const isSelected = i === selected;
              return (
                <button
                  key={action.id}
                  role="option"
                  aria-selected={isSelected}
                  type="button"
                  onMouseEnter={() => setSelected(i)}
                  onClick={() => activate(action)}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    background: isSelected ? 'var(--ks-amber-soft, rgba(255,193,7,0.10))' : 'transparent',
                    border: 'none',
                    borderLeft: isSelected ? '2px solid var(--ks-amber, #ffc107)' : '2px solid transparent',
                    padding: '10px 14px',
                    cursor: 'pointer',
                    color: 'inherit',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 2,
                  }}
                >
                  <span style={{ fontSize: 13, fontWeight: 500 }}>{action.label}</span>
                  {action.hint && (
                    <span className="ks-faint" style={{ fontSize: 11 }}>{action.hint}</span>
                  )}
                </button>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}
