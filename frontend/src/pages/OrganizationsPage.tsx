import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Organization } from '../types';
import * as api from '../api/client';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from '@/components/ui/dialog';
import { useToast } from '@/hooks/useToast';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Plus, Users, FolderOpen, AlertCircle, Trash2 } from 'lucide-react';

export function OrganizationsPage() {
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [newOrgName, setNewOrgName] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [memberCounts, setMemberCounts] = useState<Record<string, number>>({});
  const [projectCounts, setProjectCounts] = useState<Record<string, number>>({});
  const [deleteTarget, setDeleteTarget] = useState<Organization | null>(null);
  const navigate = useNavigate();
  const { toast } = useToast();

  const loadOrgs = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.listOrganizations();
      setOrgs(data);
      const mCounts: Record<string, number> = {};
      const pCounts: Record<string, number> = {};
      await Promise.all(
        data.map(async (org: Organization) => {
          try {
            const [m, p] = await Promise.all([
              api.listOrgMembers(org.id),
              api.listOrgProjects(org.id),
            ]);
            mCounts[org.id] = m.length;
            pCounts[org.id] = p.length;
          } catch {
            mCounts[org.id] = 0;
            pCounts[org.id] = 0;
          }
        })
      );
      setMemberCounts(mCounts);
      setProjectCounts(pCounts);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load organizations');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadOrgs();
  }, [loadOrgs]);

  async function handleCreateOrg(e: FormEvent) {
    e.preventDefault();
    if (!newOrgName.trim()) return;
    try {
      await api.createOrganization(newOrgName.trim());
      toast({ title: 'Organization created', description: `"${newOrgName.trim()}" has been created.` });
      setNewOrgName('');
      setShowCreate(false);
      await loadOrgs();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to create organization';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  async function handleDeleteOrg(orgId: string) {
    try {
      await api.deleteOrganization(orgId);
      toast({ title: 'Organization deleted', description: 'The organization has been removed.' });
      loadOrgs();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to delete organization';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center py-20 px-10">
        <div className="space-y-4 w-full max-w-2xl">
          <Skeleton className="h-8 w-48 mx-auto" />
          <Skeleton className="h-4 w-32 mx-auto" />
          <div className="grid grid-cols-[repeat(auto-fill,minmax(340px,1fr))] gap-4 mt-8">
            {[...Array(3)].map((_, i) => (
              <Card key={i} className="p-5">
                <div className="flex items-center gap-3 mb-4">
                  <Skeleton className="h-11 w-11 rounded-full" />
                  <div className="space-y-2 flex-1">
                    <Skeleton className="h-4 w-3/4" />
                    <Skeleton className="h-3 w-1/2" />
                  </div>
                </div>
                <div className="flex gap-2 mb-4">
                  <Skeleton className="h-6 w-24 rounded-full" />
                  <Skeleton className="h-6 w-24 rounded-full" />
                </div>
                <Skeleton className="h-9 w-full rounded" />
              </Card>
            ))}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className="flex justify-between items-start mb-6">
        <div>
          <h1 className="text-2xl font-bold text-foreground">
            Organizations
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            {orgs.length} organization{orgs.length !== 1 ? 's' : ''}
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="h-3.5 w-3.5 mr-1.5" /> New Organization
        </Button>
      </div>

      {/* Error alert */}
      {error && (
        <div className="flex justify-between items-center bg-destructive/5 text-destructive p-2.5 px-3.5 rounded-lg text-sm mb-5 border border-destructive/15">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
          <Button variant="ghost" size="sm" onClick={() => setError('')} className="text-destructive h-auto py-0.5 px-2 text-xs font-semibold">
            Dismiss
          </Button>
        </div>
      )}

      {/* Create organization dialog */}
      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Create Organization</DialogTitle>
            <DialogDescription>Create a new organization to manage teams and projects together.</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleCreateOrg} className="space-y-4">
            <div>
              <Label htmlFor="org-name">Organization name</Label>
              <Input
                id="org-name"
                placeholder="Organization name"
                value={newOrgName}
                onChange={(e) => setNewOrgName(e.target.value)}
                required
                autoFocus
                className="mt-1"
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
              <Button type="submit">Create</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Empty state */}
      {orgs.length === 0 ? (
        <Card className="flex flex-col items-center justify-center text-center py-16 px-10">
          <CardContent>
            <div className="w-16 h-16 rounded-full bg-muted border border-border flex items-center justify-center mx-auto mb-5">
              <Users className="h-8 w-8 text-muted-foreground" />
            </div>
            <p className="text-lg font-semibold text-foreground mb-1.5">
              No organizations yet
            </p>
            <p className="text-sm text-muted-foreground max-w-[340px] leading-relaxed">
              Create your first organization to manage teams and projects together.
            </p>
          </CardContent>
        </Card>
      ) : (
        /* Org grid */
        <div className="grid grid-cols-[repeat(auto-fill,minmax(340px,1fr))] gap-4">
          {orgs.map((org) => (
            <Card
              key={org.id}
              className="transition-all duration-200 hover:shadow-md"
            >
              <CardContent className="p-5">
                {/* Card top: icon + name + slug */}
                <div className="flex items-center gap-3.5">
                  <div className="w-11 h-11 rounded-full bg-gradient-to-br from-indigo-500 to-indigo-700 text-white flex items-center justify-center font-bold text-lg shrink-0">
                    {org.name.charAt(0).toUpperCase()}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-base font-bold text-foreground truncate">
                      {org.name}
                    </div>
                    <div className="text-xs text-muted-foreground mt-0.5">
                      /{org.slug}
                    </div>
                  </div>
                </div>

                {/* Card middle: badges */}
                <div className="flex gap-2 mt-4">
                  <Badge variant="outline" className="gap-1">
                    <Users className="h-3 w-3" />
                    {memberCounts[org.id] ?? 0} member{(memberCounts[org.id] ?? 0) !== 1 ? 's' : ''}
                  </Badge>
                  <Badge variant="outline" className="gap-1">
                    <FolderOpen className="h-3 w-3" />
                    {projectCounts[org.id] ?? 0} project{(projectCounts[org.id] ?? 0) !== 1 ? 's' : ''}
                  </Badge>
                </div>

                {/* Card bottom: actions */}
                <div className="flex gap-2.5 mt-4 pt-4 border-t border-border">
                  <Button
                    variant="outline"
                    size="sm"
                    className="flex-1"
                    onClick={() => navigate(`/organizations/${org.id}`)}
                  >
                    Manage
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-destructive border-destructive/25 hover:bg-destructive/10"
                    onClick={(e) => { e.stopPropagation(); setDeleteTarget(org); }}
                  >
                    <Trash2 className="h-3.5 w-3.5 mr-1" />
                    Delete
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Confirm delete dialog */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
        title="Delete Organization"
        description={deleteTarget ? `Are you sure you want to delete "${deleteTarget.name}"? This action cannot be undone.` : ''}
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleteTarget) handleDeleteOrg(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}
