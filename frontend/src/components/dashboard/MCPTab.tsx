import { useState, useEffect } from 'react';
import {
  Server,
  Wrench,
  Download,
  BarChart3,
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
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { StatCard } from '@/components/dashboard/StatCard';
import { ChartCard } from '@/components/dashboard/ChartCard';
import type { MCPServer, MCPServerWithTools, GatewayStats } from '@/types/mcp';

function statusVariant(status: MCPServer['status']) {
  switch (status) {
    case 'ready':
      return 'success' as const;
    case 'building':
      return 'warning' as const;
    case 'error':
      return 'destructive' as const;
    default:
      return 'secondary' as const;
  }
}

export function MCPTab() {
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [gatewayStats, setGatewayStats] = useState<GatewayStats[]>([]);
  const [toolServers, setToolServers] = useState<MCPServerWithTools[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function fetchAll() {
      setLoading(true);
      try {
        const [srv, stats, tools] = await Promise.all([
          api.listMyMCPServers(),
          api.getMCPGatewayStats(),
          api.listMCPTools(),
        ]);
        if (!cancelled) {
          setServers(srv);
          setGatewayStats(stats);
          setToolServers(tools);
          setError(null);
        }
      } catch (err: unknown) {
        if (!cancelled) setError((err as Error).message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchAll();
    return () => {
      cancelled = true;
    };
  }, []);

  const installedCount = servers.filter((s) => s.status === 'ready').length;
  const totalTools = toolServers.reduce((acc, s) => acc + (s.tools?.length ?? 0), 0);
  const totalGatewayRequests = gatewayStats.reduce((acc, s) => acc + s.total_calls, 0);

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="grid gap-4 sm:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24 rounded-lg" />
          ))}
        </div>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40 rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-6 text-center text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
        Failed to load MCP data: {error}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Stat cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Total Servers"
          value={servers.length}
          icon={<Server className="h-4 w-4" />}
        />
        <StatCard
          label="Installed (Ready)"
          value={installedCount}
          icon={<Download className="h-4 w-4" />}
          color="text-green-600"
        />
        <StatCard
          label="Total Tools"
          value={totalTools}
          icon={<Wrench className="h-4 w-4" />}
        />
        <StatCard
          label="Gateway Requests"
          value={totalGatewayRequests}
          icon={<BarChart3 className="h-4 w-4" />}
        />
      </div>

      {/* Server cards grid */}
      <div>
        <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Servers
        </h3>
        {servers.length === 0 ? (
          <p className="text-sm text-muted-foreground">No MCP servers registered.</p>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {servers.map((server) => {
              const serverTools =
                toolServers.find((ts) => ts.id === server.id)?.tools ?? [];
              return (
                <Card key={server.id}>
                  <CardHeader className="pb-2">
                    <div className="flex items-start justify-between gap-2">
                      <CardTitle className="text-base">{server.name}</CardTitle>
                      <Badge variant={statusVariant(server.status)}>
                        {server.status}
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-2">
                    <p className="line-clamp-2 text-xs text-muted-foreground">
                      {server.description || 'No description'}
                    </p>
                    <div className="flex items-center gap-3 text-xs text-muted-foreground">
                      <span className="flex items-center gap-1">
                        <Wrench className="h-3 w-3" />
                        {serverTools.length} tool{serverTools.length !== 1 && 's'}
                      </span>
                      {server.version && (
                        <span className="font-mono">v{server.version}</span>
                      )}
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}
      </div>

      {/* Gateway calls per server chart */}
      {gatewayStats.length > 0 && (
        <ChartCard
          title="Gateway Calls per Server"
          description="Total gateway invocations by MCP server"
        >
          <BarChart data={gatewayStats}>
            <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
            <XAxis dataKey="server_name" tick={{ fontSize: 12 }} />
            <YAxis allowDecimals={false} tick={{ fontSize: 12 }} />
            <Tooltip />
            <Bar
              dataKey="total_calls"
              fill="hsl(var(--primary))"
              radius={[4, 4, 0, 0]}
            />
          </BarChart>
        </ChartCard>
      )}

      {/* Tool catalog */}
      <div>
        <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Tool Catalog
        </h3>
        {toolServers.length === 0 ? (
          <p className="text-sm text-muted-foreground">No tools available.</p>
        ) : (
          <div className="space-y-4">
            {toolServers.map((ts) => (
              <Card key={ts.id}>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm">{ts.name}</CardTitle>
                </CardHeader>
                <CardContent>
                  {ts.tools && ts.tools.length > 0 ? (
                    <ul className="space-y-1">
                      {ts.tools.map((tool) => (
                        <li
                          key={tool.name}
                          className="flex items-start gap-2 text-sm"
                        >
                          <Wrench className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                          <div>
                            <span className="font-mono text-xs font-medium">
                              {tool.name}
                            </span>
                            {tool.description && (
                              <span className="ml-2 text-xs text-muted-foreground">
                                &mdash; {tool.description}
                              </span>
                            )}
                          </div>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="text-xs text-muted-foreground">No tools exposed.</p>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
