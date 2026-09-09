import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { login as apiLogin } from '../api/client';
import { BlackHoleScene } from '../components/cosmic/BlackHoleScene';
import { Brand } from '../components/cosmic/Brand';
import { LockKeyhole } from 'lucide-react';
import { Starfield } from '../components/cosmic/Starfield';
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
    <div className="cz-login-root">
      <Starfield />

      {/* Hero pane */}
      <aside className="cz-login-aside">
        <div className="cz-login-aside-bg">
          <BlackHoleScene />
        </div>

        <div className="cz-login-aside-head">
          <span className="cz-brand-kicker"><span /> KEEPSAVE / EVENT HORIZON</span>
        </div>

        <div className="cz-login-aside-body">
          <span className="cz-eyebrow">Entry authorization</span>
          <h1 className="cz-login-title">
            Return to
            <br />
            <em>your orbit.</em>
          </h1>
          <p className="cz-login-sub">
            Your project vaults, scoped connections, and environment changes —
            together in one workspace.
          </p>

          <div className="cz-login-stats">
            {[['Store', 'Encrypted project vaults'], ['Connect', 'Scoped access for your tools'], ['Promote', 'Review changes between environments']].map(([label, detail]) => (
              <div key={label} className="cz-login-stat"><span className="cz-lb">{label}</span><span className="cz-vl" style={{ fontSize: 12 }}>{detail}</span></div>
            ))}
          </div>
        </div>

        <div className="cz-login-aside-foot">
          <span>KeepSave · 2026</span>
          <span>Environment secrets, kept together.</span>
        </div>
      </aside>

      {/* Form pane */}
      <main className="cz-login-main">
        <div className="cz-login-card">
          <div className="cz-login-card-head">
            <Brand size={40} />
            <span className="cz-vault-badge"><LockKeyhole size={12} /> VAULT</span>
          </div>

          <h2 className="cz-login-h2">Sign in</h2>
          <p className="cz-login-lead">
            New to KeepSave?{' '}
            <Link to="/register">Open an account →</Link>
          </p>

          {error && <div className="cz-login-error" role="alert">{error}</div>}

            <form className="cz-login-form" onSubmit={handleSubmit} style={{ marginTop: 24 }}>
              <div className="cz-login-field">
                <label htmlFor="login-email">Email</label>
                <input
                  id="login-email"
                  className="cz-input"
                  type="email"
                  autoComplete="username"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@company.com"
                  required
                />
              </div>
              <div className="cz-login-field">
                <label htmlFor="login-password">
                  Password
                </label>
                <input
                  id="login-password"
                  className="cz-input"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  minLength={8}
                  required
                />
              </div>
              <p className="cz-faint" style={{ fontSize: 12 }}>Your session stays in this browser tab.</p>
              <button
                type="submit"
                className="cz-btn cz-btn-primary"
                style={{ justifyContent: 'center', width: '100%' }}
                disabled={loading}
              >
                {loading ? 'Signing in…' : 'Enter the vault →'}
              </button>
            </form>
          <div className="cz-login-divider" />
          <div className="cz-login-fine">
            <Link to="/">← Back to KeepSave</Link>
            <span>Encrypted at rest</span>
          </div>
        </div>
      </main>
    </div>
  );
}
