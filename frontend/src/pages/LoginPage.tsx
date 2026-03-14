import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { login as apiLogin } from '../api/client';
import type { User } from '../types';

interface LoginPageProps {
  onLogin: (user: User, token: string) => void;
}

export function LoginPage({ onLogin }: LoginPageProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const resp = await apiLogin(email, password);
      onLogin(resp.user, resp.token);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={styles.page}>
      <div style={styles.bgOrb1} />
      <div style={styles.bgOrb2} />
      <div style={styles.container}>
        <div style={styles.branding}>
          <div style={styles.logoIcon}>
            <svg width="40" height="40" viewBox="0 0 40 40" fill="none">
              <rect width="40" height="40" rx="10" fill="#6366f1" />
              <path d="M20 10a6 6 0 0 0-6 6v2h-1a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-1v-2a6 6 0 0 0-6-6zm-3 8v-2a3 3 0 1 1 6 0v2h-6zm4 5.7V26a1 1 0 1 1-2 0v-2.3a1.5 1.5 0 1 1 2 0z" fill="#fff"/>
            </svg>
          </div>
          <h1 style={styles.title}>KeepSave</h1>
          <p style={styles.tagline}>Secure Environment Variable Storage</p>
        </div>

        <div style={styles.card}>
          <h2 style={styles.cardTitle}>Welcome back</h2>
          <p style={styles.cardSubtitle}>Sign in to manage your secrets</p>

          <form onSubmit={handleSubmit} style={styles.form}>
            {error && <div style={styles.error}>{error}</div>}

            <div style={styles.field}>
              <label style={styles.label}>Email</label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                placeholder="you@company.com"
                style={styles.input}
              />
            </div>

            <div style={styles.field}>
              <label style={styles.label}>Password</label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={8}
                placeholder="Enter your password"
                style={styles.input}
              />
            </div>

            <button type="submit" disabled={loading} style={styles.button}>
              {loading ? 'Signing in...' : 'Sign In'}
            </button>
          </form>

          <div style={styles.divider}>
            <span style={styles.dividerLine} />
            <span style={styles.dividerText}>or</span>
            <span style={styles.dividerLine} />
          </div>

          <p style={styles.footer}>
            Don&apos;t have an account?{' '}
            <Link to="/register" style={styles.link}>Create one</Link>
          </p>
        </div>

        <p style={styles.copyright}>
          Encrypted at rest with AES-256-GCM
        </p>
      </div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    minHeight: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'var(--color-bg)',
    position: 'relative',
    overflow: 'hidden',
  },
  bgOrb1: {
    position: 'absolute',
    top: '-20%',
    right: '-10%',
    width: 600,
    height: 600,
    borderRadius: '50%',
    background: 'radial-gradient(circle, rgba(99, 102, 241, 0.08) 0%, transparent 70%)',
    pointerEvents: 'none',
  },
  bgOrb2: {
    position: 'absolute',
    bottom: '-20%',
    left: '-10%',
    width: 500,
    height: 500,
    borderRadius: '50%',
    background: 'radial-gradient(circle, rgba(139, 92, 246, 0.06) 0%, transparent 70%)',
    pointerEvents: 'none',
  },
  container: {
    position: 'relative',
    zIndex: 1,
    width: '100%',
    maxWidth: 420,
    padding: '0 20px',
  },
  branding: {
    textAlign: 'center' as const,
    marginBottom: 32,
  },
  logoIcon: {
    display: 'inline-flex',
    marginBottom: 16,
  },
  title: {
    fontSize: 32,
    fontWeight: 800,
    color: '#fff',
    letterSpacing: '-0.02em',
  },
  tagline: {
    fontSize: 14,
    color: 'var(--color-text-secondary)',
    marginTop: 4,
  },
  card: {
    background: 'var(--color-surface)',
    borderRadius: 12,
    boxShadow: 'var(--shadow-lg)',
    border: '1px solid var(--color-border)',
    padding: '32px 28px',
  },
  cardTitle: {
    fontSize: 20,
    fontWeight: 700,
    color: '#fff',
    textAlign: 'center' as const,
  },
  cardSubtitle: {
    fontSize: 14,
    color: 'var(--color-text-secondary)',
    textAlign: 'center' as const,
    marginBottom: 24,
    marginTop: 4,
  },
  form: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: 16,
  },
  field: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: 6,
  },
  label: {
    fontSize: 13,
    fontWeight: 500,
    color: 'var(--color-text-secondary)',
  },
  input: {
    padding: '10px 14px',
    background: 'var(--color-input-bg)',
    border: '1px solid var(--color-border)',
    borderRadius: 'var(--radius)',
    fontSize: 14,
    color: 'var(--color-text)',
    transition: 'border-color 0.2s, box-shadow 0.2s',
  },
  button: {
    marginTop: 4,
    padding: '12px 16px',
    background: 'linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%)',
    color: '#fff',
    border: 'none',
    borderRadius: 'var(--radius)',
    fontSize: 15,
    fontWeight: 600,
    letterSpacing: '0.01em',
    transition: 'opacity 0.2s, transform 0.1s',
    boxShadow: '0 2px 12px rgba(99, 102, 241, 0.3)',
  },
  error: {
    background: 'var(--color-error-bg)',
    color: 'var(--color-danger)',
    padding: '10px 14px',
    borderRadius: 'var(--radius)',
    fontSize: 13,
    border: '1px solid rgba(239, 68, 68, 0.2)',
  },
  divider: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    margin: '20px 0 16px',
  },
  dividerLine: {
    flex: 1,
    height: 1,
    background: 'var(--color-border)',
  },
  dividerText: {
    fontSize: 12,
    color: 'var(--color-text-secondary)',
  },
  footer: {
    fontSize: 13,
    textAlign: 'center' as const,
    color: 'var(--color-text-secondary)',
  },
  link: {
    color: 'var(--color-primary-hover)',
    fontWeight: 500,
  },
  copyright: {
    fontSize: 12,
    color: '#4b5563',
    textAlign: 'center' as const,
    marginTop: 24,
  },
};
