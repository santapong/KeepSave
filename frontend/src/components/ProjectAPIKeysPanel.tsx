import { useState, useEffect, type FormEvent } from 'react';
import { listAPIKeys, createAPIKey, deleteAPIKey } from '../api/client';
import type { APIKey } from '../types';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/ui/select';
import { Plus, Trash2, Copy, Check, Key } from 'lucide-react';

interface ProjectAPIKeysPanelProps {
  projectId: string;
}

const SCOPES = ['read', 'write', 'delete', 'promote'] as const;
const SCOPE_DESCRIPTIONS: Record<string, string> = {
  read: 'Fetch and list secrets, export .env files',
  write: 'Create and update secret values',
  delete: 'Permanently delete secrets from an environment',
  promote: 'Create promotion requests and approve/reject them',
};

export function ProjectAPIKeysPanel({ projectId }: ProjectAPIKeysPanelProps) {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteError, setDeleteError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [selectedScopes, setSelectedScopes] = useState<Set<string>>(new Set(['read']));
  const [environment, setEnvironment] = useState('');
  const [newRawKey, setNewRawKey] = useState('');
  const [creating, setCreating] = useState(false);
  const [copied, setCopied] = useState(false);
  const { toast } = useToast();

  useEffect(() => {
    listAPIKeys()
      .then((all) => setKeys(all.filter((k) => k.project_id === projectId)))
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load API keys'))
      .finally(() => setLoading(false));
  }, [projectId]);

  function toggleScope(scope: string) {
    setSelectedScopes((prev) => {
      const next = new Set(prev);
      if (next.has(scope)) next.delete(scope);
      else next.add(scope);
      return next;
    });
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setError('');
    setCreating(true);
    try {
      const resp = await createAPIKey(
        name,
        projectId,
        Array.from(selectedScopes).sort(),
        environment || undefined
      );
      setNewRawKey(resp.raw_key);
      setName('');
      setSelectedScopes(new Set(['read']));
      setEnvironment('');
      toast({ title: 'Created', description: 'API key created successfully' });
      const all = await listAPIKeys();
      setKeys(all.filter((k) => k.project_id === projectId));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create API key');
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm('Delete this API key? This cannot be undone.')) return;
    setDeleteError('');
    try {
      await deleteAPIKey(id);
      setKeys((prev) => prev.filter((k) => k.id !== id));
      toast({ title: 'Deleted', description: 'API key deleted' });
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete API key');
    }
  }

  function handleToggleCreate() {
    setShowCreate((v) => !v);
    setNewRawKey('');
    setCopied(false);
  }

  async function handleCopy() {
    await navigator.clipboard.writeText(newRawKey);
    setCopied(true);
    toast({ title: 'Copied', description: 'API key copied to clipboard' });
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div>
      {/* Header */}
      <div className="flex justify-between items-start mb-5">
        <div>
          <h2 className="text-lg font-semibold">API Keys</h2>
          <p className="text-sm text-muted-foreground mt-1">
            Scoped keys for AI agents, CI/CD pipelines, and services to access this project's secrets.
          </p>
        </div>
        <Button onClick={handleToggleCreate} variant={showCreate ? 'outline' : 'default'}>
          {showCreate ? 'Cancel' : <><Plus className="mr-2 h-4 w-4" /> Create API Key</>}
        </Button>
      </div>

      {/* Errors */}
      {error && (
        <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm mb-4">
          {error}
        </div>
      )}
      {deleteError && (
        <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm mb-4">
          {deleteError}
        </div>
      )}

      {/* One-time raw key success box */}
      {newRawKey && (
        <Card className="mb-4 border-green-500/25 bg-green-500/10">
          <CardContent className="p-4">
            <p className="font-semibold text-sm mb-1">API Key Created</p>
            <p className="text-sm text-muted-foreground mb-2.5">
              Copy this key now — it will not be shown again.
            </p>
            <div className="flex items-center gap-2">
              <code className="text-sm break-all bg-muted px-2 py-1 rounded border border-border flex-1 font-mono">
                {newRawKey}
              </code>
              <Button variant="outline" size="sm" onClick={handleCopy} className="shrink-0">
                {copied ? (
                  <><Check className="mr-1 h-3 w-3 text-green-500" /> Copied!</>
                ) : (
                  <><Copy className="mr-1 h-3 w-3" /> Copy</>
                )}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Create form */}
      {showCreate && (
        <Card className="mb-6">
          <CardContent className="p-5">
            <form onSubmit={handleCreate}>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label className="text-sm font-medium">Key Name</Label>
                  <Input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                    placeholder="my-agent-key"
                    className="mt-1"
                  />
                </div>
                <div>
                  <Label className="text-sm font-medium">Environment</Label>
                  <Select value={environment} onValueChange={setEnvironment}>
                    <SelectTrigger className="mt-1">
                      <SelectValue placeholder="All environments" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="">All environments</SelectItem>
                      <SelectItem value="alpha">Alpha</SelectItem>
                      <SelectItem value="uat">UAT</SelectItem>
                      <SelectItem value="prod">PROD</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                {/* Scopes */}
                <div className="col-span-2">
                  <Label className="text-sm font-medium block mb-2">Scopes</Label>
                  <div className="flex flex-col gap-1.5">
                    {SCOPES.map((scope) => (
                      <label
                        key={scope}
                        className={cn(
                          'flex items-start gap-2 p-2 px-2.5 rounded cursor-pointer border transition-colors',
                          selectedScopes.has(scope)
                            ? 'bg-primary/5 border-primary'
                            : 'bg-transparent border-border'
                        )}
                      >
                        <input
                          type="checkbox"
                          checked={selectedScopes.has(scope)}
                          onChange={() => toggleScope(scope)}
                          className="mt-0.5 shrink-0 rounded"
                        />
                        <div>
                          <code className="text-xs font-semibold">{scope}</code>
                          <span className="text-xs text-muted-foreground ml-2">
                            {SCOPE_DESCRIPTIONS[scope]}
                          </span>
                        </div>
                      </label>
                    ))}
                  </div>
                </div>
              </div>

              <Button
                type="submit"
                disabled={creating || selectedScopes.size === 0}
                className="mt-4"
              >
                {creating ? 'Creating...' : 'Create Key'}
              </Button>
            </form>
          </CardContent>
        </Card>
      )}

      {/* Content */}
      {loading ? (
        <div className="space-y-2">
          {[1, 2].map((i) => (
            <Skeleton key={i} className="h-14 w-full rounded-lg" />
          ))}
        </div>
      ) : keys.length === 0 ? (
        <Card className="text-center py-8 px-6">
          <CardContent className="p-0 flex flex-col items-center">
            <Key className="h-8 w-8 text-muted-foreground opacity-50 mb-3" />
            <p className="font-semibold text-sm mb-1">No API keys for this project</p>
            <p className="text-sm text-muted-foreground">
              Create a key above to allow agents, scripts, or CI/CD pipelines to access this project's secrets.
            </p>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Scopes</TableHead>
                <TableHead>Environment</TableHead>
                <TableHead>Created</TableHead>
                <TableHead></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {keys.map((k) => (
                <TableRow key={k.id}>
                  <TableCell>
                    <strong className="text-sm">{k.name}</strong>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {k.scopes?.map((s) => (
                        <Badge key={s} variant="outline" className="text-[11px] font-normal">
                          {s}
                        </Badge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell>
                    {k.environment ? (
                      <span className="text-xs font-semibold uppercase">
                        {k.environment}
                      </span>
                    ) : (
                      <span className="text-xs text-muted-foreground">All</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <span className="text-xs text-muted-foreground">
                      {new Date(k.created_at).toLocaleDateString()}
                    </span>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="outline"
                      size="sm"
                      className="text-destructive border-destructive hover:bg-destructive/10 h-7 text-xs"
                      onClick={() => handleDelete(k.id)}
                    >
                      <Trash2 className="mr-1 h-3 w-3" /> Delete
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}
    </div>
  );
}
