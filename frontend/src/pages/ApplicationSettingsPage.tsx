import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import type { APIKey } from '../types';
import * as api from '../api/client';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/hooks/useToast';
import { ArrowLeft, Plus, Copy, Check, Trash2 } from 'lucide-react';

export function ApplicationSettingsPage() {
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [newKeyScopes, setNewKeyScopes] = useState<string[]>(['read']);
  const [createdKey, setCreatedKey] = useState('');
  const [copied, setCopied] = useState(false);
  const { toast } = useToast();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const keys = await api.listAPIKeys();
      setApiKeys(keys || []);
    } catch {
      // ignore
    }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const result = await api.createAPIKey(newKeyName, '', newKeyScopes);
      setCreatedKey(result.raw_key);
      setNewKeyName('');
      setNewKeyScopes(['read']);
      setShowCreate(false);
      toast({ title: 'API key created', description: 'Copy it now — it won\'t be shown again.' });
      load();
    } catch {
      toast({ title: 'Error', description: 'Failed to create API key.', variant: 'destructive' });
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Revoke this API key? This cannot be undone.')) return;
    try {
      await api.deleteAPIKey(id);
      toast({ title: 'API key revoked', description: 'The key has been permanently deleted.' });
      load();
    } catch {
      toast({ title: 'Error', description: 'Failed to revoke API key.', variant: 'destructive' });
    }
  };

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    toast({ title: 'Copied!', description: 'API key copied to clipboard.' });
    setTimeout(() => setCopied(false), 2000);
  };

  const toggleScope = (scope: string) => {
    setNewKeyScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
    );
  };

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <Button variant="link" asChild className="p-0 h-auto text-sm mb-2">
          <Link to="/applications">
            <ArrowLeft className="h-3.5 w-3.5 mr-1" />
            Back to Applications
          </Link>
        </Button>
        <h1 className="text-xl font-bold text-foreground mt-2">
          Application Dashboard Settings
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Manage API keys and view integration documentation
        </p>
      </div>

      {/* Created Key Banner */}
      {createdKey && (
        <Card className="mb-6 border-green-300 bg-green-50 dark:bg-green-950/20 dark:border-green-800">
          <CardContent className="p-4">
            <p className="text-sm font-semibold text-green-800 dark:text-green-300 mb-2">
              API Key Created! Copy it now — it won't be shown again.
            </p>
            <div className="flex gap-2 items-center">
              <code className="flex-1 px-3 py-2 bg-white dark:bg-background rounded-lg text-xs font-mono break-all text-green-800 dark:text-green-300 border">
                {createdKey}
              </code>
              <Button size="sm" onClick={() => handleCopy(createdKey)}>
                {copied ? <><Check className="h-3.5 w-3.5 mr-1" /> Copied!</> : <><Copy className="h-3.5 w-3.5 mr-1" /> Copy</>}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Two Column Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left: API Keys */}
        <div>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
              <CardTitle className="text-base">API Keys</CardTitle>
              <Button size="sm" onClick={() => setShowCreate(!showCreate)}>
                {showCreate ? 'Cancel' : <><Plus className="h-3.5 w-3.5 mr-1" /> New Key</>}
              </Button>
            </CardHeader>
            <CardContent>
              {showCreate && (
                <form onSubmit={handleCreate} className="p-4 bg-muted rounded-lg mb-4 border space-y-3">
                  <div className="space-y-1.5">
                    <Label className="text-xs">Key Name</Label>
                    <Input value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} required placeholder="e.g. ci-pipeline" />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">Scopes</Label>
                    <div className="flex gap-3 flex-wrap">
                      {['read', 'write', 'delete'].map((scope) => (
                        <label key={scope} className="flex items-center gap-1.5 text-sm text-foreground cursor-pointer">
                          <input type="checkbox" checked={newKeyScopes.includes(scope)} onChange={() => toggleScope(scope)} />
                          {scope}
                        </label>
                      ))}
                    </div>
                  </div>
                  <Button type="submit" className="w-full">Create API Key</Button>
                </form>
              )}

              {loading ? (
                <div className="space-y-3">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <div key={i} className="flex items-center justify-between p-3 border rounded-lg">
                      <div className="space-y-2">
                        <Skeleton className="h-4 w-32" />
                        <Skeleton className="h-3 w-48" />
                      </div>
                      <Skeleton className="h-7 w-16 rounded-md" />
                    </div>
                  ))}
                </div>
              ) : apiKeys.length === 0 ? (
                <p className="text-sm text-muted-foreground">No API keys yet. Create one to get started.</p>
              ) : (
                <div className="flex flex-col gap-2">
                  {apiKeys.map((key) => (
                    <div key={key.id} className="flex items-center justify-between p-3 bg-muted rounded-lg border">
                      <div>
                        <div className="text-sm font-semibold text-foreground">{key.name}</div>
                        <div className="text-xs text-muted-foreground mt-0.5 flex items-center gap-1.5">
                          <span>Scopes:</span>
                          {(key.scopes || ['read']).map((s) => (
                            <Badge key={s} variant="secondary" className="text-[10px] px-1.5 py-0">
                              {s}
                            </Badge>
                          ))}
                          <span>&middot; Created {new Date(key.created_at).toLocaleDateString()}</span>
                        </div>
                      </div>
                      <Button variant="destructive" size="sm" className="text-xs" onClick={() => handleDelete(key.id)}>
                        <Trash2 className="h-3 w-3 mr-1" />
                        Revoke
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Right: API Reference */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">API Reference</CardTitle>
            <CardDescription>
              Use these endpoints to manage applications programmatically. Authenticate with a Bearer token (JWT or API key).
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <EndpointDoc method="GET" path="/api/v1/applications" description="List applications with search, category filter, and pagination" params="?search=&category=&limit=50&offset=0" />
            <EndpointDoc method="POST" path="/api/v1/applications" description="Create a new application" body='{"name": "My App", "url": "https://...", "description": "...", "icon": "\uD83D\uDE80", "category": "General"}' />
            <EndpointDoc method="GET" path="/api/v1/applications/:id" description="Get a single application by ID" />
            <EndpointDoc method="PUT" path="/api/v1/applications/:id" description="Update an application" body='{"name": "...", "url": "...", ...}' />
            <EndpointDoc method="DELETE" path="/api/v1/applications/:id" description="Delete an application" />
            <EndpointDoc method="POST" path="/api/v1/applications/:id/favorite" description="Toggle favorite status" />

            <h3 className="text-sm font-bold text-foreground pt-4">Example Usage</h3>
            <pre className="p-3.5 bg-muted rounded-lg border text-xs font-mono text-foreground overflow-auto whitespace-pre-wrap leading-relaxed">{`curl -X GET \\
  http://localhost:8080/api/v1/applications \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json"

curl -X POST \\
  http://localhost:8080/api/v1/applications \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"MedQCNN","url":"http://localhost:8000","icon":"\uD83E\uDDEC","category":"AI & ML"}'`}</pre>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function EndpointDoc({ method, path, description, params, body }: { method: string; path: string; description: string; params?: string; body?: string }) {
  const methodColors: Record<string, string> = {
    GET: 'bg-green-500 hover:bg-green-500',
    POST: 'bg-blue-500 hover:bg-blue-500',
    PUT: 'bg-amber-500 hover:bg-amber-500',
    DELETE: 'bg-red-500 hover:bg-red-500',
  };
  return (
    <div className="p-2.5 bg-muted rounded-lg border">
      <div className="flex items-center gap-2 mb-1">
        <Badge className={`${methodColors[method] || ''} text-white text-[11px] font-bold font-mono px-2 py-0`}>
          {method}
        </Badge>
        <code className="text-xs text-foreground font-mono">{path}{params || ''}</code>
      </div>
      <p className="text-xs text-muted-foreground">{description}</p>
      {body && <pre className="mt-1.5 text-[11px] text-muted-foreground font-mono whitespace-pre-wrap">{body}</pre>}
    </div>
  );
}
