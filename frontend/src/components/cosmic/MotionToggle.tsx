import { Pause, Play } from 'lucide-react';
import { useMotion } from '../../hooks/useMotion';

export function MotionToggle() {
  const { paused, reduced, toggle } = useMotion();
  return <button type="button" className="cz-motion-toggle" onClick={toggle} disabled={reduced}
    aria-pressed={paused} aria-label={reduced ? 'Motion reduced by system preference' : paused ? 'Resume animation' : 'Pause animation'}>
    {paused ? <Play size={13} /> : <Pause size={13} />}
    {reduced ? 'Reduced motion' : paused ? 'Resume motion' : 'Pause motion'}
  </button>;
}
