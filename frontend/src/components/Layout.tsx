import { type ReactNode, useEffect, useState, useRef } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Search, Menu, ChevronRight, BookOpen } from '@/components/icons';
import { useSidebar } from '@/hooks/useSidebar';
import { Sidebar } from './Sidebar';
import { CommandPalette } from './CommandPalette';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from './ui/dialog';
import type { User } from '../types';
import '../styles/workspace.css';

interface LayoutProps { user: User | null; onLogout: () => void; children: ReactNode }
const labels: Record<string, string> = {
  '': 'Projects', organizations: 'Organizations', templates: 'Templates', 'mcp-hub': 'MCP Hub',
  'oauth-clients': 'OAuth Clients', applications: 'Applications', admin: 'Administration', help: 'Documentation',
  projects: 'Projects', ai: 'AI Intelligence', settings: 'Settings', account: 'Account',
};
export function Layout({ user, onLogout, children }: LayoutProps) {
  const { collapsed, toggle } = useSidebar();
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const mobileButtonRef = useRef<HTMLButtonElement>(null);
  const commandButtonRef = useRef<HTMLButtonElement>(null);
  const location = useLocation();
  const segment = location.pathname.split('/').filter(Boolean)[0] || '';
  useEffect(() => {
    document.body.classList.add('ks-workspace-mode');
    return () => document.body.classList.remove('ks-workspace-mode');
  }, []);
  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); setPaletteOpen((open) => !open); }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);
  const sidebar = { user, collapsed, onToggle: toggle, onLogout };
  return <div className={`ks-workspace ${collapsed ? 'ks-workspace-collapsed' : ''}`}>
    <a className="cz-skip" href="#workspace-main">Skip to workspace</a>
    <div className="ks-desktop-sidebar"><Sidebar {...sidebar} /></div>
    <div className="ks-workspace-main">
      <header className="ks-topbar">
        <button ref={mobileButtonRef} type="button" className="ks-icon-button ks-mobile-menu" onClick={() => setMobileOpen(true)} aria-label="Open navigation"><Menu size={19} /></button>
        <nav className="ks-breadcrumb" aria-label="Breadcrumb"><Link to="/">Workspace</Link><ChevronRight size={13} /><span>{labels[segment] || 'Workspace'}</span></nav>
        <div className="ks-topbar-actions"><button ref={commandButtonRef} type="button" className="ks-command-trigger" onClick={() => setPaletteOpen(true)} aria-label="Open command palette"><Search size={15} /><span>Jump to…</span><kbd>⌘ K / Ctrl K</kbd></button><Link className="ks-icon-button ks-topbar-help" to="/help" aria-label="Documentation" title="Documentation"><BookOpen size={17} /></Link></div>
      </header>
      <main id="workspace-main" tabIndex={-1}>{children}</main>
      <footer className="ks-workspace-footer"><span><span className="ks-small-orbit" /> KeepSave</span><span>Environment secrets, kept together.</span></footer>
    </div>
    <Dialog open={mobileOpen} onOpenChange={setMobileOpen}>
      <DialogContent className="ks-mobile-drawer" onCloseAutoFocus={(event) => { event.preventDefault(); mobileButtonRef.current?.focus(); }}><DialogTitle className="sr-only">Workspace navigation</DialogTitle><DialogDescription className="sr-only">Choose a workspace page or manage your account.</DialogDescription><Sidebar {...sidebar} collapsed={false} mobile onNavigate={() => setMobileOpen(false)} /></DialogContent>
    </Dialog>
    <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} onRestoreFocus={() => commandButtonRef.current?.focus()} />
  </div>;
}
