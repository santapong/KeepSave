import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { listSecrets, createSecret, updateSecret, deleteSecret } from '../api/client';
import { formatDate } from '../utils/formatDate';
import type { Secret } from '../types';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
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
  Plus,
  X,
  Eye,
  EyeOff,
  Copy,
  Check,
  Pencil,
  Trash2,
  Search,
  Lock,
} from 'lucide-react';

const ENVIRONMENTS = ['alpha', 'uat', 'prod'] as const;
type Environment = (typeof ENVIRONMENTS)[number];

const ENV_BADGE_CLASSES: Record<Environment, string> = {
  alpha: 'bg-green-500 hover:bg-green-600 text-white',
  uat: 'bg-indigo-500 hover:bg-indigo-600 text-white',
  prod: 'bg-amber-500 hover:bg-amber-600 text-white',
};

const ENV_OUTLINE_CLASSES: Record<Environment, string> = {
  alpha: 'border-green-500 text-green-500',
  uat: 'border-indigo-500 text-indigo-500',
  prod: 'border-amber-500 text-amber-500',
};

const ENV_TEXT_CLASSES: Record<Environment, string> = {
  alpha: 'text-green-500',
  uat: 'text-indigo-500',
  prod: 'text-amber-500',
};

const ENV_BG_CLASSES: Record<Environment, string> = {
  alpha: 'bg-green-500',
  uat: 'bg-indigo-500',
  prod: 'bg-amber-500',
};

interface SecretsPanelProps {
  projectId: string;
}

