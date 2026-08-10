import { useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { login as apiLogin } from '../api/client';
import { Singularity } from '../components/cosmic/Singularity';
import { KsMark } from '../components/cosmic/KsMark';
import { Starfield } from '../components/cosmic/Starfield';
import { CometField } from '../components/cosmic/CometField';
import { useCosmicEntrance } from '../hooks/useCosmicEntrance';
import type { User } from '../types';

interface LoginPageProps {
  onLogin: (user: User, token: string) => void;
}

type Mode = 'password' | 'key' | 'sso';

const SESSION_LEDGER: Array<[string, string, 'go' | 'warn' | 'stop']> = [
  ['Logins', '1,204', 'go'],
  ['Keys issued', '318', 'go'],
  ['Leases active', '44', 'go'],
  ['Anomalies', '0', 'go'],
  ['Failed attempts', '7', 'warn'],
];

export function LoginPage({ onLogin }: LoginPageProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState<Mode>('password');

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

  const rootRef = useRef<HTMLDivElement>(null);

  useCosmicEntrance(rootRef, '.cz-login-aside-head, .cz-login-aside-body > *, .cz-login-aside-foot, .cz-login-card > *');


  return (
    <div className="cz-login-root" ref={rootRef}>
      <Starfield />
      {/* Centred on the viewport, not on the left pane: <CometField/> is a
          full-viewport canvas whose comets orbit the origin, which maps to
          the centre of the screen. With the hole drawn in the aside the
          comets were orbiting a point where nothing was rendered. */}
      <div className="cz-cosmos-hole" aria-hidden="true">
        <Singularity size={560} resolutionScale={0.5} />
      </div>
      <CometField />

      {/* Hero pane */}
      <aside className="cz-login-aside">
        <div className="cz-login-aside-head">
          <span className="cz-pill cz-pill-go">
            <span className="cz-dot cz-dot-go" /> eu-west-1 · healthy
          </span>
        </div>

        <div className="cz-login-aside-body">
          <span className="cz-eyebrow">Entry authorization</span>
          <h1 className="cz-login-title">
            Return to
            <br />
            <em>the keeping-place.</em>
          </h1>
          <p className="cz-login-sub">
            One identity provider for every secret, agent, and environment you hold — with
            lease-based access, audited end to end.
          </p>

          <div className="cz-login-stats">
            <div className="cz-eyebrow" style={{ marginBottom: 8 }}>
              Last 24 hours
            </div>
            {SESSION_LEDGER.map(([label, value, status], i) => (
              <div key={label} className="cz-login-stat">
                <span className="cz-ix">{String(i + 1).padStart(2, '0')}</span>
                <span className="cz-lb">{label}</span>
                <span className="cz-vl cz-num">{value}</span>
                <span className={`cz-dot cz-dot-${status}`} />
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

          <h2 className="cz-login-h2">Sign in</h2>
          <p className="cz-login-lead">
            New to KeepSave?{' '}
            <Link to="/register">Open an account →</Link>
          </p>

          <div className="cz-login-modes">
            {(
              [
                ['password', 'Email & password'],
                ['key', 'API key'],
                ['sso', 'SSO'],
              ] as Array<[Mode, string]>
            ).map(([m, label]) => (
              <button
                key={m}
                type="button"
                className={`cz-login-mode ${mode === m ? 'cz-on' : ''}`}
                onClick={() => setMode(m)}
              >
                {label}
              </button>
            ))}
          </div>

          {error && <div className="cz-login-error">{error}</div>}

          {mode === 'password' && (
            <form className="cz-login-form" onSubmit={handleSubmit}>
              <div className="cz-login-field">
                <label htmlFor="login-email">Email</label>
                <input
                  id="login-email"
                  className="cz-input"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@company.com"
                  required
                />
              </div>
              <div className="cz-login-field">
                <label htmlFor="login-password">
                  Password
                  <span className="cz-recover">RECOVER →</span>
                </label>
                <input
                  id="login-password"
                  className="cz-input"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  minLength={8}
                  required
                />
              </div>
              <label className="cz-login-check">
                <input type="checkbox" defaultChecked /> Trust this device for 30 days
              </label>
              <button
                type="submit"
                className="cz-btn cz-btn-primary"
                style={{ justifyContent: 'center', width: '100%' }}
                disabled={loading}
              >
                {loading ? 'Signing in…' : 'Enter the vault →'}
              </button>
            </form>
          )}

          {mode === 'key' && (
            <form className="cz-login-form" onSubmit={(e) => e.preventDefault()}>
              <div className="cz-login-field">
                <label htmlFor="login-key">API key</label>
                <input
                  id="login-key"
                  className="cz-input"
                  defaultValue="ks_live_8e42••••••••••••••••••••••1b"
                />
              </div>
              <p className="cz-faint" style={{ fontFamily: 'var(--cz-mono)', fontSize: 11 }}>
                For CLI and machine-to-machine. Keys are scoped — manage them under Agents → Leases.
              </p>
              <button
                type="submit"
                className="cz-btn cz-btn-primary"
                style={{ justifyContent: 'center', width: '100%' }}
                disabled
              >
                Authenticate →
              </button>
            </form>
          )}

          {mode === 'sso' && (
            <div className="cz-login-form">
              <button className="cz-btn" style={{ justifyContent: 'center', width: '100%' }} disabled>
                Continue with Okta
              </button>
              <button className="cz-btn" style={{ justifyContent: 'center', width: '100%' }} disabled>
                Continue with Google Workspace
              </button>
              <button className="cz-btn" style={{ justifyContent: 'center', width: '100%' }} disabled>
                Continue with GitHub
              </button>
              <p
                className="cz-faint"
                style={{ fontFamily: 'var(--cz-mono)', fontSize: 11, textAlign: 'center' }}
              >
                SAML · OIDC · SCIM configured at the org level.
              </p>
            </div>
          )}

          <div className="cz-login-divider" />
          <div className="cz-login-fine">
            <span>Signed requests · TLS 1.3</span>
            <span>CSRF · HSTS · CSP</span>
          </div>
        </div>
      </main>
    </div>
  );
}
