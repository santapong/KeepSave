import { useState, useEffect, useCallback } from 'react';
import type { MCPServer, MCPInstallation } from '../types/mcp';
import * as api from '../api/client';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Plus, Trash2, RefreshCw, Download, CheckCircle2 } from 'lucide-react';
import {
  Page,
  PageHeader,
  KpiStrip,
  Kpi,
  SectionHead,
  Segmented,
  Chip,
  CodeBlock,
  EmptyState,
} from '../components/cosmic/primitives';

type Tab = 'marketplace' | 'installed' | 'my-servers';

const STATUS_CHIP: Record<string, 'prod' | 'on' | undefined> = {
  ready: 'prod',
  building: undefined,
  pending: undefined,
  error: undefined,
};

export function MCPHubPage() {
  const [tab, setTab] = useState<Tab>('marketplace');
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

  useEffect(() => {
    load();
  }, [load]);

  const enabled = installations.filter((i) => i.enabled).length;

  return (
    <Page>
      <PageHeader
        eyebrow="Platform · MCP Hub"
        title={<>Servers for <em>agents</em></>}
        sub="Register MCP servers from GitHub and install from the marketplace. Tool calls route through a gateway that injects secrets — agents never see them."
        actions={
          <button type="button" className="cz-btn cz-btn-primary" onClick={() => setShowRegister(true)}>
            <Plus size={15} /> Register server
          </button>
        }
      />

      <KpiStrip>
        <Kpi label="Installed" value={loading ? '—' : installations.length} hint="in this org" />
        <Kpi label="Enabled" value={loading ? '—' : enabled} hint="routing tool calls" />
        <Kpi label="Marketplace" value={loading ? '—' : publicServers.length} hint="public servers" />
        <Kpi label="My servers" value={loading ? '—' : myServers.length} hint="registered by you" />
      </KpiStrip>

      <div className="cz-filter-bar">
        <Segmented<Tab>
          options={[
            { value: 'marketplace', label: 'Marketplace' },
            { value: 'installed', label: 'Installed' },
            { value: 'my-servers', label: 'My servers' },
          ]}
          value={tab}
          onChange={setTab}
        />
      </div>

      {loading ? (
        <div className="cz-mcp-grid">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-48 w-full rounded-lg" />
          ))}
        </div>
      ) : (
        <>
          {tab === 'marketplace' && (
            <ServerGrid
              servers={publicServers}
              installations={installations}
              onInstall={async (id) => {
                try {
                  await api.installMCPServer(id);
                  toast({ title: 'Installed', description: 'MCP server installed successfully' });
                  load();
                } catch {
                  toast({ title: 'Error', description: 'Failed to install server', variant: 'destructive' });
                }
              }}
            />
          )}
          {tab === 'installed' && (
            <InstalledList
              installations={installations}
              servers={[...publicServers, ...myServers]}
              onUninstall={async (id) => {
                try {
                  await api.uninstallMCPServer(id);
                  toast({ title: 'Uninstalled', description: 'MCP server uninstalled' });
                  load();
                } catch {
                  toast({ title: 'Error', description: 'Failed to uninstall server', variant: 'destructive' });
                }
              }}
            />
          )}
          {tab === 'my-servers' && (
            <MyServersList
              servers={myServers}
              onDelete={async (id) => {
                try {
                  await api.deleteMCPServer(id);
                  toast({ title: 'Deleted', description: 'MCP server deleted' });
                  load();
                } catch {
                  toast({ title: 'Error', description: 'Failed to delete server', variant: 'destructive' });
                }
              }}
              onRebuild={async (id) => {
                try {
                  await api.rebuildMCPServer(id);
                  toast({ title: 'Rebuilding', description: 'MCP server rebuild started' });
                  load();
                } catch {
                  toast({ title: 'Error', description: 'Failed to rebuild server', variant: 'destructive' });
                }
              }}
            />
          )}
        </>
      )}

      <SectionHead index="02" title="Claude config" meta="copy · paste · done" style={{ marginTop: 38 }} />
      <CodeBlock filename="~/.claude/mcp.json" style={{ maxWidth: 760 }}>
        {`{
  "mcpServers": {
    "keepsave": {
      "transport": "http",
      "url": `}
        <span className="cz-c-s">"https://vault.keepsave.io/mcp"</span>
        {`,
      "headers": {
        "Authorization": `}
        <span className="cz-c-s">"Bearer ks_live_8e42···"</span>
        {`
      }
    }
  }
}`}
      </CodeBlock>

      <RegisterServerModal
        open={showRegister}
        onClose={() => setShowRegister(false)}
        onCreated={() => {
          setShowRegister(false);
          toast({ title: 'Registered', description: 'MCP server registered successfully' });
          load();
        }}
      />
    </Page>
  );
}

function StatusChip({ status }: { status: string }) {
  return <Chip variant={STATUS_CHIP[status]}>{status}</Chip>;
}

