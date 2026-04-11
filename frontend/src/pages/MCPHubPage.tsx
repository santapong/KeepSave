import { useState, useEffect, useCallback } from 'react';
import type { MCPServer, MCPInstallation } from '../types/mcp';
import * as api from '../api/client';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from '@/components/ui/dialog';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Plus, Trash2, RefreshCw, Download, CheckCircle2 } from 'lucide-react';

const statusVariantMap: Record<string, string> = {
  ready: 'bg-green-500/20 text-green-500',
  building: 'bg-amber-500/20 text-amber-500',
  pending: 'bg-gray-500/20 text-gray-500',
  error: 'bg-red-500/20 text-red-500',
};

export function MCPHubPage() {
  const [tab, setTab] = useState<'marketplace' | 'installed' | 'my-servers'>('marketplace');
  const [publicServers, setPublicServers] = useState<MCPServer[]>([]);
  const [myServers, setMyServers] = useState<MCPServer[]>([]);
  const [installations, setInstallations] = useState<MCPInstallation[]>([]);
  const [showRegister, setShowRegister] = useState(false);
  const [loading, setLoading] = useState(true);
  const { toast } = useToast();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [pub, mine, installs] = await Promise.all([
        api.listPublicMCPServers(),
        api.listMyMCPServers(),
        api.listMCPInstallations(),
      ]);
      setPublicServers(pub);
      setMyServers(mine);
      setInstallations(installs);
    } catch {
      toast({ title: 'Error', description: 'Failed to load MCP data', variant: 'destructive' });
    }
    setLoading(false);
  }, [toast]);

  useEffect(() => { load(); }, [load]);

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <h1 className="text-2xl font-bold">MCP Server Hub</h1>
        <Button onClick={() => setShowRegister(true)}>
          <Plus className="mr-2 h-4 w-4" /> Add MCP Server
        </Button>
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)} className="mb-5">
        <TabsList>
          <TabsTrigger value="marketplace">Marketplace</TabsTrigger>
          <TabsTrigger value="installed">Installed</TabsTrigger>
          <TabsTrigger value="my-servers">My Servers</TabsTrigger>
        </TabsList>
      </Tabs>

      {loading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-48 w-full rounded-lg" />
          ))}
        </div>
      ) : (
        <>
          {tab === 'marketplace' && (
            <ServerGrid servers={publicServers} installations={installations} onInstall={async (id) => {
              try {
                await api.installMCPServer(id);
                toast({ title: 'Installed', description: 'MCP server installed successfully' });
                load();
              } catch {
                toast({ title: 'Error', description: 'Failed to install server', variant: 'destructive' });
              }
            }} />
          )}
          {tab === 'installed' && (
            <InstalledList installations={installations} servers={[...publicServers, ...myServers]} onUninstall={async (id) => {
              try {
                await api.uninstallMCPServer(id);
                toast({ title: 'Uninstalled', description: 'MCP server uninstalled' });
                load();
              } catch {
                toast({ title: 'Error', description: 'Failed to uninstall server', variant: 'destructive' });
              }
            }} />
          )}
          {tab === 'my-servers' && (
            <MyServersList servers={myServers} onDelete={async (id) => {
              try {
                await api.deleteMCPServer(id);
                toast({ title: 'Deleted', description: 'MCP server deleted' });
                load();
              } catch {
                toast({ title: 'Error', description: 'Failed to delete server', variant: 'destructive' });
              }
            }} onRebuild={async (id) => {
              try {
                await api.rebuildMCPServer(id);
                toast({ title: 'Rebuilding', description: 'MCP server rebuild started' });
                load();
              } catch {
                toast({ title: 'Error', description: 'Failed to rebuild server', variant: 'destructive' });
              }
            }} />
          )}
        </>
      )}

      <RegisterServerModal
        open={showRegister}
        onClose={() => setShowRegister(false)}
        onCreated={() => {
          setShowRegister(false);
          toast({ title: 'Registered', description: 'MCP server registered successfully' });
          load();
        }}
      />
    </div>
  );
}

