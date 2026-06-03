import type { CSSProperties } from 'react';
import { cn } from '@/lib/utils';

interface EventHorizonProps {
  /** Rendered diameter in pixels. */
  size?: number;
  className?: string;
}

/**
 * Event Horizon — a pure-CSS black hole used on the login hero and
 * empty states. Layers: outer bloom, rotating accretion disk, photon
 * ring, black core, and the lensed near-side disk that wraps in front
 * of the core. Decorative only (`aria-hidden`). The disk rotation is
 * gated by `prefers-reduced-motion` / `data-motion="off"` in CSS.
 */
export function EventHorizon({ size = 440, className }: EventHorizonProps) {
  return (
    <div
      className={cn('cz-eh', className)}
      style={{ '--cz-eh-size': `${size}px` } as CSSProperties}
      aria-hidden="true"
    >
      <div className="cz-eh__bloom" />
      <div className="cz-eh__disk" />
      <div className="cz-eh__photon" />
      <div className="cz-eh__core" />
      <div className="cz-eh__disk cz-eh__disk--front" />
    </div>
  );
}