function ServerGrid({
  servers,
  installations,
  onInstall,
}: {
  servers: MCPServer[];
  installations: MCPInstallation[];
  onInstall: (id: string) => Promise<void>;
}) {
  const installedIds = new Set(installations.map((i) => i.mcp_server_id));

  if (servers.length === 0) {
    return <EmptyState title="No public servers yet">Register a server or check back as the marketplace grows.</EmptyState>;
  }

  return (
    <div className="cz-mcp-grid">
      {servers.map((server) => (
        <div key={server.id} className="cz-card cz-mcp-card">
          <div className="cz-sig">{server.name.charAt(0).toUpperCase()}</div>
          <div className="cz-ti">
            <span>{server.name}</span>
            <StatusChip status={server.status} />
          </div>
          <div className="cz-ds">{server.description}</div>
          <div className="cz-mt">
            <span>v{server.version}</span>
            <span><b>{server.install_count}</b> installs</span>
            <span style={{ marginLeft: 'auto' }}>{server.transport}</span>
          </div>
          {installedIds.has(server.id) ? (
            <span style={{ color: 'var(--cz-go)', fontSize: 12, display: 'inline-flex', alignItems: 'center', gap: 6, fontFamily: 'var(--cz-mono)' }}>
              <CheckCircle2 size={14} /> Installed
            </span>
          ) : (
            <button type="button" className="cz-btn cz-btn-primary" style={{ alignSelf: 'flex-start' }} onClick={() => onInstall(server.id)}>
              <Download size={14} /> Install
            </button>
          )}
        </div>
      ))}
    </div>
  );
}

function InstalledList({
  installations,
  servers,
  onUninstall,
}: {
  installations: MCPInstallation[];
  servers: MCPServer[];
  onUninstall: (id: string) => Promise<void>;
}) {
  const serverMap = new Map(servers.map((s) => [s.id, s]));

  if (installations.length === 0) {
    return <EmptyState title="Nothing installed">Browse the marketplace to install your first MCP server.</EmptyState>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      {installations.map((inst) => {
        const server = serverMap.get(inst.mcp_server_id);
        return (
          <div key={inst.id} className="cz-card" style={{ padding: '14px 18px', display: 'flex', alignItems: 'center', gap: 14 }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className="cz-secret-key">{server?.name || inst.mcp_server_id}</div>
              <div className="cz-secret-note">
                {server?.description || ''} · {inst.enabled ? 'Enabled' : 'Disabled'}
              </div>
            </div>
            <button type="button" className="cz-btn cz-btn-danger" style={{ padding: '6px 12px', fontSize: 11 }} onClick={() => onUninstall(inst.id)}>
              <Trash2 size={13} /> Uninstall
            </button>
          </div>
        );
      })}
    </div>
  );
}

function MyServersList({
  servers,
  onDelete,
  onRebuild,
}: {
  servers: MCPServer[];
  onDelete: (id: string) => Promise<void>;
  onRebuild: (id: string) => Promise<void>;
}) {
  if (servers.length === 0) {
    return <EmptyState title="No servers registered">Register an MCP server from a GitHub repository to get started.</EmptyState>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      {servers.map((server) => (
        <div key={server.id} className="cz-card" style={{ padding: '14px 18px', display: 'flex', alignItems: 'center', gap: 14 }}>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span className="cz-secret-key">{server.name}</span>
              <StatusChip status={server.status} />
              {server.is_public && <Chip variant="on">public</Chip>}
            </div>
            <div className="cz-secret-note">
              {server.github_url} · {server.github_branch} · v{server.version}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button type="button" className="cz-btn" style={{ padding: '6px 12px', fontSize: 11 }} onClick={() => onRebuild(server.id)}>
              <RefreshCw size={13} /> Rebuild
            </button>
            <button type="button" className={cn('cz-btn cz-btn-danger')} style={{ padding: '6px 12px', fontSize: 11 }} onClick={() => onDelete(server.id)}>
              <Trash2 size={13} /> Delete
            </button>
          </div>
        </div>
      ))}
    </div>
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
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="my-mcp-server" className="mt-1" />
          </div>

          <div>
            <Label>GitHub URL *</Label>
            <Input value={githubUrl} onChange={(e) => setGithubUrl(e.target.value)} placeholder="https://github.com/user/mcp-server" className="mt-1" />
          </div>

          <div>
            <Label>Description</Label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What does this server do?" className="mt-1" />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Branch</Label>
              <Input value={branch} onChange={(e) => setBranch(e.target.value)} className="mt-1" />
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
            <Input value={entryCommand} onChange={(e) => setEntryCommand(e.target.value)} placeholder="node dist/index.js" className="mt-1" />
          </div>

          <label className="flex items-center gap-2 text-sm cursor-pointer">
            <input type="checkbox" checked={isPublic} onChange={(e) => setIsPublic(e.target.checked)} className="rounded" />
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
