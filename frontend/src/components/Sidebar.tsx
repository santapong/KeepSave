import { Link, useLocation } from 'react-router-dom';
import { useTheme } from '@/hooks/useTheme';
import { cn } from '@/lib/utils';
import {
  FolderOpen,
  Building2,
  FileText,
  Server,
  KeyRound,
  LayoutGrid,
  BarChart3,
  HelpCircle,
  Moon,
  Sun,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
  Shield,
} from 'lucide-react';

interface SidebarProps {
  user: { email: string } | null;
  collapsed: boolean;
  onToggle: () => void;
  onLogout: () => void;
}

interface NavItem {
  path: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
}

interface NavSection {
  label: string;
  items: NavItem[];
}

const NAV_SECTIONS: NavSection[] = [
  {
    label: 'Core',
    items: [
      { path: '/', label: 'Projects', icon: FolderOpen },
      { path: '/organizations', label: 'Organizations', icon: Building2 },
      { path: '/templates', label: 'Templates', icon: FileText },
    ],
  },
  {
    label: 'Platform',
    items: [
      { path: '/mcp-hub', label: 'MCP Hub', icon: Server },
      { path: '/oauth-clients', label: 'OAuth', icon: KeyRound },
      { path: '/applications', label: 'Applications', icon: LayoutGrid },
    ],
  },
  {
    label: 'Monitoring',
    items: [
      { path: '/admin', label: 'Dashboard', icon: BarChart3 },
    ],
  },
  {
    label: 'Help',
    items: [
      { path: '/help', label: 'Docs', icon: HelpCircle },
    ],
  },
];

function isActive(currentPath: string, itemPath: string): boolean {
  if (itemPath === '/') return currentPath === '/';
  return currentPath.startsWith(itemPath);
}

export function Sidebar({ user, collapsed, onToggle, onLogout }: SidebarProps) {
  const location = useLocation();
  const { theme, toggle: toggleTheme } = useTheme();

  return (
    <aside
      className={cn(
        'flex flex-col h-screen bg-sidebar text-sidebar-foreground border-r border-sidebar-border transition-all duration-200 shrink-0',
        collapsed ? 'w-16' : 'w-60'
      )}
    >
      {/* Logo */}
      <div className="flex items-center h-14 px-4 border-b border-sidebar-border shrink-0">
        <Link to="/" className="flex items-center gap-2.5 no-underline">
          <Shield className="h-6 w-6 text-primary shrink-0" />
          {!collapsed && (
            <span className="text-base font-bold tracking-tight text-sidebar-foreground">
              KeepSave
            </span>
          )}
        </Link>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto py-4 px-2">
        {NAV_SECTIONS.map((section) => (
          <div key={section.label} className="mb-4">
            {!collapsed && (
              <div className="px-3 mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/70">
                {section.label}
              </div>
            )}
            <div className="flex flex-col gap-0.5">
              {section.items.map((item) => {
                const active = isActive(location.pathname, item.path);
                const Icon = item.icon;

                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    title={collapsed ? item.label : undefined}
                    className={cn(
                      'flex items-center gap-3 rounded-md text-sm font-medium transition-colors relative no-underline',
                      collapsed ? 'justify-center px-2 py-2' : 'px-3 py-2',
                      active
                        ? 'bg-primary/10 text-primary'
                        : 'text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground'
                    )}
                  >
                    {active && (
                      <div className="absolute left-0 top-1 bottom-1 w-[3px] rounded-r-full bg-primary" />
                    )}
                    <Icon className={cn('h-4.5 w-4.5 shrink-0', active && 'text-primary')} />
                    {!collapsed && <span>{item.label}</span>}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      {/* User section */}
      <div className="border-t border-sidebar-border px-2 py-3 space-y-1.5 shrink-0">
        {user && (
          <div
            className={cn(
              'flex items-center gap-2 px-3 py-1.5 text-xs text-muted-foreground',
              collapsed && 'justify-center px-0'
            )}
            title={user.email}
          >
            <div className="h-6 w-6 rounded-full bg-primary/10 text-primary flex items-center justify-center text-[11px] font-semibold shrink-0">
              {user.email.charAt(0).toUpperCase()}
            </div>
            {!collapsed && (
              <span className="truncate">{user.email}</span>
            )}
          </div>
        )}

        <div className={cn('flex gap-1', collapsed ? 'flex-col items-center' : 'items-center px-1')}>
          <button
            onClick={toggleTheme}
            title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
            className={cn(
              'flex items-center justify-center rounded-md transition-colors',
              'hover:bg-sidebar-accent text-sidebar-foreground/70 hover:text-sidebar-accent-foreground',
              collapsed ? 'h-9 w-9' : 'h-8 w-8'
            )}
          >
            {theme === 'light' ? (
              <Moon className="h-4 w-4" />
            ) : (
              <Sun className="h-4 w-4" />
            )}
          </button>

          <button
            onClick={onLogout}
            title="Logout"
            className={cn(
              'flex items-center justify-center rounded-md transition-colors',
              'hover:bg-sidebar-accent text-sidebar-foreground/70 hover:text-sidebar-accent-foreground',
              collapsed ? 'h-9 w-9' : 'h-8 w-8'
            )}
          >
            <LogOut className="h-4 w-4" />
          </button>

          {!collapsed && (
            <span className="flex-1" />
          )}
        </div>
      </div>

      {/* Collapse toggle */}
      <div className="border-t border-sidebar-border px-2 py-2 shrink-0">
        <button
          onClick={onToggle}
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          className={cn(
            'flex items-center justify-center w-full rounded-md py-1.5 transition-colors',
            'hover:bg-sidebar-accent text-sidebar-foreground/70 hover:text-sidebar-accent-foreground',
            collapsed ? 'px-0' : 'gap-2 px-3'
          )}
        >
          {collapsed ? (
            <PanelLeftOpen className="h-4 w-4" />
          ) : (
            <>
              <PanelLeftClose className="h-4 w-4" />
              <span className="text-xs font-medium">Collapse</span>
            </>
          )}
        </button>
      </div>
    </aside>
  );
}
