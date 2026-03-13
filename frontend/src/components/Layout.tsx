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
    { to: '/api-keys', label: 'API Keys' },
  ];

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      <nav style={{
        background: 'var(--color-nav)',
        color: '#fff',
        padding: '0 24px',
        height: 56,
        display: 'flex',
        alignItems: 'center',
        gap: 24,
      }}>
        <Link to="/" style={{ color: '#fff', fontWeight: 700, fontSize: 18, textDecoration: 'none' }}>
          KeepSave
        </Link>
        <div style={{ display: 'flex', gap: 16, flex: 1 }}>
          {navLinks.map((link) => (
            <Link
              key={link.to}
              to={link.to}
              style={{
                color: location.pathname === link.to ? '#fff' : '#9ca3af',
                textDecoration: 'none',
                fontSize: 14,
                fontWeight: location.pathname === link.to ? 600 : 400,
              }}
            >
              {link.label}
            </Link>
          ))}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ fontSize: 13, color: '#9ca3af' }}>{user?.email}</span>
          <button
            onClick={onLogout}
            style={{
              background: 'transparent',
              border: '1px solid #4b5563',
              color: '#d1d5db',
              padding: '4px 12px',
              borderRadius: 4,
              fontSize: 13,
            }}
          >
            Logout
          </button>
        </div>
      </nav>
      <main style={{ flex: 1, padding: 24, maxWidth: 1200, width: '100%', margin: '0 auto' }}>
        {children}
      </main>
    </div>
  );
}
