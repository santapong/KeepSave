import type { CSSProperties } from 'react';
import { cn } from '@/lib/utils';

type StageId = 'alpha' | 'uat' | 'prod';

export interface PipelineStage {
  id: StageId;
  label: string;
  sub: string;
  /** Optional key count shown in the legend (omitted when undefined). */
  keys?: number | string;
  /** Optional revision tag, e.g. "r.194". */
  rev?: string;
  /** Optional accent pill, e.g. "2 of 3 signed". */
  badge?: string;
}

interface OrbitalPipelineProps {
  /** Legend stages, in order. Defaults to alpha / uat / prod. */
  stages?: PipelineStage[];
  className?: string;
}

const DEFAULT_STAGES: PipelineStage[] = [
  { id: 'alpha', label: 'alpha', sub: 'Development' },
  { id: 'uat', label: 'uat', sub: 'Staging' },
  { id: 'prod', label: 'prod', sub: 'Production' },
];

const PLANET_GRADIENT: Record<StageId, string> = {
  alpha: 'radial-gradient(circle at 35% 30%, oklch(0.8 0.12 200), oklch(0.45 0.12 220))',
  uat: 'radial-gradient(circle at 35% 30%, oklch(0.82 0.14 290), oklch(0.5 0.16 290))',
  prod: 'radial-gradient(circle at 35% 30%, oklch(0.86 0.15 82), oklch(0.6 0.16 60))',
};

/**
 * OrbitalPipeline — the promotion pipeline (alpha → uat → prod) drawn as
 * planets orbiting a star core, with a legend of stage cards beneath.
 * Orbits rotate slowly in alternating directions; rotation is gated by
 * `prefers-reduced-motion` / `data-motion="off"` in CSS.
 */
export function OrbitalPipeline({ stages = DEFAULT_STAGES, className }: OrbitalPipelineProps) {
  return (
    <div className={className}>
      <div className="cz-pipeline">
        <div className="cz-orbits" aria-hidden="true">
          <div className="cz-orbit cz-orbit-1" />
          <div className="cz-orbit cz-orbit-2" />
          <div className="cz-orbit cz-orbit-3" />
          <div className="cz-core-star" />

          <div className="cz-orbit cz-orbit-1 cz-orbit-spin">
            <div className="cz-planet" style={{ left: '50%', top: -12, transform: 'translateX(-50%)' }}>
              <div className="cz-body cz-alpha" />
              <div className="cz-lb">alpha</div>
            </div>
          </div>
          <div className="cz-orbit cz-orbit-2 cz-orbit-spin cz-rev">
            <div className="cz-planet" style={{ right: -14, top: '50%', transform: 'translateY(-50%)' }}>
              <div className="cz-body cz-uat" />
              <div className="cz-lb">uat</div>
            </div>
          </div>
          <div className="cz-orbit cz-orbit-3 cz-orbit-spin">
            <div className="cz-planet" style={{ left: '50%', bottom: -14, transform: 'translateX(-50%)' }}>
              <div className="cz-body cz-prod" />
              <div className="cz-lb">prod</div>
            </div>
          </div>
        </div>
      </div>

      <div className="cz-pipe-legend">
        {stages.map((stage) => (
          <div key={stage.id} className={cn('cz-pipe-stage', stage.id === 'prod' && 'cz-prod')}>
            <div className="cz-nm">
              <span
                className="cz-mini-body"
                style={{ background: PLANET_GRADIENT[stage.id] } as CSSProperties}
              />
              {stage.label}
              {stage.badge && (
                <span className="cz-pill cz-pill-accent" style={{ marginLeft: 'auto' }}>
                  {stage.badge}
                </span>
              )}
            </div>
            <div className="cz-sub">{stage.sub}</div>
            {(stage.keys !== undefined || stage.rev) && (
              <div className="cz-st">
                {stage.keys !== undefined && (
                  <span>
                    <b>{stage.keys}</b> keys
                  </span>
                )}
                {stage.rev && <span>{stage.rev}</span>}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
