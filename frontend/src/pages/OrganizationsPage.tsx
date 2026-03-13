import { useState, useEffect } from 'react';
import type { Organization, OrgMember } from '../types';
import * as api from '../api/client';

export function OrganizationsPage() {
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<Organization | null>(null);
  const [members, setMembers] = useState<OrgMember[]>([]);
  const [newOrgName, setNewOrgName] = useState('');
  const [newMemberUserId, setNewMemberUserId] = useState('');
  const [newMemberRole, setNewMemberRole] = useState('viewer');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    loadOrgs();
  }, []);

  async function loadOrgs() {
    try {
      setLoading(true);
      const data = await api.listOrganizations();
      setOrgs(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load organizations');
    } finally {
      setLoading(false);
    }
  }

  async function handleCreateOrg(e: React.FormEvent) {
    e.preventDefault();
    if (!newOrgName.trim()) return;
    try {
      await api.createOrganization(newOrgName.trim());
      setNewOrgName('');
      loadOrgs();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create organization');
    }
  }

  async function handleDeleteOrg(orgId: string) {
    if (!confirm('Delete this organization? This cannot be undone.')) return;
    try {
      await api.deleteOrganization(orgId);
      if (selectedOrg?.id === orgId) {
        setSelectedOrg(null);
        setMembers([]);
      }
      loadOrgs();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete organization');
    }
  }

  async function selectOrg(org: Organization) {
    setSelectedOrg(org);
    try {
      const data = await api.listOrgMembers(org.id);
      setMembers(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load members');
    }
  }

  async function handleAddMember(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedOrg || !newMemberUserId.trim()) return;
    try {
      await api.addOrgMember(selectedOrg.id, newMemberUserId.trim(), newMemberRole);
      setNewMemberUserId('');
      const data = await api.listOrgMembers(selectedOrg.id);
      setMembers(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add member');
    }
  }

  async function handleUpdateRole(userId: string, role: string) {
    if (!selectedOrg) return;
    try {
      await api.updateOrgMemberRole(selectedOrg.id, userId, role);
      const data = await api.listOrgMembers(selectedOrg.id);
      setMembers(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update role');
    }
  }

  async function handleRemoveMember(userId: string) {
    if (!selectedOrg) return;
    if (!confirm('Remove this member?')) return;
    try {
      await api.removeOrgMember(selectedOrg.id, userId);
      const data = await api.listOrgMembers(selectedOrg.id);
      setMembers(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove member');
    }
  }

  const roleColors: Record<string, string> = {
    admin: '#dc2626',
    promoter: '#d97706',
    editor: '#2563eb',
    viewer: '#6b7280',
  };

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: 24 }}>
      <h1 style={{ marginBottom: 24 }}>Organizations</h1>

      {error && (
        <div style={{ padding: 12, background: '#fef2f2', color: '#dc2626', borderRadius: 8, marginBottom: 16 }}>
          {error}
          <button onClick={() => setError('')} style={{ float: 'right', background: 'none', border: 'none', cursor: 'pointer' }}>x</button>
        </div>
      )}

      <form onSubmit={handleCreateOrg} style={{ display: 'flex', gap: 8, marginBottom: 24 }}>
        <input
          type="text"
          value={newOrgName}
          onChange={(e) => setNewOrgName(e.target.value)}
          placeholder="New organization name"
          style={{ flex: 1, padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius)' }}
        />
        <button
          type="submit"
          style={{ padding: '8px 16px', background: 'var(--color-primary)', color: '#fff', border: 'none', borderRadius: 'var(--radius)', cursor: 'pointer' }}
        >
          Create
        </button>
      </form>

      {loading ? (
        <p>Loading...</p>
      ) : (
        <div style={{ display: 'flex', gap: 24 }}>
          <div style={{ flex: 1 }}>
            <h3 style={{ marginBottom: 12 }}>Your Organizations</h3>
            {orgs.length === 0 ? (
              <p style={{ color: 'var(--color-text-secondary)' }}>No organizations yet.</p>
            ) : (
              orgs.map((org) => (
                <div
                  key={org.id}
                  onClick={() => selectOrg(org)}
                  style={{
                    padding: 16,
                    border: selectedOrg?.id === org.id ? '2px solid var(--color-primary)' : '1px solid var(--color-border)',
                    borderRadius: 'var(--radius)',
                    marginBottom: 8,
                    cursor: 'pointer',
                    background: 'var(--color-surface)',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div>
                      <strong>{org.name}</strong>
                      <div style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>/{org.slug}</div>
                    </div>
                    <button
                      onClick={(e) => { e.stopPropagation(); handleDeleteOrg(org.id); }}
                      style={{ padding: '4px 8px', background: 'var(--color-danger)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}
                    >
                      Delete
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>

          {selectedOrg && (
            <div style={{ flex: 1 }}>
              <h3 style={{ marginBottom: 12 }}>Members - {selectedOrg.name}</h3>

              <form onSubmit={handleAddMember} style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
                <input
                  type="text"
                  value={newMemberUserId}
                  onChange={(e) => setNewMemberUserId(e.target.value)}
                  placeholder="User ID"
                  style={{ flex: 1, padding: '6px 10px', border: '1px solid var(--color-border)', borderRadius: 4 }}
                />
                <select
                  value={newMemberRole}
                  onChange={(e) => setNewMemberRole(e.target.value)}
                  style={{ padding: '6px 10px', border: '1px solid var(--color-border)', borderRadius: 4 }}
                >
                  <option value="viewer">Viewer</option>
                  <option value="editor">Editor</option>
                  <option value="promoter">Promoter</option>
                  <option value="admin">Admin</option>
                </select>
                <button
                  type="submit"
                  style={{ padding: '6px 12px', background: 'var(--color-primary)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}
                >
                  Add
                </button>
              </form>

              {members.length === 0 ? (
                <p style={{ color: 'var(--color-text-secondary)' }}>No members.</p>
              ) : (
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr style={{ borderBottom: '2px solid var(--color-border)' }}>
                      <th style={{ textAlign: 'left', padding: 8 }}>User ID</th>
                      <th style={{ textAlign: 'left', padding: 8 }}>Role</th>
                      <th style={{ padding: 8 }}>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {members.map((m) => (
                      <tr key={m.id} style={{ borderBottom: '1px solid var(--color-border)' }}>
                        <td style={{ padding: 8, fontSize: 12, fontFamily: 'monospace' }}>{m.user_id}</td>
                        <td style={{ padding: 8 }}>
                          <select
                            value={m.role}
                            onChange={(e) => handleUpdateRole(m.user_id, e.target.value)}
                            style={{ padding: '4px 8px', border: '1px solid var(--color-border)', borderRadius: 4, color: roleColors[m.role] || '#000' }}
                          >
                            <option value="viewer">Viewer</option>
                            <option value="editor">Editor</option>
                            <option value="promoter">Promoter</option>
                            <option value="admin">Admin</option>
                          </select>
                        </td>
                        <td style={{ padding: 8, textAlign: 'center' }}>
                          <button
                            onClick={() => handleRemoveMember(m.user_id)}
                            style={{ padding: '4px 8px', background: 'var(--color-danger)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}
                          >
                            Remove
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
