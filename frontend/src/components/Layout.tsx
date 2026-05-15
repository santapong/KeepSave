import { type ReactNode, useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useSidebar } from '@/hooks/useSidebar';
import { Sidebar } from './Sidebar';
import { CommandPalette } from './CommandPalette';
import type { User } from '../types';

interface LayoutProps {
  user: User | null;
  onLogout: () => void;
  children: ReactNode;
}

const ROUTE_LABELS: Record<string, string> = {
  '': 'projects',
  organizations: 'organizations',
  templates: 'templates',
  'mcp-hub': 'mcp hub',
  'oauth-clients': 'oauth',
  applications: 'applications',
  admin: 'dashboard',
  help: 'docs',
  projects: 'projects',
  ai: 'ai intelligence',
  settings: 'settings',
};

export function Layout({ user, onLogout, children }: LayoutProps) {
  const { collapsed, toggle } = useSidebar();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const location = useLocation();

  // Cmd+K / Ctrl+K opens the command palette.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      // Ignore modifier-less or text-input-only contexts; we want the global
      // shortcut to fire even when an input is focused (standard palette UX).
      if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault();
        setPaletteOpen((open) => !open);
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const segments = location.pathname.split('/').filter(Boolean);
  const now = segments.length === 0
    ? 'dossier'
    : segments.map((s) => ROUTE_LABELS[s] || decodeURIComponent(s)).join(' / ');

  return (
    <div className="ks-shell">
      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 sm:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      <Sidebar
        user={user}
        collapsed={collapsed}
        onToggle={toggle}
        onLogout={onLogout}
      />

      <div className="ks-main">
        <div className="ks-topbar">
          <div className="ks-crumbs">
            <span className="ks-faint">acme-platform</span>
            <span className="ks-sep">/</span>
            <span className="ks-faint">vault</span>
            <span className="ks-sep">/</span>
            <span className="ks-now">{now}</span>
          </div>
          <button
            type="button"
            className="ks-search"
            onClick={() => setPaletteOpen(true)}
            aria-label="Open command palette"
            style={{
              cursor: 'pointer',
              background: 'transparent',
              border: 'inherit',
              font: 'inherit',
              color: 'inherit',
              textAlign: 'left',
            }}
          >
            <span className="ks-faint">⌕</span>
            <span className="ks-faint" style={{ fontSize: 12, flex: 1 }}>
              Search secrets, projects, agents, leases…
            </span>
            <span className="ks-k">
              <span className="ks-kbd">⌘</span>
              <span className="ks-kbd">K</span>
            </span>
          </button>
          <div className="ks-right">
            <span className="ks-pill"><span className="ks-dot ks-dot-go" /> 18.4k rps</span>
            <span className="ks-pill ks-pill-amber">SEALED</span>
          </div>
        </div>

        <main style={{ flex: 1, overflowY: 'auto', minWidth: 0, padding: 0 }}>
          {children}
        </main>

        <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} />

        {/* Mobile menu trigger */}
        <button
          onClick={() => setMobileOpen((p) => !p)}
          aria-label="Open menu"
          className="sm:hidden"
          style={{
            position: 'fixed',
            bottom: 16,
            right: 16,
            width: 44,
            height: 44,
            background: 'var(--ks-amber)',
            color: '#140d00',
            border: '1px solid var(--ks-amber)',
            zIndex: 60,
            cursor: 'pointer',
            fontFamily: 'var(--ks-mono)',
            fontWeight: 600,
          }}
        >
          ≡
        </button>
      </div>
    </div>
  );
}
