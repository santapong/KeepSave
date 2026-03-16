import { type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useTheme } from '../hooks/useTheme';
import type { User } from '../types';

interface LayoutProps {
  user: User | null;
  onLogout: () => void;
  children: ReactNode;
}

export function Layout({ user, onLogout, children }: LayoutProps) {
  const location = useLocation();
  const { theme, toggle: toggleTheme } = useTheme();

  const navLinks = [
    { to: '/', label: 'Projects' },
    { to: '/organizations', label: 'Organizations' },
    { to: '/templates', label: 'Templates' },
    { to: '/mcp-hub', label: 'MCP Hub' },
    { to: '/oauth-clients', label: 'OAuth' },
    { to: '/admin', label: 'Admin' },
    { to: '/help', label: 'Docs' },
  ];

  function isActive(path: string) {
    if (path === '/') return location.pathname === '/';
    return location.pathname.startsWith(path);
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--color-bg)' }}>
      <nav style={navStyle}>
        <Link to="/" style={logoStyle}>
          <svg width="26" height="26" viewBox="0 0 32 32" fill="none" style={{ marginRight: 10 }}>
            <defs>
              <linearGradient id="ks-grad" x1="0" y1="0" x2="32" y2="32">
                <stop stopColor="#6366f1" />
                <stop offset="1" stopColor="#4f46e5" />
              </linearGradient>
            </defs>
            <rect width="32" height="32" rx="8" fill="url(#ks-grad)" />
            <path d="M16 7l7.5 3.75v5.625c0 5-3.21 9.69-7.5 11.25-4.29-1.56-7.5-6.25-7.5-11.25v-5.625L16 7z" fill="rgba(255,255,255,0.15)" />
            <path d="M16 9.5l5.5 2.75v4.125c0 3.67-2.35 7.1-5.5 8.25-3.15-1.15-5.5-4.58-5.5-8.25v-4.125L16 9.5z" fill="rgba(255,255,255,0.9)" />
            <circle cx="16" cy="17.5" r="2" fill="#4f46e5" />
            <rect x="15.25" y="18.5" width="1.5" height="3" rx="0.75" fill="#4f46e5" />
          </svg>
          KeepSave
        </Link>
        <div style={navLinksContainer}>
          {navLinks.map((link) => (
            <Link
              key={link.to}
              to={link.to}
              style={{
                ...navLink,
                color: isActive(link.to) ? 'var(--color-primary)' : 'var(--color-text-secondary)',
                fontWeight: isActive(link.to) ? 600 : 500,
                background: isActive(link.to) ? 'var(--color-primary-glow)' : 'transparent',
              }}
            >
              {link.label}
            </Link>
          ))}
        </div>
        <div style={userSection}>
          <button
            onClick={toggleTheme}
            style={themeToggleBtn}
            title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
          >
            {theme === 'light' ? (
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
              </svg>
            ) : (
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="5" />
                <line x1="12" y1="1" x2="12" y2="3" />
                <line x1="12" y1="21" x2="12" y2="23" />
                <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
                <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
                <line x1="1" y1="12" x2="3" y2="12" />
                <line x1="21" y1="12" x2="23" y2="12" />
                <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
                <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
              </svg>
            )}
          </button>
          <span style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>{user?.email}</span>
          <button onClick={onLogout} style={logoutBtn}>
            Logout
          </button>
        </div>
      </nav>
      <main style={mainStyle}>
        {children}
      </main>
    </div>
  );
}

const navStyle: React.CSSProperties = {
  background: 'var(--color-nav)',
  borderBottom: '1px solid var(--color-border)',
  boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)',
  padding: '0 24px',
  height: 56,
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  position: 'sticky',
  top: 0,
  zIndex: 50,
};

const logoStyle: React.CSSProperties = {
  color: 'var(--color-text)',
  fontWeight: 700,
  fontSize: 17,
  textDecoration: 'none',
  display: 'flex',
  alignItems: 'center',
  marginRight: 24,
  letterSpacing: '-0.02em',
};

const navLinksContainer: React.CSSProperties = {
  display: 'flex',
  gap: 2,
  flex: 1,
};

const navLink: React.CSSProperties = {
  textDecoration: 'none',
  fontSize: 13,
  padding: '6px 12px',
  borderRadius: 8,
  transition: 'background 0.15s, color 0.15s',
};

const userSection: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
};

const themeToggleBtn: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: 32,
  height: 32,
  background: 'transparent',
  border: '1px solid var(--color-border)',
  borderRadius: 8,
  color: 'var(--color-text-secondary)',
  cursor: 'pointer',
  transition: 'background 0.15s, color 0.15s',
  flexShrink: 0,
};

const logoutBtn: React.CSSProperties = {
  background: 'transparent',
  border: '1px solid var(--color-border)',
  color: 'var(--color-text-secondary)',
  padding: '5px 12px',
  borderRadius: 8,
  fontSize: 12,
  fontWeight: 500,
  transition: 'background 0.15s',
};

const mainStyle: React.CSSProperties = {
  flex: 1,
  padding: 24,
  maxWidth: 1200,
  width: '100%',
  margin: '0 auto',
};
