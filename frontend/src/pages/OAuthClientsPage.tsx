import { useState, useEffect, useCallback } from 'react';
import type { OAuthClient } from '../types/mcp';
import * as api from '../api/client';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog';
import { Plus, Trash2, Copy, AlertTriangle } from 'lucide-react';
import { HoldToReveal } from '../components/cosmic/HoldToReveal';
import {
  Page,
  PageHeader,
  KpiStrip,
  Kpi,
  SectionHead,
  Chip,
  CodeBlock,
  EmptyState,
} from '../components/cosmic/primitives';

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

  useEffect(() => {
    load();
  }, [load]);

  // KPIs derived from real data.
  const publicCount = clients.filter((c) => c.is_public).length;
  const confidential = clients.length - publicCount;
  const authCode = clients.filter((c) => c.grant_types.includes('authorization_code')).length;

  return (
    <Page>
      <PageHeader
        eyebrow="Platform · OAuth 2.0 + OIDC"
        title="OAuth clients"
        sub="A full identity provider: authorization code, client credentials, PKCE, and refresh rotation. OIDC discovery lives at /.well-known/openid-configuration."
        actions={
          <button
            type="button"
            className="cz-btn cz-btn-primary"
            onClick={() => {
              setShowCreate(true);
              setNewSecret(null);
            }}
          >
            <Plus size={15} /> Register client
          </button>
        }
      />

      <KpiStrip>
        <Kpi label="Clients" value={loading ? '—' : clients.length} hint="registered" />
        <Kpi label="Confidential" value={loading ? '—' : confidential} hint="client_secret required" />
        <Kpi label="Public" value={loading ? '—' : publicCount} hint="PKCE only" />
        <Kpi label="Auth-code" value={loading ? '—' : authCode} hint="grant enabled" />
      </KpiStrip>

      {newSecret && (
        <div
          className="cz-card"
          style={{ padding: 18, marginBottom: 18, borderColor: 'var(--cz-warn)' }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
            <AlertTriangle size={15} style={{ color: 'var(--cz-warn)' }} />
            <strong style={{ fontSize: 13 }}>Client secret — copy now, shown only once</strong>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <HoldToReveal value={newSecret} bricks={16} />
            </div>
            <button
              type="button"
              className="cz-btn"
              style={{ padding: '8px 14px', fontSize: 11 }}
              onClick={() => {
                navigator.clipboard.writeText(newSecret);
                toast({ title: 'Copied', description: 'Client secret copied to clipboard' });
              }}
            >
              <Copy size={13} /> Copy
            </button>
          </div>
        </div>
      )}

      {loading ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-20 w-full rounded-lg" />
          ))}
        </div>
      ) : clients.length === 0 ? (
        <EmptyState title="No OAuth clients yet">
          Register a client application to issue tokens through KeepSave's identity provider.
        </EmptyState>
      ) : (
        <div className="cz-card cz-secrets">
          <table className="cz-dtable">
            <thead>
              <tr>
                <th>Client</th>
                <th>Client ID</th>
                <th>Grants</th>
                <th>Scopes</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {clients.map((client) => (
                <tr key={client.id}>
                  <td>
                    <div className="cz-cell-name">
                      <span className="cz-n" style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
                        {client.name}
                        {client.is_public && <Chip variant="on">public</Chip>}
                      </span>
                      {client.description && <span className="cz-d">{client.description}</span>}
                    </div>
                  </td>
                  <td style={{ fontFamily: 'var(--cz-mono)', fontSize: 12, color: 'var(--cz-accent-hi)' }}>
                    {client.client_id}
                  </td>
                  <td className="cz-mute" style={{ fontFamily: 'var(--cz-mono)', fontSize: 12 }}>
                    {client.grant_types.join(', ')}
                  </td>
                  <td>
                    <div className="cz-envs">
                      {client.scopes.map((s) => (
                        <Chip key={s}>{s}</Chip>
                      ))}
                    </div>
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <button
                      type="button"
                      className="cz-btn cz-btn-danger"
                      style={{ padding: '6px 12px', fontSize: 11 }}
                      onClick={async () => {
                        await api.deleteOAuthClient(client.id);
                        toast({ title: 'Deleted', description: 'OAuth client deleted' });
                        load();
                      }}
                    >
                      <Trash2 size={13} /> Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <SectionHead index="02" title="OIDC discovery" meta="well-known · public" style={{ marginTop: 38 }} />
      <CodeBlock filename="GET /.well-known/openid-configuration" style={{ maxWidth: 760 }}>
        {`{
  "issuer": `}
        <span className="cz-c-s">"https://vault.keepsave.io"</span>
        {`,
  "authorization_endpoint": `}
        <span className="cz-c-s">"/oauth/authorize"</span>
        {`,
  "token_endpoint": `}
        <span className="cz-c-s">"/oauth/token"</span>
        {`,
  "userinfo_endpoint": `}
        <span className="cz-c-s">"/oauth/userinfo"</span>
        {`,
  "jwks_uri": `}
        <span className="cz-c-s">"/.well-known/jwks.json"</span>
        {`,
  "grant_types_supported": ["authorization_code", "refresh_token", "client_credentials"],
  "code_challenge_methods_supported": ["S256"]
}`}
      </CodeBlock>

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
    </Page>
  );
}

function CreateClientModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (secret: string) => void;
}) {
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
        redirectURIs.split('\n').map((s) => s.trim()).filter(Boolean),
        scopes.split(',').map((s) => s.trim()).filter(Boolean),
        grantTypes.split(',').map((s) => s.trim()).filter(Boolean),
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
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="My Application" className="mt-1" />
          </div>

          <div>
            <Label>Description</Label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What does this app do?" className="mt-1" />
          </div>

          <div>
            <Label>Redirect URIs (one per line)</Label>
            <Textarea
              value={redirectURIs}
              onChange={(e) => setRedirectURIs(e.target.value)}
              placeholder="https://myapp.com/callback"
              className="mt-1 min-h-[60px]"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Scopes (comma-separated)</Label>
              <Input value={scopes} onChange={(e) => setScopes(e.target.value)} placeholder="read,write" className="mt-1" />
            </div>
            <div>
              <Label>Grant Types (comma-separated)</Label>
              <Input value={grantTypes} onChange={(e) => setGrantTypes(e.target.value)} placeholder="authorization_code,client_credentials" className="mt-1" />
            </div>
          </div>

          <label className="flex items-center gap-2 text-sm cursor-pointer mt-4">
            <input type="checkbox" checked={isPublic} onChange={(e) => setIsPublic(e.target.checked)} className="rounded" />
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
