package identity

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

type ProjectOwnership struct {
	ProjectID   uuid.UUID `json:"project_id"`
	Name        string    `json:"name"`
	Consequence string    `json:"consequence"`
}
type CredentialOwnership struct {
	ProjectID         uuid.UUID `json:"project_id"`
	SecretID          uuid.UUID `json:"secret_id"`
	Environment       string    `json:"environment"`
	Key               string    `json:"key"`
	LifecycleRevision int64     `json:"lifecycle_revision"`
	SecretRevision    int64     `json:"secret_revision"`
	Consequence       string    `json:"consequence"`
}

// A grant inherits the current authority of its actor, issuer, connection,
// workload and approved profile/package. Revoked parents can still have an
// active child run while their cancellation is being processed.
const offboardingGrantPredicate = `(g.actor_id=$2 OR g.issued_by=$2 OR g.revoked_at IS NOT NULL
 OR g.binding_id IN(SELECT b.id FROM tool_bindings b JOIN tool_connections c ON c.id=b.connection_id WHERE c.created_by=$2 OR c.revoked_at IS NOT NULL)
 OR g.workload_id IN(SELECT id FROM tool_workloads WHERE created_by=$2 OR revoked_at IS NOT NULL)
 OR g.profile_id IN(SELECT id FROM tool_profiles WHERE approved_by=$2)
 OR g.package_id IN(SELECT id FROM tool_profile_packages WHERE approved_by=$2))`

const offboardingGrantSelection = `SELECT g.id FROM tool_grants g JOIN projects p ON p.id=g.project_id WHERE p.organization_id=$1 AND ` + offboardingGrantPredicate

const offboardingRunSelection = `SELECT r.id FROM tool_runs r JOIN projects p ON p.id=r.project_id WHERE p.organization_id=$1 AND r.state IN ('preparing','active') AND (r.actor_id=$2 OR r.grant_id IN(` + offboardingGrantSelection + `))`

// This is one metadata-only snapshot. No credential hashes, provider
// credentials, proof material, secret values or encrypted payloads enter it.
// Offboard repeats the same snapshot after obtaining exclusive authority locks.
func offboardingImpact(ctx context.Context, tx *sql.Tx, org, target uuid.UUID) (string, error) {
	rows, e := tx.QueryContext(ctx, `SELECT kind,id,signature FROM (
 SELECT 'project' AS kind,p.id::text AS id,jsonb_build_object('owner',p.owner_id,'name',p.name,'deleted',p.deleted_at)::text AS signature FROM projects p WHERE p.organization_id=$1
 UNION ALL SELECT 'key',k.id::text,jsonb_build_object('project',k.project_id,'scopes',k.scopes,'environment',k.environment,'expires',k.expires_at)::text FROM api_keys k JOIN projects p ON p.id=k.project_id WHERE p.organization_id=$1 AND k.user_id=$2
 UNION ALL SELECT 'lease',l.id::text,jsonb_build_object('parent',l.api_key_id,'environment',l.environment,'keys',l.secret_keys,'expires',l.expires_at,'revoked',l.revoked)::text FROM secret_leases l JOIN api_keys k ON k.id=l.api_key_id JOIN projects p ON p.id=l.project_id WHERE p.organization_id=$1 AND k.user_id=$2
	 UNION ALL SELECT 'tool_grant',g.id::text,jsonb_build_object('actor',g.actor_id,'issuer',g.issued_by,'project',g.project_id,'profile',g.profile_id,'package',g.package_id,'binding',g.binding_id,'workload',g.workload_id,'expires',g.expires_at,'revoked',g.revoked_at,'member_epoch',g.membership_epoch,'issuer_epoch',g.issuer_epoch)::text FROM tool_grants g WHERE g.id IN(`+offboardingGrantSelection+`)
	 UNION ALL SELECT 'run',r.id::text,jsonb_build_object('grant',r.grant_id,'project',r.project_id,'actor',r.actor_id,'state',r.state,'expires',r.expires_at)::text FROM tool_runs r WHERE r.id IN(`+offboardingRunSelection+`)
	 UNION ALL SELECT 'profile_approval',f.id::text,jsonb_build_object('project',f.project_id,'digest',f.digest,'approver',f.approved_by,'epoch',f.approver_epoch,'approved_at',f.approved_at,'approved_until',f.approved_until,'revoked',f.revoked_at)::text FROM tool_profiles f JOIN projects p ON p.id=f.project_id WHERE p.organization_id=$1 AND f.approved_by=$2
	 UNION ALL SELECT 'package_approval',k.id::text,jsonb_build_object('profile',k.profile_id,'digest',k.digest,'approver',k.approved_by,'epoch',k.approver_epoch,'approved_at',k.approved_at,'approved_until',k.approved_until,'revoked',k.revoked_at)::text FROM tool_profile_packages k JOIN tool_profiles f ON f.id=k.profile_id JOIN projects p ON p.id=f.project_id WHERE p.organization_id=$1 AND k.approved_by=$2
 UNION ALL SELECT 'connection',c.id::text,jsonb_build_object('project',c.project_id,'revoked',c.revoked_at)::text FROM tool_connections c JOIN projects p ON p.id=c.project_id WHERE p.organization_id=$1 AND c.created_by=$2 AND c.revoked_at IS NULL
 UNION ALL SELECT 'workload',w.id::text,jsonb_build_object('project',w.project_id,'revoked',w.revoked_at)::text FROM tool_workloads w JOIN projects p ON p.id=w.project_id WHERE p.organization_id=$1 AND w.created_by=$2 AND w.revoked_at IS NULL
 UNION ALL SELECT 'lifecycle',l.secret_id::text,jsonb_build_object('project',l.project_id,'responsible',l.responsible_user_id,'lifecycle_revision',l.revision,'secret_revision',v.revision,'environment',v.environment_id,'key',v.secret_key,'deleted',v.deleted)::text FROM vault_lifecycle l JOIN vault_entries v ON v.secret_id=l.secret_id AND v.project_id=l.project_id JOIN projects p ON p.id=l.project_id WHERE p.organization_id=$1 AND l.responsible_user_id=$2
 UNION ALL SELECT 'approval',a.id::text,jsonb_build_object('project',a.project_id,'requester',a.requested_by,'approver',a.approved_by,'state',a.status)::text FROM promotion_requests a JOIN projects p ON p.id=a.project_id WHERE p.organization_id=$1 AND (a.requested_by=$2 OR a.approved_by=$2) AND a.status IN ('pending','approved')
 UNION ALL SELECT 'invitation',i.id::text,jsonb_build_object('target',i.target_user_id,'inviter',i.inviter_id,'role',i.intended_role,'expires',proof.expires_at,'proof_revoked',proof.revoked)::text FROM identity_invitations i JOIN identity_proofs proof ON proof.id=i.id WHERE i.organization_id=$1 AND NOT i.revoked AND i.accepted_by IS NULL AND (i.inviter_id=$2 OR i.target_user_id=$2 OR EXISTS(SELECT 1 FROM identity_verified_contacts c WHERE c.user_id=$2 AND c.contact=proof.contact AND c.revoked_at IS NULL))
 ) impact ORDER BY kind,id`, org, target)
	if e != nil {
		return "", e
	}
	defer rows.Close()
	values := [][3]string{}
	for rows.Next() {
		var row [3]string
		if e = rows.Scan(&row[0], &row[1], &row[2]); e != nil {
			return "", e
		}
		values = append(values, row)
	}
	if e = rows.Err(); e != nil {
		return "", e
	}
	encoded, e := json.Marshal(values)
	if e != nil {
		return "", e
	}
	return hashProof(string(encoded)), nil
}

