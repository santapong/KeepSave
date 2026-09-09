import { Link } from 'react-router-dom';
import { EhMark } from './EhMark';

/** Shared home link: preserve one spelling, silhouette, and accessible name. */
export function Brand({ className = '', size = 36 }: { className?: string; size?: number }) {
  return <Link to="/" className={`cz-brand ${className}`} aria-label="KeepSave home">
    <EhMark size={size} />
    <span className="cz-brand-name">Keep<span>Save</span></span>
  </Link>;
}