export function SecretsPanel({ projectId }: SecretsPanelProps) {
  const [env, setEnv] = useState<Environment>('alpha');
  const [secrets, setSecrets] = useState<Secret[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showAdd, setShowAdd] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');
  const [revealed, setRevealed] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const { toast } = useToast();

  const loadSecrets = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const data = await listSecrets(projectId, env);
      setSecrets(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load secrets');
    } finally {
      setLoading(false);
    }
  }, [projectId, env]);

  useEffect(() => {
    loadSecrets();
    setRevealed(new Set());
    setEditing(null);
    setEditValue('');
    setSearchQuery('');
  }, [loadSecrets]);

  const filteredSecrets = secrets.filter((s) =>
    s.key.toLowerCase().includes(searchQuery.toLowerCase())
  );

  async function handleAdd(e: FormEvent) {
    e.preventDefault();
    try {
      await createSecret(projectId, newKey.toUpperCase(), newValue, env);
      setNewKey('');
      setNewValue('');
      setShowAdd(false);
      toast({ title: 'Created', description: `Secret ${newKey.toUpperCase()} added to ${env.toUpperCase()}` });
      loadSecrets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create secret');
    }
  }

  async function handleUpdate(secretId: string) {
    try {
      await updateSecret(projectId, secretId, editValue);
      setEditing(null);
      setEditValue('');
      toast({ title: 'Updated', description: 'Secret value updated' });
      loadSecrets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update secret');
    }
  }

  async function handleDelete(secretId: string) {
    if (!window.confirm('Delete this secret? This action cannot be undone.')) return;
    try {
      await deleteSecret(projectId, secretId);
      toast({ title: 'Deleted', description: 'Secret deleted' });
      loadSecrets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete secret');
    }
  }

  function toggleReveal(id: string) {
    setRevealed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleRevealAll() {
    if (revealed.size === filteredSecrets.length && filteredSecrets.length > 0) {
      setRevealed(new Set());
    } else {
      setRevealed(new Set(filteredSecrets.map((s) => s.id)));
    }
  }

  async function copyToClipboard(secretId: string, value: string | undefined) {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopiedId(secretId);
      toast({ title: 'Copied', description: 'Secret value copied to clipboard' });
      setTimeout(() => setCopiedId(null), 2000);
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = value;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
      setCopiedId(secretId);
      setTimeout(() => setCopiedId(null), 2000);
    }
  }

  function maskValue(): string {
    return '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022';
  }

  const allRevealed = filteredSecrets.length > 0 && revealed.size === filteredSecrets.length;

  return (
    <div>
      {/* Environment Tabs */}
      <div className="flex gap-2 mb-4 items-center">
        {ENVIRONMENTS.map((e) => {
          const isActive = env === e;
          return (
            <Button
              key={e}
              onClick={() => setEnv(e)}
              variant="outline"
              size="sm"
              className={cn(
                'uppercase tracking-wider font-semibold text-xs',
                isActive
                  ? ENV_BADGE_CLASSES[e]
                  : ENV_OUTLINE_CLASSES[e]
              )}
            >
              {e}
            </Button>
          );
        })}
        <div className="flex-1" />
        <Button
          onClick={() => setShowAdd(!showAdd)}
          variant={showAdd ? 'outline' : 'default'}
          size="sm"
        >
          {showAdd ? (
            <><X className="mr-1 h-3.5 w-3.5" /> Cancel</>
          ) : (
            <><Plus className="mr-1 h-3.5 w-3.5" /> Add Secret</>
          )}
        </Button>
      </div>

      {/* Error Banner */}
      {error && (
        <div className="flex items-center bg-destructive/10 text-destructive border border-destructive rounded-md px-3.5 py-2.5 text-sm mb-4 font-medium">
          <span className="mr-2">!</span>
          {error}
          <button
            onClick={() => setError('')}
            className="ml-auto bg-transparent border-none text-destructive cursor-pointer text-sm font-bold px-1"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Add Secret Form */}
      {showAdd && (
        <Card className="mb-4">
          <CardContent className="p-5">
            <form onSubmit={handleAdd}>
              <div className="text-sm font-semibold mb-3">
                Add new secret to <span className={cn('uppercase', ENV_TEXT_CLASSES[env])}>{env}</span>
              </div>
              <div className="flex gap-3 mb-3">
                <div className="flex flex-col gap-1.5 flex-1">
                  <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Key</Label>
                  <Input
                    placeholder="e.g. DATABASE_URL"
                    value={newKey}
                    onChange={(e) => setNewKey(e.target.value.toUpperCase())}
                    required
                    className="font-mono font-semibold uppercase"
                  />
                </div>
                <div className="flex flex-col gap-1.5 flex-[2]">
                  <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Value</Label>
                  <Input
                    placeholder="Enter the secret value"
                    value={newValue}
                    onChange={(e) => setNewValue(e.target.value)}
                    required
                    type="password"
                  />
                </div>
              </div>
              <div className="flex justify-end gap-2 mt-1">
                <Button type="button" variant="outline" size="sm" onClick={() => setShowAdd(false)}>
                  Cancel
                </Button>
                <Button type="submit" size="sm">
                  Save
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {/* Summary Bar + Search */}
      {!loading && (
        <div className="flex items-center justify-between mb-3 gap-3">
          <div className="flex items-center">
            <span className={cn(
              'inline-flex items-center justify-center min-w-[24px] h-6 rounded-full text-white text-xs font-bold mr-2 px-1.5',
              ENV_BG_CLASSES[env]
            )}>
              {secrets.length}
            </span>
            <span className="text-sm text-muted-foreground">
              {secrets.length === 1 ? 'secret' : 'secrets'} in{' '}
              <span className={cn('font-semibold uppercase', ENV_TEXT_CLASSES[env])}>
                {env}
              </span>
            </span>
          </div>
          <div className="flex items-center gap-2">
            {secrets.length > 0 && (
              <>
                <Button variant="outline" size="sm" onClick={toggleRevealAll} className="text-xs">
                  {allRevealed ? (
                    <><EyeOff className="mr-1 h-3 w-3" /> Hide All</>
                  ) : (
                    <><Eye className="mr-1 h-3 w-3" /> Reveal All</>
                  )}
                </Button>
                <div className="relative flex items-center">
                  <Search className="absolute left-2.5 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
                  <Input
                    type="text"
                    placeholder="Filter by key name..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className="pl-8 h-8 text-sm w-[220px]"
                  />
                  {searchQuery && (
                    <button
                      onClick={() => setSearchQuery('')}
                      className="absolute right-2 bg-transparent border-none text-muted-foreground cursor-pointer p-0"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {/* Search results info */}
      {searchQuery && !loading && (
        <div className="text-xs text-muted-foreground mb-2">
          Showing {filteredSecrets.length} of {secrets.length} secrets matching &quot;{searchQuery}&quot;
        </div>
      )}

      {/* Content */}
      {loading ? (
        <div className="flex flex-col items-center justify-center py-12 gap-4">
          <Skeleton className="h-6 w-6 rounded-full" />
          <Skeleton className="h-4 w-32" />
        </div>
      ) : secrets.length === 0 ? (
        <Card className="text-center py-12 px-6">
          <CardContent className="p-0 flex flex-col items-center">
            <Lock className={cn('h-12 w-12 mb-4 opacity-70', ENV_TEXT_CLASSES[env])} />
            <h3 className="text-base font-semibold mb-2">
              No secrets in {env.toUpperCase()}
            </h3>
            <p className="text-sm text-muted-foreground mb-5 max-w-[360px] leading-relaxed">
              Secrets are encrypted key-value pairs stored securely for this environment.
              Add your first secret to get started with the{' '}
              <span className={cn('font-semibold uppercase', ENV_TEXT_CLASSES[env])}>{env}</span>{' '}
              environment.
            </p>
            <Button onClick={() => setShowAdd(true)}>
              <Plus className="mr-2 h-4 w-4" /> Add Your First Secret
            </Button>
          </CardContent>
        </Card>
      ) : filteredSecrets.length === 0 ? (
        <Card className="text-center py-10 px-6">
          <CardContent className="p-0">
            <p className="text-sm text-muted-foreground">
              No secrets matching &quot;{searchQuery}&quot; in {env.toUpperCase()}.
            </p>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Key</TableHead>
                <TableHead>Value</TableHead>
                <TableHead className="w-[160px]">Updated</TableHead>
                <TableHead className="w-[220px]">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredSecrets.map((s) => (
                <TableRow key={s.id} className="hover:bg-muted/50">
                  <TableCell>
                    <code className="text-sm font-semibold font-mono tracking-wide">{s.key}</code>
                  </TableCell>
                  <TableCell>
                    {editing === s.id ? (
                      <div className="flex gap-2 items-center">
                        <Input
                          value={editValue}
                          onChange={(e) => setEditValue(e.target.value)}
                          className="flex-1 h-8 text-sm"
                          autoFocus
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') handleUpdate(s.id);
                            if (e.key === 'Escape') setEditing(null);
                          }}
                        />
                        <Button onClick={() => handleUpdate(s.id)} size="sm" className="h-8 text-xs">
                          Save
                        </Button>
                        <Button onClick={() => setEditing(null)} variant="outline" size="sm" className="h-8 text-xs">
                          Cancel
                        </Button>
                      </div>
                    ) : (
                      <code className="text-sm font-mono text-muted-foreground">
                        {revealed.has(s.id) ? s.value : maskValue()}
                      </code>
                    )}
                  </TableCell>
                  <TableCell>
                    <span className="text-xs text-muted-foreground">
                      {formatDate(s.updated_at)}
                    </span>
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1.5">
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs px-2"
                        onClick={() => toggleReveal(s.id)}
                        title={revealed.has(s.id) ? 'Hide value' : 'Reveal value'}
                      >
                        {revealed.has(s.id) ? (
                          <EyeOff className="h-3 w-3" />
                        ) : (
                          <Eye className="h-3 w-3" />
                        )}
                      </Button>
                      {revealed.has(s.id) && (
                        <Button
                          variant="outline"
                          size="sm"
                          className={cn(
                            'h-7 text-xs px-2',
                            copiedId === s.id && 'text-green-500 border-green-500'
                          )}
                          onClick={() => copyToClipboard(s.id, s.value)}
                          title="Copy to clipboard"
                        >
                          {copiedId === s.id ? (
                            <Check className="h-3 w-3" />
                          ) : (
                            <Copy className="h-3 w-3" />
                          )}
                        </Button>
                      )}
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs px-2"
                        onClick={() => {
                          setEditing(s.id);
                          setEditValue(s.value || '');
                        }}
                        title="Edit value"
                      >
                        <Pencil className="h-3 w-3" />
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs px-2 text-destructive border-destructive hover:bg-destructive/10"
                        onClick={() => handleDelete(s.id)}
                        title="Delete secret"
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </div>
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
