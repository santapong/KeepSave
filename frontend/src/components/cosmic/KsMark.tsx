import { useId } from 'react';
import { cn } from '@/lib/utils';

interface KsMarkProps {
  /** Rendered square size in pixels. Legible down to 16. */
  size?: number;
  className?: string;
}

/**
 * KsMark — the KeepSave brand mark: an event horizon seen almost edge-on.
 *
 * Drawn in four passes so it reads as a black hole rather than a ringed
 * planet: the far half of the accretion disk, the matte core, the photon
 * ring, then the near half of the disk redrawn *in front* of the core.
 * That last pass is the whole trick — it is what lensing looks like.
 *
 * The same artwork ships as `public/keepsave.svg` for the favicon, so the
 * tab icon and the in-app mark are literally the same drawing. Replaces
 * the older CSS-only EhMark, which could not be used as a favicon and
 * blurred below ~20px.
 *
 * The gradient id is per-instance (useId) because the mark renders more
 * than once per page — duplicate SVG ids would make every instance
 * inherit the first one's gradient.
 */
export function KsMark({ size = 26, className }: KsMarkProps) {
  const gradientId = `ks-disk-${useId()}`;

  return (
    <svg
      className={cn('ks-mark-svg', className)}
      width={size}
      height={size}
      viewBox="0 0 32 32"
      fill="none"
      aria-hidden="true"
      style={{ flex: 'none', display: 'block' }}
    >
      <defs>
        <linearGradient id={gradientId} x1="1" y1="16" x2="31" y2="16" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="var(--cz-plasma)" />
          <stop offset=".3" stopColor="var(--cz-flare)" />
          <stop offset=".58" stopColor="var(--cz-hot)" />
          <stop offset=".82" stopColor="var(--cz-flare)" />
          <stop offset="1" stopColor="var(--cz-plasma)" />
        </linearGradient>
      </defs>

      <ellipse cx="16" cy="16" rx="14.3" ry="4.7" stroke={`url(#${gradientId})`} strokeWidth="2.5" />
      <circle cx="16" cy="16" r="6.7" fill="var(--cz-void)" />
      <circle cx="16" cy="16" r="7.05" stroke="var(--cz-accent-hi)" strokeWidth="1.05" />
      <path
        d="M1.7 16A14.3 4.7 0 0 0 30.3 16"
        stroke={`url(#${gradientId})`}
        strokeWidth="2.5"
        strokeLinecap="round"
      />
    </svg>
  );
}
