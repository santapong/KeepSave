import { useState, useEffect, type FormEvent } from 'react';
import type { SecretTemplate, Project } from '../types';
import * as api from '../api/client';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
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
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Trash2, Play } from 'lucide-react';
import { Page, PageHeader, KpiStrip, Kpi, SectionHead, Chip, EmptyState } from '../components/cosmic/primitives';

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
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
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

  return (
    <Page>
      <PageHeader
        eyebrow="Vault · starter kits"
        title="Secret templates"
        sub="Define a reusable set of secret keys per stack, then apply them to any project and environment in one move."
        actions={
          <button type="button" className={`cz-btn ${showCreate ? '' : 'cz-btn-primary'}`} onClick={() => setShowCreate((v) => !v)}>
            {showCreate ? 'Cancel' : '+ New template'}
          </button>
        }
      />

      <KpiStrip>
        <Kpi label="Templates" value={loading ? '—' : totalCount} hint="builtin + custom" />
        <Kpi label="Builtin" value={loading ? '—' : builtinTemplates.length} hint="ready to apply" />
        <Kpi label="Custom" value={loading ? '—' : templates.length} hint="defined by your org" />
        <Kpi label="Projects" value={loading ? '—' : projects.length} hint="targets available" />
      </KpiStrip>

      {loading ? (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 16 }}>
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-48 w-full rounded-lg" />
          ))}
        </div>
      ) : (
        <>
          <SectionHead index="01" title="Builtin templates" meta={`${builtinTemplates.length} available`} />
          {builtinTemplates.length === 0 ? (
            <p className="cz-mute" style={{ marginBottom: 28 }}>No builtin templates available.</p>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 16, marginBottom: 34 }}>
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
                    onApplyStart={() => {
                      setApplyingId(cardId);
                      setApplyProjectId('');
                      setApplyEnv('alpha');
                    }}
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

          <SectionHead index="02" title="Custom templates" meta={`${templates.length} defined`} />
          {templates.length === 0 ? (
            <EmptyState title="No custom templates yet" size={140}>
              Create a template to apply a set of secret keys to any project in one step.
            </EmptyState>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 16 }}>
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
                  onApplyStart={() => {
                    setApplyingId(tmpl.id);
                    setApplyProjectId('');
                    setApplyEnv('alpha');
                  }}
                  onApplyCancel={() => setApplyingId(null)}
                  onProjectIdChange={setApplyProjectId}
                  onEnvChange={setApplyEnv}
                  onApply={() => handleApply(tmpl.id)}
                  onDelete={() => setDeleteTarget(tmpl.id)}
                />
              ))}
            </div>
          )}
        </>
      )}

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
                <Input id="tmpl-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Template name" required className="mt-1" />
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
              <Input id="tmpl-desc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Description (optional)" className="mt-1" />
            </div>
            <div>
              <Label htmlFor="tmpl-keys">Keys (one per line, KEY=default_value)</Label>
              <Textarea
                id="tmpl-keys"
                value={keysText}
                onChange={(e) => setKeysText(e.target.value)}
                placeholder={'DATABASE_URL=\nPORT=3000\nLOG_LEVEL=info'}
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

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete Template"
        description="Are you sure you want to delete this template? This action cannot be undone."
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleteTarget) handleDelete(deleteTarget);
          setDeleteTarget(null);
        }}
      />
    </Page>
  );
}

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
    <div className="cz-card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
        <strong style={{ fontSize: 15, fontFamily: 'var(--cz-sans)' }}>{template.name}</strong>
        <Chip variant="on">{template.stack}</Chip>
        {isBuiltin && <Chip>builtin</Chip>}
      </div>

      {template.description && <p className="cz-mute" style={{ fontSize: 13, lineHeight: 1.5 }}>{template.description}</p>}

      <div className="cz-envs">
        {keys.length === 0 ? (
          <span className="cz-faint" style={{ fontSize: 12 }}>No keys defined</span>
        ) : (
          <>
            {keys.slice(0, 5).map((k) => (
              <Chip key={k}>{k}</Chip>
            ))}
            {keys.length > 5 && (
              <span className="cz-faint" style={{ fontSize: 11, alignSelf: 'center' }}>
                +{keys.length - 5} more
              </span>
            )}
          </>
        )}
      </div>

      <div style={{ marginTop: 'auto' }}>
        {isApplying ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
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
            <div style={{ display: 'flex', gap: 8 }}>
              <button type="button" className="cz-btn cz-btn-primary" style={{ flex: 1, justifyContent: 'center', padding: '7px 12px', fontSize: 11 }} onClick={onApply}>
                Confirm
              </button>
              <button type="button" className="cz-btn" style={{ flex: 1, justifyContent: 'center', padding: '7px 12px', fontSize: 11 }} onClick={onApplyCancel}>
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <div style={{ display: 'flex', gap: 8 }}>
            <button type="button" className="cz-btn cz-btn-primary" style={{ padding: '7px 12px', fontSize: 11 }} onClick={onApplyStart}>
              <Play size={13} /> Apply
            </button>
            {onDelete && (
              <button type="button" className="cz-btn cz-btn-danger" style={{ padding: '7px 12px', fontSize: 11 }} onClick={onDelete}>
                <Trash2 size={13} /> Delete
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
