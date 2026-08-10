import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { animate, stagger } from 'animejs';
import { Starfield } from '../components/cosmic/Starfield';
import { Singularity } from '../components/cosmic/Singularity';
import { KsMark } from '../components/cosmic/KsMark';

/**
 * LandingPage — the public front door.
 *
 * Visual language is Event Horizon, the same one the login screen and the
 * app shell speak: starfield ground, 14px glass panels, 99px pill buttons,
 * periwinkle-violet accent, mono for every datum. It reuses the `cz-*`
 * primitives from cosmic.css directly (.cz-btn, .cz-pill, .cz-card,
 * .cz-eyebrow, .cz-num, .cz-dot) so nothing here forks the design system.
 *
 * The hero is anchored by <Singularity>, a three.js/WebGL black hole —
 * the 3D sibling of the CSS <EventHorizon> on the login page. It falls
 * back to that CSS version if WebGL is unavailable.
 *
 * Motion is anime.js: one entrance timeline for the hero, then per-section
 * reveals triggered by IntersectionObserver. Everything is gated on
 * prefers-reduced-motion — see the guard in the layout effect and the
 * reduced-motion block in landing.css. Elements are only hidden after the
 * effect adds `.ks-anim`, so a failed anime.js load degrades to a static
 * page rather than a blank one.
 *
 * Copy rule: every claim is drawn from the repository (README.md,
 * CLAUDE.md, docs/, sdks/, integrations/). The promotion panel shows
 * illustrative product UI, not measured telemetry, and there are no
 * invented customers, logos or benchmarks.
 */

const REPO = 'https://github.com/santapong/KeepSave';

/* ------------------------------------------------------------------ */
/* data                                                                */
/* ------------------------------------------------------------------ */

type Health = 'go' | 'warn' | 'stop';
type DiffState = 'added' | 'changed' | 'same';

interface SecretRow {
  key: string;
  /** presence across [alpha, uat, prod] */
  envs: [boolean, boolean, boolean];
  state: DiffState;
}

interface Promotion {
  id: string;
  from: string;
  to: string;
  health: Health;
  note: string;
  rows: SecretRow[];
}

const PROMOTIONS: Promotion[] = [
  {
    id: 'prm_4b21c9',
    from: 'alpha',
    to: 'uat',
    health: 'go',
    note: 'applied',
    rows: [
      { key: 'DATABASE_URL', envs: [true, true, true], state: 'changed' },
      { key: 'OPENAI_API_KEY', envs: [true, true, false], state: 'added' },
      { key: 'REDIS_URL', envs: [true, true, false], state: 'same' },
      { key: 'JWT_SECRET', envs: [true, true, true], state: 'same' },
      { key: 'SMTP_PASSWORD', envs: [true, true, false], state: 'added' },
    ],
  },
  {
    id: 'prm_4b20f7',
    from: 'uat',
    to: 'prod',
    health: 'warn',
    note: 'awaiting 2nd approver',
    rows: [
      { key: 'DATABASE_URL', envs: [true, true, true], state: 'changed' },
      { key: 'STRIPE_SECRET_KEY', envs: [true, true, true], state: 'changed' },
      { key: 'WEBHOOK_SIGNING_KEY', envs: [true, true, true], state: 'added' },
      { key: 'JWT_SECRET', envs: [true, true, true], state: 'same' },
    ],
  },
  {
    id: 'prm_4b1e02',
    from: 'alpha',
    to: 'uat',
    health: 'go',
    note: 'applied',
    rows: [
      { key: 'FEATURE_MCP_HUB', envs: [true, true, false], state: 'added' },
      { key: 'LOG_LEVEL', envs: [true, true, true], state: 'changed' },
      { key: 'CORS_ORIGINS', envs: [true, true, true], state: 'same' },
    ],
  },
  {
    id: 'prm_4b1c88',
    from: 'uat',
    to: 'prod',
    health: 'stop',
    note: 'rolled back',
    rows: [
      { key: 'DATABASE_URL', envs: [true, true, true], state: 'changed' },
      { key: 'CORS_ORIGINS', envs: [true, true, true], state: 'changed' },
      { key: 'LEASE_TTL_SECONDS', envs: [true, true, false], state: 'added' },
    ],
  },
];

