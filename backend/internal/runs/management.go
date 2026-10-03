package runs

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/automation"
	"github.com/santapong/KeepSave/backend/internal/broker"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/runner"
	"time"
)

func (s *Service) CreateArtifact(ctx context.Context, p policy.Principal, project uuid.UUID, name, source string) (Artifact, error) {
	if automation.ValidateSource(name, source) != nil {
		return Artifact{}, ErrInvalid
	}
	tx, e := s.begin(ctx, p, project, policy.ManageProfiles, "")
	if e != nil {
		return Artifact{}, e
	}
	defer tx.Rollback()
	a := Artifact{uuid.New(), project, name, automation.Digest([]byte(source))}
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_artifacts(id,project_id,name,digest,source,created_by) VALUES($1,$2,$3,$4,$5,$6)`, a.ID, project, name, a.Digest, source, p.ActorID); e != nil {
		return Artifact{}, ErrConflict
	}
	if e = s.event(ctx, tx, p, project, "tool.artifact.created", models.JSONMap{"artifact_id": a.ID, "digest": a.Digest}); e != nil {
		return Artifact{}, e
	}
	return a, tx.Commit()
}
func (s *Service) CreateProfile(ctx context.Context, p policy.Principal, project, artifact uuid.UUID) (Profile, error) {
	tx, e := s.begin(ctx, p, project, policy.ManageProfiles, "")
	if e != nil {
		return Profile{}, e
	}
	defer tx.Rollback()
	var digest, name, source string
	if e = tx.QueryRowContext(ctx, `SELECT digest,name,source FROM tool_artifacts WHERE id=$1 AND project_id=$2 AND revoked_at IS NULL FOR SHARE`, artifact, project).Scan(&digest, &name, &source); e != nil || automation.ValidateSource(name, source) != nil || automation.Digest([]byte(source)) != digest {
		return Profile{}, ErrDenied
	}
	m := automation.NewManifest(digest)
	b, _ := m.Canonical()
	v := Profile{uuid.New(), project, artifact, automation.Digest(b), m, false}
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_profiles(id,project_id,artifact_id,digest,manifest,created_by) VALUES($1,$2,$3,$4,$5,$6)`, v.ID, project, artifact, v.Digest, b, p.ActorID); e != nil {
		return Profile{}, e
	}
	if e = s.event(ctx, tx, p, project, "tool.profile.created", models.JSONMap{"profile_id": v.ID, "digest": v.Digest}); e != nil {
		return Profile{}, e
	}
	return v, tx.Commit()
}
func (s *Service) ApproveProfile(ctx context.Context, p policy.Principal, project, id uuid.UUID, digest string) error {
	if !digestPattern.MatchString(digest) {
		return ErrInvalid
	}
	tx, e := s.begin(ctx, p, project, policy.ApproveProtected, "")
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var created uuid.UUID
	var stored, artifactDigest, name, source string
	var manifest []byte
	e = tx.QueryRowContext(ctx, `SELECT p.digest,p.created_by,p.manifest,a.digest,a.name,a.source FROM tool_profiles p JOIN tool_artifacts a ON a.id=p.artifact_id WHERE p.id=$1 AND p.project_id=$2 AND p.revoked_at IS NULL AND a.revoked_at IS NULL FOR UPDATE OF p`, id, project).Scan(&stored, &created, &manifest, &artifactDigest, &name, &source)
	if e != nil || stored != digest || created == p.ActorID {
		return ErrDenied
	}
	var m automation.Manifest
	if json.Unmarshal(manifest, &m) != nil {
		return ErrDenied
	}
	b, e := m.Canonical()
	if e != nil || automation.Digest(b) != digest || m.ArtifactDigest != artifactDigest || automation.Digest([]byte(source)) != artifactDigest || automation.ValidateSource(name, source) != nil {
		return ErrDenied
	}
	epoch, _, e := memberEpoch(ctx, tx, project, p.ActorID)
	if e != nil {
		return e
	}
	approved, e := tx.ExecContext(ctx, `UPDATE tool_profiles SET approved_by=$1,approver_epoch=$2,approved_at=NOW(),approved_until=LEAST(NOW()+INTERVAL '24 hours',$4) WHERE id=$3 AND approved_at IS NULL`, p.ActorID, epoch, id, approvalExpiry(p))
	if e != nil {
		return e
	}
	n, e := approved.RowsAffected()
	if e != nil || n != 1 {
		return ErrConflict
	}
	if e = s.event(ctx, tx, p, project, "tool.profile.approved", models.JSONMap{"profile_id": id, "digest": digest}); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) Package(ctx context.Context, p policy.Principal, project, profile uuid.UUID, client, version string) (uuid.UUID, automation.Package, error) {
	tx, e := s.begin(ctx, p, project, policy.ManageProfiles, "")
	if e != nil {
		return uuid.Nil, automation.Package{}, e
	}
	defer tx.Rollback()
	var name, source, digest string
	if e = tx.QueryRowContext(ctx, `SELECT a.name,a.source,p.digest FROM tool_profiles p JOIN tool_artifacts a ON a.id=p.artifact_id AND a.project_id=p.project_id WHERE p.id=$1 AND p.project_id=$2 AND p.revoked_at IS NULL AND a.revoked_at IS NULL`, profile, project).Scan(&name, &source, &digest); e != nil {
		return uuid.Nil, automation.Package{}, ErrDenied
	}
	pkg, e := automation.NativePackage(client, version, name, source, digest, automation.PackageOptions{Endpoint: s.flags.Endpoint, ProfileID: profile.String(), Revision: 1, Exporter: s.PackageExporter})
	if e != nil {
		return uuid.Nil, pkg, ErrInvalid
	}
	b, _ := json.Marshal(pkg)
	compat, _ := json.Marshal(pkg.Compatibility)
	id := uuid.New()
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_profile_packages(id,profile_id,harness,version,format,digest,content,compatibility,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, profile, client, version, pkg.Format, automation.Digest(b), b, compat, p.ActorID); e != nil {
		return uuid.Nil, pkg, ErrConflict
	}
	if e = s.event(ctx, tx, p, project, "tool.package.created", models.JSONMap{"package_id": id, "digest": automation.Digest(b)}); e != nil {
		return uuid.Nil, pkg, e
	}
	return id, pkg, tx.Commit()
}
func (s *Service) ApprovePackage(ctx context.Context, p policy.Principal, project, id uuid.UUID, digest string) error {
	tx, e := s.begin(ctx, p, project, policy.ApproveProtected, "")
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var created uuid.UUID
	var content []byte
	var stored string
	e = tx.QueryRowContext(ctx, `SELECT k.created_by,k.digest,k.content FROM tool_profile_packages k JOIN tool_profiles p ON p.id=k.profile_id WHERE k.id=$1 AND p.project_id=$2 AND k.revoked_at IS NULL AND p.approved_at IS NOT NULL AND p.revoked_at IS NULL FOR UPDATE OF k`, id, project).Scan(&created, &stored, &content)
	var pkg automation.Package
	_ = json.Unmarshal(content, &pkg)
	canonical, e2 := json.Marshal(pkg)
	if e != nil || created == p.ActorID || stored != digest || e2 != nil || automation.Digest(canonical) != digest {
		return ErrDenied
	}
	epoch, _, e := memberEpoch(ctx, tx, project, p.ActorID)
	if e != nil {
		return e
	}
	approved, e := tx.ExecContext(ctx, `UPDATE tool_profile_packages SET approved_by=$1,approver_epoch=$2,approved_at=NOW(),approved_until=LEAST(NOW()+INTERVAL '24 hours',$4) WHERE id=$3 AND approved_at IS NULL`, p.ActorID, epoch, id, approvalExpiry(p))
	if e != nil {
		return e
	}
	n, e := approved.RowsAffected()
	if e != nil || n != 1 {
		return ErrConflict
	}
	if e = s.event(ctx, tx, p, project, "tool.package.approved", models.JSONMap{"package_id": id, "digest": digest}); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) CreateConnection(ctx context.Context, p policy.Principal, project uuid.UUID, app, installation int64, key []byte) (uuid.UUID, error) {
	if app < 1 || installation < 1 || broker.ValidateKey(key) != nil {
		return uuid.Nil, ErrInvalid
	}
	tx, e := s.begin(ctx, p, project, policy.ManageConnections, "")
	if e != nil {
		return uuid.Nil, e
	}
	defer tx.Rollback()
	cipher, nonce, e := s.broker.Seal(key)
	if e != nil {
		return uuid.Nil, e
	}
	id := uuid.New()
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_connections(id,project_id,app_id,installation_id,ciphertext,nonce,created_by) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, project, app, installation, cipher, nonce, p.ActorID); e != nil {
		return uuid.Nil, e
	}
	if e = s.event(ctx, tx, p, project, "tool.connection.created", models.JSONMap{"connection_id": id, "provider": "github_app"}); e != nil {
		return uuid.Nil, e
	}
	return id, tx.Commit()
}
func connection(ctx context.Context, tx *sql.Tx, id, project uuid.UUID) (broker.Connection, error) {
	var c broker.Connection
	e := tx.QueryRowContext(ctx, `SELECT app_id,installation_id,ciphertext,nonce FROM tool_connections WHERE id=$1 AND project_id=$2 AND revoked_at IS NULL FOR SHARE`, id, project).Scan(&c.AppID, &c.InstallationID, &c.Ciphertext, &c.Nonce)
	return c, e
}

