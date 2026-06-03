import { type ReactNode, useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Search, Plus, Menu } from 'lucide-react';
import { useSidebar } from '@/hooks/useSidebar';
import { Sidebar } from './Sidebar';
import { CommandPalette } from './CommandPalette';
import { Starfield } from './cosmic/Starfield';
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
      // Fire even when an input is focused (standard palette UX).
      if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault();
        setPaletteOpen((open) => !open);
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const segments = location.pathname.split('/').filter(Boolean);
  const now =
    segments.length === 0
      ? 'projects'
      : segments.map((s) => ROUTE_LABELS[s] || decodeURIComponent(s)).join(' / ');

  return (
    <div className="cz-shell">
      <Starfield />

      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 sm:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      <Sidebar user={user} collapsed={collapsed} onToggle={toggle} onLogout={onLogout} />

      <div className="cz-main">
        <div className="cz-topbar">
          <div className="cz-crumbs">
            <span className="cz-faint">acme-platform</span>
            <span className="cz-sep">/</span>
            <span className="cz-now">{now}</span>
          </div>

          <button
            type="button"
            className="cz-searchbar"
            onClick={() => setPaletteOpen(true)}
            aria-label="Open command palette"
          >
            <Search size={15} className="cz-faint" />
            <span className="cz-ph">Search secrets, projects, agents, leases…</span>
            <span style={{ display: 'flex', gap: 4 }}>
              <span className="cz-kbd">⌘</span>
              <span className="cz-kbd">K</span>
            </span>
          </button>

          <div className="cz-topbar-right">
            <span className="cz-pill cz-pill-go">
              <span className="cz-dot cz-dot-go" /> Operational
            </span>
            <Link to="/" className="cz-btn cz-btn-primary" title="New project">
              <Plus size={15} /> New
            </Link>
          </div>
        </div>

        <main style={{ flex: 1, overflowY: 'auto', minWidth: 0, padding: 0 }}>{children}</main>

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
            width: 46,
            height: 46,
            borderRadius: 999,
            background: 'var(--color-primary)',
            color: 'var(--color-primary-foreground)',
            border: 0,
            zIndex: 60,
            cursor: 'pointer',
            display: 'grid',
            placeItems: 'center',
            boxShadow: '0 6px 20px oklch(0.6 0.16 287 / 0.45)',
          }}
        >
          <Menu size={20} />
        </button>
      </div>
    </div>
  );
}
