import { Link } from 'react-router-dom';
import { EhMark } from './EhMark';

/** Shared home link: preserve one spelling, silhouette, and accessible name. */
export function Brand({ className = '', size = 36, onNavigate }: { className?: string; size?: number; onNavigate?: () => void }) {
  return <Link to="/" className={`cz-brand ${className}`} aria-label="KeepSave home" onClick={onNavigate}>
    <EhMark size={size} />
    <span className="cz-brand-name">Keep<span>Save</span></span>
  </Link>;
}
