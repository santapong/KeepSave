package runs

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/automation"
	"github.com/santapong/KeepSave/backend/internal/mcpgateway/catalog"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"time"
)

type ConnectionMetadata struct {
	ID             uuid.UUID `json:"id"`
	ProjectID      uuid.UUID `json:"project_id"`
	AppID          int64     `json:"app_id"`
	InstallationID int64     `json:"installation_id"`
	Provider       string    `json:"provider"`
}
type PackageMetadata struct {
	ID        uuid.UUID `json:"id"`
	ProfileID uuid.UUID `json:"profile_id"`
	Harness   string    `json:"harness"`
	Version   string    `json:"version"`
	Digest    string    `json:"digest"`
	Approved  bool      `json:"approved"`
}
type Receipt struct {
	ID          uuid.UUID      `json:"id"`
	RunID       uuid.UUID      `json:"run_id"`
	OperationID uuid.NullUUID  `json:"operation_id"`
	Action      string         `json:"action"`
	Outcome     string         `json:"outcome"`
	Details     models.JSONMap `json:"details"`
}

func (s *Service) ListConnections(ctx context.Context, p policy.Principal, project uuid.UUID) ([]ConnectionMetadata, error) {
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id,project_id,app_id,installation_id FROM tool_connections WHERE project_id=$1 AND revoked_at IS NULL ORDER BY created_at,id`, project)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ConnectionMetadata{}
	for rows.Next() {
		var c ConnectionMetadata
		c.Provider = "github_app"
		if e = rows.Scan(&c.ID, &c.ProjectID, &c.AppID, &c.InstallationID); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Service) ListBindings(ctx context.Context, p policy.Principal, project uuid.UUID) ([]Binding, error) {
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id,project_id,connection_id,environment_id,repository_id,owner_name,repository_name,reference FROM tool_bindings WHERE project_id=$1 AND revoked_at IS NULL ORDER BY created_at,id`, project)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Binding{}
	for rows.Next() {
		var b Binding
		if e = rows.Scan(&b.ID, &b.ProjectID, &b.ConnectionID, &b.EnvironmentID, &b.Target.RepositoryID, &b.Target.Owner, &b.Target.Repository, &b.Target.Reference); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (s *Service) ListWorkloads(ctx context.Context, p policy.Principal, project uuid.UUID) ([]Workload, error) {
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id,project_id,certificate_sha256,image_digest FROM tool_workloads WHERE project_id=$1 AND revoked_at IS NULL ORDER BY created_at,id`, project)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Workload{}
	for rows.Next() {
		var w Workload
		if e = rows.Scan(&w.ID, &w.ProjectID, &w.CertificateSHA256, &w.ImageDigest); e != nil {
			return nil, e
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func (s *Service) ListGrants(ctx context.Context, p policy.Principal, project uuid.UUID) ([]Grant, error) {
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id,project_id,profile_id,package_id,binding_id,workload_id,actor_id,client_id,expires_at FROM tool_grants WHERE project_id=$1 AND revoked_at IS NULL ORDER BY created_at,id`, project)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Grant{}
	for rows.Next() {
		var g Grant
		if e = rows.Scan(&g.ID, &g.ProjectID, &g.ProfileID, &g.PackageID, &g.BindingID, &g.WorkloadID, &g.ActorID, &g.ClientID, &g.ExpiresAt); e != nil {
			return nil, e
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
func (s *Service) ListPackages(ctx context.Context, p policy.Principal, project uuid.UUID) ([]PackageMetadata, error) {
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT k.id,k.profile_id,k.harness,k.version,k.digest,`+approvalValidSQL("k", "pr")+` AND `+approvalValidSQL("p", "pr")+` FROM tool_profile_packages k JOIN tool_profiles p ON p.id=k.profile_id JOIN projects pr ON pr.id=p.project_id JOIN tool_artifacts ar ON ar.id=p.artifact_id WHERE p.project_id=$1 AND p.revoked_at IS NULL AND ar.revoked_at IS NULL AND k.revoked_at IS NULL ORDER BY k.id`, project)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []PackageMetadata{}
	for rows.Next() {
		var k PackageMetadata
		if e = rows.Scan(&k.ID, &k.ProfileID, &k.Harness, &k.Version, &k.Digest, &k.Approved); e != nil {
			return nil, e
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
func (s *Service) GetPackage(ctx context.Context, p policy.Principal, project, id uuid.UUID) (automation.Package, error) {
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return automation.Package{}, e
	}
	defer tx.Rollback()
	var b []byte
	var digest string
	e = tx.QueryRowContext(ctx, `SELECT k.content,k.digest FROM tool_profile_packages k JOIN tool_profiles p ON p.id=k.profile_id JOIN tool_artifacts a ON a.id=p.artifact_id WHERE k.id=$1 AND p.project_id=$2 AND p.revoked_at IS NULL AND a.revoked_at IS NULL AND k.revoked_at IS NULL`, id, project).Scan(&b, &digest)
	var pkg automation.Package
	if e != nil || json.Unmarshal(b, &pkg) != nil {
		return pkg, ErrDenied
	}
	canonical, _ := json.Marshal(pkg)
	if automation.Digest(canonical) != digest {
		return pkg, ErrDenied
	}
	return pkg, nil
}
func (s *Service) Receipts(ctx context.Context, p policy.Principal, client string, id uuid.UUID) ([]Receipt, error) {
	tx, e := s.beginRun(ctx, p, client, id, policy.InspectTeam)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, _, e = s.runTx(ctx, tx, p, client, id, false); e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,run_id,operation_id,action,outcome,details FROM tool_receipts WHERE run_id=$1 ORDER BY created_at,id LIMIT 1000`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Receipt{}
	for rows.Next() {
		var r Receipt
		var b []byte
		if e = rows.Scan(&r.ID, &r.RunID, &r.OperationID, &r.Action, &r.Outcome, &b); e != nil {
			return nil, e
		}
		if json.Unmarshal(b, &r.Details) != nil {
			return nil, ErrUnavailable
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Service) Catalog(ctx context.Context, p policy.Principal, project uuid.UUID) ([]catalog.Definition, error) {
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return nil, e
	}
	tx.Rollback()
	return catalog.Definitions(), nil
}
func (s *Service) CreateProjectRun(ctx context.Context, p policy.Principal, project uuid.UUID, client string, input CreateRunRequest) (Run, error) {
	var stored uuid.UUID
	if s == nil || !s.Enabled() {
		return Run{}, ErrUnavailable
	}
	if e := s.db.QueryRowContext(ctx, `SELECT project_id FROM tool_grants WHERE id=$1`, input.GrantID).Scan(&stored); e != nil || stored != project {
		return Run{}, ErrDenied
	}
	return s.CreateRunWithRequest(ctx, p, client, input)
}

func (s *Service) ListRuns(ctx context.Context, p policy.Principal, project uuid.UUID, client string) ([]Run, error) {
	if p.Kind != policy.Human {
		return nil, ErrDenied
	}
	tx, e := s.begin(ctx, p, project, policy.InspectTeam, "")
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT r.id,r.project_id,g.profile_id,g.binding_id,r.client_id,r.actor_id,COALESCE(r.commit_sha,''),r.expires_at,r.state,g.revoked_at IS NOT NULL,r.reference,r.scope FROM tool_runs r JOIN tool_grants g ON g.id=r.grant_id WHERE r.project_id=$1 AND r.actor_id=$2 AND($3='' OR r.client_id=$3) ORDER BY r.created_at DESC,r.id LIMIT 100`, project, p.ActorID, client)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var r Run
		var revoked bool
		var scope []byte
		if e = rows.Scan(&r.ID, &r.ProjectID, &r.ProfileID, &r.BindingID, &r.ClientID, &r.OwnerID, &r.Commit, &r.ExpiresAt, &r.State, &revoked, &r.Reference, &scope); e != nil {
			return nil, e
		}
		if _, e = readScope(&r, scope); e != nil {
			return nil, e
		}
		if revoked {
			r.State = "revoked"
		}
		if !r.ExpiresAt.After(time.Now()) {
			r.State = "expired"
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
