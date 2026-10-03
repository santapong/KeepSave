import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { Link } from 'react-router-dom';
import { ArrowDown, ArrowRight, ArrowUpRight, Bot, Check, ChevronDown, Code2, FileCode2, FolderClosed, GitBranch, History, KeyRound, LockKeyhole, Menu, Server, ShieldCheck, Terminal, X } from '@/components/icons';
import { Brand } from '../components/cosmic/Brand';
import { EhMark } from '../components/cosmic/EhMark';
import { EventHorizon } from '../components/cosmic/EventHorizon';
import '../styles/landing.css';

const REPO = 'https://github.com/santapong/KeepSave';
const DOCS = `${REPO}/tree/develop/docs/system`;
const DOC_FILE = `${REPO}/blob/develop/docs/system`;
const ENVIRONMENTS = [
  { id: 'alpha', name: 'Alpha', purpose: 'Build and experiment', description: 'Development keys live here. Work on your app without reaching into testing or production.' },
  { id: 'uat', name: 'UAT', purpose: 'Test before release', description: 'User acceptance testing has its own values. Try a change here before it reaches your live app.' },
  { id: 'prod', name: 'PROD', purpose: 'Run your live app', description: 'Production values stay in their own environment. In this example, promotion requires a review.' },
] as const;
type Environment = typeof ENVIRONMENTS[number]['id'];
const KEYS = ['DATABASE_URL', 'PAYMENTS_API_KEY', 'SESSION_SECRET'];
const CHAPTERS = [
  { title: 'Store your secrets', short: 'Store', icon: LockKeyhole, caption: 'A home for every key.', detail: 'Add your keys to a project. KeepSave encrypts the values at rest and keeps each environment separate.' },
  { title: 'Scope the access', short: 'Scope', icon: KeyRound, caption: 'The right access. The right environment.', detail: 'Give a tool a read-only API key for one project and environment. Its access ends at that boundary.' },
  { title: 'Review the change', short: 'Review', icon: GitBranch, caption: 'A deliberate step toward production.', detail: 'Compare selected changes before promotion. Configure production approvals and keep an audit record of the change.' },
];

function EnvironmentPicker({ value, onChange, label }: { value: Environment; onChange: (value: Environment) => void; label: string }) {
  return <div className="ks-env-picker" role="group" aria-label={label}>
    {ENVIRONMENTS.map(env => <button type="button" key={env.id} aria-pressed={value === env.id} onClick={() => onChange(env.id)}>
      <span className={`ks-env-dot ks-env-${env.id}`} /><span>{env.name}<small>{env.purpose}</small></span>
      {value === env.id && <Check size={15} aria-hidden="true" />}
    </button>)}
  </div>;
}

