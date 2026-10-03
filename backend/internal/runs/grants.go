package runs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/automation"
	"github.com/santapong/KeepSave/backend/internal/broker"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"time"
)

type grantState struct {
	Grant
	Issuer                                     uuid.UUID
	ActorEpoch, IssuerEpoch                    int64
	ProfileDigest, PackageDigest               string
	ProfileApprover, PackageApprover           uuid.UUID
	ProfileApproverEpoch, PackageApproverEpoch int64
	Manifest                                   automation.Manifest
	Target                                     broker.Target
	ConnectionID                               uuid.UUID
	Connection                                 broker.Connection
	EnvironmentID                              uuid.UUID
	Environment                                string
	Image                                      string
}

func (s *Service) subjectsForGrant(ctx context.Context, id uuid.UUID) (uuid.UUID, []uuid.UUID, error) {
	var project, issuer, owner, profileApprover, packageApprover uuid.UUID
	e := s.db.QueryRowContext(ctx, `SELECT g.project_id,g.issued_by,g.actor_id,p.approved_by,k.approved_by FROM tool_grants g JOIN tool_profiles p ON p.id=g.profile_id JOIN tool_profile_packages k ON k.id=g.package_id WHERE g.id=$1`, id).Scan(&project, &issuer, &owner, &profileApprover, &packageApprover)
	if e != nil {
		return uuid.Nil, nil, ErrDenied
	}
	return project, []uuid.UUID{issuer, owner, profileApprover, packageApprover}, nil
}
func (s *Service) grantTx(ctx context.Context, tx *sql.Tx, id, user uuid.UUID, client string) (grantState, error) {
	var g grantState
	var manifest, content []byte
	var source, name, artifactDigest, harnessName string
	e := tx.QueryRowContext(ctx, `SELECT g.id,g.project_id,g.profile_id,g.package_id,g.binding_id,g.workload_id,g.actor_id,g.client_id,g.expires_at,g.issued_by,g.membership_epoch,g.issuer_epoch,g.profile_digest,g.package_digest,p.approved_by,p.approver_epoch,k.approved_by,k.approver_epoch,p.manifest,a.source,a.name,a.digest,k.content,cl.harness,b.repository_id,b.owner_name,b.repository_name,b.reference,env.id,env.name,c.id,c.app_id,c.installation_id,c.ciphertext,c.nonce,w.image_digest FROM tool_grants g JOIN tool_profiles p ON p.id=g.profile_id AND p.project_id=g.project_id AND p.digest=g.profile_digest JOIN tool_artifacts a ON a.id=p.artifact_id AND a.project_id=g.project_id JOIN tool_profile_packages k ON k.id=g.package_id AND k.profile_id=p.id AND k.digest=g.package_digest JOIN mcp_oauth_clients cl ON cl.client_id=g.client_id JOIN tool_bindings b ON b.id=g.binding_id AND b.project_id=g.project_id JOIN environments env ON env.id=b.environment_id AND env.project_id=g.project_id AND env.name IN('development','alpha','uat') JOIN tool_connections c ON c.id=b.connection_id AND c.project_id=g.project_id JOIN tool_workloads w ON w.id=g.workload_id AND w.project_id=g.project_id WHERE g.id=$1 AND g.actor_id=$2 AND g.client_id=$3 AND g.revoked_at IS NULL AND g.expires_at>NOW() AND p.approved_at IS NOT NULL AND p.approved_until>NOW() AND p.revoked_at IS NULL AND a.revoked_at IS NULL AND k.approved_at IS NOT NULL AND k.approved_until>NOW() AND k.revoked_at IS NULL AND b.revoked_at IS NULL AND c.revoked_at IS NULL AND w.revoked_at IS NULL AND cl.enabled FOR SHARE OF g,p,a,k,cl,b,env,c,w`, id, user, client).Scan(&g.ID, &g.ProjectID, &g.ProfileID, &g.PackageID, &g.BindingID, &g.WorkloadID, &g.ActorID, &g.ClientID, &g.ExpiresAt, &g.Issuer, &g.ActorEpoch, &g.IssuerEpoch, &g.ProfileDigest, &g.PackageDigest, &g.ProfileApprover, &g.ProfileApproverEpoch, &g.PackageApprover, &g.PackageApproverEpoch, &manifest, &source, &name, &artifactDigest, &content, &harnessName, &g.Target.RepositoryID, &g.Target.Owner, &g.Target.Repository, &g.Target.Reference, &g.EnvironmentID, &g.Environment, &g.ConnectionID, &g.Connection.AppID, &g.Connection.InstallationID, &g.Connection.Ciphertext, &g.Connection.Nonce, &g.Image)
	if e != nil {
		return g, ErrDenied
	}
	for _, subject := range []struct {
		id    uuid.UUID
		epoch int64
		cap   policy.Capability
	}{{g.ActorID, g.ActorEpoch, policy.StartApprovedRun}, {g.Issuer, g.IssuerEpoch, policy.ManageConnections}, {g.ProfileApprover, g.ProfileApproverEpoch, policy.ApproveProtected}, {g.PackageApprover, g.PackageApproverEpoch, policy.ApproveProtected}} {
		epoch, role, e := memberEpoch(ctx, tx, g.ProjectID, subject.id)
		if e != nil || epoch != subject.epoch || !policy.CapabilityAllows(role, subject.cap) {
			return g, ErrDenied
		}
	}
	if json.Unmarshal(manifest, &g.Manifest) != nil {
		return g, ErrDenied
	}
	canonical, e := g.Manifest.Canonical()
	if e != nil || automation.Digest(canonical) != g.ProfileDigest || g.Manifest.ArtifactDigest != artifactDigest || automation.Digest([]byte(source)) != artifactDigest || automation.ValidateSource(name, source) != nil {
		return g, ErrDenied
	}
	var pkg automation.Package
	if json.Unmarshal(content, &pkg) != nil {
		return g, ErrDenied
	}
	canonical, e = json.Marshal(pkg)
	if e != nil || automation.Digest(canonical) != g.PackageDigest || pkg.ProfileDigest != g.ProfileDigest || pkg.Harness != harnessName {
		return g, ErrDenied
	}
	expected, e := automation.NativePackage(pkg.Harness, pkg.Version, name, source, g.ProfileDigest, automation.PackageOptions{Endpoint: s.flags.Endpoint, ProfileID: g.ProfileID.String(), Revision: 1, Exporter: s.PackageExporter})
	if e != nil {
		return g, ErrDenied
	}
	expectedBytes, _ := json.Marshal(expected)
	if automation.Digest(expectedBytes) != g.PackageDigest {
		return g, ErrDenied
	}
	return g, nil
}
func (s *Service) IssueGrant(ctx context.Context, p policy.Principal, g Grant) (Grant, error) {
	if !s.Enabled() || !s.flags.Admission {
		return Grant{}, ErrUnavailable
	}
	if !validClient(g.ClientID) || g.ActorID == uuid.Nil || !g.ExpiresAt.After(time.Now()) || g.ExpiresAt.After(time.Now().Add(24*time.Hour)) {
		return Grant{}, ErrInvalid
	}
	var profileApprover, packageApprover uuid.UUID
	e := s.db.QueryRowContext(ctx, `SELECT p.approved_by,k.approved_by FROM tool_profiles p JOIN tool_profile_packages k ON k.profile_id=p.id JOIN mcp_oauth_clients cl ON cl.harness=k.harness WHERE p.id=$1 AND p.project_id=$2 AND k.id=$3 AND cl.client_id=$4 AND cl.enabled AND p.approved_at IS NOT NULL AND k.approved_at IS NOT NULL`, g.ProfileID, g.ProjectID, g.PackageID, g.ClientID).Scan(&profileApprover, &packageApprover)
	if e != nil {
		return Grant{}, ErrDenied
	}
	tx, e := s.begin(ctx, p, g.ProjectID, policy.ManageConnections, "", g.ActorID, profileApprover, packageApprover)
	if e != nil {
		return Grant{}, e
	}
	defer tx.Rollback()
	epoch, role, e := memberEpoch(ctx, tx, g.ProjectID, g.ActorID)
	if e != nil || !policy.CapabilityAllows(role, policy.StartApprovedRun) {
		return Grant{}, ErrDenied
	}
	issuerEpoch, _, e := memberEpoch(ctx, tx, g.ProjectID, p.ActorID)
	if e != nil {
		return Grant{}, e
	}
	var digest, pkgDigest string
	e = tx.QueryRowContext(ctx, `SELECT p.digest,k.digest FROM tool_profiles p JOIN tool_profile_packages k ON k.profile_id=p.id WHERE p.id=$1 AND k.id=$2 AND p.project_id=$3 AND p.revoked_at IS NULL AND k.revoked_at IS NULL AND p.approved_at IS NOT NULL AND k.approved_at IS NOT NULL`, g.ProfileID, g.PackageID, g.ProjectID).Scan(&digest, &pkgDigest)
	if e != nil {
		return Grant{}, ErrDenied
	}
	g.ExpiresAt = earliest(g.ExpiresAt, p.ExpiresAt)
	g.ID = uuid.New()
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_grants(id,project_id,profile_id,package_id,profile_digest,package_digest,binding_id,workload_id,actor_id,client_id,issued_by,membership_epoch,issuer_epoch,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, g.ID, g.ProjectID, g.ProfileID, g.PackageID, digest, pkgDigest, g.BindingID, g.WorkloadID, g.ActorID, g.ClientID, p.ActorID, epoch, issuerEpoch, g.ExpiresAt); e != nil {
		return Grant{}, ErrDenied
	}
	if _, e = s.grantTx(ctx, tx, g.ID, g.ActorID, g.ClientID); e != nil {
		return Grant{}, e
	}
	if e = s.event(ctx, tx, p, g.ProjectID, "tool.grant.issued", models.JSONMap{"grant_id": g.ID, "actor_id": g.ActorID, "client_id": g.ClientID}); e != nil {
		return Grant{}, e
	}
	return g, tx.Commit()
}

