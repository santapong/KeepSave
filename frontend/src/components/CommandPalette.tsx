import { useEffect, useRef, useState, useMemo, type ComponentType } from 'react';
import { useNavigate } from 'react-router-dom';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from './ui/dialog';
import {
  Search,
  FolderClosed,
  Building2,
  Boxes,
  OrbitHub,
  KeyRound,
  AppWindow,
  Bot,
  LayoutGrid,
  BookOpen,
  CornerDownLeft,
  UserRound,
} from '@/components/icons';

import { useCapabilities, routeCapability } from '../hooks/useCapabilities';

type IconType = ComponentType<{ size?: number; className?: string }>;

interface CommandAction {
  id: string;
  label: string;
  hint?: string;
  path: string;
  icon: IconType;
}

const ACTIONS: CommandAction[] = [
  { id: 'new-project', label: 'Create a new project', hint: 'a vault for your application', path: '/?new=project', icon: FolderClosed },
  { id: 'account', label: 'Go to Account', hint: 'GitHub · Google · sign-in connections', path: '/account', icon: UserRound },
  { id: 'projects', label: 'Go to Projects', hint: 'project vaults · environments', path: '/', icon: FolderClosed },
  { id: 'organizations', label: 'Go to Organizations', hint: 'teams and members', path: '/organizations', icon: Building2 },
  { id: 'templates', label: 'Go to Templates', hint: 'reusable secret templates', path: '/templates', icon: Boxes },
  { id: 'mcp-hub', label: 'Go to MCP Hub', hint: 'agent gateway', path: '/mcp-hub', icon: OrbitHub },
  { id: 'oauth', label: 'Go to OAuth Clients', hint: 'integrations', path: '/oauth-clients', icon: KeyRound },
  { id: 'applications', label: 'Go to Applications', hint: 'app dashboard', path: '/applications', icon: AppWindow },
  { id: 'ai', label: 'Go to AI Intelligence', hint: 'drift · anomaly · usage', path: '/ai', icon: Bot },
  { id: 'admin', label: 'Go to Admin Dashboard', hint: 'observability · audit', path: '/admin', icon: LayoutGrid },
  { id: 'help', label: 'Go to Docs', hint: 'embed widget · integration', path: '/help', icon: BookOpen },
];

interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
  onRestoreFocus?: () => void;
}

/**
 * Searchable workspace navigation. The shared dialog traps focus and restores it
 * on close; arrow keys and Enter select an action from the search input.
 */
export function CommandPalette({ open, onClose, onRestoreFocus }: CommandPaletteProps) {
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const navigate = useNavigate();
  const { capabilities } = useCapabilities();

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const available = ACTIONS.filter((action) => { const feature = routeCapability(action.path); return !feature || (!!capabilities && (!capabilities.restricted_profile || !capabilities.unavailable.includes(feature))); });
    if (!q) return available;
    return available.filter(
      (a) =>
        a.label.toLowerCase().includes(q) ||
        (a.hint ?? '').toLowerCase().includes(q) ||
        a.id.toLowerCase().includes(q),
    );
  }, [query, capabilities]);

  useEffect(() => {
    if (open) {
      setQuery('');
      setSelected(0);
      // defer focus so the input has mounted
      const timer = setTimeout(() => inputRef.current?.focus(), 30);
      return () => clearTimeout(timer);
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
    if (e.target !== inputRef.current) return;
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
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="cz-cmdk ks-command-dialog" onKeyDown={handleKey} onCloseAutoFocus={(event) => { if (onRestoreFocus) { event.preventDefault(); onRestoreFocus(); } }}>
        <DialogTitle className="sr-only">Command palette</DialogTitle>
        <DialogDescription className="sr-only">Find a workspace page or create a project. Use arrow keys to choose and Enter to open.</DialogDescription>
        <div className="cz-cmdk-head">
          <Search size={18} className="cz-faint" />
          <input
            ref={inputRef}
            role="combobox"
            aria-label="Search workspace commands"
            aria-expanded="true"
            aria-controls="ks-command-options"
            aria-activedescendant={filtered[selected] ? `ks-command-${filtered[selected].id}` : undefined}
            className="cz-cmdk-input"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search or jump to…"
            spellCheck={false}
            autoComplete="off"
          />
        </div>
        <div className="cz-cmdk-list" role="listbox" id="ks-command-options" aria-label="Workspace actions">
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
                    id={`ks-command-${action.id}`}
                    tabIndex={-1}
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
      </DialogContent>
    </Dialog>
  );
}
