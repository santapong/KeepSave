import { useState, useEffect, useCallback, type FormEvent } from 'react';
import type { Organization, OrgMember, Project } from '../types';
import * as api from '../api/client';

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
  const [hoveredCard, setHoveredCard] = useState<string | null>(null);
  const [memberCounts, setMemberCounts] = useState<Record<string, number>>({});
  const [projectCounts, setProjectCounts] = useState<Record<string, number>>({});

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
      await loadOrgs();
      selectOrg(org);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create organization');
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
      loadOrgs();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete organization');
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
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to add member';
      if (message.toLowerCase().includes('not found') || message.toLowerCase().includes('no user')) {
        setMemberError('No user found with this email');
      } else {
        setMemberError(message);
      }
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
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update role');
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
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove member');
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
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to assign project');
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
      <div style={loadingContainer}>
        <div style={loadingSpinner} />
        <p style={{ fontSize: 14, color: 'var(--color-text-secondary)', marginTop: 16 }}>
          Loading organizations...
        </p>
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div style={headerRow}>
        <div>
          <h1 style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-text)', margin: 0 }}>
            Organizations
          </h1>
          <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginTop: 4 }}>
            {orgs.length} organization{orgs.length !== 1 ? 's' : ''}
          </p>
        </div>
        <button onClick={() => setShowCreate(!showCreate)} style={btnPrimary}>
          {showCreate ? 'Cancel' : '+ New Organization'}
        </button>
      </div>

      {/* Error alert */}
      {error && (
        <div style={errorBanner}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" style={{ flexShrink: 0 }}>
              <circle cx="8" cy="8" r="7" stroke="var(--color-danger)" strokeWidth="1.5" />
              <path d="M8 4.5v4M8 10.5v.5" stroke="var(--color-danger)" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
            <span>{error}</span>
          </div>
          <button onClick={() => setError('')} style={dismissBtn}>
            Dismiss
          </button>
        </div>
      )}

      {/* Create form (collapsible card) */}
      {showCreate && (
        <form onSubmit={handleCreateOrg} style={createFormCard}>
          <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 16, color: 'var(--color-text)' }}>
            Create Organization
          </h3>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
            <input
              placeholder="Organization name"
              value={newOrgName}
              onChange={(e) => setNewOrgName(e.target.value)}
              required
              autoFocus
              style={inputStyle}
            />
            <button type="submit" style={btnPrimary}>
              Create
            </button>
          </div>
        </form>
      )}

      {/* Empty state */}
      {orgs.length === 0 && !showCreate ? (
        <div style={emptyStateCard}>
          <div style={emptyIconCircle}>
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-secondary)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
              <path d="M16 3.13a4 4 0 0 1 0 7.75" />
            </svg>
          </div>
          <p style={{ fontSize: 17, fontWeight: 600, color: 'var(--color-text)', marginBottom: 6 }}>
            No organizations yet
          </p>
          <p style={{ fontSize: 14, color: 'var(--color-text-secondary)', maxWidth: 340, lineHeight: 1.5 }}>
            Create your first organization to manage teams and projects together.
          </p>
        </div>
      ) : (
        <>
          {/* Org grid */}
          <div style={orgGrid}>
            {orgs.map((org) => {
              const isSelected = selectedOrg?.id === org.id;
              const isHovered = hoveredCard === org.id;
              return (
                <div
                  key={org.id}
                  onMouseEnter={() => setHoveredCard(org.id)}
                  onMouseLeave={() => setHoveredCard(null)}
                  style={{
                    ...orgCard,
                    border: isSelected
                      ? '1.5px solid var(--color-primary)'
                      : '1px solid var(--color-border)',
                    boxShadow: isSelected
                      ? '0 0 0 3px var(--color-primary-glow), var(--shadow)'
                      : isHovered
                        ? 'var(--shadow-lg)'
                        : 'var(--shadow)',
                    transform: isHovered && !isSelected ? 'translateY(-1px)' : 'none',
                    borderColor: isSelected
                      ? 'var(--color-primary)'
                      : isHovered
                        ? '#cbd5e1'
                        : 'var(--color-border)',
                  }}
                >
                  {/* Card top: icon + name + slug */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                    <div style={orgIconCircle}>
                      {org.name.charAt(0).toUpperCase()}
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--color-text)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {org.name}
                      </div>
                      <div style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 2 }}>
                        /{org.slug}
                      </div>
                    </div>
                  </div>

                  {/* Card middle: badges */}
                  <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
                    <span style={badgePill}>
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
                        <circle cx="9" cy="7" r="4" />
                      </svg>
                      {memberCounts[org.id] ?? 0} member{(memberCounts[org.id] ?? 0) !== 1 ? 's' : ''}
                    </span>
                    <span style={badgePill}>
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
                      </svg>
                      {projectCounts[org.id] ?? 0} project{(projectCounts[org.id] ?? 0) !== 1 ? 's' : ''}
                    </span>
                  </div>

                  {/* Card bottom: actions */}
                  <div style={{ display: 'flex', gap: 10, marginTop: 18, paddingTop: 16, borderTop: '1px solid var(--color-border)' }}>
                    <button
                      onClick={(e) => handleManageClick(org, e)}
                      style={{
                        ...btnOutlinePrimary,
                        background: isSelected ? 'var(--color-primary-glow)' : 'transparent',
                      }}
                    >
                      {isSelected ? 'Close' : 'Manage'}
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); handleDeleteOrg(org.id); }}
                      style={btnOutlineDanger}
                    >
                      Delete
                    </button>
                  </div>
                </div>
              );
            })}
          </div>

          {/* Management panel (below grid, full width) */}
          {selectedOrg && (
            <div style={managementPanel}>
              <div style={managementPanelHeader}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <div style={managePanelIcon}>
                    {selectedOrg.name.charAt(0).toUpperCase()}
                  </div>
                  <div>
                    <h2 style={{ fontSize: 18, fontWeight: 700, color: 'var(--color-text)', margin: 0 }}>
                      {selectedOrg.name}
                    </h2>
                    <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 2 }}>
                      /{selectedOrg.slug}
                    </p>
                  </div>
                </div>
              </div>

              <div style={twoColumnPanel}>
                {/* Left column: Members */}
                <div style={{ ...panelSection, borderRight: '1px solid var(--color-border)' }}>
                  <div style={panelSectionHeader}>
                    <h3 style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text)', margin: 0 }}>
                      Members
                    </h3>
                    {members.length > 0 && (
                      <span style={countBadge}>{members.length}</span>
                    )}
                  </div>

                  {/* Add member form */}
                  <form onSubmit={handleAddMember} style={addMemberRow}>
                    <input
                      type="email"
                      placeholder="Email address"
                      value={newMemberEmail}
                      onChange={(e) => { setNewMemberEmail(e.target.value); setMemberError(''); }}
                      required
                      style={{ ...inputStyle, flex: 1, minWidth: 160 }}
                    />
                    <select
                      value={newMemberRole}
                      onChange={(e) => setNewMemberRole(e.target.value)}
                      style={selectStyle}
                    >
                      <option value="viewer">Viewer</option>
                      <option value="editor">Editor</option>
                      <option value="promoter">Promoter</option>
                      <option value="admin">Admin</option>
                    </select>
                    <button type="submit" disabled={addingMember} style={{
                      ...btnPrimary,
                      opacity: addingMember ? 0.6 : 1,
                      cursor: addingMember ? 'not-allowed' : 'pointer',
                    }}>
                      {addingMember ? 'Adding...' : 'Add'}
                    </button>
                  </form>

                  {memberError && (
                    <div style={memberErrorBox}>
                      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" style={{ flexShrink: 0 }}>
                        <circle cx="8" cy="8" r="7" stroke="var(--color-danger)" strokeWidth="1.5" />
                        <path d="M8 4.5v4M8 10.5v.5" stroke="var(--color-danger)" strokeWidth="1.5" strokeLinecap="round" />
                      </svg>
                      {memberError}
                    </div>
                  )}

                  {/* Member list as cards */}
                  {members.length === 0 ? (
                    <div style={emptyStateSmall}>
                      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-secondary)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ opacity: 0.5 }}>
                        <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
                        <circle cx="9" cy="7" r="4" />
                        <line x1="23" y1="11" x2="17" y2="11" />
                      </svg>
                      <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginTop: 8 }}>
                        No members yet. Add team members by email above.
                      </p>
                    </div>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                      {members.map((m) => (
                        <div key={m.id} style={memberCard}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flex: 1, minWidth: 0 }}>
                            <div style={memberAvatar}>
                              {m.user_id.charAt(0).toUpperCase()}
                            </div>
                            <div style={{ flex: 1, minWidth: 0 }}>
                              <code style={userIdCode} title={m.user_id}>
                                {m.user_id.slice(0, 8)}...
                              </code>
                            </div>
                          </div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                            <select
                              value={m.role}
                              onChange={(e) => handleUpdateRole(m.user_id, e.target.value)}
                              style={{
                                ...roleSelectStyle,
                                color: roleColors[m.role] || 'var(--color-text)',
                                borderColor: roleColors[m.role] ? `${roleColors[m.role]}33` : 'var(--color-border)',
                                background: roleColors[m.role] ? `${roleColors[m.role]}0d` : 'var(--color-input-bg)',
                              }}
                            >
                              <option value="viewer">Viewer</option>
                              <option value="editor">Editor</option>
                              <option value="promoter">Promoter</option>
                              <option value="admin">Admin</option>
                            </select>
                            <button
                              onClick={() => handleRemoveMember(m.user_id)}
                              style={btnRemove}
                              title="Remove member"
                            >
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <line x1="18" y1="6" x2="6" y2="18" />
                                <line x1="6" y1="6" x2="18" y2="18" />
                              </svg>
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Right column: Projects */}
                <div style={panelSection}>
                  <div style={panelSectionHeader}>
                    <h3 style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text)', margin: 0 }}>
                      Projects
                    </h3>
                    {orgProjects.length > 0 && (
                      <span style={countBadge}>{orgProjects.length}</span>
                    )}
                  </div>

                  {/* Assign project */}
                  {unassignedProjects.length > 0 && (
                    <div style={assignProjectRow}>
                      <select
                        value={assignProjectId}
                        onChange={(e) => setAssignProjectId(e.target.value)}
                        style={{ ...selectStyle, flex: 1 }}
                      >
                        <option value="">Select a project to assign...</option>
                        {unassignedProjects.map((p) => (
                          <option key={p.id} value={p.id}>{p.name}</option>
                        ))}
                      </select>
                      <button
                        onClick={handleAssignProject}
                        disabled={!assignProjectId}
                        style={{
                          ...btnPrimary,
                          opacity: assignProjectId ? 1 : 0.5,
                          cursor: assignProjectId ? 'pointer' : 'not-allowed',
                        }}
                      >
                        Assign
                      </button>
                    </div>
                  )}

                  {/* Project list as cards */}
                  {orgProjects.length === 0 ? (
                    <div style={emptyStateSmall}>
                      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-secondary)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ opacity: 0.5 }}>
                        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
                      </svg>
                      <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginTop: 8 }}>
                        No projects assigned yet.
                      </p>
                    </div>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                      {orgProjects.map((p) => (
                        <div key={p.id} style={projectCard}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                            <div style={projectIconCircle}>
                              {p.name.charAt(0).toUpperCase()}
                            </div>
                            <div style={{ flex: 1, minWidth: 0 }}>
                              <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text)' }}>
                                {p.name}
                              </div>
                              {p.description && (
                                <div style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 3, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
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
            </div>
          )}
        </>
      )}
    </div>
  );
}

