import { useState, useEffect, useMemo } from 'react';
import { listAuditLog } from '../api/client';
import { formatDate } from '../utils/formatDate';
import type { AuditEntry } from '../types';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Search, X, Info, ChevronDown, ChevronUp } from 'lucide-react';

interface AuditLogViewerProps {
  projectId: string;
}

const EXAMPLE_ENTRIES: AuditEntry[] = [
  {
    id: 'ex-1',
    action: 'secret.create',
    environment: 'alpha',
    details: { key: 'DATABASE_URL' },
    ip_address: '127.0.0.1',
    created_at: new Date(Date.now() - 1 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: 'ex-2',
    action: 'secret.update',
    environment: 'alpha',
    details: { key: 'API_KEY', version: 2 },
    ip_address: '127.0.0.1',
    created_at: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: 'ex-3',
    action: 'promotion.create',
    environment: 'uat',
    details: { source: 'alpha', target: 'uat', keys_count: 5 },
    ip_address: '127.0.0.1',
    created_at: new Date(Date.now() - 1 * 24 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: 'ex-4',
    action: 'promotion.approve',
    environment: 'uat',
    details: { promotion_id: 'abc-123' },
    ip_address: '10.0.0.1',
    created_at: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: 'ex-5',
    action: 'apikey.create',
    environment: '',
    details: { name: 'ci-pipeline', scopes: ['read'] },
    ip_address: '192.168.1.1',
    created_at: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString(),
  },
];

function getActionBadgeClass(action: string): string {
  if (action.startsWith('secret.create') || action.startsWith('secret.update')) {
    return 'bg-green-500/15 text-green-500';
  }
  if (action.startsWith('secret.delete')) {
    return 'bg-red-500/15 text-destructive';
  }
  if (action.startsWith('promotion.')) {
    return 'bg-indigo-500/15 text-primary';
  }
  if (action.startsWith('apikey.')) {
    return 'bg-blue-500/15 text-blue-500';
  }
  if (action.startsWith('auth.')) {
    return 'bg-gray-500/15 text-muted-foreground';
  }
  return 'bg-gray-500/15 text-muted-foreground';
}

function getEnvBadgeClass(env: string): string {
  switch (env.toLowerCase()) {
    case 'alpha':
      return 'bg-green-500/15 text-green-500';
    case 'uat':
      return 'bg-indigo-500/15 text-primary';
    case 'prod':
      return 'bg-amber-500/15 text-amber-500';
    default:
      return 'bg-gray-500/15 text-muted-foreground';
  }
}

export function AuditLogViewer({ projectId }: AuditLogViewerProps) {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());
  useEffect(() => {
    setLoading(true);
    setError('');
    listAuditLog(projectId)
      .then(setEntries)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load audit log'))
      .finally(() => setLoading(false));
  }, [projectId]);

  const isShowingExamples = !loading && entries.length === 0;
  const displayEntries = isShowingExamples ? EXAMPLE_ENTRIES : entries;

  const filteredEntries = useMemo(() => {
    if (!search.trim()) return displayEntries;
    const query = search.toLowerCase();
    return displayEntries.filter(
      (entry) =>
        entry.action.toLowerCase().includes(query) ||
        (entry.environment && entry.environment.toLowerCase().includes(query))
    );
  }, [displayEntries, search]);

  function toggleRow(id: string) {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center py-12 gap-4">
        <Skeleton className="h-6 w-6 rounded-full" />
        <Skeleton className="h-4 w-36" />
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className="mb-4">
        <div className="flex items-center gap-2.5">
          <h3 className="text-lg font-bold">Audit Log</h3>
          <Badge className="bg-primary text-white text-[11px] font-bold px-2 min-w-[24px] h-[22px]">
            {filteredEntries.length}
          </Badge>
        </div>
        <p className="text-sm text-muted-foreground mt-1">
          Track all changes, promotions, and access events for this project.
        </p>
      </div>

      {/* Error Banner */}
      {error && (
        <div className="flex items-center bg-destructive/10 text-destructive border border-destructive rounded-md px-3.5 py-2.5 text-sm mb-4 font-medium">
          <span className="mr-2 font-bold">!</span>
          {error}
          <button onClick={() => setError('')} className="ml-auto bg-transparent border-none text-destructive cursor-pointer text-sm font-bold px-1">
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Example entries info banner */}
      {isShowingExamples && (
        <div className="flex items-center gap-2.5 bg-blue-500/10 border border-blue-500/25 text-blue-500 rounded-md px-3.5 py-2.5 text-sm mb-4 leading-relaxed">
          <Info className="h-4 w-4 shrink-0" />
          <span>
            These are example entries showing what audit activity looks like.
            Create secrets or promote to see real entries.
          </span>
        </div>
      )}

      {/* Search */}
      <div className="flex items-center gap-3 mb-3">
        <div className="relative flex items-center flex-1 max-w-[360px]">
          <Search className="absolute left-2.5 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
          <Input
            type="text"
            placeholder="Filter by action or environment..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8 text-sm"
          />
          {search && (
            <button onClick={() => setSearch('')} className="absolute right-2 bg-transparent border-none text-muted-foreground cursor-pointer p-0">
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
        {search && (
          <span className="text-xs text-muted-foreground">
            Showing {filteredEntries.length} of {displayEntries.length} entries
          </span>
        )}
      </div>

      {/* Table */}
      {filteredEntries.length === 0 ? (
        <Card className="text-center py-10 px-6">
          <CardContent className="p-0">
            <p className="text-sm font-semibold mb-1">No matching entries</p>
            <p className="text-sm text-muted-foreground">
              Try a different search term or clear the filter.
            </p>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Environment</TableHead>
                <TableHead>Details</TableHead>
                <TableHead>IP Address</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredEntries.map((entry) => {
                const isExpanded = expandedRows.has(entry.id);
                return (
                  <TableRow key={entry.id} className="hover:bg-muted/50">
                    <TableCell className="align-top">
                      <span className="text-xs text-muted-foreground whitespace-nowrap" title={new Date(entry.created_at).toLocaleString()}>
                        {formatDate(entry.created_at)}
                      </span>
                    </TableCell>
                    <TableCell className="align-top">
                      <Badge
                        variant="secondary"
                        className={cn(
                          'text-[11px] font-semibold font-mono whitespace-nowrap',
                          getActionBadgeClass(entry.action)
                        )}
                      >
                        {entry.action}
                      </Badge>
                    </TableCell>
                    <TableCell className="align-top">
                      {entry.environment ? (
                        <Badge
                          variant="secondary"
                          className={cn(
                            'text-[11px] font-semibold uppercase tracking-wide',
                            getEnvBadgeClass(entry.environment)
                          )}
                        >
                          {entry.environment.toUpperCase()}
                        </Badge>
                      ) : (
                        <span className="text-xs text-muted-foreground">--</span>
                      )}
                    </TableCell>
                    <TableCell className="align-top">
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-6 text-[11px] px-2"
                        onClick={() => toggleRow(entry.id)}
                      >
                        {isExpanded ? (
                          <><ChevronUp className="mr-1 h-3 w-3" /> Hide</>
                        ) : (
                          <><ChevronDown className="mr-1 h-3 w-3" /> Details</>
                        )}
                      </Button>
                      {isExpanded && (
                        <pre className="mt-2 mb-0 p-2 px-3 bg-muted border border-border rounded-md text-xs font-mono overflow-auto max-w-[400px] whitespace-pre-wrap leading-relaxed">
                          {JSON.stringify(entry.details, null, 2)}
                        </pre>
                      )}
                    </TableCell>
                    <TableCell className="align-top">
                      <span className="text-xs font-mono text-muted-foreground">{entry.ip_address}</span>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </Card>
      )}
    </div>
  );
}