function VaultWalkthrough() {
  const [chapter, setChapter] = useState(0);
  const [environment, setEnvironment] = useState<Environment>('alpha');
  const [scope, setScope] = useState<Environment>('uat');
  const activeEnvironment = ENVIRONMENTS.find(env => env.id === environment)!;
  const activeScope = ENVIRONMENTS.find(env => env.id === scope)!;
  const current = CHAPTERS[chapter];

  function moveTab(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let next = index;
    if (event.key === 'ArrowRight') next = (index + 1) % CHAPTERS.length;
    else if (event.key === 'ArrowLeft') next = (index + CHAPTERS.length - 1) % CHAPTERS.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = CHAPTERS.length - 1;
    else return;
    event.preventDefault();
    setChapter(next);
    document.getElementById(`ks-chapter-${next}`)?.focus();
  }

  return <div className="ks-walkthrough">
    <div className="ks-chapters" role="tablist" aria-label="Explore the KeepSave workflow">
      {CHAPTERS.map((item, index) => <button type="button" role="tab" id={`ks-chapter-${index}`} key={item.short}
        aria-selected={chapter === index} aria-controls="ks-demo-panel" tabIndex={chapter === index ? 0 : -1}
        onKeyDown={event => moveTab(event, index)} onClick={() => setChapter(index)}>
        <span className="ks-chapter-number">0{index + 1}</span><span>{item.title}</span><item.icon size={17} aria-hidden="true" />
      </button>)}
    </div>
    <div className="ks-demo-window" role="tabpanel" id="ks-demo-panel" aria-labelledby={`ks-chapter-${chapter}`} tabIndex={0}>
      <aside className="ks-demo-sidebar" aria-label="Example project context">
        <div className="ks-demo-brand"><EhMark size={30} />KeepSave</div>
        <span className="ks-sidebar-caption">WORKSPACE</span>
        <div className={chapter === 0 ? 'ks-sidebar-item is-current' : 'ks-sidebar-item'}><FolderClosed size={17} /> Projects</div>
        <div className={chapter === 1 ? 'ks-sidebar-item is-current' : 'ks-sidebar-item'}><KeyRound size={17} /> API keys</div>
        <div className={chapter === 2 ? 'ks-sidebar-item is-current' : 'ks-sidebar-item'}><GitBranch size={17} /> Promotions</div>
        <div className="ks-sidebar-item"><History size={17} /> Audit log</div>
        <div className="ks-sidebar-bottom"><ShieldCheck size={19} /><span>Your project.<br />Your boundaries.</span></div>
      </aside>
      <div className="ks-demo-main">
        <div className="ks-demo-topline"><span><FolderClosed size={13} /> Projects <span>/</span> storefront</span><span className="ks-example-label">Illustrative example</span></div>
        <div className="ks-demo-heading"><div><h3>{chapter === 0 ? 'One project. Every environment.' : chapter === 1 ? 'Give your tool a clear boundary.' : 'Know what moves forward.'}</h3><p>{chapter === 0 ? 'The same key names. Separate values for each stage.' : chapter === 1 ? 'A sample read-only key for the storefront project.' : 'Example promotion · UAT → PROD'}</p></div><current.icon size={23} aria-hidden="true" /></div>
        {chapter === 0 && <div className="ks-stage-content">
          <EnvironmentPicker value={environment} onChange={setEnvironment} label="Example vault environment" />
          <div className="ks-key-table" role="table" aria-label={`${activeEnvironment.name} example secrets`}>
            <div role="row" className="ks-key-head"><span role="columnheader">KEY</span><span role="columnheader">VALUE</span><span role="columnheader">VISIBILITY</span></div>
            {KEYS.map(key => <div role="row" key={key} className="ks-key-row"><span role="cell"><KeyRound size={14} aria-hidden="true" />{key}</span><span role="cell" className="ks-masked" aria-label="Masked example value">••••••••••••</span><span role="cell" className="ks-hidden-label"><LockKeyhole size={12} aria-hidden="true" /> Hidden</span></div>)}
          </div>
          <p className="ks-environment-note" role="status"><ShieldCheck size={16} aria-hidden="true" /><span><strong>{activeEnvironment.name}:</strong> {activeEnvironment.description}</span></p>
        </div>}
        {chapter === 1 && <div className="ks-stage-content">
          <EnvironmentPicker value={scope} onChange={setScope} label="Example agent access environment" />
          <div className="ks-access-example">
            <div className="ks-access-identity"><div className="ks-tool-icon"><Bot size={27} /></div><div><strong>storefront-agent</strong><span>Project: storefront · Read-only</span></div><span className="ks-scope-chip">{activeScope.name}</span></div>
            <div className="ks-permission allowed"><Check size={16} /><span>Read secrets in {activeScope.name}</span><strong>Allowed</strong></div>
            <div className="ks-permission"><X size={16} /><span>Access other environments or projects</span><strong>Not allowed</strong></div>
            <div className="ks-permission"><X size={16} /><span>Create, edit, or delete secrets</span><strong>Not allowed</strong></div>
          </div>
          <p className="ks-environment-note" role="status"><KeyRound size={16} aria-hidden="true" /><span>This example key is scoped to <strong>storefront / {activeScope.name}</strong>. No key is created by this walkthrough.</span></p>
        </div>}
        {chapter === 2 && <div className="ks-stage-content">
          <div className="ks-review-route"><span><span className="ks-env-dot ks-env-uat" /> UAT <small>Tested configuration</small></span><ArrowRight size={22} /><span><span className="ks-env-dot ks-env-prod" /> PROD <small>Live application</small></span></div>
          <div className="ks-change-list"><div><KeyRound size={15} /><span>PAYMENTS_API_KEY</span><span className="ks-change-tag">Updated</span></div><div><KeyRound size={15} /><span>SESSION_SECRET</span><span className="ks-change-tag">Updated</span></div><div><KeyRound size={15} /><span>DATABASE_URL</span><span className="ks-unchanged-tag">Unchanged</span></div></div>
          <details className="ks-diff"><summary>What would change?<ChevronDown size={15} /></summary><p>The two updated keys are selected for this example promotion. DATABASE_URL stays as it is in PROD. Values stay masked in this walkthrough.</p></details>
          <p className="ks-review-note"><ShieldCheck size={18} aria-hidden="true" /><span><strong>Approval required in this example.</strong> Your team configures the production approval policy. Nothing is promoted here.</span></p>
        </div>}
      </div>
    </div>
    <div className="ks-demo-caption"><div aria-live="polite"><span className="ks-eyebrow">0{chapter + 1} / {current.short}</span><h3>{current.caption}</h3><p>{current.detail}</p></div><button type="button" className="ks-text-link" onClick={() => setChapter((chapter + 1) % CHAPTERS.length)}>{chapter === 0 ? 'Next: scope the access' : chapter === 1 ? 'Next: review the change' : 'Back to the vault'}<ArrowRight size={17} aria-hidden="true" /></button></div>
  </div>;
}

