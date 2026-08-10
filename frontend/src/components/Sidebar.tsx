import type { ComponentType } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  LayoutGrid,
  FolderClosed,
  Building2,
  Boxes,
  Server,
  ShieldCheck,
  AppWindow,
  Bot,
  BookOpen,
  ChevronDown,
  LogOut,
  Moon,
  Sun,
} from 'lucide-react';
import { useTheme } from '@/hooks/useTheme';
import { cn } from '@/lib/utils';
import { KsMark } from './cosmic/KsMark';

interface SidebarProps {
  user: { email: string } | null;
  collapsed: boolean;
  onToggle: () => void;
  onLogout: () => void;
}

interface NavItem {
  path: string;
  label: string;
  icon: ComponentType<{ size?: number; className?: string; strokeWidth?: number }>;
  count?: string;
}

interface NavSection {
  label: string;
  items: NavItem[];
}

const NAV_SECTIONS: NavSection[] = [
  {
    label: 'Overview',
    items: [{ path: '/admin', label: 'Dashboard', icon: LayoutGrid }],
  },
  {
    label: 'Vault',
    items: [
      { path: '/', label: 'Projects', icon: FolderClosed },
      { path: '/organizations', label: 'Organizations', icon: Building2 },
      { path: '/templates', label: 'Templates', icon: Boxes },
    ],
  },
  {
    label: 'Platform',
    items: [
      { path: '/mcp-hub', label: 'MCP Hub', icon: Server },
      { path: '/oauth-clients', label: 'OAuth Clients', icon: ShieldCheck },
      { path: '/applications', label: 'Applications', icon: AppWindow },
    ],
  },
  {
    label: 'Intelligence',
    items: [
      { path: '/ai', label: 'AI Intelligence', icon: Bot },
      { path: '/help', label: 'Docs', icon: BookOpen },
    ],
  },
];

function isActive(currentPath: string, itemPath: string): boolean {
  if (itemPath === '/') return currentPath === '/' || currentPath.startsWith('/projects');
  return currentPath.startsWith(itemPath);
}

export function Sidebar({ user, onLogout }: SidebarProps) {
  const location = useLocation();
  const { theme, toggle: toggleTheme } = useTheme();

  const initial = user?.email?.charAt(0).toUpperCase() ?? 'S';
  const name = user?.email?.split('@')[0] ?? 'guest';

  return (
    <aside className="cz-rail">
      <Link to="/" className="cz-wordmark">
        <KsMark />
        <span className="cz-mk">
          Keep<em>save</em>
        </span>
      </Link>

      <button type="button" className="cz-org">
        <span className="cz-nm">acme-platform</span>
        <ChevronDown className="cz-ar" size={14} />
      </button>

      <nav className="cz-nav">
        {NAV_SECTIONS.map((section) => (
          <div key={section.label}>
            <div className="cz-nav-section">{section.label}</div>
            {section.items.map((item) => {
              const Icon = item.icon;
              const active = isActive(location.pathname, item.path);
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  className={cn('cz-nav-item', active && 'cz-active')}
                >
                  <Icon className="cz-ic" size={16} strokeWidth={1.6} />
                  <span className="cz-lb">{item.label}</span>
                  {item.count && <span className="cz-ct">{item.count}</span>}
                </Link>
              );
            })}
          </div>
        ))}
      </nav>

      <div className="cz-rail-foot">
        <div className="cz-row">
          <span>Region</span>
          <b>eu-west-1</b>
        </div>
        <div className="cz-row">
          <span>Master key</span>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
            <span className="cz-dot cz-dot-go" /> HSM
          </span>
        </div>
        <div className="cz-row">
          <span>Uptime</span>
          <b className="cz-num">99.997%</b>
        </div>

        <div className="cz-user-row">
          <span className="cz-avatar">{initial}</span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div
              style={{
                color: 'var(--cz-ink)',
                fontSize: 13,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {name}
            </div>
            <div className="cz-faint" style={{ fontFamily: 'var(--cz-mono)', fontSize: 10 }}>
              member
            </div>
          </div>
          <button
            type="button"
            onClick={onLogout}
            title="Sign out"
            aria-label="Sign out"
            style={{
              background: 'transparent',
              border: 0,
              color: 'var(--cz-accent-hi)',
              display: 'inline-flex',
              alignItems: 'center',
            }}
          >
            <LogOut size={15} />
          </button>
        </div>

        <div className="cz-row" style={{ marginTop: 2 }}>
          <button
            type="button"
            onClick={toggleTheme}
            title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`}
            className="cz-faint"
            style={{
              background: 'transparent',
              border: 0,
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              cursor: 'pointer',
              fontFamily: 'var(--cz-mono)',
              fontSize: 11,
            }}
          >
            {theme === 'dark' ? <Moon size={13} /> : <Sun size={13} />}
            {theme === 'dark' ? 'Dark' : 'Light'}
          </button>
          <span className="cz-faint">build 14.0</span>
        </div>
      </div>
    </aside>
  );
}
