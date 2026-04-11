import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import type { DashboardApplication } from '../types';
import { AppChatbot } from '../components/AppChatbot';
import * as api from '../api/client';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { useToast } from '@/hooks/useToast';
import { Settings, Plus, Pencil, Trash2, Star, Rocket } from 'lucide-react';

export function ApplicationDashboardPage() {
  const [apps, setApps] = useState<DashboardApplication[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [activeCategory, setActiveCategory] = useState('All');
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingApp, setEditingApp] = useState<DashboardApplication | null>(null);
  const [page, setPage] = useState(0);
  const pageSize = 50;
  const { toast } = useToast();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.listApplications(search, activeCategory, pageSize, page * pageSize);
      setApps(data.applications || []);
      setCategories(data.categories || []);
      setTotal(data.total || 0);
    } catch {
      // ignore
    }
    setLoading(false);
  }, [search, activeCategory, page]);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete application "${name}"?`)) return;
    try {
      await api.deleteApplication(id);
      toast({ title: 'Application deleted', description: `"${name}" has been removed.` });
      load();
    } catch {
      toast({ title: 'Error', description: 'Failed to delete application.', variant: 'destructive' });
    }
  };

  const handleToggleFavorite = async (id: string) => {
    try {
      await api.toggleApplicationFavorite(id);
      load();
    } catch {
      toast({ title: 'Error', description: 'Failed to toggle favorite.', variant: 'destructive' });
    }
  };

  return (
    <div>
      {/* Header */}
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-xl font-bold text-foreground">
            Application Dashboard
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Manage your services and applications &middot; {total} registered
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link to="/applications/settings" className="flex items-center gap-1.5">
              <Settings className="h-3.5 w-3.5" />
              Settings &amp; API
            </Link>
          </Button>
          <Button onClick={() => { setEditingApp(null); setShowAddForm(true); }}>
            <Plus className="h-3.5 w-3.5 mr-1.5" />
            Register Service
          </Button>
        </div>
      </div>

      {/* Search & Filter */}
      <div className="flex gap-3 mb-5 flex-wrap items-center">
        <Input
          type="text"
          placeholder="Search applications..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="min-w-[220px] flex-1 max-w-[360px]"
        />
        <div className="flex gap-1 flex-wrap">
          {['All', ...categories].map((cat) => (
            <Badge
              key={cat}
              variant={activeCategory === cat ? 'default' : 'outline'}
              className={cn(
                'cursor-pointer transition-all px-3 py-1',
                activeCategory === cat
                  ? 'bg-primary text-primary-foreground'
                  : 'hover:bg-accent'
              )}
              onClick={() => setActiveCategory(cat)}
            >
              {cat}
            </Badge>
          ))}
        </div>
      </div>

      {/* App Grid */}
      {loading ? (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-4">
          {[...Array(6)].map((_, i) => (
            <Card key={i} className="p-5">
              <div className="flex justify-between items-start mb-3">
                <Skeleton className="h-12 w-12 rounded-xl" />
                <div className="flex gap-1">
                  <Skeleton className="h-6 w-6 rounded" />
                  <Skeleton className="h-6 w-6 rounded" />
                </div>
              </div>
              <Skeleton className="h-5 w-3/4 mb-2" />
              <Skeleton className="h-4 w-1/3 mb-3" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-2/3 mt-1" />
            </Card>
          ))}
        </div>
      ) : apps.length === 0 ? (
        <div className="text-center py-16 px-5">
          <Rocket className="h-12 w-12 mx-auto text-muted-foreground mb-3" />
          <h3 className="text-base font-bold text-foreground">No Applications Registered</h3>
          <p className="mt-2 text-sm text-muted-foreground">
            Register your first service to get started
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-4">
          {apps.map((app) => (
            <AppCard
              key={app.id}
              app={app}
              onEdit={() => { setEditingApp(app); setShowAddForm(true); }}
              onDelete={() => handleDelete(app.id, app.name)}
              onToggleFavorite={() => handleToggleFavorite(app.id)}
            />
          ))}
        </div>
      )}

      {/* Pagination */}
      {total > pageSize && (
        <div className="flex justify-center gap-2 mt-6">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
          >
            Previous
          </Button>
          <span className="flex items-center text-sm text-muted-foreground">
            Page {page + 1} of {Math.ceil(total / pageSize)}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage((p) => p + 1)}
            disabled={(page + 1) * pageSize >= total}
          >
            Next
          </Button>
        </div>
      )}

      {/* Add/Edit Form Modal */}
      <Dialog open={showAddForm} onOpenChange={(open) => { if (!open) { setShowAddForm(false); setEditingApp(null); } }}>
        <DialogContent className="sm:max-w-[420px] max-h-[90vh] overflow-y-auto">
          <AddEditForm
            app={editingApp}
            onClose={() => { setShowAddForm(false); setEditingApp(null); }}
            onSaved={load}
          />
        </DialogContent>
      </Dialog>

      {/* AI Chatbot */}
      <AppChatbot applications={apps} />
    </div>
  );
}

// --- AppCard Component ---

function AppCard({
  app,
  onEdit,
  onDelete,
  onToggleFavorite,
}: {
  app: DashboardApplication;
  onEdit: () => void;
  onDelete: () => void;
  onToggleFavorite: () => void;
}) {
  const isImage = app.icon?.startsWith('data:image');

  return (
    <Card className="flex flex-col transition-shadow duration-200 hover:shadow-md">
      <CardContent className="p-5 flex flex-col flex-1">
        <div className="flex justify-between items-start mb-3">
          <div className="w-12 h-12 rounded-xl bg-background border border-border flex items-center justify-center overflow-hidden shrink-0">
            {isImage ? (
              <img src={app.icon} alt={app.name} className="w-full h-full object-cover rounded-xl" />
            ) : (
              <span className="text-[28px]">{app.icon || '\uD83C\uDF10'}</span>
            )}
          </div>
          <div className="flex gap-1">
            <Button
              variant="ghost"
              size="icon"
              className={cn('h-7 w-7', app.is_favorite ? 'text-amber-500' : 'text-muted-foreground')}
              onClick={onToggleFavorite}
              title="Favorite"
            >
              <Star className={cn('h-3.5 w-3.5', app.is_favorite && 'fill-current')} />
            </Button>
            <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground" onClick={onEdit} title="Edit">
              <Pencil className="h-3.5 w-3.5" />
            </Button>
            <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground" onClick={onDelete} title="Delete">
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>

        <a href={app.url} target="_blank" rel="noopener noreferrer" className="no-underline flex-1">
          <h3 className="mb-1.5 text-base font-bold text-foreground">
            {app.name}
          </h3>
          <Badge variant="secondary" className="text-xs">
            {app.category || 'General'}
          </Badge>
          <p className="mt-2.5 text-sm text-muted-foreground leading-relaxed line-clamp-3">
            {app.description || 'No description provided.'}
          </p>
        </a>

        <div className="mt-4 pt-3 border-t border-border flex justify-between items-center">
          <div className="flex items-center gap-1.5">
            <div className="w-1.5 h-1.5 rounded-full bg-green-500" />
            <span className="text-[11px] text-muted-foreground font-medium">Active</span>
          </div>
          <span className="text-[11px] text-muted-foreground">
            {new Date(app.created_at).toLocaleDateString()}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}

// --- Add/Edit Form ---

function AddEditForm({
  app,
  onClose,
  onSaved,
}: {
  app: DashboardApplication | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(app?.name || '');
  const [url, setUrl] = useState(app?.url || '');
  const [description, setDescription] = useState(app?.description || '');
  const [icon, setIcon] = useState(app?.icon || '\uD83D\uDE80');
  const [category, setCategory] = useState(app?.category || 'General');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const { toast } = useToast();

  const isEdit = !!app;

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      const reader = new FileReader();
      reader.onloadend = () => setIcon(reader.result as string);
      reader.readAsDataURL(file);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError('');
    try {
      if (isEdit) {
        await api.updateApplication(app.id, name, url, description, icon, category);
        toast({ title: 'Application updated', description: `"${name}" has been updated.` });
      } else {
        await api.createApplication(name, url, description, icon, category);
        toast({ title: 'Application registered', description: `"${name}" has been created.` });
      }
      onSaved();
      onClose();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to save';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
    setSaving(false);
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>
          {isEdit ? 'Edit Application' : 'Register New Service'}
        </DialogTitle>
      </DialogHeader>

      {error && (
        <div className="p-2 px-3 rounded-lg bg-destructive/10 text-destructive text-sm mb-4">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="app-url">URL</Label>
          <Input id="app-url" value={url} onChange={(e) => setUrl(e.target.value)} required placeholder="https://..." />
        </div>

        <div className="flex items-center gap-4 p-4 bg-muted rounded-xl border border-border">
          <div className="relative w-14 h-14 rounded-xl bg-background border border-border flex items-center justify-center overflow-hidden cursor-pointer shrink-0">
            {icon.startsWith('data:image') ? (
              <img src={icon} alt="Icon" className="w-full h-full object-cover" />
            ) : (
              <span className="text-[28px]">{icon}</span>
            )}
            <input type="file" accept="image/*" onChange={handleFileUpload} className="absolute inset-0 opacity-0 cursor-pointer" />
          </div>
          <div className="flex-1 space-y-1">
            <Label>Icon (emoji or upload image)</Label>
            <Input value={icon} onChange={(e) => setIcon(e.target.value || '\uD83D\uDE80')} maxLength={2} placeholder="Emoji" />
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="app-name">Name</Label>
          <Input id="app-name" value={name} onChange={(e) => setName(e.target.value)} required placeholder="Service name" />
        </div>

        <div className="space-y-2">
          <Label htmlFor="app-description">Description</Label>
          <Textarea id="app-description" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What does this service do?" rows={3} />
        </div>

        <div className="space-y-2">
          <Label>Category</Label>
          <Select value={category} onValueChange={setCategory}>
            <SelectTrigger>
              <SelectValue placeholder="Select category" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="General">General</SelectItem>
              <SelectItem value="Automation">Automation</SelectItem>
              <SelectItem value="Dev Tools">Dev Tools</SelectItem>
              <SelectItem value="Infrastructure">Infrastructure</SelectItem>
              <SelectItem value="Personal">Personal</SelectItem>
              <SelectItem value="AI & ML">AI &amp; ML</SelectItem>
              <SelectItem value="Monitoring">Monitoring</SelectItem>
              <SelectItem value="Security">Security</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <Button type="submit" disabled={saving} className="w-full mt-2">
          {saving ? 'Saving...' : isEdit ? 'Update Application' : 'Register Service'}
        </Button>
      </form>
    </>
  );
}
