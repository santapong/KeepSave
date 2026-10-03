import type { CSSProperties } from 'react';
import { cn } from '@/lib/utils';

interface EventHorizonProps {
  /** Rendered diameter in pixels. */
  size?: number;
  className?: string;
}

/**
 * Static CSS companion for empty states and the WebGL fallback.
 * The light-plane and lensed arcs approximate the live scene's silhouette;
 * only BlackHoleScene computes actual geodesic intersections.
 */
export function EventHorizon({ size = 440, className }: EventHorizonProps) {
  return (
    <div
      className={cn('cz-eh', className)}
      style={{ '--cz-eh-size': `${size}px` } as CSSProperties}
      aria-hidden="true"
    >
      <div className="cz-eh__bloom" />
      <div className="cz-eh__lensing cz-eh__lensing--upper" />
      <div className="cz-eh__lensing cz-eh__lensing--lower" />
      <div className="cz-eh__disk" />
      <div className="cz-eh__photon" />
      <div className="cz-eh__core" />
      <div className="cz-eh__disk cz-eh__disk--front" />
    </div>
  );
}