/* -- Role color map --------------------------------------------------- */

const roleColors: Record<string, string> = {
  admin: '#dc2626',
  promoter: '#d97706',
  editor: '#3b82f6',
  viewer: '#64748b',
};

/* -- Style constants -------------------------------------------------- */

const headerRow: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  marginBottom: 24,
};

const btnPrimary: React.CSSProperties = {
  padding: '9px 18px',
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 600,
  cursor: 'pointer',
  transition: 'background 0.15s ease',
  whiteSpace: 'nowrap',
};

const btnOutlinePrimary: React.CSSProperties = {
  padding: '7px 16px',
  background: 'transparent',
  color: 'var(--color-primary)',
  border: '1px solid var(--color-primary)',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 600,
  cursor: 'pointer',
  transition: 'all 0.15s ease',
  flex: 1,
};

const btnOutlineDanger: React.CSSProperties = {
  padding: '7px 16px',
  background: 'transparent',
  color: 'var(--color-danger)',
  border: '1px solid rgba(220, 38, 38, 0.25)',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  fontWeight: 500,
  cursor: 'pointer',
  transition: 'all 0.15s ease',
};

const btnRemove: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: 30,
  height: 30,
  background: 'transparent',
  color: 'var(--color-danger)',
  border: '1px solid rgba(220, 38, 38, 0.2)',
  borderRadius: 'var(--radius)',
  cursor: 'pointer',
  transition: 'all 0.15s ease',
  flexShrink: 0,
};

