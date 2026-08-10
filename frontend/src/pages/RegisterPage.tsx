import { useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { register as apiRegister } from '../api/client';
import { EventHorizon } from '../components/cosmic/EventHorizon';
import { KsMark } from '../components/cosmic/KsMark';
import { Starfield } from '../components/cosmic/Starfield';
import { CometField } from '../components/cosmic/CometField';
import { useCosmicEntrance } from '../hooks/useCosmicEntrance';
import type { User } from '../types';

interface RegisterPageProps {
  onLogin: (user: User, token: string) => void;
}

const INCLUDED: Array<[string, string]> = [
  ['Encrypted at rest', 'AES-256-GCM'],
  ['Lease-based access', 'just-in-time'],
  ['Audited end to end', 'tamper-evident'],
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

  const rootRef = useRef<HTMLDivElement>(null);

  useCosmicEntrance(rootRef, '.cz-login-aside-head, .cz-login-aside-body > *, .cz-login-aside-foot, .cz-login-card > *');


  return (
    <div className="cz-login-root" ref={rootRef}>
      <Starfield />
      <CometField />

      {/* Hero pane */}
      <aside className="cz-login-aside">
        <div className="cz-login-aside-bg" aria-hidden="true">
          <EventHorizon size={560} />
        </div>

        <div className="cz-login-aside-head">
          <span className="cz-pill cz-pill-go">
            <span className="cz-dot cz-dot-go" /> All systems operational
          </span>
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
          <span>build 14.0</span>
        </div>
      </aside>

      {/* Form pane */}
      <main className="cz-login-main">
        <div className="cz-login-card">
          <div className="cz-login-card-head">
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <KsMark />
              <span className="cz-mk" style={{ fontWeight: 500, fontSize: 20 }}>
                Keep<em style={{ fontStyle: 'normal', color: 'var(--cz-accent-hi)' }}>save</em>
              </span>
            </div>
            <span className="cz-faint" style={{ fontFamily: 'var(--cz-mono)', fontSize: 11 }}>
              v14.0
            </span>
          </div>

          <h2 className="cz-login-h2">Create account</h2>
          <p className="cz-login-lead">
            Already have an account? <Link to="/login">Sign in →</Link>
          </p>

          <form className="cz-login-form" onSubmit={handleSubmit} style={{ marginTop: 22 }}>
            {error && <div className="cz-login-error">{error}</div>}

            <div className="cz-login-field">
              <label htmlFor="register-email">Email</label>
              <input
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
            <span>Encrypted at rest · AES-256-GCM</span>
            <span>CSRF · HSTS · CSP</span>
          </div>
        </div>
      </main>
    </div>
  );
}
