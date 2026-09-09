import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { register as apiRegister } from '../api/client';
import { BlackHoleScene } from '../components/cosmic/BlackHoleScene';
import { Brand } from '../components/cosmic/Brand';
import { LockKeyhole } from 'lucide-react';
import { Starfield } from '../components/cosmic/Starfield';
import type { User } from '../types';

interface RegisterPageProps {
  onLogin: (user: User, token: string) => void;
}

const INCLUDED: Array<[string, string]> = [
  ['Encrypted at rest', 'AES-256-GCM'],
  ['Scoped access', 'API keys'],
  ['Change history', 'audit trail'],
  ['Self-host or hosted', 'docker compose'],
];

export function RegisterPage({ onLogin }: RegisterPageProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    if (password !== confirm) {
      setError('Passwords do not match');
      return;
    }
    setLoading(true);
    try {
      const resp = await apiRegister(email, password);
      onLogin(resp.user, resp.token);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
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
          <span className="cz-eyebrow">Create account</span>
          <h1 className="cz-login-title">
            Start keeping
            <br />
            <em>secrets properly.</em>
          </h1>
          <p className="cz-login-sub">
            An encrypted vault, OAuth 2.0 identity provider, and central MCP hub for the teams
            whose agents and pipelines reach into production.
          </p>

          <div className="cz-login-stats">
            <div className="cz-eyebrow" style={{ marginBottom: 8 }}>
              Included
            </div>
            {INCLUDED.map(([label, value], i) => (
              <div key={label} className="cz-login-stat">
                <span className="cz-ix">{String(i + 1).padStart(2, '0')}</span>
                <span className="cz-lb">{label}</span>
                <span className="cz-vl cz-mono">{value}</span>
                <span className="cz-dot cz-dot-go" />
              </div>
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

          <h2 className="cz-login-h2">Create account</h2>
          <p className="cz-login-lead">
            Already have an account? <Link to="/login">Sign in →</Link>
          </p>

          <form className="cz-login-form" onSubmit={handleSubmit} style={{ marginTop: 22 }}>
            {error && <div className="cz-login-error" role="alert">{error}</div>}

            <div className="cz-login-field">
              <label htmlFor="register-email">Email</label>
              <input
                autoComplete="email"
                id="register-email"
                className="cz-input"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@company.com"
                required
              />
            </div>
            <div className="cz-login-field">
              <label htmlFor="register-password">Password</label>
              <input
                id="register-password"
                className="cz-input"
                autoComplete="new-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Min. 8 characters"
                minLength={8}
                required
              />
            </div>
            <div className="cz-login-field">
              <label htmlFor="register-confirm">Confirm password</label>
              <input
                id="register-confirm"
                className="cz-input"
                autoComplete="new-password"
                type="password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                placeholder="Repeat your password"
                minLength={8}
                required
              />
            </div>
            <button
              type="submit"
              className="cz-btn cz-btn-primary"
              style={{ justifyContent: 'center', width: '100%' }}
              disabled={loading}
            >
              {loading ? 'Creating account…' : 'Create account →'}
            </button>
          </form>

          <div className="cz-login-divider" />
          <div className="cz-login-fine">
            <Link to="/">← Back to KeepSave</Link>
            <span>Encrypted at rest · AES-256-GCM</span>
            <span>Scoped access</span>
          </div>
        </div>
      </main>
    </div>
  );
}
