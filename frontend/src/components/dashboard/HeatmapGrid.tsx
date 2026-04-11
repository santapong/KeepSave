import { useMemo } from 'react';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';

interface HeatmapGridProps {
  data: { hour: number; day: number; count: number }[];
  maxCount?: number;
}

const DAY_LABELS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const HOURS = Array.from({ length: 24 }, (_, i) => i);

function getCellColor(count: number, max: number): string {
  if (max === 0 || count === 0) return 'transparent';
  const ratio = count / max;
  if (ratio <= 0.25) return 'hsl(var(--primary) / 0.2)';
  if (ratio <= 0.5) return 'hsl(var(--primary) / 0.4)';
  if (ratio <= 0.75) return 'hsl(var(--primary) / 0.6)';
  return 'hsl(var(--primary))';
}

function formatHour(hour: number): string {
  return `${hour.toString().padStart(2, '0')}:00`;
}

export function HeatmapGrid({ data, maxCount }: HeatmapGridProps) {
  const lookup = useMemo(() => {
    const map = new Map<string, number>();
    for (const d of data) {
      map.set(`${d.day}-${d.hour}`, d.count);
    }
    return map;
  }, [data]);

  const resolvedMax = useMemo(() => {
    if (maxCount !== undefined) return maxCount;
    let max = 0;
    for (const d of data) {
      if (d.count > max) max = d.count;
    }
    return max;
  }, [data, maxCount]);

  return (
    <TooltipProvider delayDuration={100}>
      <div className="overflow-x-auto">
        <div className="inline-block">
          {/* Hour labels row */}
          <div className="flex" style={{ marginLeft: 36 }}>
            {HOURS.map((hour) => (
              <div
                key={hour}
                className="text-[10px] text-muted-foreground text-center"
                style={{ width: 16, marginRight: 2 }}
              >
                {hour % 3 === 0 ? hour : ''}
              </div>
            ))}
          </div>

          {/* Grid rows */}
          {DAY_LABELS.map((dayLabel, dayIndex) => (
            <div key={dayLabel} className="flex items-center">
              <span
                className="text-[10px] text-muted-foreground w-8 shrink-0 text-right pr-1"
              >
                {dayLabel}
              </span>
              {HOURS.map((hour) => {
                const count = lookup.get(`${dayIndex}-${hour}`) ?? 0;
                return (
                  <Tooltip key={hour}>
                    <TooltipTrigger asChild>
                      <div
                        className="rounded-sm shrink-0 cursor-default"
                        style={{
                          width: 16,
                          height: 16,
                          marginRight: 2,
                          marginBottom: 2,
                          backgroundColor: getCellColor(count, resolvedMax),
                          border:
                            count === 0
                              ? '1px solid hsl(var(--border))'
                              : 'none',
                        }}
                      />
                    </TooltipTrigger>
                    <TooltipContent side="top" className="text-xs">
                      {dayLabel}, {formatHour(hour)}: {count} accesses
                    </TooltipContent>
                  </Tooltip>
                );
              })}
            </div>
          ))}
        </div>
      </div>
    </TooltipProvider>
  );
}