// Binding stores an approved structured target. Provider validation happens only
// after durable, audited run preparation; this metadata call performs no reads.
func (s *Service) CreateBinding(ctx context.Context, p policy.Principal, project, conn uuid.UUID, t broker.Target) (Binding, error) {
	return s.CreateBindingInEnvironment(ctx, p, project, conn, "development", t)
}
func (s *Service) CreateBindingInEnvironment(ctx context.Context, p policy.Principal, project, conn uuid.UUID, environment string, t broker.Target) (Binding, error) {
	if environment != "development" && environment != "alpha" && environment != "uat" {
		return Binding{}, ErrDenied
	}
	if !broker.ValidTargetSpec(t) {
		return Binding{}, ErrInvalid
	}
	if t.Reference == "" {
		t.Reference = t.Commit
	}
	t.Commit = ""
	t.TreeSHA = ""
	tx, e := s.begin(ctx, p, project, policy.ManageConnections, "")
	if e != nil {
		return Binding{}, e
	}
	defer tx.Rollback()
	if _, e = connection(ctx, tx, conn, project); e != nil {
		return Binding{}, ErrDenied
	}
	var envID uuid.UUID
	if e = tx.QueryRowContext(ctx, `SELECT id FROM environments WHERE project_id=$1 AND name=$2 FOR SHARE`, project, environment).Scan(&envID); e != nil {
		return Binding{}, ErrDenied
	}
	b := Binding{EnvironmentID: envID, ID: uuid.New(), ProjectID: project, ConnectionID: conn, Target: t}
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_bindings(id,project_id,connection_id,environment_id,repository_id,owner_name,repository_name,reference) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, b.ID, project, conn, envID, t.RepositoryID, t.Owner, t.Repository, t.Reference); e != nil {
		return Binding{}, e
	}
	if e = s.event(ctx, tx, p, project, "tool.binding.created", models.JSONMap{"binding_id": b.ID, "repository_id": t.RepositoryID, "external_read": false}); e != nil {
		return Binding{}, e
	}
	return b, tx.Commit()
}
func (s *Service) EnrollWorkload(ctx context.Context, p policy.Principal, project uuid.UUID, fingerprint, image string) (Workload, error) {
	if !digestPattern.MatchString(fingerprint) || !runner.ValidImage(image) {
		return Workload{}, ErrInvalid
	}
	tx, e := s.begin(ctx, p, project, policy.ManageWorkloads, "")
	if e != nil {
		return Workload{}, e
	}
	defer tx.Rollback()
	w := Workload{uuid.New(), project, fingerprint, image}
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_workloads(id,project_id,certificate_sha256,image_digest,created_by) VALUES($1,$2,$3,$4,$5)`, w.ID, project, fingerprint, image, p.ActorID); e != nil {
		return Workload{}, e
	}
	if e = s.event(ctx, tx, p, project, "tool.workload.enrolled", models.JSONMap{"workload_id": w.ID, "image_digest": image}); e != nil {
		return Workload{}, e
	}
	return w, tx.Commit()
}
func (s *Service) ListArtifacts(ctx context.Context, p policy.Principal, project uuid.UUID) ([]Artifact, error) {
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id,project_id,name,digest FROM tool_artifacts WHERE project_id=$1 AND revoked_at IS NULL ORDER BY created_at,id`, project)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Artifact{}
	for rows.Next() {
		var a Artifact
		if e = rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Digest); e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Service) ListProfiles(ctx context.Context, p policy.Principal, project uuid.UUID) ([]Profile, error) {
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT p.id,p.project_id,p.artifact_id,p.digest,p.manifest,`+approvalValidSQL("p", "pr")+` FROM tool_profiles p JOIN projects pr ON pr.id=p.project_id JOIN tool_artifacts ar ON ar.id=p.artifact_id WHERE p.project_id=$1 AND p.revoked_at IS NULL AND ar.revoked_at IS NULL ORDER BY p.created_at,p.id`, project)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		var a Profile
		var b []byte
		if e = rows.Scan(&a.ID, &a.ProjectID, &a.ArtifactID, &a.Digest, &b, &a.Approved); e != nil {
			return nil, e
		}
		if json.Unmarshal(b, &a.Manifest) != nil {
			return nil, ErrUnavailable
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Service) Revoke(ctx context.Context, p policy.Principal, project, id uuid.UUID, kind string) error {
	tables := map[string]string{"artifact": "tool_artifacts", "profile": "tool_profiles", "connection": "tool_connections", "binding": "tool_bindings", "workload": "tool_workloads", "grant": "tool_grants"}
	table, ok := tables[kind]
	if !ok {
		return ErrInvalid
	}
	cap := policy.ManageProfiles
	if kind == "connection" || kind == "binding" {
		cap = policy.ManageConnections
	}
	if kind == "workload" {
		cap = policy.ManageWorkloads
	}
	tx, e := s.beginControl(ctx, p, project, cap)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	res, e := tx.ExecContext(ctx, `UPDATE `+table+` SET revoked_at=COALESCE(revoked_at,NOW()) WHERE id=$1 AND project_id=$2`, id, project)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e != nil || n != 1 {
		return ErrDenied
	}
	if e = revokeAffectedRunsTx(ctx, tx, project, id, kind); e != nil {
		return e
	}
	if e = s.event(ctx, tx, p, project, "tool."+kind+".revoked", models.JSONMap{"resource_id": id}); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) RevokePackage(ctx context.Context, p policy.Principal, project, id uuid.UUID) error {
	tx, e := s.beginControl(ctx, p, project, policy.ManageProfiles)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := tx.ExecContext(ctx, `UPDATE tool_profile_packages SET revoked_at=COALESCE(revoked_at,NOW()) WHERE id=$1 AND profile_id IN(SELECT id FROM tool_profiles WHERE project_id=$2)`, id, project)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrDenied
	}
	if e = revokeAffectedRunsTx(ctx, tx, project, id, "package"); e != nil {
		return e
	}
	if e = s.event(ctx, tx, p, project, "tool.package.revoked", models.JSONMap{"package_id": id}); e != nil {
		return e
	}
	return tx.Commit()
}

func approvalExpiry(p policy.Principal) time.Time {
	return earliest(time.Now().Add(24*time.Hour), p.ExpiresAt)
}

// Approval metadata describes currently usable approvals, not a stale event.
func approvalValidSQL(alias, project string) string {
	return `(` + alias + `.approved_at IS NOT NULL AND ` + alias + `.approved_until>NOW() AND CASE WHEN ` + project + `.organization_id IS NULL THEN ` + alias + `.approved_by=` + project + `.owner_id ELSE EXISTS(SELECT 1 FROM member_authority_state st JOIN organization_members m ON m.organization_id=st.organization_id AND m.user_id=st.user_id WHERE st.organization_id=` + project + `.organization_id AND st.user_id=` + alias + `.approved_by AND st.active AND st.epoch=` + alias + `.approver_epoch AND m.role IN('admin','promoter')) END)`
}
