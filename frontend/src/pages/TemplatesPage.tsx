import { useState, useEffect, type FormEvent } from 'react';
import type { SecretTemplate, Project } from '../types';
import * as api from '../api/client';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from '@/components/ui/dialog';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Plus, Trash2, Play, X } from 'lucide-react';

const stackColorMap: Record<string, string> = {
  nodejs: 'bg-green-600',
  python: 'bg-blue-600',
  go: 'bg-cyan-500',
  aws: 'bg-orange-500',
  custom: 'bg-gray-500',
};

export function TemplatesPage() {
  const [templates, setTemplates] = useState<SecretTemplate[]>([]);
  const [builtinTemplates, setBuiltinTemplates] = useState<SecretTemplate[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [stack, setStack] = useState('custom');
  const [keysText, setKeysText] = useState('');
  const [applyProjectId, setApplyProjectId] = useState('');
  const [applyEnv, setApplyEnv] = useState('alpha');
  const [applyingId, setApplyingId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const { toast } = useToast();

  useEffect(() => {
    loadData();
  }, []);

  async function loadData() {
    try {
      setLoading(true);
      const [custom, builtin, projectList] = await Promise.all([
        api.listTemplates(),
        api.listBuiltinTemplates(),
        api.listProjects(),
      ]);
      setTemplates(custom);
      setBuiltinTemplates(builtin);
      setProjects(projectList);
    } catch (err) {
      toast({ title: 'Error', description: err instanceof Error ? err.message : 'Failed to load templates', variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  }

  function parseKeysInput(text: string): Record<string, unknown> {
    const result: Record<string, unknown> = {};
    text
      .split('\n')
      .filter((line) => line.trim())
      .forEach((line) => {
        const eqIndex = line.indexOf('=');
        let key: string;
        let defaultValue: string;
        if (eqIndex === -1) {
          key = line.trim();
          defaultValue = '';
        } else {
          key = line.substring(0, eqIndex).trim();
          defaultValue = line.substring(eqIndex + 1).trim();
        }
        if (key) {
          result[key] = { default_value: defaultValue, required: true };
        }
      });
    return result;
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    try {
      const keys = parseKeysInput(keysText);
      await api.createTemplate(name, description, stack, keys);
      setShowCreate(false);
      setName('');
      setDescription('');
      setStack('custom');
      setKeysText('');
      toast({ title: 'Success', description: 'Template created successfully' });
      loadData();
    } catch (err) {
      toast({ title: 'Error', description: err instanceof Error ? err.message : 'Failed to create template', variant: 'destructive' });
    }
  }

  async function handleDelete(templateId: string) {
    if (!window.confirm('Delete this template? This cannot be undone.')) return;
    try {
      await api.deleteTemplate(templateId);
      toast({ title: 'Deleted', description: 'Template deleted' });
      loadData();
    } catch (err) {
      toast({ title: 'Error', description: err instanceof Error ? err.message : 'Failed to delete template', variant: 'destructive' });
    }
  }

  async function handleApply(templateId: string) {
    if (!applyProjectId) {
      toast({ title: 'Error', description: 'Please select a project', variant: 'destructive' });
      return;
    }
    try {
      const secrets = await api.applyTemplate(templateId, applyProjectId, applyEnv);
      toast({ title: 'Applied', description: `Applied template: ${secrets.length} secret${secrets.length !== 1 ? 's' : ''} created` });
      setApplyingId(null);
      setApplyProjectId('');
      setApplyEnv('alpha');
    } catch (err) {
      toast({ title: 'Error', description: err instanceof Error ? err.message : 'Failed to apply template', variant: 'destructive' });
    }
  }

  function getTemplateKeyNames(tmpl: SecretTemplate): string[] {
    return Object.keys(tmpl.keys || {});
  }

  const totalCount = builtinTemplates.length + templates.length;

  if (loading) {
    return (
      <div className="space-y-4 p-10">
        <Skeleton className="h-8 w-48" />
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-48 w-full rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className="flex justify-between items-start mb-6">
        <div>
          <h1 className="text-2xl font-bold">Secret Templates</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            {totalCount} template{totalCount !== 1 ? 's' : ''}
          </p>
        </div>
        <Button onClick={() => setShowCreate(!showCreate)} variant={showCreate ? 'outline' : 'default'}>
          {showCreate ? <><X className="mr-2 h-4 w-4" /> Cancel</> : <><Plus className="mr-2 h-4 w-4" /> New Template</>}
        </Button>
      </div>

      {/* Create form dialog */}
      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Create Template</DialogTitle>
            <DialogDescription>Define a reusable set of secret keys for quick project setup.</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleCreate} className="space-y-4">
            <div className="flex gap-3 flex-wrap">
              <div className="flex-1 min-w-[200px]">
                <Label htmlFor="tmpl-name">Template name</Label>
                <Input
                  id="tmpl-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Template name"
                  required
                  className="mt-1"
                />
              </div>
              <div className="min-w-[140px]">
                <Label>Stack</Label>
                <Select value={stack} onValueChange={setStack}>
                  <SelectTrigger className="mt-1">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="custom">Custom</SelectItem>
                    <SelectItem value="nodejs">Node.js</SelectItem>
                    <SelectItem value="python">Python</SelectItem>
                    <SelectItem value="go">Go</SelectItem>
                    <SelectItem value="aws">AWS</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div>
              <Label htmlFor="tmpl-desc">Description (optional)</Label>
              <Input
                id="tmpl-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Description (optional)"
                className="mt-1"
              />
            </div>
            <div>
              <Label htmlFor="tmpl-keys">Keys (one per line, KEY=default_value)</Label>
              <Textarea
                id="tmpl-keys"
                value={keysText}
                onChange={(e) => setKeysText(e.target.value)}
                placeholder={"DATABASE_URL=\nPORT=3000\nLOG_LEVEL=info"}
                rows={5}
                className="mt-1 font-mono text-sm resize-y"
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
              <Button type="submit">Create Template</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Builtin Templates */}
      <h2 className="text-base font-semibold mb-3">Builtin Templates</h2>
      {builtinTemplates.length === 0 ? (
        <p className="text-muted-foreground text-sm mb-6">No builtin templates available.</p>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-8">
          {builtinTemplates.map((tmpl, idx) => {
            const cardId = `builtin-${idx}`;
            return (
              <TemplateCard
                key={cardId}
                template={tmpl}
                cardId={cardId}
                keys={getTemplateKeyNames(tmpl)}
                applyingId={applyingId}
                applyProjectId={applyProjectId}
                applyEnv={applyEnv}
                projects={projects}
                onApplyStart={() => { setApplyingId(cardId); setApplyProjectId(''); setApplyEnv('alpha'); }}
                onApplyCancel={() => setApplyingId(null)}
                onProjectIdChange={setApplyProjectId}
                onEnvChange={setApplyEnv}
                onApply={() => handleApply(tmpl.id)}
                isBuiltin
              />
            );
          })}
        </div>
      )}

      {/* Custom Templates */}
      <h2 className="text-base font-semibold mb-3">Custom Templates</h2>
      {templates.length === 0 ? (
        <Card className="text-center py-16 px-6">
          <CardContent className="p-0">
            <p className="text-base font-medium mb-1">No custom templates yet</p>
            <p className="text-sm text-muted-foreground">
              Create a template to quickly apply a set of secret keys to any project.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-8">
          {templates.map((tmpl) => (
            <TemplateCard
              key={tmpl.id}
              template={tmpl}
              cardId={tmpl.id}
              keys={getTemplateKeyNames(tmpl)}
              applyingId={applyingId}
              applyProjectId={applyProjectId}
              applyEnv={applyEnv}
              projects={projects}
              onApplyStart={() => { setApplyingId(tmpl.id); setApplyProjectId(''); setApplyEnv('alpha'); }}
              onApplyCancel={() => setApplyingId(null)}
              onProjectIdChange={setApplyProjectId}
              onEnvChange={setApplyEnv}
              onApply={() => handleApply(tmpl.id)}
              onDelete={() => handleDelete(tmpl.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* ───────────────────── TemplateCard Component ───────────────────── */

function TemplateCard({
  template,
  cardId,
  keys,
  applyingId,
  applyProjectId,
  applyEnv,
  projects,
  onApplyStart,
  onApplyCancel,
  onProjectIdChange,
  onEnvChange,
  onApply,
  onDelete,
  isBuiltin,
}: {
  template: SecretTemplate;
  cardId: string;
  keys: string[];
  applyingId: string | null;
  applyProjectId: string;
  applyEnv: string;
  projects: Project[];
  onApplyStart: () => void;
  onApplyCancel: () => void;
  onProjectIdChange: (v: string) => void;
  onEnvChange: (v: string) => void;
  onApply: () => void;
  onDelete?: () => void;
  isBuiltin?: boolean;
}) {
  const isApplying = applyingId === cardId;

  return (
    <Card className="flex flex-col">
      <CardContent className="p-5 flex flex-col flex-1">
        {/* Header: name + stack badge */}
        <div className="flex justify-between items-start mb-2">
          <div className="flex items-center gap-2 flex-wrap">
            <strong className="text-[15px]">{template.name}</strong>
            <Badge className={cn('text-white text-[11px]', stackColorMap[template.stack] || stackColorMap.custom)}>
              {template.stack}
            </Badge>
            {isBuiltin && <Badge variant="outline" className="text-[10px]">builtin</Badge>}
          </div>
        </div>

        {/* Description */}
        {template.description && (
          <p className="text-sm text-muted-foreground mb-3 leading-snug">
            {template.description}
          </p>
        )}

        {/* Keys list */}
        <div className="text-xs font-mono text-muted-foreground mb-3">
          {keys.length === 0 ? (
            <span className="italic font-sans">No keys defined</span>
          ) : (
            <>
              {keys.slice(0, 5).map((k) => (
                <span key={k} className="inline-block px-1.5 py-0.5 m-0.5 bg-muted border border-border rounded text-[11px]">
                  {k}
                </span>
              ))}
              {keys.length > 5 && (
                <div className="mt-1 text-[11px] font-sans">
                  +{keys.length - 5} more key{keys.length - 5 !== 1 ? 's' : ''}
                </div>
              )}
            </>
          )}
        </div>

        {/* Actions */}
        <div className="mt-auto">
          {isApplying ? (
            <div className="flex flex-col gap-2">
              <Select value={applyProjectId} onValueChange={onProjectIdChange}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="-- Select project --" />
                </SelectTrigger>
                <SelectContent>
                  {projects.map((p) => (
                    <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={applyEnv} onValueChange={onEnvChange}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="alpha">Alpha</SelectItem>
                  <SelectItem value="uat">UAT</SelectItem>
                  <SelectItem value="prod">Prod</SelectItem>
                </SelectContent>
              </Select>
              <div className="flex gap-2">
                <Button onClick={onApply} size="sm" className="flex-1">
                  Confirm
                </Button>
                <Button onClick={onApplyCancel} variant="outline" size="sm" className="flex-1">
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex gap-2">
              <Button onClick={onApplyStart} size="sm">
                <Play className="mr-1 h-3 w-3" /> Apply
              </Button>
              {onDelete && (
                <Button onClick={onDelete} variant="destructive" size="sm">
                  <Trash2 className="mr-1 h-3 w-3" /> Delete
                </Button>
              )}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
