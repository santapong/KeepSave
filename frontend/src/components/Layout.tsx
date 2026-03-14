import { type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import type { User } from '../types';

interface LayoutProps {
  user: User | null;
  onLogout: () => void;
  children: ReactNode;
}

export function Layout({ user, onLogout, children }: LayoutProps) {
  const location = useLocation();

  const navLinks = [
    { to: '/', label: 'Projects' },
    { to: '/organizations', label: 'Organizations' },
    { to: '/templates', label: 'Templates' },
    { to: '/api-keys', label: 'API Keys' },
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
          <svg width="22" height="22" viewBox="0 0 40 40" fill="none" style={{ marginRight: 8 }}>
            <rect width="40" height="40" rx="8" fill="#6366f1" />
            <path d="M20 10a6 6 0 0 0-6 6v2h-1a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-1v-2a6 6 0 0 0-6-6zm-3 8v-2a3 3 0 1 1 6 0v2h-6z" fill="#fff"/>
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
                color: isActive(link.to) ? '#fff' : '#9ca3af',
                fontWeight: isActive(link.to) ? 600 : 400,
                background: isActive(link.to) ? 'rgba(99, 102, 241, 0.12)' : 'transparent',
              }}
            >
              {link.label}
            </Link>
          ))}
        </div>
        <div style={userSection}>
          <span style={{ fontSize: 13, color: '#9ca3af' }}>{user?.email}</span>
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
  padding: '0 24px',
  height: 56,
  display: 'flex',
  alignItems: 'center',
  gap: 8,
};

const logoStyle: React.CSSProperties = {
  color: '#fff',
  fontWeight: 700,
  fontSize: 17,
  textDecoration: 'none',
  display: 'flex',
  alignItems: 'center',
  marginRight: 16,
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
  borderRadius: 6,
  transition: 'background 0.15s, color 0.15s',
};

const userSection: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
};

const logoutBtn: React.CSSProperties = {
  background: 'transparent',
  border: '1px solid #374151',
  color: '#d1d5db',
  padding: '5px 12px',
  borderRadius: 6,
  fontSize: 12,
};

const mainStyle: React.CSSProperties = {
  flex: 1,
  padding: 24,
  maxWidth: 1200,
  width: '100%',
  margin: '0 auto',
};
