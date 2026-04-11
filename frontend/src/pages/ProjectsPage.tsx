import { useState, useEffect, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { listProjects, createProject, deleteProject } from '../api/client';
import type { Project } from '../types';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/hooks/useToast';
import { Plus, Trash2, FolderOpen } from 'lucide-react';


export function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');
  const navigate = useNavigate();
  const { toast } = useToast();

  useEffect(() => {
    loadProjects();
  }, []);

  async function loadProjects() {
    try {
      const data = await listProjects();
      setProjects(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load projects');
    } finally {
      setLoading(false);
    }
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    try {
      await createProject(name, description);
      setName('');
      setDescription('');
      setShowCreate(false);
      loadProjects();
      toast({ title: 'Project created', description: `"${name}" has been created successfully.` });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project');
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm('Delete this project? This cannot be undone.')) return;
    try {
      await deleteProject(id);
      loadProjects();
      toast({ title: 'Project deleted', description: 'The project has been removed.', variant: 'destructive' });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete project');
    }
  }

  if (loading) {
    return (
      <div className="space-y-3 p-10">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-32" />
        <div className="grid gap-3 mt-6">
          <Skeleton className="h-24 w-full rounded-lg" />
          <Skeleton className="h-24 w-full rounded-lg" />
          <Skeleton className="h-24 w-full rounded-lg" />
        </div>
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className="flex justify-between items-start mb-6">
        <div>
          <h1 className="text-2xl font-bold">Projects</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            {projects.length} project{projects.length !== 1 ? 's' : ''}
          </p>
        </div>
        <Button onClick={() => setShowCreate(!showCreate)} variant={showCreate ? 'outline' : 'default'}>
          {showCreate ? (
            'Cancel'
          ) : (
            <>
              <Plus className="mr-2 h-4 w-4" />
              New Project
            </>
          )}
        </Button>
      </div>

      {/* Error */}
      {error && (
        <div className="bg-destructive/10 text-destructive px-3.5 py-2.5 rounded-md text-sm border border-destructive/20 mb-4">
          {error}
        </div>
      )}

      {/* Create form */}
      {showCreate && (
        <Card className="mb-6">
          <CardHeader className="pb-3">
            <CardTitle className="text-[15px] font-semibold">Create Project</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleCreate} className="flex gap-3 flex-wrap">
              <Input
                placeholder="Project name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                className="flex-1 min-w-[200px]"
              />
              <Input
                placeholder="Description (optional)"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="flex-[2] min-w-[200px]"
              />
              <Button type="submit">Create</Button>
            </form>
          </CardContent>
        </Card>
      )}

      {/* Project list */}
      {projects.length === 0 ? (
        <Card className="text-center py-16">
          <CardContent>
            <FolderOpen className="mx-auto h-10 w-10 text-muted-foreground mb-3" />
            <p className="text-base font-medium mb-1">No projects yet</p>
            <p className="text-sm text-muted-foreground">Create your first project to start managing secrets securely.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3">
          {projects.map((p) => (
            <Card key={p.id} className="flex items-start gap-4 p-5">
              <div
                className="flex-1 cursor-pointer"
                onClick={() => navigate(`/projects/${p.id}`)}
              >
                <div className="flex items-center gap-2.5">
                  <div className="w-9 h-9 rounded-lg bg-primary/15 text-primary flex items-center justify-center font-bold text-base shrink-0">
                    {p.name.charAt(0).toUpperCase()}
                  </div>
                  <div>
                    <h3 className="text-[15px] font-semibold">{p.name}</h3>
                    {p.description && (
                      <p className="text-sm text-muted-foreground mt-0.5">
                        {p.description}
                      </p>
                    )}
                  </div>
                </div>
                <div className="flex gap-4 mt-3">
                  <Badge variant="outline" className="text-[11px] font-normal">
                    Created {new Date(p.created_at).toLocaleDateString()}
                  </Badge>
                  <Badge variant="outline" className="text-[11px] font-normal">
                    3 environments
                  </Badge>
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                className="text-destructive border-destructive/30 hover:bg-destructive/10 hover:text-destructive shrink-0"
                onClick={(e) => { e.stopPropagation(); handleDelete(p.id); }}
              >
                <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                Delete
              </Button>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