const dismissBtn: React.CSSProperties = {
  background: 'none',
  border: 'none',
  color: 'var(--color-danger)',
  cursor: 'pointer',
  fontWeight: 600,
  fontSize: 12,
  padding: '2px 8px',
  flexShrink: 0,
};

const errorBanner: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  background: 'rgba(220, 38, 38, 0.05)',
  color: 'var(--color-danger)',
  padding: '10px 14px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 20,
  border: '1px solid rgba(220, 38, 38, 0.15)',
};

const createFormCard: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: 24,
  marginBottom: 24,
  boxShadow: 'var(--shadow)',
};

const orgGrid: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))',
  gap: 16,
};

const orgCard: React.CSSProperties = {
  padding: 20,
  borderRadius: 'var(--radius)',
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  boxShadow: 'var(--shadow)',
  transition: 'all 0.2s ease',
  cursor: 'default',
};

const orgIconCircle: React.CSSProperties = {
  width: 44,
  height: 44,
  borderRadius: '50%',
  background: 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)',
  color: '#ffffff',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontWeight: 700,
  fontSize: 18,
  flexShrink: 0,
  letterSpacing: '-0.02em',
};

const badgePill: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: 5,
  padding: '4px 10px',
  fontSize: 12,
  fontWeight: 500,
  color: 'var(--color-text-secondary)',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 20,
};

