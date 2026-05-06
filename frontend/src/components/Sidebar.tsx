import { Link, useLocation } from 'react-router-dom';
import { useTheme } from '@/hooks/useTheme';
import { cn } from '@/lib/utils';

interface SidebarProps {
  user: { email: string } | null;
  collapsed: boolean;
  onToggle: () => void;
  onLogout: () => void;
}

interface NavItem {
  path: string;
  label: string;
  count?: string;
}

interface NavSection {
  label: string;
  items: NavItem[];
}

const NAV_SECTIONS: NavSection[] = [
  {
    label: 'Vault',
    items: [
      { path: '/', label: 'Projects' },
      { path: '/organizations', label: 'Organizations' },
      { path: '/templates', label: 'Templates' },
    ],
  },
  {
    label: 'Platform',
    items: [
      { path: '/mcp-hub', label: 'MCP Hub' },
      { path: '/oauth-clients', label: 'OAuth Clients' },
      { path: '/applications', label: 'Applications' },
    ],
  },
  {
    label: 'Intelligence',
    items: [
      { path: '/ai', label: 'AI Intelligence' },
      { path: '/admin', label: 'Dashboard' },
    ],
  },
  {
    label: 'Help',
    items: [
      { path: '/help', label: 'Docs' },
    ],
  },
];

function isActive(currentPath: string, itemPath: string): boolean {
  if (itemPath === '/') return currentPath === '/';
  return currentPath.startsWith(itemPath);
}

export function Sidebar({ user, onLogout }: SidebarProps) {
  const location = useLocation();
  const { theme, toggle: toggleTheme } = useTheme();

  const initial = user?.email?.charAt(0).toUpperCase() ?? 's';

  return (
    <aside className="ks-rail" style={{ width: 220 }}>
      <Link to="/" className="ks-wordmark">
        <span className="ks-mk">Keep<em>save</em></span>
        <span className="ks-sub">/ VAULT · v.13.42</span>
      </Link>

      <div style={{ padding: '14px 20px 0', fontSize: 10, color: 'var(--ks-ink-mute)', display: 'flex', flexDirection: 'column', gap: 4 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <span className="ks-faint">Org</span>
          <span className="ks-amber">acme-platform ▾</span>
        </div>
      </div>

      <nav className="ks-nav">
        {NAV_SECTIONS.map((section) => (
          <div key={section.label}>
            <div className="ks-nav-section">{section.label}</div>
            {section.items.map((item) => {
              const active = isActive(location.pathname, item.path);
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  className={cn('ks-nav-item', active && 'active')}
                >
                  <span>{item.label}</span>
                  {item.count && <span className="ks-count">{item.count}</span>}
                </Link>
              );
            })}
          </div>
        ))}
      </nav>

      <div className="ks-rail-foot">
        <div className="ks-row"><span>Region</span><span className="ks-amber">eu-west-1</span></div>
        <div className="ks-row"><span>Master key</span><span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}><span className="ks-dot ks-dot-go" /> HSM-ATT</span></div>
        <div className="ks-row"><span>Uptime</span><span className="ks-num">99.997%</span></div>
        <hr className="ks-hair-soft" style={{ margin: '6px 0' }} />
        <div className="ks-row" style={{ alignItems: 'center' }}>
          <span style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <span style={{
              width: 20, height: 20, background: 'var(--ks-amber)', color: '#140d00',
              fontFamily: 'var(--ks-serif)', fontStyle: 'italic',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 12, fontWeight: 600,
            }}>{initial}</span>
            <span style={{ color: 'var(--ks-ink)' }}>{user?.email?.split('@')[0] ?? 'guest'}</span>
          </span>
          <span className="ks-amber" style={{ cursor: 'pointer' }} onClick={onLogout}>OUT ↗</span>
        </div>
        <div className="ks-row" style={{ marginTop: 4 }}>
          <span
            className="ks-faint"
            style={{ cursor: 'pointer' }}
            onClick={toggleTheme}
            title={`Switch to ${theme === 'dark' ? 'light' : 'dark'}`}
          >
            {theme === 'dark' ? '☾ DARK' : '☀ LIGHT'}
          </span>
          <span className="ks-faint">build.13.42.7</span>
        </div>
      </div>
    </aside>
  );
}
