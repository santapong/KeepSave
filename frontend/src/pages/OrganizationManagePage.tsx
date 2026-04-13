import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import type { Organization, OrgMember, Project } from '../types';
import * as api from '../api/client';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useToast } from '@/hooks/useToast';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { ArrowLeft, Users, FolderOpen, AlertCircle, Trash2, X } from 'lucide-react';

export function OrganizationManagePage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();

  const [org, setOrg] = useState<Organization | null>(null);
  const [members, setMembers] = useState<OrgMember[]>([]);
  const [orgProjects, setOrgProjects] = useState<Project[]>([]);
  const [allProjects, setAllProjects] = useState<Project[]>([]);
  const [newMemberEmail, setNewMemberEmail] = useState('');
  const [newMemberRole, setNewMemberRole] = useState('viewer');
  const [assignProjectId, setAssignProjectId] = useState('');
  const [error, setError] = useState('');
  const [memberError, setMemberError] = useState('');
  const [loading, setLoading] = useState(true);
  const [addingMember, setAddingMember] = useState(false);
  const [deleteOrgOpen, setDeleteOrgOpen] = useState(false);
  const [removeMemberId, setRemoveMemberId] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    if (!id) return;
    try {
      setLoading(true);
      const orgData = await api.getOrganization(id);
      setOrg(orgData);
      const [memberData, orgProjectData, allProjectData] = await Promise.all([
        api.listOrgMembers(id),
        api.listOrgProjects(id),
        api.listProjects(),
      ]);
      setMembers(memberData);
      setOrgProjects(orgProjectData);
      setAllProjects(allProjectData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load organization');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  async function handleAddMember(e: FormEvent) {
    e.preventDefault();
    if (!id || !newMemberEmail.trim()) return;
    setMemberError('');
    setAddingMember(true);
    try {
      const user = await api.lookupUserByEmail(newMemberEmail.trim());
      await api.addOrgMember(id, user.id, newMemberRole);
      setNewMemberEmail('');
      setNewMemberRole('viewer');
      const data = await api.listOrgMembers(id);
      setMembers(data);
      toast({ title: 'Member added', description: `Added member to ${org?.name}.` });
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
    if (!id) return;
    try {
      await api.updateOrgMemberRole(id, userId, role);
      const data = await api.listOrgMembers(id);
      setMembers(data);
      toast({ title: 'Role updated', description: `Member role changed to ${role}.` });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to update role';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  async function handleRemoveMember(userId: string) {
    if (!id) return;
    try {
      await api.removeOrgMember(id, userId);
      const data = await api.listOrgMembers(id);
      setMembers(data);
      toast({ title: 'Member removed', description: 'The member has been removed from the organization.' });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to remove member';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  async function handleAssignProject() {
    if (!id || !assignProjectId) return;
    try {
      await api.assignProjectToOrg(id, assignProjectId);
      setAssignProjectId('');
      const data = await api.listOrgProjects(id);
      setOrgProjects(data);
      toast({ title: 'Project assigned', description: 'Project has been assigned to the organization.' });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to assign project';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  async function handleDeleteOrg() {
    if (!id) return;
    try {
      await api.deleteOrganization(id);
      toast({ title: 'Organization deleted', description: 'The organization has been removed.' });
      navigate('/organizations');
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to delete organization';
      setError(msg);
      toast({ title: 'Error', description: msg, variant: 'destructive' });
    }
  }

  const assignedIds = new Set(orgProjects.map((p) => p.id));
  const unassignedProjects = allProjects.filter((p) => !assignedIds.has(p.id));

  if (loading) {
    return (
      <div className="space-y-4 p-10">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-8 w-48" />
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mt-6">
          <Skeleton className="h-64 w-full rounded-lg" />
          <Skeleton className="h-64 w-full rounded-lg" />
        </div>
      </div>
    );
  }

  if (error && !org) {
    return (
      <div className="p-10">
        <Link
          to="/organizations"
          className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground mb-4"
        >
          <ArrowLeft className="mr-1.5 h-4 w-4" />
          Back to Organizations
        </Link>
        <div className="bg-destructive/10 text-destructive px-3.5 py-2.5 rounded-md text-sm border border-destructive/20">
          {error}
        </div>
      </div>
    );
  }

  if (!org) return null;

  return (
    <div>
      {/* Back link */}
      <Link
        to="/organizations"
        className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground mb-4"
      >
        <ArrowLeft className="mr-1.5 h-4 w-4" />
        Back to Organizations
      </Link>

      {/* Org header */}
      <div className="flex items-center gap-3.5 mb-6">
        <div className="w-11 h-11 rounded-full bg-gradient-to-br from-indigo-500 to-indigo-700 text-white flex items-center justify-center font-bold text-lg shrink-0">
          {org.name.charAt(0).toUpperCase()}
        </div>
        <div>
          <h1 className="text-2xl font-bold text-foreground">{org.name}</h1>
          <p className="text-sm text-muted-foreground">/{org.slug}</p>
        </div>
        <div className="flex gap-2 ml-4">
          <Badge variant="outline" className="gap-1">
            <Users className="h-3 w-3" />
            {members.length} member{members.length !== 1 ? 's' : ''}
          </Badge>
          <Badge variant="outline" className="gap-1">
            <FolderOpen className="h-3 w-3" />
            {orgProjects.length} project{orgProjects.length !== 1 ? 's' : ''}
          </Badge>
        </div>
      </div>

      {/* Error */}
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

      {/* Management panel */}
      <Card className="overflow-hidden shadow-lg">
        <div className="grid grid-cols-1 md:grid-cols-2">
          {/* Left column: Members */}
          <div className="p-6 md:border-r border-border">
            <div className="flex items-center gap-2 mb-4">
              <h3 className="text-sm font-bold text-foreground">Members</h3>
              {members.length > 0 && (
                <Badge variant="secondary" className="text-xs">{members.length}</Badge>
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
              <Button type="submit" disabled={addingMember} size="sm">
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
              <div className="text-center py-7 border border-dashed border-border rounded-lg bg-muted/30">
                <Users className="h-5 w-5 mx-auto text-muted-foreground opacity-50" />
                <p className="text-sm text-muted-foreground mt-2">
                  No members yet. Add team members by email above.
                </p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>User</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead className="text-right">Action</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {members.map((m) => (
                    <TableRow key={m.id}>
                      <TableCell>
                        <div className="flex items-center gap-2.5">
                          <div className="w-7 h-7 rounded-full bg-border text-muted-foreground flex items-center justify-center font-semibold text-xs shrink-0">
                            {m.user_id.charAt(0).toUpperCase()}
                          </div>
                          <code className="text-xs font-mono text-muted-foreground" title={m.user_id}>
                            {m.user_id.slice(0, 8)}...
                          </code>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Select value={m.role} onValueChange={(val) => handleUpdateRole(m.user_id, val)}>
                          <SelectTrigger className="w-[110px] h-7 text-xs">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="viewer">Viewer</SelectItem>
                            <SelectItem value="editor">Editor</SelectItem>
                            <SelectItem value="promoter">Promoter</SelectItem>
                            <SelectItem value="admin">Admin</SelectItem>
                          </SelectContent>
                        </Select>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-destructive hover:bg-destructive/10"
                          onClick={() => setRemoveMemberId(m.user_id)}
                          title="Remove member"
                        >
                          <X className="h-3.5 w-3.5" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>

          {/* Right column: Projects */}
          <div className="p-6">
            <div className="flex items-center gap-2 mb-4">
              <h3 className="text-sm font-bold text-foreground">Projects</h3>
              {orgProjects.length > 0 && (
                <Badge variant="secondary" className="text-xs">{orgProjects.length}</Badge>
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
                <Button
                  onClick={handleAssignProject}
                  disabled={!assignProjectId}
                  size="sm"
                >
                  Assign
                </Button>
              </div>
            )}

            {/* Project list */}
            {orgProjects.length === 0 ? (
              <div className="text-center py-7 border border-dashed border-border rounded-lg bg-muted/30">
                <FolderOpen className="h-5 w-5 mx-auto text-muted-foreground opacity-50" />
                <p className="text-sm text-muted-foreground mt-2">
                  No projects assigned yet.
                </p>
              </div>
            ) : (
              <div className="flex flex-col gap-2">
                {orgProjects.map((p) => (
                  <Card key={p.id} className="shadow-none">
                    <CardContent className="p-3 flex items-center gap-3">
                      <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-indigo-500 to-violet-400 text-white flex items-center justify-center font-bold text-sm shrink-0">
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
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </div>
        </div>
      </Card>

      {/* Danger zone */}
      <div className="mt-8 pt-6 border-t border-border">
        <h3 className="text-sm font-bold text-destructive mb-3">Danger Zone</h3>
        <Button
          variant="destructive"
          size="sm"
          onClick={() => setDeleteOrgOpen(true)}
        >
          <Trash2 className="mr-1.5 h-3.5 w-3.5" />
          Delete Organization
        </Button>
      </div>

      {/* Confirm delete org */}
      <ConfirmDialog
        open={deleteOrgOpen}
        onOpenChange={setDeleteOrgOpen}
        title="Delete Organization"
        description={`Are you sure you want to delete "${org.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        onConfirm={handleDeleteOrg}
      />

      {/* Confirm remove member */}
      <ConfirmDialog
        open={!!removeMemberId}
        onOpenChange={(open) => { if (!open) setRemoveMemberId(null); }}
        title="Remove Member"
        description="Are you sure you want to remove this member from the organization?"
        confirmLabel="Remove"
        onConfirm={() => {
          if (removeMemberId) handleRemoveMember(removeMemberId);
          setRemoveMemberId(null);
        }}
      />
    </div>
  );
}
