import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { Link } from 'react-router-dom';
import { ArrowDown, ArrowRight, ArrowUpRight, Bot, Check, ChevronDown, Code2, FolderClosed, GitBranch, History, KeyRound, LockKeyhole, Menu, MessageCircle, Server, ShieldCheck, Users, X } from '@/components/icons';
import { Brand } from '../components/cosmic/Brand';
import { EhMark } from '../components/cosmic/EhMark';
import { EventHorizon } from '../components/cosmic/EventHorizon';
import '../styles/landing.css';

const REPO = 'https://github.com/santapong/KeepSave';
const DOCS = `${REPO}/tree/develop/docs`;
const SETUP = `${REPO}#local-development`;
const FEEDBACK = `${REPO}/issues/new?title=KeepSave%20workflow%20feedback&body=My%20team%27s%20workflow%3A%20%0A%0AWhat%20I%27d%20like%20to%20improve%3A%20%0A%0APlease%20do%20not%20include%20credentials%20or%20private%20data.`;
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

const USE_CASES = [
  { number: '01', label: 'FOR YOUR APPLICATIONS', title: 'Ship without the secret sprawl.', description: 'Give each project a home for its keys, from the first local build to the next production change.', icon: Code2, destination: '#product', link: 'Explore the vault' },
  { number: '02', label: 'FOR YOUR TEAM', title: 'Bring everyone into the same orbit.', description: 'Keep environments distinct and make access a deliberate choice, even as your team grows.', icon: Users, destination: '#features', link: 'See team controls' },
  { number: '03', label: 'FOR YOUR TOOLS', title: 'A smaller boundary. A clearer task.', description: 'Explore controlled access for developer tools, with permission checks around the work they do.', icon: Bot, destination: '#tool-access', link: 'Explore the preview' },
];
const FEATURES = [
  { icon: LockKeyhole, label: '01 / ORGANIZE', title: 'Every environment, in its place.', text: 'Keep Alpha, UAT and production values separate inside an encrypted project vault.', detail: 'Encrypted storage · Project vaults' },
  { icon: KeyRound, label: '02 / CONTROL', title: 'Give access a clear boundary.', text: 'Scope keys to a project and environment, and revoke browser sessions when access should end.', detail: 'Scoped keys · Revocable sessions' },
  { icon: History, label: '03 / RECOVER', title: 'A history you can return to.', text: 'Inspect earlier revisions and restore a selected value as a new change, with retained key history.', detail: 'Secret revisions · Selected restoration' },
  { icon: GitBranch, label: '04 / REVIEW', title: 'Know what moves forward.', text: 'Compare environment changes, configure promotion approvals, and follow the recorded activity.', detail: 'Deliberate promotion · Audit trail' },
];
const VOICES = [
  { initials: 'PE', role: 'Platform engineer', quote: 'One place for the keys. One clear answer when someone asks which value is current.' },
  { initials: 'DL', role: 'Development lead', quote: 'I can give a tool the access it needs without opening up every project.' },
  { initials: 'FD', role: 'Full-stack developer', quote: 'Seeing the change before it reaches production makes the whole handoff feel calmer.' },
];
const FAQS = [
  { question: 'What does KeepSave help me manage?', answer: 'API keys, database URLs, passwords and other configuration secrets. Group them by project, keep different values for Alpha, UAT and production, and decide which people or tools can access them.' },
  { question: 'Does changing a secret update production automatically?', answer: 'No. Updating a value in one environment does not promote it to another. Promotion is a separate action, and your team can configure approval requirements for production changes.' },
  { question: 'Can I self-host KeepSave?', answer: 'Yes. The repository contains the self-hosted application and setup references. Your team operates the service, database and encryption-key infrastructure. The current source candidate still needs installation-specific recovery, security review and operational acceptance before production use.' },
  { question: 'Can I look back at an earlier secret?', answer: 'The PostgreSQL vault records immutable revisions. Authorized users can inspect history and restore a selected revision as a new change. Recovery requires the documented key material; backups do not restore login sessions or access grants.' },
  { question: 'Will it work with my AI coding tool?', answer: 'Controlled tool access is a preview built around shared MCP and authorization contracts. Codex and Hermes are initial qualification candidates. Compatibility is tested per client version; KeepSave does not promise universal harness support or expose provider tokens to the model.' },
  { question: 'Can I sign up for a hosted workspace today?', answer: 'This page introduces the project and includes an illustrative product walkthrough. The public hosted application has not launched. Explore the source and self-hosting references, or share your workflow to help shape the next release.' },
];