const managementPanel: React.CSSProperties = {
  marginTop: 24,
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow-lg)',
  overflow: 'hidden',
};

const managementPanelHeader: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  padding: '20px 24px',
  borderBottom: '1px solid var(--color-border)',
  background: 'var(--color-input-bg)',
};

const managePanelIcon: React.CSSProperties = {
  width: 36,
  height: 36,
  borderRadius: '50%',
  background: 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)',
  color: '#ffffff',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontWeight: 700,
  fontSize: 15,
  flexShrink: 0,
};

const twoColumnPanel: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: '1fr 1fr',
  gap: 0,
};

const panelSection: React.CSSProperties = {
  padding: 24,
};

const panelSectionHeader: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  marginBottom: 16,
};

const countBadge: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  minWidth: 22,
  height: 22,
  padding: '0 7px',
  fontSize: 11,
  fontWeight: 700,
  color: 'var(--color-primary)',
  background: 'var(--color-primary-glow)',
  borderRadius: 20,
};

const addMemberRow: React.CSSProperties = {
  display: 'flex',
  gap: 8,
  marginBottom: 16,
  flexWrap: 'wrap',
};

const assignProjectRow: React.CSSProperties = {
  display: 'flex',
  gap: 8,
  marginBottom: 16,
};

const inputStyle: React.CSSProperties = {
  padding: '9px 12px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 14,
  color: 'var(--color-text)',
  outline: 'none',
  transition: 'border-color 0.15s ease',
};

const selectStyle: React.CSSProperties = {
  padding: '9px 12px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  color: 'var(--color-text)',
  cursor: 'pointer',
};

const roleSelectStyle: React.CSSProperties = {
  padding: '5px 10px',
  border: '1px solid var(--color-border)',
  borderRadius: 20,
  fontSize: 12,
  fontWeight: 600,
  cursor: 'pointer',
  outline: 'none',
  appearance: 'none' as const,
  WebkitAppearance: 'none' as const,
  paddingRight: 10,
};

const memberCard: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: '10px 14px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  gap: 12,
};

const memberAvatar: React.CSSProperties = {
  width: 28,
  height: 28,
  borderRadius: '50%',
  background: 'var(--color-border)',
  color: 'var(--color-text-secondary)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontWeight: 600,
  fontSize: 12,
  flexShrink: 0,
};

const userIdCode: React.CSSProperties = {
  fontSize: 12,
  fontFamily: "'SF Mono', 'Fira Code', 'Fira Mono', 'Roboto Mono', monospace",
  color: 'var(--color-text-secondary)',
  letterSpacing: '0.01em',
};

const projectCard: React.CSSProperties = {
  padding: '12px 14px',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  transition: 'border-color 0.15s ease',
};

const projectIconCircle: React.CSSProperties = {
  width: 32,
  height: 32,
  borderRadius: 8,
  background: 'linear-gradient(135deg, #6366f1 0%, #818cf8 100%)',
  color: '#ffffff',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontWeight: 700,
  fontSize: 14,
  flexShrink: 0,
};

const emptyStateCard: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  textAlign: 'center',
  padding: '64px 40px',
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  boxShadow: 'var(--shadow)',
};

const emptyIconCircle: React.CSSProperties = {
  width: 64,
  height: 64,
  borderRadius: '50%',
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  margin: '0 auto 20px',
};

const emptyStateSmall: React.CSSProperties = {
  textAlign: 'center',
  padding: 28,
  border: '1px dashed var(--color-border)',
  borderRadius: 'var(--radius)',
  background: 'rgba(248, 250, 252, 0.5)',
};

const memberErrorBox: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  background: 'rgba(220, 38, 38, 0.05)',
  color: 'var(--color-danger)',
  padding: '8px 12px',
  borderRadius: 'var(--radius)',
  fontSize: 13,
  marginBottom: 16,
  border: '1px solid rgba(220, 38, 38, 0.15)',
};

const loadingContainer: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  padding: '80px 40px',
};

const loadingSpinner: React.CSSProperties = {
  width: 32,
  height: 32,
  border: '3px solid var(--color-border)',
  borderTopColor: 'var(--color-primary)',
  borderRadius: '50%',
  animation: 'spin 0.8s linear infinite',
};
