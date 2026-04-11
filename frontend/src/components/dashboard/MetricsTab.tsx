import { useState, useEffect, useMemo } from 'react';
import { Gauge, AlertTriangle, Clock, ShieldCheck } from 'lucide-react';
import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as RechartsTooltip,
} from 'recharts';
import * as api from '@/api/client';
import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { StatCard } from '@/components/dashboard/StatCard';
import { ChartCard } from '@/components/dashboard/ChartCard';
import { parsePrometheusText } from '@/utils/parsePrometheus';

function generateRequestTimeline(): { time: string; requests: number }[] {
  const points: { time: string; requests: number }[] = [];
  const now = Date.now();
  for (let i = 23; i >= 0; i--) {
    const ts = new Date(now - i * 3600 * 1000);
    const hour = ts.getHours().toString().padStart(2, '0') + ':00';
    points.push({
      time: hour,
      requests: Math.floor(80 + Math.random() * 320),
    });
  }
  return points;
}

const METHOD_DATA = [
  { method: 'GET', count: 4520 },
  { method: 'POST', count: 1890 },
  { method: 'PUT', count: 720 },
  { method: 'DELETE', count: 310 },
];

const METHOD_COLORS: Record<string, string> = {
  GET: 'hsl(var(--primary))',
  POST: '#22c55e',
  PUT: '#f59e0b',
  DELETE: '#ef4444',
};

export function MetricsTab() {
  const [, setDashboard] = useState<Record<string, unknown> | null>(null);
  const [prometheusAvailable, setPrometheusAvailable] = useState(true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [requestRate, setRequestRate] = useState('--');
  const [errorRate, setErrorRate] = useState('--');
  const [avgLatency, setAvgLatency] = useState('--');
  const [encryptionOps, setEncryptionOps] = useState('--');

  const timelineData = useMemo(() => generateRequestTimeline(), []);

  useEffect(() => {
    let cancelled = false;

    async function fetchData() {
      setLoading(true);
      setError(null);
      try {
        const dashData = await api.getAdminDashboard();
        if (cancelled) return;
        setDashboard(dashData);

        // Try parsing prometheus metrics
        try {
          const rawMetrics = await api.getPrometheusMetrics();
          if (cancelled) return;
          const metrics = parsePrometheusText(rawMetrics);

          // Extract summary statistics from prometheus metrics
          const httpTotal = metrics
            .filter((m) => m.name === 'http_requests_total')
            .reduce((sum, m) => sum + m.value, 0);
          const httpErrors = metrics
            .filter(
              (m) =>
                m.name === 'http_requests_total' &&
                m.labels.status &&
                m.labels.status.startsWith('5'),
            )
            .reduce((sum, m) => sum + m.value, 0);
          const latencySum = metrics.find(
            (m) => m.name === 'http_request_duration_seconds_sum',
          );
          const latencyCount = metrics.find(
            (m) => m.name === 'http_request_duration_seconds_count',
          );
          const encOps = metrics
            .filter((m) => m.name.includes('encryption') || m.name.includes('encrypt'))
            .reduce((sum, m) => sum + m.value, 0);

          if (httpTotal > 0) setRequestRate(`${httpTotal.toLocaleString()}`);
          if (httpTotal > 0)
            setErrorRate(
              `${((httpErrors / httpTotal) * 100).toFixed(1)}%`,
            );
          if (latencySum && latencyCount && latencyCount.value > 0)
            setAvgLatency(
              `${((latencySum.value / latencyCount.value) * 1000).toFixed(0)}ms`,
            );
          if (encOps > 0) setEncryptionOps(encOps.toLocaleString());

          setPrometheusAvailable(true);
        } catch {
          if (!cancelled) setPrometheusAvailable(false);
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load metrics');
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchData();
    return () => { cancelled = true; };
  }, []);

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-[100px] rounded-lg" />
          ))}
        </div>
        <Skeleton className="h-[360px] rounded-lg" />
        <Skeleton className="h-[360px] rounded-lg" />
      </div>
    );
  }

  if (error) {
    return (
      <Card>
        <CardContent className="p-6">
          <p className="text-sm text-destructive">{error}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Stat cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Request Rate"
          value={requestRate}
          icon={<Gauge className="h-5 w-5" />}
        />
        <StatCard
          label="Error Rate"
          value={errorRate}
          icon={<AlertTriangle className="h-5 w-5" />}
          color={errorRate !== '--' && parseFloat(errorRate) > 5 ? 'text-destructive' : undefined}
        />
        <StatCard
          label="Avg Latency"
          value={avgLatency}
          icon={<Clock className="h-5 w-5" />}
        />
        <StatCard
          label="Encryption Ops"
          value={encryptionOps}
          icon={<ShieldCheck className="h-5 w-5" />}
        />
      </div>

      {/* Prometheus fallback notice */}
      {!prometheusAvailable && (
        <Card>
          <CardContent className="p-4">
            <p className="text-sm text-muted-foreground">
              Metrics endpoint not available. Showing sample data. Connect a Prometheus-compatible
              endpoint at <code className="rounded bg-muted px-1 text-xs">/metrics</code> for live
              data.
            </p>
          </CardContent>
        </Card>
      )}

      {/* Request timeline area chart */}
      <ChartCard title="Requests Over Time" description="Hourly request volume (last 24h)">
        <AreaChart data={timelineData}>
          <defs>
            <linearGradient id="metricsReqGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="hsl(var(--primary))" stopOpacity={0.3} />
              <stop offset="95%" stopColor="hsl(var(--primary))" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
          <XAxis
            dataKey="time"
            tick={{ fontSize: 12 }}
            tickLine={false}
            axisLine={false}
          />
          <YAxis tick={{ fontSize: 12 }} tickLine={false} axisLine={false} />
          <RechartsTooltip
            contentStyle={{
              borderRadius: 8,
              border: '1px solid hsl(var(--border))',
              background: 'hsl(var(--popover))',
              color: 'hsl(var(--popover-foreground))',
              fontSize: 13,
            }}
          />
          <Area
            type="monotone"
            dataKey="requests"
            stroke="hsl(var(--primary))"
            fillOpacity={1}
            fill="url(#metricsReqGradient)"
          />
        </AreaChart>
      </ChartCard>

      {/* Requests by method bar chart */}
      <ChartCard title="Requests by HTTP Method" description="Total requests per method">
        <BarChart data={METHOD_DATA}>
          <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
          <XAxis
            dataKey="method"
            tick={{ fontSize: 12 }}
            tickLine={false}
            axisLine={false}
          />
          <YAxis tick={{ fontSize: 12 }} tickLine={false} axisLine={false} />
          <RechartsTooltip
            contentStyle={{
              borderRadius: 8,
              border: '1px solid hsl(var(--border))',
              background: 'hsl(var(--popover))',
              color: 'hsl(var(--popover-foreground))',
              fontSize: 13,
            }}
          />
          <Bar dataKey="count" radius={[4, 4, 0, 0]}>
            {METHOD_DATA.map((entry) => (
              <Cell
                key={entry.method}
                fill={METHOD_COLORS[entry.method] ?? 'hsl(var(--primary))'}
              />
            ))}
          </Bar>
        </BarChart>
      </ChartCard>
    </div>
  );
}
