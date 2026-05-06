import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { login as apiLogin } from '../api/client';
import type { User } from '../types';

interface LoginPageProps {
  onLogin: (user: User, token: string) => void;
}

type Mode = 'password' | 'key' | 'sso';

const SESSION_LEDGER: Array<[string, string, 'go' | 'warn' | 'stop']> = [
  ['LOGINS', '1,204', 'go'],
  ['KEYS ISSUED', '  318', 'go'],
  ['LEASES ACTIVE', '   44', 'go'],
  ['ANOMALIES', '    0', 'go'],
  ['FAILED ATTEMPTS', '    7', 'warn'],
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

  return (
    <div className="ks-login-root">
      {/* Editorial left pane */}
      <aside className="ks-login-aside">
        <div className="ks-login-aside-head">
          <span className="ks-eyebrow ks-faint">— KeepSave · MMXXVI</span>
          <span className="ks-pill ks-pill-go">
            <span className="ks-dot ks-dot-go" /> EU-WEST-1 · HEALTHY
          </span>
        </div>

        <div className="ks-login-aside-body">
          <div className="ks-eyebrow ks-amber">Entry authorization</div>
          <h1 className="ks-login-title">
            Return to<br />
            <em>the keeping-place.</em>
          </h1>
          <p className="ks-dim" style={{ maxWidth: '42ch', marginTop: 14, fontSize: 13, lineHeight: 1.6 }}>
            One identity provider for every secret, agent, and environment you
            hold. Lease-based access. Audited end-to-end.
          </p>

          <div className="ks-login-inventory">
            <div className="ks-eyebrow ks-mute" style={{ marginBottom: 10 }}>
              SESSION LEDGER · LAST 24H
            </div>
            {SESSION_LEDGER.map(([label, value, status], i) => (
              <div key={label} className="ks-login-inv-row">
                <span className="ks-faint">{String(i + 1).padStart(2, '0')}</span>
                <span className="ks-dim">{label}</span>
                <span className="ks-num ks-amber" style={{ marginLeft: 'auto' }}>{value}</span>
                <span className={`ks-dot ks-dot-${status}`} />
              </div>
            ))}
          </div>
        </div>

        <div className="ks-login-aside-foot">
          <span className="ks-faint">MMXXVI · KeepSave Co-operative</span>
          <span className="ks-faint">build.13.42.7</span>
        </div>
      </aside>

      {/* Form pane */}
      <main className="ks-login-main">
        <div className="ks-login-card">
          <div className="ks-login-card-head">
            <span className="ks-eyebrow">— Check-in —</span>
            <span className="ks-faint" style={{ fontSize: 10 }}>Form IV-b</span>
          </div>

          <h2 className="ks-login-h2">Sign in</h2>
          <p className="ks-dim" style={{ fontSize: 12, marginTop: 6 }}>
            New to KeepSave?{' '}
            <Link
              to="/register"
              className="ks-amber"
              style={{ borderBottom: '1px dashed', textDecoration: 'none', cursor: 'pointer' }}
            >
              Open an account →
            </Link>
          </p>

          <div className="ks-login-modes">
            {(['password', 'key', 'sso'] as Mode[]).map((m) => (
              <button
                key={m}
                type="button"
                className={`ks-login-mode ${mode === m ? 'on' : ''}`}
                onClick={() => setMode(m)}
              >
                {m === 'password' && 'Email & password'}
                {m === 'key' && 'API key'}
                {m === 'sso' && 'SSO'}
              </button>
            ))}
          </div>

          {error && <div className="ks-error">{error}</div>}

          {mode === 'password' && (
            <form className="ks-login-form" onSubmit={handleSubmit}>
              <div className="ks-tweak-row">
                <label htmlFor="login-email">Email</label>
                <input
                  id="login-email"
                  className="ks-input"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@company.com"
                  required
                />
              </div>
              <div className="ks-tweak-row">
                <label htmlFor="login-password">
                  Password
                  <span
                    className="ks-amber"
                    style={{ float: 'right', fontSize: 9, cursor: 'pointer' }}
                  >
                    RECOVER →
                  </span>
                </label>
                <input
                  id="login-password"
                  className="ks-input"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  minLength={8}
                  required
                />
              </div>
              <label className="ks-login-check">
                <input type="checkbox" defaultChecked /> Trust this device · 30d
              </label>
              <button
                type="submit"
                className="ks-btn ks-btn-primary"
                style={{ justifyContent: 'center', width: '100%' }}
                disabled={loading}
              >
                {loading ? 'Signing in…' : 'Enter the vault →'}
              </button>
            </form>
          )}

          {mode === 'key' && (
            <form className="ks-login-form" onSubmit={(e) => e.preventDefault()}>
              <div className="ks-tweak-row">
                <label>API key</label>
                <input
                  className="ks-input"
                  defaultValue="ks_live_8e42••••••••••••••••••••••1b"
                />
              </div>
              <p className="ks-faint" style={{ fontSize: 10 }}>
                For CLI and M2M. Keys are scoped; see Agents → Leases.
              </p>
              <button
                type="submit"
                className="ks-btn ks-btn-primary"
                style={{ justifyContent: 'center', width: '100%' }}
                disabled
              >
                Authenticate →
              </button>
            </form>
          )}

          {mode === 'sso' && (
            <div className="ks-login-form">
              <button className="ks-btn" style={{ justifyContent: 'center', width: '100%' }} disabled>
                Continue with Okta
              </button>
              <button className="ks-btn" style={{ justifyContent: 'center', width: '100%' }} disabled>
                Continue with Google Workspace
              </button>
              <button className="ks-btn" style={{ justifyContent: 'center', width: '100%' }} disabled>
                Continue with GitHub
              </button>
              <p className="ks-faint" style={{ fontSize: 10, textAlign: 'center' }}>
                SAML · OIDC · SCIM configured at the org level.
              </p>
            </div>
          )}

          <hr className="ks-hair-soft" style={{ margin: '22px 0 14px' }} />
          <div className="ks-login-fine">
            <span className="ks-faint">Signed requests only · TLS 1.3</span>
            <span className="ks-faint">CSRF · HSTS · CSP</span>
          </div>
        </div>
      </main>
    </div>
  );
}
