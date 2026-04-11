import { useState, useEffect, useMemo } from 'react';
import {
  Search,
  Activity,
  CheckCircle2,
  XCircle,
} from 'lucide-react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts';
import * as api from '@/api/client';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '@/components/ui/table';
import { StatCard } from '@/components/dashboard/StatCard';
import { ChartCard } from '@/components/dashboard/ChartCard';
import { TraceWaterfall } from '@/components/dashboard/TraceWaterfall';
import type { TraceSpan } from '@/types';

/** Parse a human-readable duration string into milliseconds. */
function parseDurationMs(duration: string): number {
  const t = duration.trim().toLowerCase();
  if (t.endsWith('ms')) return parseFloat(t.slice(0, -2));
  if (t.endsWith('us') || t.endsWith('\u00b5s')) return parseFloat(t.slice(0, -2)) / 1000;
  if (t.endsWith('ns')) return parseFloat(t.slice(0, -2)) / 1_000_000;
  if (t.endsWith('s')) return parseFloat(t.slice(0, -1)) * 1000;
  const num = parseFloat(t);
  return Number.isNaN(num) ? 0 : num;
}

/** Bucket a duration (ms) into a histogram label. */
function bucketLabel(ms: number): string {
  if (ms < 10) return '<10ms';
  if (ms < 50) return '10-50ms';
  if (ms < 100) return '50-100ms';
  if (ms < 500) return '100-500ms';
  if (ms < 1000) return '0.5-1s';
  return '>1s';
}

const BUCKET_ORDER = ['<10ms', '10-50ms', '50-100ms', '100-500ms', '0.5-1s', '>1s'];

export function TracesTab() {
  const [traces, setTraces] = useState<TraceSpan[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [expandedTraceId, setExpandedTraceId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .getTraces()
      .then((data) => {
        if (!cancelled) {
          setTraces(data as unknown as TraceSpan[]);
          setError(null);
        }
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const filtered = useMemo(() => {
    if (!search.trim()) return traces;
    const q = search.toLowerCase();
    return traces.filter(
      (t) =>
        t.operation.toLowerCase().includes(q) ||
        t.trace_id.toLowerCase().includes(q),
    );
  }, [traces, search]);

  const okCount = useMemo(
    () => filtered.filter((t) => t.status.toLowerCase() === 'ok').length,
    [filtered],
  );
  const errorCount = useMemo(() => filtered.length - okCount, [filtered, okCount]);

  const durationDistribution = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const label of BUCKET_ORDER) counts[label] = 0;
    for (const t of filtered) {
      const ms = parseDurationMs(t.duration);
      const label = bucketLabel(ms);
      counts[label] = (counts[label] || 0) + 1;
    }
    return BUCKET_ORDER.map((label) => ({ bucket: label, count: counts[label] }));
  }, [filtered]);

  /** Collect spans that share the same trace_id for the waterfall view. */
  const waterfallSpans = useMemo(() => {
    if (!expandedTraceId) return [];
    return traces.filter((t) => t.trace_id === expandedTraceId);
  }, [traces, expandedTraceId]);

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="grid gap-4 sm:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-24 rounded-lg" />
          ))}
        </div>
        <Skeleton className="h-10 w-full rounded-lg" />
        <Skeleton className="h-64 rounded-lg" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-6 text-center text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
        Failed to load traces: {error}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Stat cards */}
      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard
          label="Total Traces"
          value={filtered.length}
          icon={<Activity className="h-4 w-4" />}
        />
        <StatCard
          label="OK"
          value={okCount}
          icon={<CheckCircle2 className="h-4 w-4" />}
          color="text-green-600"
        />
        <StatCard
          label="Errors"
          value={errorCount}
          icon={<XCircle className="h-4 w-4" />}
          color="text-red-600"
        />
      </div>

      {/* Search */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Filter by operation name or trace ID..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="pl-9"
        />
      </div>

      {/* Traces table */}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Operation</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Duration</TableHead>
              <TableHead>Trace ID</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.length === 0 ? (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-muted-foreground">
                  No traces found.
                </TableCell>
              </TableRow>
            ) : (
              filtered.map((trace) => {
                const isExpanded = expandedTraceId === trace.trace_id;
                return (
                  <TableRow
                    key={trace.span_id}
                    className={cn(
                      'cursor-pointer',
                      isExpanded && 'bg-muted/50',
                    )}
                    onClick={() =>
                      setExpandedTraceId(isExpanded ? null : trace.trace_id)
                    }
                  >
                    <TableCell className="font-medium">{trace.operation}</TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          trace.status.toLowerCase() === 'ok' ? 'success' : 'destructive'
                        }
                      >
                        {trace.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {trace.duration}
                    </TableCell>
                    <TableCell
                      className="max-w-[160px] truncate font-mono text-xs text-muted-foreground"
                      title={trace.trace_id}
                    >
                      {trace.trace_id}
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </div>

      {/* Expanded waterfall */}
      {expandedTraceId && waterfallSpans.length > 0 && (
        <div className="rounded-lg border p-4">
          <h3 className="mb-3 text-sm font-semibold">
            Trace Waterfall &mdash;{' '}
            <span className="font-mono text-xs text-muted-foreground">
              {expandedTraceId}
            </span>
          </h3>
          <TraceWaterfall spans={waterfallSpans} />
        </div>
      )}

      {/* Duration distribution chart */}
      <ChartCard
        title="Trace Duration Distribution"
        description="Distribution of trace durations across time buckets"
      >
        <BarChart data={durationDistribution}>
          <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
          <XAxis dataKey="bucket" tick={{ fontSize: 12 }} />
          <YAxis allowDecimals={false} tick={{ fontSize: 12 }} />
          <Tooltip />
          <Bar dataKey="count" fill="hsl(var(--primary))" radius={[4, 4, 0, 0]} />
        </BarChart>
      </ChartCard>
    </div>
  );
}
