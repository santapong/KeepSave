import { useState, useEffect } from 'react';
import {
  Puzzle,
  Plus,
  Trash2,
  ShieldCheck,
  Power,
  PowerOff,
} from 'lucide-react';
import * as api from '@/api/client';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Skeleton } from '@/components/ui/skeleton';
import { StatCard } from '@/components/dashboard/StatCard';
import type { Plugin, AccessPolicy, Project } from '@/types';

const PLUGIN_TYPES = ['auth', 'storage', 'notification', 'encryption', 'audit', 'custom'];

export function PluginsTab() {
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [policies, setPolicies] = useState<AccessPolicy[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [deletingPolicyId, setDeletingPolicyId] = useState<string | null>(null);
  const [policiesLoading, setPoliciesLoading] = useState(false);

  // Registration form state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [formName, setFormName] = useState('');
  const [formType, setFormType] = useState('');
  const [formVersion, setFormVersion] = useState('');
  const [registering, setRegistering] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function fetchAll() {
      setLoading(true);
      try {
        const [plgs, prjs] = await Promise.all([
          api.getPlugins(),
          api.listProjects(),
        ]);
        if (!cancelled) {
          setPlugins(plgs as unknown as Plugin[]);
          setProjects(prjs);
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

  // Fetch access policies when selected project changes
  useEffect(() => {
    if (!selectedProjectId) {
      setPolicies([]);
      return;
    }

    let cancelled = false;
    setPoliciesLoading(true);

    api
      .listAccessPolicies(selectedProjectId)
      .then((data) => {
        if (!cancelled) {
          setPolicies(data as unknown as AccessPolicy[]);
        }
      })
      .catch(() => {
        if (!cancelled) setPolicies([]);
      })
      .finally(() => {
        if (!cancelled) setPoliciesLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [selectedProjectId]);

  async function handleToggle(plugin: Plugin) {
    setTogglingId(plugin.id);
    try {
      const updated = await api.togglePlugin(plugin.id, !plugin.enabled);
      setPlugins((prev) =>
        prev.map((p) =>
          p.id === plugin.id
            ? { ...p, enabled: (updated as unknown as Plugin).enabled }
            : p,
        ),
      );
    } catch {
      // Toggle failed silently
    } finally {
      setTogglingId(null);
    }
  }

  async function handleRegister() {
    if (!formName.trim() || !formType || !formVersion.trim()) return;
    setRegistering(true);
    try {
      const created = await api.registerPlugin(formName, formType, formVersion);
      setPlugins((prev) => [...prev, created as unknown as Plugin]);
      setFormName('');
      setFormType('');
      setFormVersion('');
      setDialogOpen(false);
    } catch {
      // Registration failed silently
    } finally {
      setRegistering(false);
    }
  }

  async function handleDeletePolicy(policyId: string) {
    if (!selectedProjectId) return;
    setDeletingPolicyId(policyId);
    try {
      await api.deleteAccessPolicy(selectedProjectId, policyId);
      setPolicies((prev) => prev.filter((p) => p.id !== policyId));
    } catch {
      // Deletion failed silently
    } finally {
      setDeletingPolicyId(null);
    }
  }

  const enabledCount = plugins.filter((p) => p.enabled).length;
  const disabledCount = plugins.length - enabledCount;

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24 rounded-lg" />
          ))}
        </div>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40 rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-6 text-center text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
        Failed to load plugins: {error}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Stat cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Total Plugins"
          value={plugins.length}
          icon={<Puzzle className="h-4 w-4" />}
        />
        <StatCard
          label="Enabled"
          value={enabledCount}
          icon={<Power className="h-4 w-4" />}
          color="text-green-600"
        />
        <StatCard
          label="Disabled"
          value={disabledCount}
          icon={<PowerOff className="h-4 w-4" />}
          color="text-muted-foreground"
        />
        <StatCard
          label="Projects"
          value={projects.length}
          icon={<ShieldCheck className="h-4 w-4" />}
        />
      </div>

      {/* Plugins section */}
      <section>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
            Plugins
          </h3>

          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button size="sm" variant="outline">
                <Plus className="mr-1 h-3.5 w-3.5" />
                Register Plugin
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Register Plugin</DialogTitle>
                <DialogDescription>
                  Add a new plugin to the platform. Fill in all fields to register.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-2">
                <div className="space-y-2">
                  <Label htmlFor="plugin-name">Name</Label>
                  <Input
                    id="plugin-name"
                    placeholder="my-plugin"
                    value={formName}
                    onChange={(e) => setFormName(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="plugin-type">Type</Label>
                  <Select value={formType} onValueChange={setFormType}>
                    <SelectTrigger id="plugin-type">
                      <SelectValue placeholder="Select type" />
                    </SelectTrigger>
                    <SelectContent>
                      {PLUGIN_TYPES.map((t) => (
                        <SelectItem key={t} value={t}>
                          {t}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="plugin-version">Version</Label>
                  <Input
                    id="plugin-version"
                    placeholder="1.0.0"
                    value={formVersion}
                    onChange={(e) => setFormVersion(e.target.value)}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button
                  onClick={handleRegister}
                  disabled={registering || !formName.trim() || !formType || !formVersion.trim()}
                >
                  {registering ? 'Registering...' : 'Register'}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        {plugins.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Puzzle className="mb-3 h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                No plugins registered yet. Click &ldquo;Register Plugin&rdquo; to add one.
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {plugins.map((plugin) => (
              <Card key={plugin.id}>
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between gap-2">
                    <CardTitle className="text-base">{plugin.name}</CardTitle>
                    <Badge variant="secondary">{plugin.plugin_type}</Badge>
                  </div>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <span className="font-mono">v{plugin.version}</span>
                    <Badge
                      variant={plugin.enabled ? 'success' : 'secondary'}
                      className="text-[10px]"
                    >
                      {plugin.enabled ? 'enabled' : 'disabled'}
                    </Badge>
                  </div>
                  <Button
                    size="sm"
                    variant={plugin.enabled ? 'outline' : 'default'}
                    className="w-full"
                    disabled={togglingId === plugin.id}
                    onClick={() => handleToggle(plugin)}
                  >
                    {plugin.enabled ? (
                      <>
                        <PowerOff className="mr-1 h-3.5 w-3.5" />
                        Disable
                      </>
                    ) : (
                      <>
                        <Power className="mr-1 h-3.5 w-3.5" />
                        Enable
                      </>
                    )}
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </section>

      <Separator />

      {/* Access policies section */}
      <section>
        <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Access Policies
        </h3>

        <div className="mb-4 max-w-xs">
          <Label htmlFor="policy-project" className="mb-2 block text-xs text-muted-foreground">
            Select Project
          </Label>
          <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
            <SelectTrigger id="policy-project">
              <SelectValue placeholder="Choose a project" />
            </SelectTrigger>
            <SelectContent>
              {projects.map((project) => (
                <SelectItem key={project.id} value={project.id}>
                  {project.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {!selectedProjectId ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <ShieldCheck className="mb-3 h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                Select a project to view its access policies.
              </p>
            </CardContent>
          </Card>
        ) : policiesLoading ? (
          <div className="space-y-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-14 rounded-lg" />
            ))}
          </div>
        ) : policies.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <ShieldCheck className="mb-3 h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                No access policies defined for this project.
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-2">
            {policies.map((policy) => (
              <div
                key={policy.id}
                className="flex items-center justify-between rounded-lg border px-4 py-3"
              >
                <div className="flex items-center gap-3">
                  <Badge variant="outline">{policy.policy_type}</Badge>
                  <span className="font-mono text-xs text-muted-foreground">
                    {policy.id}
                  </span>
                </div>
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                  disabled={deletingPolicyId === policy.id}
                  onClick={() => handleDeletePolicy(policy.id)}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