type runState struct {
	Run
	GrantID, SessionID uuid.UUID
	Kind               policy.Kind
	Family, Token      uuid.NullUUID
	AuthorityExpiry    time.Time
	Tree               string
	ScopeKnown         bool
	ProviderOps        int
	Bytes              int64
}

func readRun(ctx context.Context, tx *sql.Tx, id uuid.UUID, lock bool) (runState, error) {
	var r runState
	var sha, tree sql.NullString
	var scope []byte
	q := `SELECT r.id,r.project_id,g.profile_id,g.binding_id,r.client_id,r.actor_id,r.commit_sha,r.expires_at,r.state,r.grant_id,r.session_id,r.parent_kind,r.parent_family,r.parent_token,r.authority_expires_at,r.reference,r.tree_sha,r.provider_operations,r.result_bytes,r.scope FROM tool_runs r JOIN tool_grants g ON g.id=r.grant_id WHERE r.id=$1`
	if lock {
		q += ` FOR UPDATE OF r`
	}
	e := tx.QueryRowContext(ctx, q, id).Scan(&r.ID, &r.ProjectID, &r.ProfileID, &r.BindingID, &r.ClientID, &r.OwnerID, &sha, &r.ExpiresAt, &r.State, &r.GrantID, &r.SessionID, &r.Kind, &r.Family, &r.Token, &r.AuthorityExpiry, &r.Reference, &tree, &r.ProviderOps, &r.Bytes, &scope)
	if e == nil {
		r.ScopeKnown, e = readScope(&r.Run, scope)
	}
	r.Commit = sha.String
	r.Tree = tree.String
	return r, e
}
func principalForRun(r runState) policy.Principal {
	p := policy.Principal{Kind: r.Kind, SubjectID: r.OwnerID, ActorID: r.OwnerID, SessionID: r.SessionID, ExpiresAt: earliest(r.AuthorityExpiry, r.ExpiresAt)}
	if r.Family.Valid {
		p.ParentGrantID = r.Family.UUID
	}
	if r.Token.Valid {
		p.TokenID = r.Token.UUID.String()
	}
	return p
}
func (s *Service) discoverRun(ctx context.Context, id uuid.UUID) (uuid.UUID, []uuid.UUID, error) {
	var grant uuid.UUID
	if e := s.db.QueryRowContext(ctx, `SELECT grant_id FROM tool_runs WHERE id=$1`, id).Scan(&grant); e != nil {
		return uuid.Nil, nil, ErrDenied
	}
	return s.subjectsForGrant(ctx, grant)
}
func (s *Service) runTx(ctx context.Context, tx *sql.Tx, p policy.Principal, client string, id uuid.UUID, active bool) (runState, grantState, error) {
	r, e := readRun(ctx, tx, id, true)
	if e != nil || r.OwnerID != actor(p) || r.ClientID != client || p.Kind == policy.OAuthDelegation && r.SessionID != p.SessionID {
		return r, grantState{}, ErrDenied
	}
	if p.Kind == policy.OAuthDelegation && (!r.Family.Valid || r.Kind != policy.OAuthDelegation || r.Family.UUID != p.ParentGrantID) {
		return r, grantState{}, ErrDenied
	}
	if !active {
		return r, grantState{}, nil
	}
	if r.State != "active" || !r.ExpiresAt.After(time.Now()) || !r.AuthorityExpiry.After(time.Now()) {
		return r, grantState{}, ErrDenied
	}
	g, e := s.grantTx(ctx, tx, r.GrantID, r.OwnerID, client)
	if e != nil {
		return r, g, e
	}
	if !scopeMatchesGrant(r, g) {
		return r, g, ErrDenied
	}
	g.Target.Commit = r.Commit
	g.Target.TreeSHA = r.Tree
	return r, g, nil
}
func (s *Service) CreateRun(ctx context.Context, p policy.Principal, client string, grant uuid.UUID, key string) (Run, error) {
	return s.CreateRunWithRequest(ctx, p, client, CreateRunRequest{GrantID: grant, RequestKey: key})
}
func (s *Service) CreateRunWithRequest(ctx context.Context, p policy.Principal, client string, input CreateRunRequest) (Run, error) {
	if !s.Enabled() || !s.flags.Admission {
		return Run{}, ErrUnavailable
	}
	if !validClient(client) || !keyPattern.MatchString(input.RequestKey) || input.DurationSeconds < 0 || input.DurationSeconds > 600 {
		return Run{}, ErrInvalid
	}
	if input.DurationSeconds == 0 {
		input.DurationSeconds = 600
	}
	project, subjects, e := s.subjectsForGrant(ctx, input.GrantID)
	if e != nil {
		return Run{}, e
	}
	tx, e := s.begin(ctx, p, project, policy.StartApprovedRun, client, subjects...)
	if e != nil {
		return Run{}, e
	}
	defer tx.Rollback()
	if p.Kind == policy.Human {
		if input.FamilyID == uuid.Nil || s.FamilyPrincipalTx == nil {
			return Run{}, ErrDenied
		}
		delegated, err := s.FamilyPrincipalTx(ctx, tx, p, client, input.FamilyID)
		if err != nil || delegated.ActorID != p.ActorID {
			return Run{}, ErrDenied
		}
		p = delegated
	} else if input.FamilyID != uuid.Nil && input.FamilyID != p.ParentGrantID {
		return Run{}, ErrDenied
	}
	g, e := s.grantTx(ctx, tx, input.GrantID, p.ActorID, client)
	if e != nil {
		return Run{}, e
	}
	if input.Reference == "" {
		input.Reference = g.Target.Reference
	}
	if input.Reference != g.Target.Reference {
		return Run{}, ErrDenied
	}
	digestBytes, _ := json.Marshal(struct {
		GrantID   uuid.UUID
		Reference string
		Seconds   int
		Session   uuid.UUID
		Family    uuid.UUID
		Token     string
	}{input.GrantID, input.Reference, input.DurationSeconds, p.SessionID, p.ParentGrantID, p.TokenID})
	digest := automation.Digest(digestBytes)
	var existing uuid.UUID
	var old string
	e = tx.QueryRowContext(ctx, `SELECT id,request_digest FROM tool_runs WHERE actor_id=$1 AND client_id=$2 AND request_key=$3`, p.ActorID, client, input.RequestKey).Scan(&existing, &old)
	if e == nil {
		if old != digest {
			return Run{}, ErrConflict
		}
		tx.Rollback()
		return s.GetRun(ctx, p, client, existing)
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Run{}, e
	}
	var sessionExpiry time.Time
	if e = tx.QueryRowContext(ctx, `SELECT expires_at FROM session_tokens WHERE id=$1`, p.SessionID).Scan(&sessionExpiry); e != nil {
		return Run{}, ErrDenied
	}
	end := earliest(time.Now().Add(time.Duration(input.DurationSeconds)*time.Second), time.Now().Add(time.Duration(g.Manifest.MaxSeconds)*time.Second), g.ExpiresAt, p.ExpiresAt, sessionExpiry)
	id := uuid.New()
	scope, _ := json.Marshal(scopeForGrant(g))
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_runs(id,project_id,grant_id,actor_id,session_id,client_id,parent_kind,parent_family,parent_token,authority_expires_at,expires_at,reference,request_key,request_digest,scope) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11,$12,$13,$14)`, id, project, input.GrantID, p.ActorID, p.SessionID, client, p.Kind, familyID(p), tokenID(p), end, input.Reference, input.RequestKey, digest, scope); e != nil {
		return Run{}, ErrConflict
	}
	attempt := uuid.New()
	deadline := earliest(end, time.Now().Add(30*time.Second))
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_resolution_attempts(id,run_id,state,deadline) VALUES($1,$2,'admitted',$3)`, attempt, id, deadline); e != nil {
		return Run{}, e
	}
	if _, e = s.receipt(ctx, tx, p, project, id, uuid.Nil, "tool.run.preparing", "admitted", models.JSONMap{"resolution_id": attempt, "grant_id": g.ID, "repository_id": g.Target.RepositoryID}); e != nil {
		return Run{}, e
	}
	if e = tx.Commit(); e != nil {
		return Run{}, e
	}
	resolveCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	resolved, e := s.broker.ResolveAuthorized(resolveCtx, g.Connection, g.Target, func(c context.Context, stage string) error {
		return s.resolutionAdmission(c, p, client, id, attempt, stage)
	})
	if e != nil {
		outcome := "failed"
		if errors.Is(e, broker.ErrUncertain) {
			outcome = "uncertain"
		}
		s.finishResolution(context.WithoutCancel(ctx), p, client, id, attempt, broker.Target{}, outcome)
		return Run{}, ErrDenied
	}
	if e = s.finishResolution(ctx, p, client, id, attempt, resolved, "succeeded"); e != nil {
		return Run{}, e
	}
	return s.GetRun(ctx, p, client, id)
}
func (s *Service) resolutionAdmission(ctx context.Context, p policy.Principal, client string, id, attempt uuid.UUID, stage string) error {
	project, subjects, e := s.discoverRun(ctx, id)
	if e != nil {
		return e
	}
	tx, e := s.begin(ctx, p, project, policy.StartApprovedRun, client, subjects...)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := readRun(ctx, tx, id, true)
	if e != nil || r.State != "preparing" || r.OwnerID != p.ActorID || r.ClientID != client || p.Kind == policy.OAuthDelegation && r.SessionID != p.SessionID || !r.ExpiresAt.After(time.Now()) {
		return ErrDenied
	}
	g, e := s.grantTx(ctx, tx, r.GrantID, p.ActorID, client)
	if e != nil {
		return e
	}
	if !scopeMatchesGrant(r, g) {
		return ErrDenied
	}
	var deadline time.Time
	var state string
	if e = tx.QueryRowContext(ctx, `SELECT state,deadline FROM tool_resolution_attempts WHERE id=$1 AND run_id=$2 FOR UPDATE`, attempt, id).Scan(&state, &deadline); e != nil || state != "admitted" && state != "dispatched" || !deadline.After(time.Now()) {
		return ErrDenied
	}
	if stage == "provider_request" {
		if r.ProviderOps >= g.Manifest.MaxProviderOperations {
			return ErrDenied
		}
		if _, e = tx.ExecContext(ctx, `UPDATE tool_runs SET provider_operations=provider_operations+1 WHERE id=$1`, id); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE tool_resolution_attempts SET state='dispatched' WHERE id=$1`, attempt); e != nil {
			return e
		}
	}
	if _, e = s.receipt(ctx, tx, p, project, id, uuid.Nil, "tool.resolution.admitted", stage, models.JSONMap{"resolution_id": attempt}); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) finishResolution(ctx context.Context, p policy.Principal, client string, id, attempt uuid.UUID, target broker.Target, outcome string) error {
	project, subjects, e := s.discoverRun(ctx, id)
	if e != nil {
		return e
	}
	tx, e := s.begin(ctx, p, project, policy.StartApprovedRun, client, subjects...)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := readRun(ctx, tx, id, true)
	if e != nil || r.State != "preparing" || r.OwnerID != p.ActorID || r.ClientID != client || p.Kind == policy.OAuthDelegation && r.SessionID != p.SessionID {
		return ErrDenied
	}
	g, e := s.grantTx(ctx, tx, r.GrantID, p.ActorID, client)
	if e != nil {
		return e
	}
	if !scopeMatchesGrant(r, g) || outcome == "succeeded" && (target.RepositoryID != r.RepositoryID || target.Owner+"/"+target.Repository != r.Repository || target.Reference != r.Reference) {
		return ErrDenied
	}
	var attemptState string
	var resolutionDeadline time.Time
	if e = tx.QueryRowContext(ctx, `SELECT state,deadline FROM tool_resolution_attempts WHERE id=$1 AND run_id=$2 FOR UPDATE`, attempt, id).Scan(&attemptState, &resolutionDeadline); e != nil || attemptState != "admitted" && attemptState != "dispatched" {
		return ErrDenied
	}
	if outcome == "succeeded" && !resolutionDeadline.After(time.Now()) {
		return ErrDenied
	}
	if outcome == "succeeded" {
		if !broker.ValidTarget(target) || !r.ExpiresAt.After(time.Now()) {
			return ErrDenied
		}
		if _, e = tx.ExecContext(ctx, `UPDATE tool_runs SET state='active',commit_sha=$2,tree_sha=$3 WHERE id=$1`, id, target.Commit, target.TreeSHA); e != nil {
			return e
		}
	} else {
		if _, e = tx.ExecContext(ctx, `UPDATE tool_runs SET state='failed' WHERE id=$1`, id); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE tool_resolution_attempts SET state=$2,outcome=$2,finished_at=NOW() WHERE id=$1 AND run_id=$3`, attempt, outcome, id); e != nil {
		return e
	}
	if _, e = s.receipt(ctx, tx, p, project, id, uuid.Nil, "tool.run.resolved", outcome, models.JSONMap{"resolution_id": attempt, "commit": target.Commit}); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) GetRun(ctx context.Context, p policy.Principal, client string, id uuid.UUID) (Run, error) {
	tx, e := s.beginRun(ctx, p, client, id, policy.InspectTeam)
	if e != nil {
		return Run{}, e
	}
	defer tx.Rollback()
	r, _, e := s.runTx(ctx, tx, p, client, id, false)
	if e != nil {
		return Run{}, e
	}
	if !r.ExpiresAt.After(time.Now()) {
		r.State = "expired"
	}
	return r.Run, nil
}
func (s *Service) AvailableRuns(ctx context.Context, p policy.Principal, client string) ([]Run, error) {
	if !s.Enabled() {
		return nil, ErrUnavailable
	}
	if actor(p) == uuid.Nil || !validClient(client) {
		return nil, ErrDenied
	}
	rows, e := s.db.QueryContext(ctx, `SELECT id FROM tool_runs WHERE actor_id=$1 AND session_id=$2 AND client_id=$3 AND state='active' AND expires_at>NOW() ORDER BY created_at DESC LIMIT 100`, p.ActorID, p.SessionID, client)
	if e != nil {
		return nil, ErrUnavailable
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, ErrUnavailable
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, ErrUnavailable
	}
	out := []Run{}
	for _, id := range ids {
		_, _, e := s.discoverRun(ctx, id)
		if e != nil {
			continue
		}
		tx, e := s.beginRun(ctx, p, client, id, policy.StartApprovedRun)
		if e != nil {
			continue
		}
		r, _, e := s.runTx(ctx, tx, p, client, id, true)
		tx.Rollback()
		if e == nil {
			out = append(out, r.Run)
		}
	}
	return out, nil
}