function ServerGrid({ servers, installations, onInstall }: {
  servers: MCPServer[];
  installations: MCPInstallation[];
  onInstall: (id: string) => Promise<void>;
}) {
  const installedIds = new Set(installations.map(i => i.mcp_server_id));

  if (servers.length === 0) {
    return (
      <Card className="text-center py-16 px-6">
        <CardContent className="p-0">
          <p className="text-sm text-muted-foreground">No public MCP servers available yet.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {servers.map(server => (
        <Card key={server.id}>
          <CardContent className="p-4 flex flex-col">
            <div className="flex justify-between items-start">
              <div>
                <h3 className="text-[15px] font-semibold mb-1">{server.name}</h3>
                <p className="text-xs text-muted-foreground mb-2">{server.description}</p>
              </div>
              <StatusBadge status={server.status} />
            </div>
            <div className="text-[11px] text-muted-foreground mb-2">
              v{server.version} &middot; {server.install_count} installs &middot; {server.transport}
            </div>
            <div className="text-[11px] text-muted-foreground mb-3 break-all">
              {server.github_url}
            </div>
            {installedIds.has(server.id) ? (
              <span className="text-xs font-semibold text-green-500 flex items-center gap-1">
                <CheckCircle2 className="h-3.5 w-3.5" /> Installed
              </span>
            ) : (
              <Button onClick={() => onInstall(server.id)} size="sm">
                <Download className="mr-1 h-3 w-3" /> Install
              </Button>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function InstalledList({ installations, servers, onUninstall }: {
  installations: MCPInstallation[];
  servers: MCPServer[];
  onUninstall: (id: string) => Promise<void>;
}) {
  const serverMap = new Map(servers.map(s => [s.id, s]));

  if (installations.length === 0) {
    return (
      <Card className="text-center py-16 px-6">
        <CardContent className="p-0">
          <p className="text-sm text-muted-foreground">No MCP servers installed. Browse the marketplace to get started.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {installations.map(inst => {
        const server = serverMap.get(inst.mcp_server_id);
        return (
          <Card key={inst.id}>
            <CardContent className="p-3 px-4 flex items-center">
              <div className="flex-1">
                <div className="font-semibold text-sm">
                  {server?.name || inst.mcp_server_id}
                </div>
                <div className="text-xs text-muted-foreground">
                  {server?.description || ''} &middot; {inst.enabled ? 'Enabled' : 'Disabled'}
                </div>
              </div>
              <Button onClick={() => onUninstall(inst.id)} variant="destructive" size="sm">
                <Trash2 className="mr-1 h-3 w-3" /> Uninstall
              </Button>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}

function MyServersList({ servers, onDelete, onRebuild }: {
  servers: MCPServer[];
  onDelete: (id: string) => Promise<void>;
  onRebuild: (id: string) => Promise<void>;
}) {
  if (servers.length === 0) {
    return (
      <Card className="text-center py-16 px-6">
        <CardContent className="p-0">
          <p className="text-sm text-muted-foreground">You haven't registered any MCP servers yet.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {servers.map(server => (
        <Card key={server.id}>
          <CardContent className="p-3 px-4 flex items-center">
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <span className="font-semibold text-sm">{server.name}</span>
                <StatusBadge status={server.status} />
                {server.is_public && (
                  <Badge variant="secondary" className="text-[10px] bg-indigo-500/10 text-indigo-500">
                    Public
                  </Badge>
                )}
              </div>
              <div className="text-xs text-muted-foreground mt-0.5">
                {server.github_url} &middot; {server.github_branch} &middot; v{server.version}
              </div>
            </div>
            <div className="flex gap-1.5">
              <Button onClick={() => onRebuild(server.id)} variant="outline" size="sm">
                <RefreshCw className="mr-1 h-3 w-3" /> Rebuild
              </Button>
              <Button onClick={() => onDelete(server.id)} variant="destructive" size="sm">
                <Trash2 className="mr-1 h-3 w-3" /> Delete
              </Button>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  return (
    <Badge
      variant="secondary"
      className={cn(
        'text-[10px] font-semibold uppercase',
        statusVariantMap[status] || 'bg-gray-500/20 text-gray-500'
      )}
    >
      {status}
    </Badge>
  );
}

function RegisterServerModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [githubUrl, setGithubUrl] = useState('');
  const [branch, setBranch] = useState('main');
  const [entryCommand, setEntryCommand] = useState('');
  const [transport, setTransport] = useState('stdio');
  const [isPublic, setIsPublic] = useState(false);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!name || !githubUrl) {
      setError('Name and GitHub URL are required');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      await api.registerMCPServer(name, description, githubUrl, branch, entryCommand, transport, {}, isPublic);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to register server');
    }
    setSubmitting(false);
  };

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Register MCP Server</DialogTitle>
          <DialogDescription>Add a new MCP server from a GitHub repository.</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div>
            <Label>Name *</Label>
            <Input value={name} onChange={e => setName(e.target.value)} placeholder="my-mcp-server" className="mt-1" />
          </div>

          <div>
            <Label>GitHub URL *</Label>
            <Input value={githubUrl} onChange={e => setGithubUrl(e.target.value)} placeholder="https://github.com/user/mcp-server" className="mt-1" />
          </div>

          <div>
            <Label>Description</Label>
            <Input value={description} onChange={e => setDescription(e.target.value)} placeholder="What does this server do?" className="mt-1" />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Branch</Label>
              <Input value={branch} onChange={e => setBranch(e.target.value)} className="mt-1" />
            </div>
            <div>
              <Label>Transport</Label>
              <Select value={transport} onValueChange={setTransport}>
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="stdio">stdio</SelectItem>
                  <SelectItem value="sse">SSE</SelectItem>
                  <SelectItem value="streamable-http">Streamable HTTP</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div>
            <Label>Entry Command (auto-detected if empty)</Label>
            <Input value={entryCommand} onChange={e => setEntryCommand(e.target.value)} placeholder="node dist/index.js" className="mt-1" />
          </div>

          <label className="flex items-center gap-2 text-sm cursor-pointer">
            <input type="checkbox" checked={isPublic} onChange={e => setIsPublic(e.target.checked)} className="rounded" />
            Make this server public in the marketplace
          </label>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={submitting}>
            {submitting ? 'Registering...' : 'Register Server'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
