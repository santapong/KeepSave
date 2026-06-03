import { useEffect, useRef, useState, useMemo, type ComponentType } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Search,
  FolderClosed,
  Building2,
  Boxes,
  Server,
  ShieldCheck,
  AppWindow,
  Bot,
  LayoutGrid,
  BookOpen,
  CornerDownLeft,
} from 'lucide-react';

type IconType = ComponentType<{ size?: number; className?: string }>;

interface CommandAction {
  id: string;
  label: string;
  hint?: string;
  path: string;
  icon: IconType;
}

const ACTIONS: CommandAction[] = [
  { id: 'projects', label: 'Go to Projects', hint: 'shelf · the dossier of vaults', path: '/', icon: FolderClosed },
  { id: 'organizations', label: 'Go to Organizations', hint: 'tenancy', path: '/organizations', icon: Building2 },
  { id: 'templates', label: 'Go to Templates', hint: 'stack starter kits', path: '/templates', icon: Boxes },
  { id: 'mcp-hub', label: 'Go to MCP Hub', hint: 'agent gateway', path: '/mcp-hub', icon: Server },
  { id: 'oauth', label: 'Go to OAuth Clients', hint: 'integrations', path: '/oauth-clients', icon: ShieldCheck },
  { id: 'applications', label: 'Go to Applications', hint: 'app dashboard', path: '/applications', icon: AppWindow },
  { id: 'ai', label: 'Go to AI Intelligence', hint: 'drift · anomaly · usage', path: '/ai', icon: Bot },
  { id: 'admin', label: 'Go to Admin Dashboard', hint: 'observability · audit', path: '/admin', icon: LayoutGrid },
  { id: 'help', label: 'Go to Docs', hint: 'embed widget · integration', path: '/help', icon: BookOpen },
];

interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Cmd+K / Ctrl+K command palette — Event Horizon glass panel.
 *
 * Esc or click-outside closes. Up/Down move selection; Enter activates.
 * Behaviour is unchanged from the prior implementation; only the chrome
 * is re-skinned to the cosmic ⌘K spec (.cz-cmdk*).
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
      className="cz-cmdk-back"
      role="dialog"
      aria-modal="true"
      aria-label="Command palette"
      onMouseDown={(e) => {
        // click-outside closes
        if (e.target === e.currentTarget) onClose();
      }}
      onKeyDown={handleKey}
    >
      <div className="cz-cmdk">
        <div className="cz-cmdk-head">
          <Search size={18} className="cz-faint" />
          <input
            ref={inputRef}
            className="cz-cmdk-input"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search or jump to…"
            spellCheck={false}
            autoComplete="off"
          />
          <span className="cz-kbd">ESC</span>
        </div>
        <div className="cz-cmdk-list" role="listbox">
          {filtered.length === 0 ? (
            <div className="cz-cmdk-empty">No actions match &quot;{query}&quot;.</div>
          ) : (
            <>
              <div className="cz-cmdk-section">Jump to</div>
              {filtered.map((action, i) => {
                const isSelected = i === selected;
                const Icon = action.icon;
                return (
                  <button
                    key={action.id}
                    role="option"
                    aria-selected={isSelected}
                    type="button"
                    className={`cz-cmdk-item${isSelected ? ' cz-sel' : ''}`}
                    onMouseEnter={() => setSelected(i)}
                    onClick={() => activate(action)}
                  >
                    <Icon size={16} className="cz-faint" />
                    <span>{action.label}</span>
                    {isSelected ? (
                      <CornerDownLeft size={14} className="cz-sp" />
                    ) : (
                      action.hint && <span className="cz-sp">{action.hint}</span>
                    )}
                  </button>
                );
              })}
            </>
          )}
        </div>
      </div>
    </div>
  );
}
