import { useState, useEffect, useMemo } from 'react';
import {
  listPromotions,
  approvePromotion,
  rejectPromotion,
  rollbackPromotion,
} from '../api/client';
import { formatDate } from '../utils/formatDate';
import type { PromotionRequest } from '../types';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Layers, RotateCcw, CheckCircle2, XCircle } from 'lucide-react';

interface PromotionsListProps {
  projectId: string;
}

type StatusFilter = 'all' | 'pending' | 'completed' | 'rejected';

const FILTERS: { key: StatusFilter; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'pending', label: 'Pending' },
  { key: 'completed', label: 'Completed' },
  { key: 'rejected', label: 'Rejected' },
];

const statusBadgeClasses: Record<string, string> = {
  pending: 'bg-amber-500/15 text-amber-500',
  approved: 'bg-indigo-500/15 text-indigo-400',
  completed: 'bg-green-500/15 text-green-500',
  rejected: 'bg-red-500/15 text-red-500',
};

function getLeftBorderClass(source: string, target: string): string {
  if (source === 'uat' && target === 'prod') return 'border-l-amber-500';
  return 'border-l-primary';
}

export function PromotionsList({ projectId }: PromotionsListProps) {
  const [promotions, setPromotions] = useState<PromotionRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState<StatusFilter>('all');
  const { toast } = useToast();

  useEffect(() => {
    loadPromotions();
  }, [projectId]); // eslint-disable-line react-hooks/exhaustive-deps

  async function loadPromotions() {
    setLoading(true);
    try {
      const data = await listPromotions(projectId);
      setPromotions(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load promotions');
    } finally {
      setLoading(false);
    }
  }

  async function handleApprove(id: string) {
    try {
      await approvePromotion(projectId, id);
      toast({ title: 'Approved', description: 'Promotion approved successfully' });
      loadPromotions();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to approve');
    }
  }

  async function handleReject(id: string) {
    try {
      await rejectPromotion(projectId, id);
      toast({ title: 'Rejected', description: 'Promotion rejected' });
      loadPromotions();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reject');
    }
  }

  async function handleRollback(id: string) {
    if (!window.confirm('Rollback this promotion? Target environment secrets will be restored.')) return;
    try {
      await rollbackPromotion(projectId, id);
      toast({ title: 'Rolled back', description: 'Promotion rolled back successfully' });
      loadPromotions();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to rollback');
    }
  }

  const counts = useMemo(() => {
    const map: Record<StatusFilter, number> = { all: promotions.length, pending: 0, completed: 0, rejected: 0 };
    for (const p of promotions) {
      if (p.status === 'pending') map.pending++;
      else if (p.status === 'completed') map.completed++;
      else if (p.status === 'rejected') map.rejected++;
    }
    return map;
  }, [promotions]);

  const filtered = useMemo(() => {
    if (filter === 'all') return promotions;
    return promotions.filter((p) => p.status === filter);
  }, [promotions, filter]);

  if (loading) {
    return (
      <div className="space-y-3">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-24 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  return (
    <div>
      {error && (
        <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm mb-4">
          {error}
        </div>
      )}

      {/* Status filter bar */}
      <div className="flex gap-2 mb-4 flex-wrap">
        {FILTERS.map((f) => {
          const active = filter === f.key;
          return (
            <Button
              key={f.key}
              onClick={() => setFilter(f.key)}
              variant={active ? 'default' : 'outline'}
              size="sm"
              className="rounded-full font-semibold"
            >
              {f.label}
              <Badge
                variant="secondary"
                className={cn(
                  'ml-1.5 text-[11px] font-bold px-1.5 min-w-[20px] h-4',
                  active ? 'bg-white/25 text-white' : 'bg-muted text-muted-foreground'
                )}
              >
                {counts[f.key]}
              </Badge>
            </Button>
          );
        })}
      </div>

      {/* Result count */}
      {filter !== 'all' && (
        <p className="text-xs text-muted-foreground mb-3">
          Showing {filtered.length} of {promotions.length} promotions
        </p>
      )}

      {filtered.length === 0 ? (
        <Card className="text-center py-8 px-4">
          <CardContent className="p-0 flex flex-col items-center">
            <Layers className="h-10 w-10 text-muted-foreground opacity-50 mb-3" />
            <p className="text-sm font-semibold mb-1">
              {filter === 'all' ? 'No promotions yet' : `No ${filter} promotions`}
            </p>
            <p className="text-sm text-muted-foreground">
              {filter === 'all'
                ? 'Promote secrets between environments to see them here.'
                : 'Try a different filter to see more results.'}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3">
          {filtered.map((p) => (
            <Card
              key={p.id}
              className={cn('border-l-[3px]', getLeftBorderClass(p.source_environment, p.target_environment))}
            >
              <CardContent className="p-4">
                <div className="flex justify-between items-start">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-semibold">
                        {p.source_environment.toUpperCase()} &rarr; {p.target_environment.toUpperCase()}
                      </span>
                      <Badge
                        variant="secondary"
                        className={cn(
                          'text-[11px] uppercase font-semibold',
                          statusBadgeClasses[p.status] || 'bg-muted text-muted-foreground'
                        )}
                      >
                        {p.status}
                      </Badge>
                    </div>
                    {p.notes && (
                      <p className="text-sm text-muted-foreground mt-1">
                        {p.notes}
                      </p>
                    )}
                    <p className="text-xs text-muted-foreground mt-1">
                      Policy: {p.override_policy} | {formatDate(p.created_at)}
                    </p>
                    {p.keys_filter && p.keys_filter.length > 0 && (
                      <p className="text-xs text-muted-foreground mt-0.5">
                        Keys: {p.keys_filter.join(', ')}
                      </p>
                    )}
                  </div>
                  <div className="flex gap-1.5">
                    {p.status === 'pending' && (
                      <>
                        <Button onClick={() => handleApprove(p.id)} size="sm" className="bg-green-500 hover:bg-green-600 text-white">
                          <CheckCircle2 className="mr-1 h-3 w-3" /> Approve
                        </Button>
                        <Button onClick={() => handleReject(p.id)} size="sm" variant="destructive">
                          <XCircle className="mr-1 h-3 w-3" /> Reject
                        </Button>
                      </>
                    )}
                    {p.status === 'completed' && (
                      <Button onClick={() => handleRollback(p.id)} size="sm" variant="outline">
                        <RotateCcw className="mr-1 h-3 w-3" /> Rollback
                      </Button>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
