import type { ComponentType } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { FolderClosed, Building2, Boxes, OrbitHub, KeyRound, ShieldCheck, AppWindow, Bot, BookOpen, LogOut, Moon, Sun, PanelLeftClose, PanelLeftOpen, ArrowUpRight } from '@/components/icons';
import { useTheme } from '@/hooks/useTheme';
import { Brand } from './cosmic/Brand';
import { useCapabilities, routeCapability } from '../hooks/useCapabilities';

interface SidebarProps {
  user: { email: string } | null;
  collapsed: boolean;
  onToggle: () => void;
  onLogout: () => void;
  onNavigate?: () => void;
  mobile?: boolean;
}
interface NavItem { path: string; label: string; icon: ComponentType<{ size?: number; strokeWidth?: number }> }
const sections: { label: string; items: NavItem[] }[] = [
  { label: 'Workspace', items: [
    { path: '/', label: 'Projects', icon: FolderClosed },
    { path: '/organizations', label: 'Organizations', icon: Building2 },
    { path: '/templates', label: 'Templates', icon: Boxes },
  ] },
  { label: 'Connections', items: [
    { path: '/applications', label: 'Applications', icon: AppWindow },
    { path: '/mcp-hub', label: 'MCP Hub', icon: OrbitHub },
    { path: '/oauth-clients', label: 'OAuth Clients', icon: KeyRound },
  ] },
  { label: 'Tools', items: [
    { path: '/ai', label: 'AI Intelligence', icon: Bot },
    { path: '/admin', label: 'Administration', icon: ShieldCheck },
  ] },
];

export function Sidebar({ user, collapsed, onToggle, onLogout, onNavigate, mobile = false }: SidebarProps) {
  const { pathname } = useLocation();
  const { unavailable } = useCapabilities();
  const { theme, toggle } = useTheme();
  const active = (path: string) => path === '/' ? pathname === '/' || pathname.startsWith('/projects') : pathname.startsWith(path);
  return <aside className={`ks-sidebar ${collapsed ? 'ks-sidebar-collapsed' : ''} ${mobile ? 'ks-sidebar-mobile' : ''}`}>
    <div className="ks-sidebar-brand"><Brand size={35} onNavigate={onNavigate} />
      {!mobile && <button type="button" className="ks-icon-button ks-collapse" aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'} title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'} onClick={onToggle}>{collapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}</button>}
    </div>
    <div className="ks-sidebar-caption"><span className="ks-small-orbit" /> YOUR WORKSPACE</div>
    <nav className="ks-navigation" aria-label="Workspace navigation">
      {sections.map((section) => <div className="ks-nav-section" key={section.label}>
        <div className="ks-nav-label">{section.label}</div>
        {section.items.map(({ path, label, icon: Icon }) => routeCapability(path) && unavailable(routeCapability(path)!) ? <span key={path} className="ks-nav-link" aria-disabled="true" title={`${label}: unavailable in this release`} style={{ opacity: .45 }}><Icon size={18} strokeWidth={1.6} /><span>{label} · Planned</span></span> : <Link key={path} to={path} onClick={onNavigate} className={`ks-nav-link ${active(path) ? 'ks-nav-active' : ''}`} aria-label={label} aria-current={active(path) ? 'page' : undefined} title={collapsed ? label : undefined}>
          <Icon size={18} strokeWidth={1.6} /><span>{label}</span>{active(path) && <i aria-hidden="true" />}
        </Link>)}
      </div>)}
    </nav>
    <div className="ks-sidebar-bottom">
      <Link className="ks-nav-link ks-help-link" to="/help" onClick={onNavigate} aria-label="Documentation" title={collapsed ? 'Documentation' : undefined}><BookOpen size={17} /><span>Documentation</span><ArrowUpRight size={13} /></Link>
      <div className="ks-account-row">
        <Link to="/account" className="ks-account-link" onClick={onNavigate} aria-label="Account settings" title={user?.email || 'Account settings'}>
          <span className="ks-account-avatar">{user?.email?.charAt(0).toUpperCase() || 'K'}</span>
          <span className="ks-account-copy"><strong>{user?.email?.split('@')[0] || 'Your account'}</strong><span>Account settings</span></span>
        </Link>
      </div>
      <div className="ks-sidebar-utilities">
        <button type="button" className="ks-utility-button" onClick={toggle} aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`} title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}>
          {theme === 'dark' ? <Moon size={16} /> : <Sun size={16} />}<span>{theme === 'dark' ? 'Dark mode' : 'Light mode'}</span>
        </button>
        <button type="button" className="ks-icon-button" onClick={onLogout} title="Sign out" aria-label="Sign out"><LogOut size={16} /></button>
      </div>
    </div>
  </aside>;
}
