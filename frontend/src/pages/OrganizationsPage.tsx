import { useState, useEffect, useCallback, type CSSProperties, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Organization } from '../types';
import * as api from '../api/client';
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
import { useToast } from '@/hooks/useToast';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Users, FolderOpen, AlertCircle, Trash2 } from 'lucide-react';
import { Page, PageHeader, KpiStrip, Kpi, EmptyState } from '../components/cosmic/primitives';

const avatarStyle: CSSProperties = {
  width: 44,
  height: 44,
  borderRadius: '50%',
  display: 'grid',
  placeItems: 'center',
  flex: 'none',
  background: 'linear-gradient(135deg, var(--cz-accent-hi), var(--cz-plasma))',
  color: 'oklch(0.13 0.02 280)',
  fontWeight: 600,
  fontSize: 18,
  fontFamily: 'var(--cz-sans)',
};

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
            const [m, p] = await Promise.all([api.listOrgMembers(org.id), api.listOrgProjects(org.id)]);
            mCounts[org.id] = m.length;
            pCounts[org.id] = p.length;
          } catch {
            mCounts[org.id] = 0;
            pCounts[org.id] = 0;
          }
        }),
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

  const totalMembers = Object.values(memberCounts).reduce((a, b) => a + b, 0);
  const totalProjects = Object.values(projectCounts).reduce((a, b) => a + b, 0);
  const largest = Object.values(memberCounts).reduce((a, b) => Math.max(a, b), 0);

  return (
    <Page>
      <PageHeader
        eyebrow="Organization · tenancy"
        title="Organizations"
        sub="Group teams and projects under a shared tenancy. Members, roles, and policy are managed per organization."
        actions={
          <button type="button" className="cz-btn cz-btn-primary" onClick={() => setShowCreate(true)}>
            + New organization
          </button>
        }
      />

      <KpiStrip>
        <Kpi label="Organizations" value={loading ? '—' : orgs.length} hint="in this tenancy" />
        <Kpi label="Members" value={loading ? '—' : totalMembers} hint="across all orgs" />
        <Kpi label="Projects" value={loading ? '—' : totalProjects} hint="under management" />
        <Kpi label="Largest" value={loading ? '—' : largest} hint="members in one org" />
      </KpiStrip>

      {error && (
        <div className="cz-login-error" style={{ marginBottom: 18, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
            <AlertCircle size={15} /> {error}
          </span>
          <button type="button" className="cz-btn cz-btn-ghost" style={{ padding: '4px 10px', fontSize: 11 }} onClick={() => setError('')}>
            Dismiss
          </button>
        </div>
      )}

      {loading ? (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(min(100%, 340px), 1fr))', gap: 16 }}>
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-40 w-full rounded-lg" />
          ))}
        </div>
      ) : orgs.length === 0 ? (
        <EmptyState title="No organizations yet">
          Create your first organization to manage teams and projects together.
        </EmptyState>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(min(100%, 340px), 1fr))', gap: 16 }}>
          {orgs.map((org) => (
            <div key={org.id} className="cz-card cz-proj-card" style={{ cursor: 'default' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                <div style={avatarStyle}>{org.name.charAt(0).toUpperCase()}</div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className="cz-nm" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {org.name}
                  </div>
                  <div className="cz-faint" style={{ fontFamily: 'var(--cz-mono)', fontSize: 12 }}>/{org.slug}</div>
                </div>
              </div>

              <div className="cz-envs" style={{ marginTop: 4 }}>
                <span className="cz-chip">
                  <Users size={12} /> {memberCounts[org.id] ?? 0} member{(memberCounts[org.id] ?? 0) !== 1 ? 's' : ''}
                </span>
                <span className="cz-chip">
                  <FolderOpen size={12} /> {projectCounts[org.id] ?? 0} project{(projectCounts[org.id] ?? 0) !== 1 ? 's' : ''}
                </span>
              </div>

              <div className="cz-ft" style={{ gap: 8 }}>
                <button type="button" className="cz-btn" style={{ flex: 1, justifyContent: 'center', padding: '7px 12px', fontSize: 11 }} onClick={() => navigate(`/organizations/${org.id}`)}>
                  Manage
                </button>
                <button
                  type="button"
                  className="cz-btn cz-btn-danger"
                  style={{ padding: '7px 12px', fontSize: 11 }}
                  onClick={(e) => {
                    e.stopPropagation();
                    setDeleteTarget(org);
                  }}
                >
                  <Trash2 size={13} /> Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

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

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete Organization"
        description={deleteTarget ? `Are you sure you want to delete "${deleteTarget.name}"? This action cannot be undone.` : ''}
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleteTarget) handleDeleteOrg(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
    </Page>
  );
}
