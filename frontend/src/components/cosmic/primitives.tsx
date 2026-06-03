import { type ReactNode, type CSSProperties } from 'react';
import { cn } from '@/lib/utils';
import { EventHorizon } from './EventHorizon';

/** Page wrapper with the cosmic max-width + padding. */
export function Page({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return (
    <div className="cz-page" style={style}>
      {children}
    </div>
  );
}

/** Eyebrow + display title (accent words via <em>) + sub + right-aligned actions. */
export function PageHeader({
  eyebrow,
  title,
  sub,
  actions,
}: {
  eyebrow?: ReactNode;
  title: ReactNode;
  sub?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="cz-page-head">
      <div>
        {eyebrow && <div className="cz-eyebrow">{eyebrow}</div>}
        <h1 className="cz-page-title">{title}</h1>
        {sub && <p className="cz-page-sub">{sub}</p>}
      </div>
      {actions && <div className="cz-page-actions">{actions}</div>}
    </div>
  );
}

export interface KpiProps {
  label: ReactNode;
  value: ReactNode;
  hint?: ReactNode;
  trend?: 'up' | 'down';
  /** Sparkline bars (relative heights). */
  bars?: number[];
  /** Render the value with the accretion gradient. */
  flux?: boolean;
}

export function Kpi({ label, value, hint, trend, bars, flux }: KpiProps) {
  const max = Math.max(...(bars ?? [1]), 1);
  return (
    <div className="cz-card cz-kpi">
      <div className="cz-k">{label}</div>
      <div className={cn('cz-v', flux && 'cz-flux')}>{value}</div>
      {hint && (
        <div className={cn('cz-d', trend === 'up' && 'cz-up', trend === 'down' && 'cz-down')}>{hint}</div>
      )}
      {bars && (
        <div className="cz-sparkbars">
          {bars.map((b, i) => (
            <div
              key={i}
              className={cn('cz-bar', i === bars.length - 1 && 'cz-hi')}
              style={{ height: `${15 + (b / max) * 85}%` }}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function KpiStrip({ children }: { children: ReactNode }) {
  return <div className="cz-kpi-strip">{children}</div>;
}

export function SectionHead({
  index,
  title,
  meta,
  style,
}: {
  index?: string;
  title: ReactNode;
  meta?: ReactNode;
  style?: CSSProperties;
}) {
  return (
    <div className="cz-sec-head" style={style}>
      <h2>
        {index && <span className="cz-ix">{index}</span>} {title}
      </h2>
      {meta && <div className="cz-meta">{meta}</div>}
    </div>
  );
}

export function Segmented<T extends string>({
  options,
  value,
  onChange,
}: {
  options: ReadonlyArray<T | { value: T; label: ReactNode }>;
  value: T;
  onChange: (v: T) => void;
}) {
  return (
    <div className="cz-seg">
      {options.map((opt) => {
        const v = typeof opt === 'string' ? opt : opt.value;
        const label = typeof opt === 'string' ? opt : opt.label;
        return (
          <button key={v} type="button" className={value === v ? 'cz-on' : ''} onClick={() => onChange(v)}>
            {label}
          </button>
        );
      })}
    </div>
  );
}

/** Black-hole empty state. */
export function EmptyState({
  title,
  children,
  size = 150,
}: {
  title: ReactNode;
  children?: ReactNode;
  size?: number;
}) {
  return (
    <div className="cz-empty-state">
      <EventHorizon size={size} />
      <h3>{title}</h3>
      {children && <p>{children}</p>}
    </div>
  );
}

export function CodeBlock({
  filename,
  action,
  children,
  style,
}: {
  filename: ReactNode;
  action?: ReactNode;
  children: ReactNode;
  style?: CSSProperties;
}) {
  return (
    <div className="cz-code" style={style}>
      <div className="cz-code-head">
        <span>{filename}</span>
        {action}
      </div>
      <pre className="cz-code-body">{children}</pre>
    </div>
  );
}

export function Chip({
  children,
  variant,
  className,
}: {
  children: ReactNode;
  variant?: 'prod' | 'on';
  className?: string;
}) {
  return (
    <span className={cn('cz-chip', variant === 'prod' && 'cz-prod', variant === 'on' && 'cz-on', className)}>
      {children}
    </span>
  );
}

export function Pill({
  children,
  variant,
  className,
}: {
  children: ReactNode;
  variant?: 'go' | 'stop' | 'accent';
  className?: string;
}) {
  return (
    <span
      className={cn(
        'cz-pill',
        variant === 'go' && 'cz-pill-go',
        variant === 'stop' && 'cz-pill-stop',
        variant === 'accent' && 'cz-pill-accent',
        className,
      )}
    >
      {children}
    </span>
  );
}

export function Dot({ status }: { status: 'go' | 'warn' | 'stop' }) {
  return <span className={cn('cz-dot', `cz-dot-${status}`)} />;
}