export function LandingPage() {
  const [menuOpen, setMenuOpen] = useState(false);
  const [artFailed, setArtFailed] = useState(false);
  const [integration, setIntegration] = useState<'apps' | 'mcp'>('apps');
  const header = useRef<HTMLElement>(null);
  const menuButton = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!menuOpen) return;
    const dismiss = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') { setMenuOpen(false); menuButton.current?.focus(); }
    };
    const outside = (event: PointerEvent) => { if (event.target instanceof Node && !header.current?.contains(event.target)) setMenuOpen(false); };
    document.addEventListener('keydown', dismiss);
    document.addEventListener('pointerdown', outside);
    return () => { document.removeEventListener('keydown', dismiss); document.removeEventListener('pointerdown', outside); };
  }, [menuOpen]);

  return <div className="ks-landing" id="top">
    <a className="ks-skip" href="#main">Skip to content</a>
    <header ref={header} className="ks-header">
      <div className="ks-nav ks-container">
        <Brand className="ks-brand" size={36} />
        <nav className="ks-desktop-nav" aria-label="Main navigation"><a href="#product">Product</a><a href="#workflow">How it works</a><a href="#developers">Developers</a><a href={DOCS}>Docs <ArrowUpRight size={13} /></a></nav>
        <div className="ks-nav-actions"><Link to="/login" className="ks-sign-in">Sign in</Link><Link to="/register" className="ks-button ks-button-small">Get started <ArrowRight size={14} /></Link><button ref={menuButton} type="button" className="ks-menu-toggle" aria-expanded={menuOpen} aria-controls="ks-mobile-nav" aria-label={menuOpen ? 'Close navigation' : 'Open navigation'} onClick={() => setMenuOpen(!menuOpen)}>{menuOpen ? <X size={21} /> : <Menu size={21} />}</button></div>
      </div>
      {menuOpen && <nav id="ks-mobile-nav" className="ks-mobile-nav" aria-label="Mobile navigation" onClick={() => setMenuOpen(false)}><a href="#product">Product</a><a href="#workflow">How it works</a><a href="#developers">Developers</a><a href={DOCS}>Documentation <ArrowUpRight size={15} /></a><Link to="/login">Sign in</Link></nav>}
    </header>
    <main id="main" tabIndex={-1}>
      <section className="ks-hero" aria-labelledby="ks-hero-title">
        <div className="ks-hero-art" aria-hidden="true">{artFailed ? <div className="ks-art-fallback"><EventHorizon size={540} /></div> : <img src="/images/event-horizon.webp" width="1536" height="1024" fetchPriority="high" alt="" onError={() => setArtFailed(true)} />}</div>
        <div className="ks-hero-inner ks-container"><div className="ks-hero-copy"><div className="ks-eyebrow"><span className="ks-live-dot" /> SECRETS MANAGEMENT FOR TEAMS & AGENTS</div><h1 id="ks-hero-title">Your secrets.<br />In the right<br /><span>orbit.</span></h1><p className="ks-lead">Your apps need secret keys. KeepSave gives them one encrypted home, with controlled access and a clear path to production.</p><div className="ks-actions"><Link to="/register" className="ks-button">Create your vault <ArrowRight size={17} /></Link><a href="#product" className="ks-button ks-button-secondary">See how it works <ArrowDown size={16} /></a></div></div></div>
        <div className="ks-proof ks-container"><span><LockKeyhole size={14} />Encrypted at rest</span><span><KeyRound size={14} />Scoped access</span><span><Code2 size={15} />Open source & self-hostable</span></div>
      </section>

      <section id="product" className="ks-product-section ks-container" aria-labelledby="ks-product-title">
        <div className="ks-section-intro"><div><span className="ks-eyebrow">LET’S FOLLOW ONE PROJECT</span><h2 id="ks-product-title">It starts with a few keys.</h2></div><p>A database URL. A payment key. A session secret.<br className="ks-desktop-break" /> Then a teammate, a test environment, an AI agent.<br className="ks-desktop-break" /> Here’s how KeepSave keeps it all organized.</p></div>
        <VaultWalkthrough />
      </section>

      <section id="workflow" className="ks-story ks-container" aria-labelledby="ks-story-title">
        <div className="ks-story-intro"><span className="ks-eyebrow">LESS COPY-PASTE. MORE CONTROL.</span><h2 id="ks-story-title">A few keys.<br /><span>Too many copies.</span></h2><p>When configuration is scattered across files, messages, and deployment notes, a simple change becomes a guessing game.</p></div>
        <div className="ks-story-columns"><div className="ks-scattered" aria-label="Illustration of scattered secret copies"><span className="ks-small-label">A FAMILIAR STARTING POINT</span><div className="ks-file-card"><FileCode2 size={20} /><div><strong>.env.local</strong><span>Which value is current?</span></div><span className="ks-file-mask">••••••</span></div><div className="ks-file-card"><Terminal size={20} /><div><strong>deployment notes</strong><span>Was production updated?</span></div><span className="ks-file-mask">••••••</span></div><div className="ks-file-card"><Code2 size={20} /><div><strong>team chat</strong><span>Who else has this copy?</span></div><span className="ks-file-mask">••••••</span></div><div className="ks-scattered-bottom"><ArrowRight size={19} /><span>Give those keys a home in KeepSave.</span></div></div>
        <div className="ks-story-outcomes"><article><span>01</span><div><h3>One project. One place to look.</h3><p>Store named secrets in an encrypted vault. Separate values for development, testing, and production keep each stage clear.</p></div></article><article><span>02</span><div><h3>Access with a boundary.</h3><p>Give an application or agent a key scoped to its project and environment. Development access doesn’t need to include production.</p></div></article><article><span>03</span><div><h3>A change you can follow.</h3><p>Review what moves between environments, configure approvals for production, and look back through the audit trail.</p></div></article></div></div>
      </section>

      <section id="developers" className="ks-developers" aria-labelledby="ks-developers-title"><div className="ks-container"><div className="ks-section-intro"><div><span className="ks-eyebrow">KEEP THE TOOLS YOU KNOW</span><h2 id="ks-developers-title">Meet your secrets<br /><span>where you build.</span></h2></div><p>Use KeepSave from your code and pipelines, or connect tools through its MCP gateway. Choose the path that fits your project.</p></div>
        <div className="ks-integration-picker" role="group" aria-label="Integration example"><button type="button" aria-pressed={integration === 'apps'} onClick={() => setIntegration('apps')}><Terminal size={17} /> Apps & pipelines</button><button type="button" aria-pressed={integration === 'mcp'} onClick={() => setIntegration('mcp')}><Bot size={17} /> MCP tools</button></div>
        <div className="ks-integration-content"><div className="ks-integration-copy"><h3>{integration === 'apps' ? 'Your configuration. A central source.' : 'Connect tools through one gateway.'}</h3><p>{integration === 'apps' ? 'Access the secrets for your project and environment through the CLI or an SDK. Keep access scoped to the work that needs it.' : 'KeepSave resolves configured secrets for an MCP server at execution time. Your agent calls the tool through the gateway.'}</p><a className="ks-text-link" href={integration === 'apps' ? `${DOC_FILE}/13-sdks-integrations.md` : `${DOC_FILE}/03-api-reference.md`}>{integration === 'apps' ? 'Explore SDKs & integrations' : 'Read the gateway documentation'}<ArrowUpRight size={16} /></a></div>
          <div className="ks-connection" aria-label={integration === 'apps' ? 'Illustrative flow from scoped project secrets to an application or pipeline' : 'Illustrative flow from an agent through KeepSave to an MCP server'}><span className="ks-small-label">ILLUSTRATIVE FLOW</span><div className="ks-connection-nodes"><div><div className="ks-connection-icon">{integration === 'apps' ? <FolderClosed size={26} /> : <Bot size={26} />}</div><strong>{integration === 'apps' ? 'storefront / UAT' : 'Your agent'}</strong><span>{integration === 'apps' ? 'Scoped project secrets' : 'Requests a tool call'}</span></div><ArrowRight className="ks-connection-arrow" size={22} /><div><div className="ks-connection-icon ks-connection-core"><EhMark size={43} /></div><strong>KeepSave</strong><span>{integration === 'apps' ? 'CLI or SDK' : 'MCP gateway'}</span></div><ArrowRight className="ks-connection-arrow" size={22} /><div><div className="ks-connection-icon">{integration === 'apps' ? <Terminal size={26} /> : <Server size={26} />}</div><strong>{integration === 'apps' ? 'Your application' : 'MCP server'}</strong><span>{integration === 'apps' ? 'Uses its configuration' : 'Runs the tool'}</span></div></div></div>
        </div><div className="ks-stack-row"><span>BUILD YOUR WAY</span><a href={`${REPO}/tree/develop/backend/cmd/keepsave`}>CLI <ArrowUpRight size={12} /></a><a href={`${REPO}/tree/develop/sdks/python`}>Python <ArrowUpRight size={12} /></a><a href={`${REPO}/tree/develop/sdks/nodejs`}>Node.js <ArrowUpRight size={12} /></a><a href={`${REPO}/tree/develop/sdks/go`}>Go <ArrowUpRight size={12} /></a><a href={`${REPO}/tree/develop/integrations`}>CI/CD <ArrowUpRight size={12} /></a></div>
      </div></section>

      <section className="ks-faq ks-container" aria-labelledby="ks-faq-title"><div><span className="ks-eyebrow">A FEW THINGS WORTH KNOWING</span><h2 id="ks-faq-title">Before you<br /><span>get started.</span></h2></div><div className="ks-faq-list">
        <details><summary>What counts as a secret?<ChevronDown size={18} /></summary><p>An API key, a database connection URL, a password, or another configuration value your software needs to access a service. KeepSave stores these as named values inside a project.</p></details>
        <details><summary>What are Alpha, UAT, and PROD?<ChevronDown size={18} /></summary><p>Alpha is for development. UAT means user acceptance testing: a place to check a change before release. PROD is your live application. KeepSave gives each environment its own secret values.</p></details>
        <details><summary>Does changing a key update production automatically?<ChevronDown size={18} /></summary><p>No. Changing a value in one environment does not promote it to another. A promotion is a separate action. Your team can configure approval requirements for production changes.</p></details>
        <details><summary>Can I run KeepSave on my own infrastructure?<ChevronDown size={18} /></summary><p>Yes. KeepSave is open source and self-hostable. The repository includes Docker Compose setup instructions. You operate the service, its database, and its root encryption key. <a href={`${REPO}#quick-start`}>Read the setup guide <ArrowUpRight size={13} /></a></p></details>
      </div></section>

      <section className="ks-final ks-container" aria-labelledby="ks-final-title"><div className="ks-final-orbit" aria-hidden="true" /><span className="ks-eyebrow">YOUR NEXT PROJECT STARTS HERE</span><h2 id="ks-final-title">Keep your secrets.<br /><span>Keep your momentum.</span></h2><p>Give your team and your tools a clearer way to work.</p><div className="ks-actions"><Link to="/register" className="ks-button">Create your vault <ArrowRight size={17} /></Link><a href={`${REPO}#quick-start`} className="ks-button ks-button-secondary">Self-host KeepSave <ArrowUpRight size={16} /></a></div></section>
    </main>
    <footer className="ks-footer ks-container"><div><Brand className="ks-brand" size={30} /><p>Encrypted secrets. Clear boundaries.</p></div><nav aria-label="Footer navigation"><a href={DOCS}>Documentation <ArrowUpRight size={13} /></a><a href={REPO}>GitHub <ArrowUpRight size={13} /></a><Link to="/login">Sign in</Link><a href="#top">Back to top <ArrowUpRight size={13} /></a></nav><span className="ks-footer-caption">KEEPSAVE / EVENT HORIZON</span></footer>
  </div>;
}
