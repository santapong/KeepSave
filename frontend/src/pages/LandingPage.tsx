import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, ArrowUpRight, Check, Code2, FolderClosed, GitBranch, KeyRound, LockKeyhole, ShieldCheck, Terminal } from 'lucide-react';
import { Brand } from '../components/cosmic/Brand';
import { EhMark } from '../components/cosmic/EhMark';
import { BlackHoleScene } from '../components/cosmic/BlackHoleScene';
import '../styles/landing.css';

const REPO = 'https://github.com/santapong/KeepSave';
const EXAMPLE_KEYS = ['DATABASE_URL', 'PAYMENTS_API_KEY', 'SESSION_SECRET'];

export function LandingPage() {
  const [environment, setEnvironment] = useState('alpha');

  return (
    <div className="ks-landing">
      <a className="ks-skip" href="#main">Skip to content</a>
      <header className="ks-nav ks-container">
        <Brand className="ks-brand" size={40} />
        <nav aria-label="Main navigation"><a href="#workflow">How it works</a><a href="#built-for">For your stack</a><a href={`${REPO}/tree/main/docs/system`}>Docs <ArrowUpRight size={13} /></a></nav>
        <div className="ks-nav-actions"><Link to="/login">Sign in</Link><Link className="ks-button ks-button-small" to="/register">Get started <ArrowRight size={15} /></Link></div>
      </header>

      <main id="main">
        <section className="ks-hero ks-container">
          <div className="ks-hero-copy">
            <div className="ks-eyebrow"><span /> KEEPSAVE / EVENT HORIZON</div>
            <h1>Your secrets.<br />In the <span>right orbit.</span></h1>
            <p className="ks-lead">One place for the keys that keep your software running. Store them encrypted, give agents scoped access, and move changes to production with a review.</p>
            <div className="ks-actions"><Link to="/register" className="ks-button">Create your vault <ArrowRight size={17} /></Link><a href={REPO} className="ks-button ks-button-secondary">Explore the source <ArrowUpRight size={16} /></a></div>
            <div className="ks-proof"><span><Check size={14} /> Per-project encryption</span><span><Check size={14} /> Self-hostable</span></div>
          </div>
          <div className="ks-gravity-stage">
            <div className="ks-orbit ks-orbit-outer" />
            <div className="ks-orbit ks-orbit-inner" />
            <span className="ks-scene-label">A LITTLE GRAVITY. A LOT OF CONTROL.</span>
            <BlackHoleScene className="ks-live-blackhole" />
            <span className="ks-orbit-note ks-orbit-note-top"><span /> ENCRYPT</span>
            <span className="ks-orbit-note ks-orbit-note-bottom"><span /> CONNECT</span>
            <span className="ks-scene-caption">THE EVENT HORIZON</span>
          </div>
        </section>

        <section className="ks-vault-tour ks-container" aria-labelledby="vault-tour-title">
          <div className="ks-tour-copy">
            <div className="ks-eyebrow">UNDER THE SURFACE / YOUR WORKSPACE</div>
            <h2 id="vault-tour-title">One workspace.<br /><span>Three environments.</span></h2>
            <p>Create a project, add your keys, and choose who can access them. Keep development, testing, and production clearly separated.</p>
            <div className="ks-tour-detail"><ShieldCheck size={18} /><div><strong>Private by default</strong><span>Values stay hidden until you choose to reveal them.</span></div></div>
            <div className="ks-tour-detail"><GitBranch size={18} /><div><strong>Move forward with a review</strong><span>Compare changes before promoting between environments.</span></div></div>
            <Link to="/register" className="ks-text-link">Create your first project <ArrowRight size={16} /></Link>
          </div>
          <div className="ks-product" aria-label="Illustrative vault preview">
            <div className="ks-preview-top"><span><EhMark size={24} /> KeepSave</span><span className="ks-example">PRODUCT EXAMPLE</span></div>
            <div className="ks-preview-body">
              <div className="ks-preview-crumb"><FolderClosed size={14} /> Projects <span>/</span> storefront</div>
              <div className="ks-preview-heading"><div><h2>storefront</h2><p>A separate home for every environment.</p></div><LockKeyhole size={21} /></div>
              <div className="ks-env-tabs" role="group" aria-label="Preview environment">{['alpha', 'uat', 'prod'].map(env => <button type="button" aria-pressed={env === environment} key={env} onClick={() => setEnvironment(env)}><span className={`ks-env-dot ks-env-${env}`} />{env.toUpperCase()}</button>)}</div>
              <div className="ks-preview-table"><div className="ks-preview-table-head"><span>KEY</span><span>VALUE</span></div>{EXAMPLE_KEYS.map(key => <div key={key} className="ks-preview-row"><span><KeyRound size={13} />{key}</span><span className="ks-masked">•••••••••••• <LockKeyhole size={12} /></span></div>)}</div>
              <div className="ks-preview-foot" aria-live="polite"><ShieldCheck size={15} /><span>{environment.toUpperCase()} · Values hidden by default</span><span>3 example keys</span></div>
            </div>
            <div className="ks-preview-pipeline"><GitBranch size={17} /><span>Develop</span><span className="ks-line" /><span>Review</span><span className="ks-line" /><span>Promote <ArrowRight size={14} /></span></div>
          </div>
        </section>

        <div className="ks-stack-strip"><div className="ks-container"><span>FITS THE WAY YOU BUILD</span><span><Terminal size={17} /> CLI & CI/CD</span><span><Code2 size={17} /> Python · Node.js · Go</span><span><KeyRound size={17} /> Scoped API keys</span><span><GitBranch size={17} /> MCP tools</span></div></div>

        <section id="workflow" className="ks-section ks-container">
          <div className="ks-section-heading"><div><div className="ks-eyebrow">01 / A CLEAR PATH TO PRODUCTION</div><h2>Less copy-paste.<br />More control.</h2></div><p>Give each project its own vault. Keep Alpha, UAT, and Production separate. Make every change intentional.</p></div>
          <div className="ks-steps">
            <article><div className="ks-step-top"><LockKeyhole size={22} /><span>01</span></div><h3>Keep it encrypted</h3><p>Add a secret or import an existing .env file. Values are encrypted at rest with AES-256-GCM and a key for each project.</p><span className="ks-step-tag">YOUR PROJECT. YOUR VAULT.</span></article>
            <article><div className="ks-step-top"><KeyRound size={22} /><span>02</span></div><h3>Grant just enough access</h3><p>Scope API keys to projects, environments, and actions. Connect your applications, pipelines, and agent tools.</p><span className="ks-step-tag">ACCESS WITH BOUNDARIES.</span></article>
            <article><div className="ks-step-top"><GitBranch size={22} /><span>03</span></div><h3>Review. Then promote.</h3><p>Compare changes before moving them between environments. Use approval rules and audit history to track what happened.</p><span className="ks-step-tag">A RECORD OF THE CHANGE.</span></article>
          </div>
        </section>

        <section id="built-for" className="ks-integration ks-container">
          <div><div className="ks-eyebrow">02 / YOUR TOOLS, CONNECTED</div><h2>Made for developers.<br />Ready for agents.</h2><p>KeepSave brings environment secrets and MCP connections into one workspace, with a REST API, language SDKs, and a CLI for your existing workflows.</p><a href={`${REPO}/tree/main/sdks`} className="ks-text-link">Browse SDKs and integration guides <ArrowRight size={16} /></a></div>
          <div className="ks-code"><div><Terminal size={15} /> a familiar workflow <span>CLI</span></div><pre><code><span># Open your project vault</span>{'\n'}keepsave projects{'\n\n'}<span># Read the command reference</span>{'\n'}keepsave --help</code></pre><div className="ks-code-note"><LockKeyhole size={14} /> Configure credentials locally using the CLI guide.</div></div>
        </section>

        <section className="ks-bottom-cta ks-container"><div><div className="ks-eyebrow">A BETTER PLACE TO KEEP THINGS</div><h2>Start with one project.</h2><p>Create a vault, add a secret, and take it from there.</p></div><Link to="/register" className="ks-button">Create your vault <ArrowRight size={17} /></Link></section>
      </main>
      <footer className="ks-footer ks-container"><Brand className="ks-brand" size={36} /><span>Built for the things you shouldn’t share.</span><div><a href={REPO}>GitHub <ArrowUpRight size={13} /></a><a href={`${REPO}/blob/main/docs/THREAT_MODEL.md`}>Security model</a></div></footer>
    </div>
  );
}
