import { useState, useEffect, useCallback, type FormEvent } from 'react';
import type { Organization, OrgMember, Project } from '../types';
import * as api from '../api/client';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/hooks/useToast';
import { Plus, Users, FolderOpen, X, AlertCircle } from 'lucide-react';

const roleBadgeVariants: Record<string, string> = {
  admin: 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-400',
  promoter: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-400',
  editor: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-400',
  viewer: 'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-400',
};

export function OrganizationsPage() {
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<Organization | null>(null);
  const [members, setMembers] = useState<OrgMember[]>([]);
  const [orgProjects, setOrgProjects] = useState<Project[]>([]);
  const [allProjects, setAllProjects] = useState<Project[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [newOrgName, setNewOrgName] = useState('');
  const [newMemberEmail, setNewMemberEmail] = useState('');
  const [newMemberRole, setNewMemberRole] = useState('viewer');
  const [assignProjectId, setAssignProjectId] = useState('');
  const [error, setError] = useState('');
  const [memberError, setMemberError] = useState('');
  const [loading, setLoading] = useState(true);
  const [addingMember, setAddingMember] = useState(false);
  const [memberCounts, setMemberCounts] = useState<Record<string, number>>({});
  const [projectCounts, setProjectCounts] = useState<Record<string, number>>({});
  const { toast } = useToast();

  const loadOrgs = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.listOrganizations();
      setOrgs(data);
      // Load counts for each org
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
      const org = await api.createOrganization(newOrgName.trim());
      setNewOrgName('');
      setShowCreate(false);
      toast({ title: 'Organization created', description: `"${org.name}" has been created.` });
      await loadOrgs();
      selectOrg(org);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to create organization';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  async function handleDeleteOrg(orgId: string) {
    if (!window.confirm('Delete this organization? This cannot be undone.')) return;
    try {
      await api.deleteOrganization(orgId);
      if (selectedOrg?.id === orgId) {
        setSelectedOrg(null);
        setMembers([]);
        setOrgProjects([]);
      }
      toast({ title: 'Organization deleted', description: 'The organization has been removed.' });
      loadOrgs();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to delete organization';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  async function selectOrg(org: Organization) {
    setSelectedOrg(org);
    setMemberError('');
    try {
      const [memberData, orgProjectData, allProjectData] = await Promise.all([
        api.listOrgMembers(org.id),
        api.listOrgProjects(org.id),
        api.listProjects(),
      ]);
      setMembers(memberData);
      setOrgProjects(orgProjectData);
      setAllProjects(allProjectData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load organization details');
    }
  }

  async function handleAddMember(e: FormEvent) {
    e.preventDefault();
    if (!selectedOrg || !newMemberEmail.trim()) return;
    setMemberError('');
    setAddingMember(true);
    try {
      const user = await api.lookupUserByEmail(newMemberEmail.trim());
      await api.addOrgMember(selectedOrg.id, user.id, newMemberRole);
      setNewMemberEmail('');
      setNewMemberRole('viewer');
      const data = await api.listOrgMembers(selectedOrg.id);
      setMembers(data);
      setMemberCounts((prev) => ({ ...prev, [selectedOrg.id]: data.length }));
      toast({ title: 'Member added', description: 'The user has been added to the organization.' });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to add member';
      if (message.toLowerCase().includes('not found') || message.toLowerCase().includes('no user')) {
        setMemberError('No user found with this email');
      } else {
        setMemberError(message);
      }
      toast({ title: 'Error', description: message, variant: 'destructive' });
    } finally {
      setAddingMember(false);
    }
  }

  async function handleUpdateRole(userId: string, role: string) {
    if (!selectedOrg) return;
    try {
      await api.updateOrgMemberRole(selectedOrg.id, userId, role);
      const data = await api.listOrgMembers(selectedOrg.id);
      setMembers(data);
      toast({ title: 'Role updated', description: `Member role changed to ${role}.` });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to update role';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  async function handleRemoveMember(userId: string) {
    if (!selectedOrg) return;
    if (!window.confirm('Remove this member?')) return;
    try {
      await api.removeOrgMember(selectedOrg.id, userId);
      const data = await api.listOrgMembers(selectedOrg.id);
      setMembers(data);
      setMemberCounts((prev) => ({ ...prev, [selectedOrg.id]: data.length }));
      toast({ title: 'Member removed', description: 'The member has been removed from the organization.' });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to remove member';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  async function handleAssignProject() {
    if (!selectedOrg || !assignProjectId) return;
    try {
      await api.assignProjectToOrg(selectedOrg.id, assignProjectId);
      setAssignProjectId('');
      const data = await api.listOrgProjects(selectedOrg.id);
      setOrgProjects(data);
      setProjectCounts((prev) => ({ ...prev, [selectedOrg.id]: data.length }));
      toast({ title: 'Project assigned', description: 'The project has been assigned to the organization.' });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to assign project';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  function handleManageClick(org: Organization, e: React.MouseEvent) {
    e.stopPropagation();
    if (selectedOrg?.id === org.id) {
      setSelectedOrg(null);
      setMembers([]);
      setOrgProjects([]);
    } else {
      selectOrg(org);
    }
  }

  const assignedIds = new Set(orgProjects.map((p) => p.id));
  const unassignedProjects = allProjects.filter((p) => !assignedIds.has(p.id));

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center py-20 px-10">
        <Skeleton className="h-8 w-8 rounded-full mb-4" />
        <Skeleton className="h-4 w-48" />
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
        <Button onClick={() => setShowCreate(!showCreate)}>
          {showCreate ? 'Cancel' : <><Plus className="h-3.5 w-3.5 mr-1" /> New Organization</>}
        </Button>
      </div>

      {/* Error alert */}
      {error && (
        <div className="flex justify-between items-center bg-destructive/5 text-destructive p-2.5 px-3.5 rounded-lg text-sm mb-5 border border-destructive/15">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
          <Button variant="ghost" size="sm" className="text-destructive text-xs h-auto p-1 px-2" onClick={() => setError('')}>
            Dismiss
          </Button>
        </div>
      )}

      {/* Create form (collapsible card) */}
      {showCreate && (
        <Card className="mb-6">
          <CardContent className="p-6">
            <form onSubmit={handleCreateOrg}>
              <h3 className="text-sm font-semibold mb-4 text-foreground">Create Organization</h3>
              <div className="flex gap-3 items-center">
                <Input
                  placeholder="Organization name"
                  value={newOrgName}
                  onChange={(e) => setNewOrgName(e.target.value)}
                  required
                  autoFocus
                  className="flex-1"
                />
                <Button type="submit">Create</Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {/* Empty state */}
      {orgs.length === 0 && !showCreate ? (
        <Card className="text-center py-16 px-10">
          <CardContent className="flex flex-col items-center">
            <div className="w-16 h-16 rounded-full bg-muted border flex items-center justify-center mb-5">
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
        <>
          {/* Org grid */}
          <div className="grid grid-cols-[repeat(auto-fill,minmax(340px,1fr))] gap-4">
            {orgs.map((org) => {
              const isSelected = selectedOrg?.id === org.id;
              return (
                <Card
                  key={org.id}
                  className={cn(
                    'transition-all cursor-default',
                    isSelected
                      ? 'border-primary ring-2 ring-primary/20 shadow-lg'
                      : 'hover:shadow-md hover:-translate-y-px'
                  )}
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
                      <Badge variant="outline" className="gap-1.5 font-medium">
                        <Users className="h-3 w-3" />
                        {memberCounts[org.id] ?? 0} member{(memberCounts[org.id] ?? 0) !== 1 ? 's' : ''}
                      </Badge>
                      <Badge variant="outline" className="gap-1.5 font-medium">
                        <FolderOpen className="h-3 w-3" />
                        {projectCounts[org.id] ?? 0} project{(projectCounts[org.id] ?? 0) !== 1 ? 's' : ''}
                      </Badge>
                    </div>

                    {/* Card bottom: actions */}
                    <div className="flex gap-2.5 mt-4 pt-4 border-t">
                      <Button
                        variant={isSelected ? 'default' : 'outline'}
                        size="sm"
                        className="flex-1"
                        onClick={(e) => handleManageClick(org, e)}
                      >
                        {isSelected ? 'Close' : 'Manage'}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="text-destructive border-destructive/25 hover:bg-destructive/10"
                        onClick={(e) => { e.stopPropagation(); handleDeleteOrg(org.id); }}
                      >
                        Delete
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>

          {/* Management panel (below grid, full width) */}
          {selectedOrg && (
            <Card className="mt-6 shadow-lg overflow-hidden">
              {/* Panel header */}
              <div className="flex justify-between items-center px-6 py-5 border-b bg-muted/50">
                <div className="flex items-center gap-3">
                  <div className="w-9 h-9 rounded-full bg-gradient-to-br from-indigo-500 to-indigo-700 text-white flex items-center justify-center font-bold text-sm shrink-0">
                    {selectedOrg.name.charAt(0).toUpperCase()}
                  </div>
                  <div>
                    <h2 className="text-lg font-bold text-foreground">
                      {selectedOrg.name}
                    </h2>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      /{selectedOrg.slug}
                    </p>
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-1 lg:grid-cols-2">
                {/* Left column: Members */}
                <div className="p-6 lg:border-r">
                  <div className="flex items-center gap-2 mb-4">
                    <h3 className="text-[15px] font-bold text-foreground">Members</h3>
                    {members.length > 0 && (
                      <Badge variant="secondary" className="text-xs font-bold">
                        {members.length}
                      </Badge>
                    )}
                  </div>

                  {/* Add member form */}
                  <form onSubmit={handleAddMember} className="flex gap-2 mb-4 flex-wrap">
                    <Input
                      type="email"
                      placeholder="Email address"
                      value={newMemberEmail}
                      onChange={(e) => { setNewMemberEmail(e.target.value); setMemberError(''); }}
                      required
                      className="flex-1 min-w-[160px]"
                    />
                    <Select value={newMemberRole} onValueChange={setNewMemberRole}>
                      <SelectTrigger className="w-[120px]">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="viewer">Viewer</SelectItem>
                        <SelectItem value="editor">Editor</SelectItem>
                        <SelectItem value="promoter">Promoter</SelectItem>
                        <SelectItem value="admin">Admin</SelectItem>
                      </SelectContent>
                    </Select>
                    <Button type="submit" disabled={addingMember}>
                      {addingMember ? 'Adding...' : 'Add'}
                    </Button>
                  </form>

                  {memberError && (
                    <div className="flex items-center gap-2 bg-destructive/5 text-destructive p-2 px-3 rounded-lg text-sm mb-4 border border-destructive/15">
                      <AlertCircle className="h-3.5 w-3.5 shrink-0" />
                      {memberError}
                    </div>
                  )}

                  {/* Member list */}
                  {members.length === 0 ? (
                    <div className="text-center p-7 border border-dashed rounded-lg bg-muted/30">
                      <Users className="h-5 w-5 mx-auto text-muted-foreground opacity-50" />
                      <p className="text-sm text-muted-foreground mt-2">
                        No members yet. Add team members by email above.
                      </p>
                    </div>
                  ) : (
                    <div className="flex flex-col gap-2">
                      {members.map((m) => (
                        <div key={m.id} className="flex items-center justify-between p-2.5 px-3.5 bg-muted border rounded-lg gap-3">
                          <div className="flex items-center gap-2.5 flex-1 min-w-0">
                            <div className="w-7 h-7 rounded-full bg-border text-muted-foreground flex items-center justify-center font-semibold text-xs shrink-0">
                              {m.user_id.charAt(0).toUpperCase()}
                            </div>
                            <code className="text-xs font-mono text-muted-foreground truncate" title={m.user_id}>
                              {m.user_id.slice(0, 8)}...
                            </code>
                          </div>
                          <div className="flex items-center gap-2">
                            <Select value={m.role} onValueChange={(val) => handleUpdateRole(m.user_id, val)}>
                              <SelectTrigger className={cn('w-[110px] h-7 text-xs font-semibold rounded-full', roleBadgeVariants[m.role] || '')}>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="viewer">Viewer</SelectItem>
                                <SelectItem value="editor">Editor</SelectItem>
                                <SelectItem value="promoter">Promoter</SelectItem>
                                <SelectItem value="admin">Admin</SelectItem>
                              </SelectContent>
                            </Select>
                            <Button
                              variant="outline"
                              size="icon"
                              className="h-7 w-7 text-destructive border-destructive/20 hover:bg-destructive/10 shrink-0"
                              onClick={() => handleRemoveMember(m.user_id)}
                              title="Remove member"
                            >
                              <X className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Right column: Projects */}
                <div className="p-6">
                  <div className="flex items-center gap-2 mb-4">
                    <h3 className="text-[15px] font-bold text-foreground">Projects</h3>
                    {orgProjects.length > 0 && (
                      <Badge variant="secondary" className="text-xs font-bold">
                        {orgProjects.length}
                      </Badge>
                    )}
                  </div>

                  {/* Assign project */}
                  {unassignedProjects.length > 0 && (
                    <div className="flex gap-2 mb-4">
                      <Select value={assignProjectId} onValueChange={setAssignProjectId}>
                        <SelectTrigger className="flex-1">
                          <SelectValue placeholder="Select a project to assign..." />
                        </SelectTrigger>
                        <SelectContent>
                          {unassignedProjects.map((p) => (
                            <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <Button onClick={handleAssignProject} disabled={!assignProjectId}>
                        Assign
                      </Button>
                    </div>
                  )}

                  {/* Project list */}
                  {orgProjects.length === 0 ? (
                    <div className="text-center p-7 border border-dashed rounded-lg bg-muted/30">
                      <FolderOpen className="h-5 w-5 mx-auto text-muted-foreground opacity-50" />
                      <p className="text-sm text-muted-foreground mt-2">
                        No projects assigned yet.
                      </p>
                    </div>
                  ) : (
                    <div className="flex flex-col gap-2">
                      {orgProjects.map((p) => (
                        <div key={p.id} className="p-3 px-3.5 bg-muted border rounded-lg transition-colors hover:border-primary/30">
                          <div className="flex items-center gap-3">
                            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-indigo-500 to-indigo-400 text-white flex items-center justify-center font-bold text-sm shrink-0">
                              {p.name.charAt(0).toUpperCase()}
                            </div>
                            <div className="flex-1 min-w-0">
                              <div className="text-sm font-semibold text-foreground">
                                {p.name}
                              </div>
                              {p.description && (
                                <div className="text-xs text-muted-foreground mt-0.5 truncate">
                                  {p.description}
                                </div>
                              )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </Card>
          )}
        </>
      )}
    </div>
  );
}
