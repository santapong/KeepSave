import { useState, useEffect, useCallback } from 'react';
import type { OAuthClient } from '../types/mcp';
import * as api from '../api/client';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from '@/components/ui/dialog';
import { Plus, Trash2, Copy, AlertTriangle } from 'lucide-react';

export function OAuthClientsPage() {
  const [clients, setClients] = useState<OAuthClient[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [newSecret, setNewSecret] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const { toast } = useToast();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setClients(await api.listOAuthClients());
    } catch {
      toast({ title: 'Error', description: 'Failed to load OAuth clients', variant: 'destructive' });
    }
    setLoading(false);
  }, [toast]);

  useEffect(() => { load(); }, [load]);

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <div>
          <h1 className="text-2xl font-bold">OAuth Clients</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Manage OAuth 2.0 client applications that authenticate via KeepSave
          </p>
        </div>
        <Button onClick={() => { setShowCreate(true); setNewSecret(null); }}>
          <Plus className="mr-2 h-4 w-4" /> Register Client
        </Button>
      </div>

      {newSecret && (
        <Card className="mb-4 border-amber-500 bg-amber-50 dark:bg-amber-950/30">
          <CardContent className="p-4">
            <div className="flex items-center gap-2 mb-1">
              <AlertTriangle className="h-4 w-4 text-amber-600" />
              <strong className="text-sm text-amber-800 dark:text-amber-200">Client Secret (copy now, shown only once):</strong>
            </div>
            <code className="block mt-2 p-2 bg-white dark:bg-black/20 border border-border rounded text-xs font-mono break-all">
              {newSecret}
            </code>
            <Button
              size="sm"
              variant="outline"
              className="mt-2"
              onClick={() => {
                navigator.clipboard.writeText(newSecret);
                toast({ title: 'Copied', description: 'Client secret copied to clipboard' });
              }}
            >
              <Copy className="mr-1 h-3 w-3" /> Copy
            </Button>
          </CardContent>
        </Card>
      )}

      {loading ? (
        <div className="space-y-2">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-28 w-full rounded-lg" />
          ))}
        </div>
      ) : clients.length === 0 ? (
        <Card className="text-center py-16 px-6">
          <CardContent className="p-0">
            <p className="text-sm text-muted-foreground">No OAuth clients registered yet.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="flex flex-col gap-2">
          {clients.map(client => (
            <Card key={client.id}>
              <CardContent className="p-4 flex items-start gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-semibold text-sm">{client.name}</span>
                    {client.is_public && (
                      <Badge variant="secondary" className="text-[10px] bg-indigo-500/10 text-indigo-500">
                        Public
                      </Badge>
                    )}
                  </div>
                  <div className="text-xs text-muted-foreground mb-1">
                    {client.description}
                  </div>
                  <div className="text-[11px] text-muted-foreground">
                    <strong>Client ID:</strong> <code className="text-[11px]">{client.client_id}</code>
                  </div>
                  <div className="text-[11px] text-muted-foreground mt-0.5">
                    <strong>Scopes:</strong> {client.scopes.join(', ')} &middot; <strong>Grants:</strong> {client.grant_types.join(', ')}
                  </div>
                  <div className="text-[11px] text-muted-foreground mt-0.5">
                    <strong>Redirect URIs:</strong> {client.redirect_uris.join(', ') || 'none'}
                  </div>
                </div>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={async () => {
                    await api.deleteOAuthClient(client.id);
                    toast({ title: 'Deleted', description: 'OAuth client deleted' });
                    load();
                  }}
                >
                  <Trash2 className="mr-1 h-3 w-3" /> Delete
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <CreateClientModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onCreated={(secret) => {
          setShowCreate(false);
          setNewSecret(secret);
          toast({ title: 'Registered', description: 'OAuth client registered successfully' });
          load();
        }}
      />
    </div>
  );
}

function CreateClientModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: (secret: string) => void }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [redirectURIs, setRedirectURIs] = useState('');
  const [scopes, setScopes] = useState('read');
  const [grantTypes, setGrantTypes] = useState('authorization_code');
  const [isPublic, setIsPublic] = useState(false);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!name) {
      setError('Name is required');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      const result = await api.registerOAuthClient(
        name,
        description,
        redirectURIs.split('\n').map(s => s.trim()).filter(Boolean),
        scopes.split(',').map(s => s.trim()).filter(Boolean),
        grantTypes.split(',').map(s => s.trim()).filter(Boolean),
        isPublic,
      );
      onCreated(result.client_secret);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to register client');
    }
    setSubmitting(false);
  };

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Register OAuth Client</DialogTitle>
          <DialogDescription>Register a new OAuth 2.0 client application.</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div>
            <Label>Application Name *</Label>
            <Input value={name} onChange={e => setName(e.target.value)} placeholder="My Application" className="mt-1" />
          </div>

          <div>
            <Label>Description</Label>
            <Input value={description} onChange={e => setDescription(e.target.value)} placeholder="What does this app do?" className="mt-1" />
          </div>

          <div>
            <Label>Redirect URIs (one per line)</Label>
            <Textarea
              value={redirectURIs}
              onChange={e => setRedirectURIs(e.target.value)}
              placeholder="https://myapp.com/callback"
              className="mt-1 min-h-[60px]"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Scopes (comma-separated)</Label>
              <Input value={scopes} onChange={e => setScopes(e.target.value)} placeholder="read,write" className="mt-1" />
            </div>
            <div>
              <Label>Grant Types (comma-separated)</Label>
              <Input value={grantTypes} onChange={e => setGrantTypes(e.target.value)} placeholder="authorization_code,client_credentials" className="mt-1" />
            </div>
          </div>

          <label className="flex items-center gap-2 text-sm cursor-pointer mt-4">
            <input type="checkbox" checked={isPublic} onChange={e => setIsPublic(e.target.checked)} className="rounded" />
            Public client (no client_secret required for token exchange)
          </label>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={submitting}>
            {submitting ? 'Registering...' : 'Register Client'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