export function LandingPage({ landingOnly = false }: { landingOnly?: boolean }) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [artFailed, setArtFailed] = useState(false);
  const header = useRef<HTMLElement>(null);
  const menuButton = useRef<HTMLButtonElement>(null);
  const navItems = [{ href: '#use-cases', label: 'Use cases' }, { href: '#product', label: 'Our approach' }, { href: '#features', label: 'Features' }, { href: '#faq', label: 'FAQ' }];

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
        <Brand className="ks-brand" size={35} />
        <nav className="ks-desktop-nav" aria-label="Main navigation">{navItems.map(item => <a key={item.href} href={item.href}>{item.label}</a>)}<a href={DOCS}>Docs <ArrowUpRight size={12} /></a></nav>
        <div className="ks-nav-actions">{!landingOnly && <Link to="/login" className="ks-sign-in">Sign in</Link>}<a href="#contact" className="ks-button ks-button-small">Let’s talk <ArrowUpRight size={14} /></a><button ref={menuButton} type="button" className="ks-menu-toggle" aria-expanded={menuOpen} aria-controls="ks-mobile-nav" aria-label={menuOpen ? 'Close navigation' : 'Open navigation'} onClick={() => setMenuOpen(!menuOpen)}>{menuOpen ? <X size={21} /> : <Menu size={21} />}</button></div>
      </div>
      {menuOpen && <nav id="ks-mobile-nav" className="ks-mobile-nav" aria-label="Mobile navigation" onClick={() => setMenuOpen(false)}>{navItems.map(item => <a key={item.href} href={item.href}>{item.label}</a>)}<a href="#voices">User feedback</a><a href="#contact">Contact</a><a href={DOCS}>Documentation <ArrowUpRight size={15} /></a>{!landingOnly && <Link to="/login">Sign in</Link>}</nav>}
    </header>
    <main id="main" tabIndex={-1}>
      <section className="ks-hero" aria-labelledby="ks-hero-title">
        <div className="ks-hero-art" aria-hidden="true">{artFailed ? <div className="ks-art-fallback"><EventHorizon size={540} /></div> : <img src="/images/gargantua-horizon.webp" width="1536" height="1024" fetchPriority="high" alt="" onError={() => setArtFailed(true)} />}</div>
        <div className="ks-hero-inner ks-container">
          <div className="ks-hero-copy"><div className="ks-eyebrow"><span className="ks-live-dot" /> A CLEARER HOME FOR YOUR SECRETS</div><h1 id="ks-hero-title">Your secrets.<br /><span>In the right orbit.</span></h1><p className="ks-lead">One place for your application secrets.<br className="ks-desktop-break" /> Clear access for your team and tools.</p><div className="ks-actions"><a href="#use-cases" className="ks-button">Find your use case <ArrowDown size={16} /></a><a href={REPO} className="ks-text-link">Explore on GitHub <ArrowUpRight size={15} /></a></div><div className="ks-hero-proof"><span><LockKeyhole size={13} /> Encrypted at rest</span><span><Code2 size={14} /> Open source</span><span><Server size={13} /> Self-hostable</span></div></div>
          <span className="ks-art-caption"><span /> GARGANTUA STUDY / 01</span>
        </div>
        <div className="ks-hero-bottom ks-container"><span>LESS SPRAWL. MORE INTENTION.</span><a href="#use-cases">Start with the work you do <ArrowDown size={14} /></a></div>
      </section>

      <section id="use-cases" className="ks-use-cases ks-container" aria-labelledby="ks-use-cases-title">
        <div className="ks-section-intro ks-intro-compact"><div><span className="ks-eyebrow">01 / YOUR USE CASE</span><h2 id="ks-use-cases-title">Different work.<br /><span>One place to begin.</span></h2></div><p>From your first application to the tools helping you build it, start with the access your work actually needs.</p></div>
        <div className="ks-use-case-grid">{USE_CASES.map(item => <a className="ks-use-case" href={item.destination} key={item.number}><div className="ks-use-case-top"><item.icon size={23} /><span>{item.number}</span></div><span className="ks-small-label">{item.label}</span><h3>{item.title}</h3><p>{item.description}</p><span className="ks-text-link">{item.link}<ArrowUpRight size={16} /></span></a>)}</div>
      </section>

      <section id="product" className="ks-product-section" aria-labelledby="ks-product-title"><div className="ks-container">
        <div className="ks-section-intro"><div><span className="ks-eyebrow">02 / WHAT WE SERVE</span><h2 id="ks-product-title">A home for your keys.<br /><span>A path for your team.</span></h2></div><p>KeepSave brings encrypted project secrets, scoped access and reviewed environment changes into one workspace. Follow a small example.</p></div>
        <VaultWalkthrough />
        <div className="ks-product-note"><ShieldCheck size={15} /><p>An interactive example, with masked sample values. Your browser creates no keys, saves no secrets and calls no provider.</p></div>
      </div></section>

      <section id="features" className="ks-features ks-container" aria-labelledby="ks-features-title">
        <div className="ks-section-intro"><div><span className="ks-eyebrow">03 / THE CAPABILITIES</span><h2 id="ks-features-title">The details that keep<br /><span>your work in order.</span></h2></div><a href={`${DOCS}/system`} className="ks-text-link">Explore the documentation <ArrowUpRight size={16} /></a></div>
        <div className="ks-feature-grid">{FEATURES.map(feature => <article className="ks-feature" key={feature.title}><div className="ks-feature-icon"><feature.icon size={25} /></div><span className="ks-small-label">{feature.label}</span><h3>{feature.title}</h3><p>{feature.text}</p><span className="ks-feature-detail">{feature.detail}</span></article>)}</div>
        <div id="tool-access" className="ks-tools-preview"><div className="ks-tools-copy"><span className="ks-preview-tag">CONTROLLED TOOL ACCESS / PREVIEW</span><h3>For the tools you use.<br /><span>And the ones that come next.</span></h3><p>A harness-neutral direction: check the task, its scope and its permission before a provider operation. Credentials stay with the broker.</p><a href={`${REPO}/blob/develop/docs/STATUS.md`} className="ks-text-link">See what’s implemented and what’s next <ArrowUpRight size={15} /></a></div><div className="ks-tools-diagram" aria-label="Illustrative preview: your tool requests a call, KeepSave checks access, then the broker performs an approved provider operation"><div><Bot size={26} /><strong>Your tool</strong><small>Request a task</small></div><ArrowRight size={19} /><div className="ks-tools-core"><EhMark size={49} /><strong>KeepSave</strong><small>Check the boundary</small></div><ArrowRight size={19} /><div><Server size={26} /><strong>Provider</strong><small>Approved operation</small></div><p>Illustrative flow · client and installation qualification required.</p></div></div>
      </section>

      <section id="voices" className="ks-voices" aria-labelledby="ks-voices-title"><div className="ks-container"><div className="ks-section-intro"><div><span className="ks-eyebrow">04 / THE PEOPLE BEHIND THE WORK</span><h2 id="ks-voices-title">Less second-guessing.<br /><span>More room to build.</span></h2></div><p>What a clearer way of managing secrets could feel like.<br /><span className="ks-mock-label">Illustrative feedback · design mockup</span></p></div><div className="ks-quote-grid">{VOICES.map(voice => <figure className="ks-quote" key={voice.initials}><span className="ks-quote-mark" aria-hidden="true">“</span><blockquote>{voice.quote}</blockquote><figcaption><span className="ks-avatar" aria-hidden="true">{voice.initials}</span><span><strong>{voice.role}</strong><small>Sample quote</small></span></figcaption></figure>)}</div><p className="ks-feedback-note">These quotes are fictional examples for this design preview, not customer endorsements.</p></div></section>

      <section id="faq" className="ks-faq ks-container" aria-labelledby="ks-faq-title"><div><span className="ks-eyebrow">05 / A LITTLE MORE CLARITY</span><h2 id="ks-faq-title">Good questions.<br /><span>Clear answers.</span></h2><p>Still curious?<br /><a href="#contact" className="ks-text-link">Let’s talk <ArrowUpRight size={15} /></a></p></div><div className="ks-faq-list">{FAQS.map(faq => <details key={faq.question}><summary>{faq.question}<ChevronDown size={18} /></summary><p>{faq.answer}</p></details>)}</div></section>

      <section id="contact" className="ks-contact ks-container" aria-labelledby="ks-contact-title"><div className="ks-contact-ring" aria-hidden="true" /><span className="ks-eyebrow">06 / LET’S BUILD SOMETHING CLEARER</span><h2 id="ks-contact-title">Your next chapter.<br /><span>In a better orbit.</span></h2><p>Tell us how your team works with secrets.<br />Help shape the KeepSave you’d want to use.</p><div className="ks-actions"><a href={FEEDBACK} className="ks-button">Share your workflow <MessageCircle size={17} /></a><a href={SETUP} className="ks-button ks-button-secondary">Explore self-hosting <ArrowUpRight size={15} /></a></div><span className="ks-contact-note">Starts a conversation on GitHub. Please keep credentials and private data out of it.</span></section>
    </main>
    <footer className="ks-footer ks-container"><div><Brand className="ks-brand" size={31} /><p>Your secrets. In the right orbit.</p></div><nav aria-label="Footer navigation"><a href={DOCS}>Documentation <ArrowUpRight size={12} /></a><a href={REPO}>GitHub <ArrowUpRight size={12} /></a><a href="#contact">Contact <ArrowUpRight size={12} /></a><a href="#top">Back to top <ArrowUpRight size={12} /></a></nav><div className="ks-footer-bottom"><span>KEEPSAVE / A DRAVENIQ PROJECT</span><a href={`${REPO}/blob/develop/docs/STATUS.md`}>Source candidate · hosted application not yet launched <ArrowUpRight size={12} /></a><span>GARGANTUA STUDY / 2026</span></div></footer>
  </div>;
}
