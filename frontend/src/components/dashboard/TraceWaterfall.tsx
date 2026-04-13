import { useMemo } from 'react';
import { cn } from '@/lib/utils';

interface Span {
  span_id: string;
  operation: string;
  status: string;
  duration: string | number;
  start_time?: string;
}

interface TraceWaterfallProps {
  spans: Span[];
}

/**
 * Parse a duration value into milliseconds.
 * Go's time.Duration JSON-serializes as a number (nanoseconds).
 */
function parseDurationMs(duration: string | number): number {
  if (typeof duration === 'number') {
    return duration / 1_000_000; // nanoseconds -> ms
  }
  const trimmed = String(duration).trim().toLowerCase();

  if (trimmed.endsWith('ms')) {
    return parseFloat(trimmed.slice(0, -2));
  }
  if (trimmed.endsWith('us') || trimmed.endsWith('\u00b5s')) {
    return parseFloat(trimmed.slice(0, -2)) / 1000;
  }
  if (trimmed.endsWith('ns')) {
    return parseFloat(trimmed.slice(0, -2)) / 1_000_000;
  }
  if (trimmed.endsWith('s')) {
    return parseFloat(trimmed.slice(0, -1)) * 1000;
  }

  // Fallback: try parsing as plain number (assumed ms)
  const num = parseFloat(trimmed);
  return Number.isNaN(num) ? 0 : num;
}

function getStatusColor(status: string): {
  bar: string;
  bg: string;
} {
  const s = status.toLowerCase();
  if (s === 'error' || s === 'err' || s === 'failed') {
    return {
      bar: 'bg-red-500',
      bg: 'bg-red-500/10',
    };
  }
  return {
    bar: 'bg-green-500',
    bg: 'bg-green-500/10',
  };
}

export function TraceWaterfall({ spans }: TraceWaterfallProps) {
  const { durations, maxDuration } = useMemo(() => {
    const parsed = spans.map((s) => parseDurationMs(s.duration));
    const max = Math.max(...parsed, 1); // Avoid division by zero
    return { durations: parsed, maxDuration: max };
  }, [spans]);

  if (spans.length === 0) {
    return (
      <p className="text-sm text-muted-foreground py-4 text-center">
        No spans to display.
      </p>
    );
  }

  return (
    <div className="space-y-1">
      {spans.map((span, index) => {
        const durationMs = durations[index];
        const widthPercent = Math.max((durationMs / maxDuration) * 100, 2);
        const colors = getStatusColor(span.status);

        return (
          <div
            key={span.span_id}
            className="flex items-center gap-3 text-sm"
          >
            {/* Operation name */}
            <div
              className="w-40 shrink-0 truncate text-right font-mono text-xs text-muted-foreground"
              title={span.operation}
            >
              {span.operation}
            </div>

            {/* Bar area */}
            <div className={cn('relative flex-1 h-6 rounded', colors.bg)}>
              <div
                className={cn('absolute left-0 top-0 h-full rounded', colors.bar)}
                style={{ width: `${widthPercent}%` }}
              />
              {/* Status indicator */}
              <div className="absolute inset-0 flex items-center px-2">
                <span
                  className={cn(
                    'text-[10px] font-medium uppercase tracking-wide',
                    span.status.toLowerCase() === 'error' ||
                      span.status.toLowerCase() === 'err' ||
                      span.status.toLowerCase() === 'failed'
                      ? 'text-red-700 dark:text-red-300'
                      : 'text-green-700 dark:text-green-300',
                  )}
                >
                  {span.status}
                </span>
              </div>
            </div>

            {/* Duration label */}
            <div className="w-20 shrink-0 text-right font-mono text-xs text-muted-foreground">
              {typeof span.duration === 'number'
                ? `${(span.duration / 1_000_000).toFixed(2)}ms`
                : span.duration}
            </div>
          </div>
        );
      })}
    </div>
  );
}
