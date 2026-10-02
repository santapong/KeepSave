import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, LockKeyhole } from '@/components/icons';
import { Brand } from '../cosmic/Brand';
import '../../styles/auth.css';
export function AuthShell({ children }: { children: ReactNode }) {
  return <div className="ks-auth">
    <a className="ks-auth-skip" href="#sign-in">Skip to sign in</a>
    <header className="ks-auth-header"><Brand size={36} /><Link to="/"><ArrowLeft size={14} /> Back to KeepSave</Link></header>
    <div className="ks-auth-body">
      <aside className="ks-auth-story" aria-label="Welcome to KeepSave">
        <div className="ks-auth-art" aria-hidden="true"><img src="/images/event-horizon.webp" alt="" /></div>
        <div className="ks-auth-intro"><span className="ks-auth-eyebrow"><span /> YOUR WORKSPACE, WITHIN REACH</span>
          <h1>Welcome back<br /><em>to your orbit.</em></h1>
          <p>Your projects and environments,<br className="ks-auth-desktop-break" /> right where you left them.</p>
        </div>
        <div className="ks-auth-story-foot"><span>One home for your secrets.</span><span>KEEPSAVE / EVENT HORIZON</span></div>
      </aside>
      <main id="sign-in" className="ks-auth-main" tabIndex={-1}><div className="ks-auth-form-panel">{children}
        <div className="ks-auth-security"><LockKeyhole size={13} /> Your secrets stay encrypted at rest.</div>
      </div></main>
    </div>
  </div>;
}
