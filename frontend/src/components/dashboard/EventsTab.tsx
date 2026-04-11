import { useState, useEffect, useMemo } from 'react';
import {
  Search,
  Radio,
  Send,
  Clock,
  RefreshCw,
} from 'lucide-react';
import * as api from '@/api/client';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
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
import type { PlatformEvent, WebhookDelivery } from '@/types';

function formatTimestamp(iso: string): string {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

export function EventsTab() {
  const [events, setEvents] = useState<PlatformEvent[]>([]);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [eventSearch, setEventSearch] = useState('');
  const [replayingType, setReplayingType] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function fetchAll() {
      setLoading(true);
      try {
        const [evts, dlvr] = await Promise.all([
          api.getEvents(),
          api.getWebhookDeliveries(),
        ]);
        if (!cancelled) {
          setEvents(evts as unknown as PlatformEvent[]);
          setDeliveries(dlvr as unknown as WebhookDelivery[]);
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

  const filteredEvents = useMemo(() => {
    if (!eventSearch.trim()) return events;
    const q = eventSearch.toLowerCase();
    return events.filter(
      (e) =>
        e.event_type.toLowerCase().includes(q) ||
        e.aggregate_id.toLowerCase().includes(q),
    );
  }, [events, eventSearch]);

  const publishedCount = events.filter((e) => e.published).length;
  const pendingCount = events.length - publishedCount;

  async function handleReplay(eventType: string) {
    setReplayingType(eventType);
    try {
      await api.replayEvents(eventType);
    } catch {
      // Silently handle -- the UI will still reset the button state
    } finally {
      setReplayingType(null);
    }
  }

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="grid gap-4 sm:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24 rounded-lg" />
          ))}
        </div>
        <Skeleton className="h-64 rounded-lg" />
        <Skeleton className="h-64 rounded-lg" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-6 text-center text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
        Failed to load events: {error}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Stat cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Total Events"
          value={events.length}
          icon={<Radio className="h-4 w-4" />}
        />
        <StatCard
          label="Published"
          value={publishedCount}
          icon={<Send className="h-4 w-4" />}
          color="text-green-600"
        />
        <StatCard
          label="Pending"
          value={pendingCount}
          icon={<Clock className="h-4 w-4" />}
          color="text-amber-600"
        />
        <StatCard
          label="Webhook Deliveries"
          value={deliveries.length}
          icon={<RefreshCw className="h-4 w-4" />}
        />
      </div>

      {/* Event Bus */}
      <section>
        <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Event Bus
        </h3>

        <div className="relative mb-4">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search by event type or aggregate ID..."
            value={eventSearch}
            onChange={(e) => setEventSearch(e.target.value)}
            className="pl-9"
          />
        </div>

        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Event Type</TableHead>
                <TableHead>Aggregate ID</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Timestamp</TableHead>
                <TableHead className="w-[100px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredEvents.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">
                    No events found.
                  </TableCell>
                </TableRow>
              ) : (
                filteredEvents.map((evt) => (
                  <TableRow key={evt.id}>
                    <TableCell className="font-mono text-xs">
                      {evt.event_type}
                    </TableCell>
                    <TableCell
                      className="max-w-[140px] truncate font-mono text-xs text-muted-foreground"
                      title={evt.aggregate_id}
                    >
                      {evt.aggregate_id}
                    </TableCell>
                    <TableCell>
                      <Badge variant={evt.published ? 'success' : 'warning'}>
                        {evt.published ? 'published' : 'pending'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatTimestamp(evt.created_at)}
                    </TableCell>
                    <TableCell>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={replayingType === evt.event_type}
                        onClick={() => handleReplay(evt.event_type)}
                      >
                        <RefreshCw
                          className={`mr-1 h-3 w-3 ${
                            replayingType === evt.event_type ? 'animate-spin' : ''
                          }`}
                        />
                        Replay
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </section>

      <Separator />

      {/* Webhook Deliveries */}
      <section>
        <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Webhook Deliveries
        </h3>

        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>URL</TableHead>
                <TableHead>Event Type</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Attempts</TableHead>
                <TableHead>Timestamp</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deliveries.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">
                    No webhook deliveries.
                  </TableCell>
                </TableRow>
              ) : (
                deliveries.map((d) => (
                  <TableRow key={d.id}>
                    <TableCell
                      className="max-w-[200px] truncate font-mono text-xs text-muted-foreground"
                      title={d.webhook_url}
                    >
                      {d.webhook_url}
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {d.event_type}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          d.status === 'success'
                            ? 'success'
                            : d.status === 'failed'
                              ? 'destructive'
                              : 'secondary'
                        }
                      >
                        {d.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs">{d.attempts}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatTimestamp(d.created_at)}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </section>
    </div>
  );
}
