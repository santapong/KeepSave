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
    <div style={styles.container}>
      <div style={styles.card}>
        <h1 style={styles.title}>KeepSave</h1>
        <p style={styles.subtitle}>Sign in to manage your secrets</p>
        <form onSubmit={handleSubmit} style={styles.form}>
          {error && <div style={styles.error}>{error}</div>}
          <label style={styles.label}>
            Email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              style={styles.input}
            />
          </label>
          <label style={styles.label}>
            Password
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              style={styles.input}
            />
          </label>
          <button type="submit" disabled={loading} style={styles.button}>
            {loading ? 'Signing in...' : 'Sign In'}
          </button>
        </form>
        <p style={styles.footer}>
          Don&apos;t have an account? <Link to="/register">Register</Link>
        </p>
      </div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'var(--color-bg)',
  },
  card: {
    background: 'var(--color-surface)',
    borderRadius: 'var(--radius)',
    boxShadow: 'var(--shadow)',
    padding: 40,
    width: '100%',
    maxWidth: 400,
  },
  title: { fontSize: 24, fontWeight: 700, textAlign: 'center' as const },
  subtitle: { fontSize: 14, color: 'var(--color-text-secondary)', textAlign: 'center' as const, marginBottom: 24 },
  form: { display: 'flex', flexDirection: 'column' as const, gap: 16 },
  label: { display: 'flex', flexDirection: 'column' as const, gap: 4, fontSize: 14, fontWeight: 500 },
  input: {
    padding: '8px 12px',
    border: '1px solid var(--color-border)',
    borderRadius: 'var(--radius)',
    fontSize: 14,
  },
  button: {
    padding: '10px 16px',
    background: 'var(--color-primary)',
    color: '#fff',
    border: 'none',
    borderRadius: 'var(--radius)',
    fontSize: 14,
    fontWeight: 600,
  },
  error: {
    background: '#fef2f2',
    color: 'var(--color-danger)',
    padding: '8px 12px',
    borderRadius: 'var(--radius)',
    fontSize: 13,
  },
  footer: { fontSize: 13, textAlign: 'center' as const, marginTop: 16 },
};