const ENV_ORDER = ['alpha', 'uat', 'prod'] as const;

const OAUTH_FLOWS = ['authorization code', 'client credentials', 'pkce', 'refresh token'];

const REACH: Array<[string, string]> = [
  ['sdks', 'go · node.js · python'],
  ['ci', 'github action · gitlab ci'],
  ['infra', 'terraform provider'],
  ['embed', '<keepsave-widget>'],
  ['deploy', 'docker compose · helm'],
];

const METRICS: Array<[string, string, string]> = [
  ['256', 'bit', 'AES-GCM envelope encryption. Per-project data keys, master key held in a KMS and never written to the database.'],
  ['4', 'flows', 'A full OAuth 2.0 provider: authorization code, client credentials, PKCE and refresh token.'],
  ['3', 'stages', 'Alpha to UAT to PROD. Every promotion is diff-reviewed, audit-logged and reversible.'],
  ['0', 'plaintext', 'No secret value is written to disk unsealed, and none is returned in an error response.'],
];

const NAV: Array<[string, string]> = [
  ['Vault', '#vault'],
  ['Gateway', '#gateway'],
  ['Security', '#guarantees'],
  ['Docs', REPO],
];

function motionOff(): boolean {
  if (typeof window === 'undefined') return true;
  if (document.documentElement.dataset.motion === 'off') return true;
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/* ------------------------------------------------------------------ */

export function LandingPage() {
  const [menuOpen, setMenuOpen] = useState(false);
  const [stuck, setStuck] = useState(false);
  const [selected, setSelected] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const promotion = PROMOTIONS[selected];

  /* --- header shadow on scroll ------------------------------------ */
  useEffect(() => {
    const onScrollY = () => setStuck(window.scrollY > 24);
    onScrollY();
    window.addEventListener('scroll', onScrollY, { passive: true });
    return () => window.removeEventListener('scroll', onScrollY);
  }, []);

  /* --- anime.js: hero entrance + per-section reveals ---------------
     useLayoutEffect, not useEffect: the .ks-anim class is what makes
     .ks-rise elements transparent, so it has to land before the browser
     paints or the page flashes its content and then hides it. */
  useLayoutEffect(() => {
    const root = rootRef.current;
    if (!root) return;

    // Reduced motion: never hide anything, animate nothing.
    if (motionOff()) return;

    // Opt in to the hidden start state only now that we know we can undo it.
    root.classList.add('ks-anim');

    const hero = Array.from(root.querySelectorAll<HTMLElement>('[data-hero] .ks-rise'));
    if (hero.length) {
      animate(hero, {
        opacity: [0, 1],
        translateY: [26, 0],
        filter: ['blur(10px)', 'blur(0px)'],
        duration: 1100,
        delay: stagger(110, { start: 180 }),
        ease: 'out(3)',
      });
    }

    // Sections reveal as they cross into view, once each.
    const sections = Array.from(root.querySelectorAll<HTMLElement>('[data-reveal]'));
    const io = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          const el = entry.target as HTMLElement;
          io.unobserve(el);
          const items = Array.from(el.querySelectorAll<HTMLElement>('.ks-rise'));
          if (!items.length) continue;
          animate(items, {
            opacity: [0, 1],
            translateY: [30, 0],
            duration: 900,
            delay: stagger(80),
            ease: 'out(3)',
          });
        }
      },
      { rootMargin: '0px 0px -12% 0px', threshold: 0.05 },
    );
    sections.forEach((s) => io.observe(s));

    // The scroll rail breathes so the hint reads as live.
    const rail = root.querySelector<HTMLElement>('.ks-scroll-rail');
    if (rail) {
      animate(rail, {
        opacity: [0.25, 1, 0.25],
        duration: 2600,
        loop: true,
        ease: 'inOut(2)',
      });
    }

    return () => io.disconnect();
  }, []);

  return (
    <div className="ks-landing" ref={rootRef}>
      <Starfield />

      {/* ---------------------------------------------------------- */}
      {/* header                                                      */}
      {/* ---------------------------------------------------------- */}
      <header className={`ks-header${stuck ? ' ks-stuck' : ''}`}>
        <a className="ks-brand" href="#top">
          <KsMark />
          <span className="ks-mk">
            Keep<em>save</em>
          </span>
        </a>

        <nav className="ks-nav" aria-label="Primary">
          {NAV.map(([label, href]) => (
            <a key={label} href={href}>
              {label}
            </a>
          ))}
        </nav>

        <div className="ks-header-actions">
          <Link className="cz-btn cz-btn-ghost ks-hide-sm" to="/login">
            Sign in
          </Link>
          <Link className="cz-btn cz-btn-primary" to="/register">
            Open an account →
          </Link>
          <button
            type="button"
            className="cz-btn ks-menu-btn"
            onClick={() => setMenuOpen(true)}
            aria-label="Open menu"
          >
            ☰
          </button>
        </div>
      </header>

      {menuOpen && (
        <div className="ks-menu">
          <div className="ks-menu-top">
            <span className="ks-brand">
              <KsMark />
              <span className="ks-mk">
                Keep<em>save</em>
              </span>
            </span>
            <button type="button" className="cz-btn" onClick={() => setMenuOpen(false)} aria-label="Close menu">
              ✕
            </button>
          </div>
          <nav>
            {NAV.map(([label, href]) => (
              <a key={label} href={href} onClick={() => setMenuOpen(false)}>
                {label}
              </a>
            ))}
          </nav>
          <div className="ks-menu-foot">
            <Link className="cz-btn" to="/login" onClick={() => setMenuOpen(false)}>
              Sign in
            </Link>
            <Link className="cz-btn cz-btn-primary" to="/register" onClick={() => setMenuOpen(false)}>
              Open an account →
            </Link>
          </div>
        </div>
      )}

      {/* ---------------------------------------------------------- */}
      {/* hero — the singularity                                      */}
      {/* ---------------------------------------------------------- */}
      <section className="ks-hero" id="top" data-hero>
        <div className="ks-hero-stage">
          <Singularity size={860} />
        </div>

        <div className="ks-hero-inner">
          <span className="cz-pill cz-pill-accent ks-rise">
            <span className="cz-dot cz-dot-go" /> self-hosted · docker · helm
          </span>

          <h1 className="ks-h1 ks-rise">
            Nothing escapes
            <br />
            <em>the keeping-place.</em>
          </h1>

          <p className="ks-lede ks-rise">
            KeepSave seals every value with AES-256-GCM under a per-project key, then releases it to your agents,
            pipelines and MCP servers on demand — so it never lands in a <code className="cz-mono">.env</code>, a prompt,
            or a chat log.
          </p>

          <div className="ks-hero-cta ks-rise">
            <Link className="cz-btn cz-btn-primary" to="/register">
              Open an account →
            </Link>
            <a className="cz-btn" href={REPO} target="_blank" rel="noreferrer">
              Read the docs
            </a>
          </div>

          <p className="ks-fine ks-hero-foot ks-rise">
            go 1.24 · postgresql · mysql · sqlite · prometheus · opentelemetry
          </p>
        </div>

        <div className="ks-scroll-hint" aria-hidden="true">
          <span className="ks-fine">scroll</span>
          <span className="ks-scroll-rail" />
        </div>
      </section>

      {/* ---------------------------------------------------------- */}
      {/* 01 · vault                                                  */}
      {/* ---------------------------------------------------------- */}
      <section className="ks-section" id="vault" data-reveal>
        <div className="ks-shell">
          <div className="ks-head">
            <div className="ks-eyebrow-row ks-rise">
              <span className="ks-ix">01</span>
              <span className="cz-eyebrow">Vault</span>
            </div>
            <h2 className="ks-h2 ks-rise">
              Promote environments.
              <br />
              <em>Don&rsquo;t copy-paste them.</em>
            </h2>
            <p className="ks-lede ks-rise">
              Every promotion previews its diff before it applies, writes an audit row when it does, and can be rolled
              back. PROD can require a second approver. Select a promotion to see what moved.
            </p>
          </div>

          <div className="cz-card ks-panel ks-rise">
            <div className="ks-panel-bar">
              <span className="cz-num" style={{ fontSize: 13, color: 'var(--cz-accent-hi)' }}>
                {promotion.id}
              </span>
              <div className="ks-panel-chips">
                <span className="cz-pill">
                  {promotion.from} → {promotion.to}
                </span>
                <span className="cz-pill">{promotion.rows.length} keys</span>
                <span className={`cz-pill ${promotion.health === 'go' ? 'cz-pill-go' : promotion.health === 'stop' ? 'cz-pill-stop' : ''}`}>
                  <span className={`cz-dot cz-dot-${promotion.health}`} />
                  {promotion.note}
                </span>
              </div>
            </div>

            <div className="ks-panel-body">
              <div className="ks-panel-side" role="tablist" aria-label="Promotions">
                {PROMOTIONS.map((p, i) => (
                  <button
                    key={p.id}
                    type="button"
                    role="tab"
                    className="ks-run"
                    aria-selected={i === selected}
                    onClick={() => setSelected(i)}
                  >
                    <span className={`cz-dot cz-dot-${p.health}`} />
                    {p.id}
                    <span className="ks-run-to">{p.to}</span>
                  </button>
                ))}
              </div>

              <div className="ks-panel-main">
                <div className="ks-drow ks-dhead">
                  <span>key</span>
                  <span>alpha · uat · prod</span>
                  <span style={{ textAlign: 'right' }}>state</span>
                </div>

                {promotion.rows.map((row) => (
                  <div className="ks-drow" key={row.key}>
                    <span className="ks-dkey">{row.key}</span>

                    <span className="ks-track" aria-hidden="true">
                      {row.envs.map((present, i) =>
                        present ? (
                          <span
                            key={ENV_ORDER[i]}
                            className={`ks-seg${
                              ENV_ORDER[i] === promotion.to && row.state !== 'same' ? ` is-${row.state}` : ''
                            }`}
                            style={{ left: `${i * 34}%`, width: '30%' }}
                          />
                        ) : null,
                      )}
                    </span>

                    <span className={`ks-dstate is-${row.state}`}>{row.state}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ---------------------------------------------------------- */}
      {/* 02 · gateway                                                */}
      {/* ---------------------------------------------------------- */}
      <section className="ks-section ks-section--tight" id="gateway" data-reveal>
        <div className="ks-shell">
          <div className="ks-head">
            <div className="ks-eyebrow-row ks-rise">
              <span className="ks-ix">02</span>
              <span className="cz-eyebrow">Gateway</span>
            </div>
            <h2 className="ks-h2 ks-rise">
              One hop between your agents
              <br />
              and <em>everything else.</em>
            </h2>
            <p className="ks-lede ks-rise">
              Register MCP servers from GitHub, route tool calls through a single gateway, and let KeepSave inject the
              secrets each server needs as environment variables at call time.
            </p>
          </div>

          <div className="ks-bento">
            <article className="cz-card ks-rise">
              <span className="cz-eyebrow">MCP hub</span>
              <h3 className="ks-h3">Secrets injected at call time.</h3>
              <p>
                JSON-RPC 2.0 in, tool result out. The agent never receives the credential — the gateway resolves it,
                hands it to the server as an env var, and logs the call.
              </p>
              <pre className="ks-codeblock">
                <span className="i">{'POST /mcp/gateway/tools/call\n'}</span>
                {'{ "server": '}
                <span className="a">{'"gmail"'}</span>
                {', "tool": '}
                <span className="a">{'"send"'}</span>
                {' }\n\n'}
                {'resolve  '}
                <span className="g">{'GMAIL_TOKEN → env\n'}</span>
                {'audit    '}
                <span className="i">{'mcp.tool.called'}</span>
              </pre>
            </article>

            <article className="cz-card ks-rise">
              <span className="cz-eyebrow">Identity</span>
              <h3 className="ks-h3">A full OAuth 2.0 provider.</h3>
              <p>
                Issue and rotate tokens for your own apps and agents without standing up a second identity service.
                Scoped API keys cover the machine-to-machine case.
              </p>
              <div className="ks-pillrow">
                {OAUTH_FLOWS.map((flow) => (
                  <span className="cz-pill" key={flow}>
                    {flow}
                  </span>
                ))}
              </div>
            </article>

            <article className="cz-card ks-rise">
              <span className="cz-eyebrow">Reach</span>
              <h3 className="ks-h3">Wherever the secret is needed.</h3>
              <p>Three SDKs, two CI integrations, a Terraform provider, and an embeddable widget for your own dashboard.</p>
              <div className="ks-kv">
                {REACH.map(([k, v]) => (
                  <div className="ks-kv-row" key={k}>
                    <span>{k}</span>
                    <span>{v}</span>
                  </div>
                ))}
              </div>
            </article>
          </div>
        </div>
      </section>

      {/* ---------------------------------------------------------- */}
      {/* 03 · guarantees                                             */}
      {/* ---------------------------------------------------------- */}
      <section className="ks-section ks-section--tight" id="guarantees" data-reveal>
        <div className="ks-shell">
          <div className="ks-head">
            <div className="ks-eyebrow-row ks-rise">
              <span className="ks-ix">03</span>
              <span className="cz-eyebrow">Guarantees</span>
            </div>
            <h2 className="ks-h2 ks-rise">
              What the design <em>actually commits to.</em>
            </h2>
            <p className="ks-lede ks-rise">
              These are properties of the system, not benchmarks. The threat model and the ASVS audit are in the
              repository if you want the working.
            </p>
          </div>

          <div className="ks-metrics ks-rise">
            {METRICS.map(([value, unit, label]) => (
              <div className="ks-metric" key={label}>
                <div>
                  <span className="ks-metric-value">{value}</span>
                  <span className="ks-metric-unit">{unit}</span>
                </div>
                <p className="ks-metric-label">{label}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ---------------------------------------------------------- */}
      {/* close                                                       */}
      {/* ---------------------------------------------------------- */}
      <section className="ks-close" data-reveal>
        <h2 className="ks-h2 ks-rise">
          Run it <em>yourself.</em>
        </h2>
        <p className="ks-lede ks-rise">
          Clone the repository, generate a master key, and bring the stack up with Docker Compose. The API listens on
          8080, the dashboard on 3000.
        </p>
        <div className="ks-close-cta ks-rise">
          <Link className="cz-btn cz-btn-primary" to="/register">
            Open an account →
          </Link>
          <a className="cz-btn" href={REPO} target="_blank" rel="noreferrer">
            View on GitHub
          </a>
        </div>
      </section>

      {/* ---------------------------------------------------------- */}
      {/* footer                                                      */}
      {/* ---------------------------------------------------------- */}
      <footer className="ks-footer">
        <span className="ks-fine">KeepSave · encrypted vault · oauth provider · mcp hub</span>
        <div className="ks-footer-links">
          <a href={`${REPO}#readme`} target="_blank" rel="noreferrer">
            Docs
          </a>
          <a href={`${REPO}/blob/main/SECURITY_AUDIT.md`} target="_blank" rel="noreferrer">
            Security
          </a>
          <a href={`${REPO}/blob/main/docs/THREAT_MODEL.md`} target="_blank" rel="noreferrer">
            Threat model
          </a>
          <a href={REPO} target="_blank" rel="noreferrer">
            GitHub
          </a>
        </div>
      </footer>
    </div>
  );
}