func ownedProjects(ctx context.Context, tx *sql.Tx, org, target uuid.UUID) ([]ProjectOwnership, error) {
	result := []ProjectOwnership{}
	rows, e := tx.QueryContext(ctx, `SELECT id,name FROM projects WHERE organization_id=$1 AND owner_id=$2 AND deleted_at IS NULL ORDER BY id`, org, target)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		row := ProjectOwnership{Consequence: "Organization access is revoked. Stored project ownership remains and requires explicit reassignment by an administrator."}
		if e = rows.Scan(&row.ProjectID, &row.Name); e != nil {
			return nil, e
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func ownedCredentials(ctx context.Context, tx *sql.Tx, org, target uuid.UUID) ([]CredentialOwnership, error) {
	result := []CredentialOwnership{}
	rows, e := tx.QueryContext(ctx, `SELECT l.project_id,l.secret_id,e.name,v.secret_key,l.revision,v.revision FROM vault_lifecycle l JOIN vault_entries v ON v.secret_id=l.secret_id AND v.project_id=l.project_id JOIN environments e ON e.id=v.environment_id JOIN projects p ON p.id=l.project_id WHERE p.organization_id=$1 AND l.responsible_user_id=$2 AND NOT v.deleted AND p.deleted_at IS NULL ORDER BY l.project_id,l.secret_id`, org, target)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		row := CredentialOwnership{Consequence: "Responsibility remains assigned to the departing member and reminders stop for that member. An administrator must explicitly reassign responsibility."}
		if e = rows.Scan(&row.ProjectID, &row.SecretID, &row.Environment, &row.Key, &row.LifecycleRevision, &row.SecretRevision); e != nil {
			return nil, e
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// OffboardWithPreview is the production entry point. The older Offboard method
// remains a compatibility adapter for existing service fixtures only.
func (s *Service) OffboardWithPreview(ctx context.Context, p policy.Principal, org, target uuid.UUID, expected int64, key string, preview uuid.UUID) (Offboarding, error) {
	if preview == uuid.Nil {
		return Offboarding{}, ErrInvalid
	}
	return s.offboard(ctx, p, org, target, expected, key, preview)
}
