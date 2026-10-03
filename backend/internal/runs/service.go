// Package runs owns authenticated-client admission, durable attempts and result
// publication. A run ID, harness name or skill is never bearer authority.
package runs

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/automation"
	"github.com/santapong/KeepSave/backend/internal/broker"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/runner"
	"regexp"
	"time"
)

var ErrDenied = errors.New("tool access denied")
var ErrUnavailable = errors.New("tool platform unavailable")
var ErrInvalid = errors.New("invalid tool request")
var ErrConflict = errors.New("tool revision conflict")
var ErrNoWork = errors.New("no tool work ready")

type AuthorizeTx func(context.Context, *sql.Tx, policy.Principal, policy.Action, policy.Resource) (policy.Decision, error)
type ProjectLocker func(context.Context, *sql.Tx, policy.Principal, uuid.UUID, bool, []uuid.UUID) error
type GrantChecker func(context.Context, *sql.Tx, policy.Principal, string) error
type Auditor interface {
	CreateTx(*sql.Tx, *uuid.UUID, *uuid.UUID, string, string, models.JSONMap, string) error
}
type Flags struct {
	Enabled, Admission, Dispatch bool
	Endpoint                     string
}
type MaintenanceLocker func(context.Context, *sql.Tx, uuid.UUID, []uuid.UUID) error
type FamilyResolver func(context.Context, *sql.Tx, policy.Principal, string, uuid.UUID) (policy.Principal, error)
type Service struct {
	db                      *sql.DB
	broker                  *broker.Service
	authorize               AuthorizeTx
	audit                   Auditor
	flags                   Flags
	LockProjectSubjects     ProjectLocker
	RequireGrantTx          GrantChecker
	FamilyPrincipalTx       FamilyResolver
	LockMaintenanceSubjects MaintenanceLocker
	PackageExporter         automation.NativeExporter
}

func New(db *sql.DB, b *broker.Service, a AuthorizeTx, log Auditor, f Flags) *Service {
	return &Service{db: db, broker: b, authorize: a, audit: log, flags: f}
}
func (s *Service) Enabled() bool {
	return s.Ready() && s.flags.Enabled
}

type Arguments = runner.Arguments
type Operation struct {
	ID              uuid.UUID       `json:"operation_id"`
	RunID           uuid.UUID       `json:"run_id"`
	Status          string          `json:"status"`
	Outcome         string          `json:"outcome,omitempty"`
	Result          json.RawMessage `json:"result,omitempty"`
	ExpiresAt       time.Time       `json:"expires_at"`
	ReceiptID       uuid.UUID       `json:"receipt_id,omitempty"`
	CancelRequested bool            `json:"cancel_requested"`
}
type Run struct {
	ID             uuid.UUID `json:"id"`
	ProjectID      uuid.UUID `json:"project_id"`
	ProfileID      uuid.UUID `json:"profile_id"`
	BindingID      uuid.UUID `json:"binding_id"`
	ClientID       string    `json:"client_id"`
	OwnerID        uuid.UUID `json:"owner_id"`
	RepositoryID   int64     `json:"repository_id"`
	Repository     string    `json:"repository"`
	EnvironmentID  uuid.UUID `json:"environment_id"`
	Environment    string    `json:"environment"`
	Reference      string    `json:"reference"`
	InstallationID int64     `json:"installation_id"`
	Commit         string    `json:"commit"`
	ExpiresAt      time.Time `json:"expires_at"`
	State          string    `json:"state"`
}
type Grant struct {
	ID         uuid.UUID `json:"id"`
	ProjectID  uuid.UUID `json:"project_id"`
	ProfileID  uuid.UUID `json:"profile_id"`
	PackageID  uuid.UUID `json:"package_id"`
	BindingID  uuid.UUID `json:"binding_id"`
	WorkloadID uuid.UUID `json:"workload_id"`
	ActorID    uuid.UUID `json:"actor_id"`
	ClientID   string    `json:"client_id"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type Artifact struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Name      string    `json:"name"`
	Digest    string    `json:"digest"`
}
type Profile struct {
	ID         uuid.UUID           `json:"id"`
	ProjectID  uuid.UUID           `json:"project_id"`
	ArtifactID uuid.UUID           `json:"artifact_id"`
	Digest     string              `json:"digest"`
	Manifest   automation.Manifest `json:"manifest"`
	Approved   bool                `json:"approved"`
}
type Binding struct {
	EnvironmentID uuid.UUID     `json:"environment_id"`
	ID            uuid.UUID     `json:"id"`
	ProjectID     uuid.UUID     `json:"project_id"`
	ConnectionID  uuid.UUID     `json:"connection_id"`
	Target        broker.Target `json:"target"`
}
type Workload struct {
	ID                uuid.UUID `json:"id"`
	ProjectID         uuid.UUID `json:"project_id"`
	CertificateSHA256 string    `json:"certificate_sha256"`
	ImageDigest       string    `json:"image_digest"`
}
type Ticket = runner.Ticket
type CreateRunRequest struct {
	FamilyID        uuid.UUID `json:"family_id"`
	GrantID         uuid.UUID `json:"grant_id"`
	RequestKey      string    `json:"request_key"`
	Reference       string    `json:"reference,omitempty"`
	DurationSeconds int       `json:"duration_seconds,omitempty"`
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validClient(c string) bool { return keyPattern.MatchString(c) }
func actor(p policy.Principal) uuid.UUID {
	if (p.Kind == policy.Human || p.Kind == policy.OAuthDelegation) && p.ActorID == p.SubjectID && p.SubjectID != uuid.Nil {
		return p.SubjectID
	}
	return uuid.Nil
}
func (s *Service) begin(ctx context.Context, p policy.Principal, project uuid.UUID, cap policy.Capability, client string, extra ...uuid.UUID) (*sql.Tx, error) {
	return s.beginMode(ctx, p, project, cap, client, false, extra...)
}
func (s *Service) beginControl(ctx context.Context, p policy.Principal, project uuid.UUID, cap policy.Capability, extra ...uuid.UUID) (*sql.Tx, error) {
	return s.beginMode(ctx, p, project, cap, "", true, extra...)
}
func (s *Service) beginMode(ctx context.Context, p policy.Principal, project uuid.UUID, cap policy.Capability, client string, control bool, extra ...uuid.UUID) (*sql.Tx, error) {
	if !s.Ready() || !s.flags.Enabled && cap != policy.InspectTeam && !control {
		return nil, ErrUnavailable
	}
	management := cap != policy.StartApprovedRun && cap != policy.InspectTeam
	if actor(p) == uuid.Nil || p.SessionID == uuid.Nil || !p.ExpiresAt.IsZero() && !p.ExpiresAt.After(time.Now()) || management && p.Kind != policy.Human {
		return nil, ErrDenied
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, ErrUnavailable
	}
	fail := func(e error) (*sql.Tx, error) { tx.Rollback(); return nil, e }
	if _, e = tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s';SET LOCAL statement_timeout='10s'`); e != nil {
		return fail(ErrUnavailable)
	}
	if e = s.LockProjectSubjects(ctx, tx, p, project, management, extra); e != nil {
		return fail(ErrDenied)
	}
	d, e := s.authorize(ctx, tx, p, policy.ReadMetadata, policy.Resource{ProjectID: project, Type: "tool_platform"})
	if e != nil || !d.Allowed {
		return fail(ErrDenied)
	}
	_, role, e := memberEpoch(ctx, tx, project, p.ActorID)
	if e != nil || !policy.CapabilityAllows(role, cap) {
		return fail(ErrDenied)
	}
	if p.Kind == policy.OAuthDelegation {
		if s.RequireGrantTx == nil || client == "" {
			return fail(ErrDenied)
		}
		if e = s.RequireGrantTx(ctx, tx, p, client); e != nil {
			return fail(ErrDenied)
		}
	}
	return tx, nil
}
func memberEpoch(ctx context.Context, tx *sql.Tx, project, user uuid.UUID) (int64, string, error) {
	var org uuid.NullUUID
	var owner uuid.UUID
	if e := tx.QueryRowContext(ctx, `SELECT organization_id,owner_id FROM projects WHERE id=$1 AND deleted_at IS NULL`, project).Scan(&org, &owner); e != nil {
		return 0, "", ErrDenied
	}
	if !org.Valid {
		if owner != user {
			return 0, "", ErrDenied
		}
		return 0, "admin", nil
	}
	var epoch int64
	var active bool
	var role string
	if e := tx.QueryRowContext(ctx, `SELECT a.epoch,a.active,m.role FROM member_authority_state a JOIN organization_members m ON m.organization_id=a.organization_id AND m.user_id=a.user_id WHERE a.organization_id=$1 AND a.user_id=$2`, org.UUID, user).Scan(&epoch, &active, &role); e != nil || !active || epoch < 1 {
		return 0, "", ErrDenied
	}
	return epoch, role, nil
}
func (s *Service) event(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, action string, details models.JSONMap) error {
	if e := s.audit.CreateTx(tx, &p.ActorID, &project, action, "", details, ""); e != nil {
		return e
	}
	b, e := json.Marshal(map[string]any{"project_id": project, "action": action})
	if e != nil {
		return e
	}
	return jobs.EnqueueTx(ctx, tx, uuid.New(), "tool.event", b, "local", 3)
}
func randomToken() (string, error) {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
func hash(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func tokenID(p policy.Principal) any {
	if p.Kind != policy.OAuthDelegation {
		return nil
	}
	id, e := uuid.Parse(p.TokenID)
	if e != nil {
		return nil
	}
	return id
}
func familyID(p policy.Principal) any {
	if p.Kind != policy.OAuthDelegation {
		return nil
	}
	return p.ParentGrantID
}
func earliest(ts ...time.Time) time.Time {
	var out time.Time
	for _, v := range ts {
		if !v.IsZero() && (out.IsZero() || v.Before(out)) {
			out = v
		}
	}
	return out
}
func (s *Service) receipt(ctx context.Context, tx *sql.Tx, p policy.Principal, project, run, op uuid.UUID, action, outcome string, details models.JSONMap) (uuid.UUID, error) {
	id := uuid.New()
	b, e := json.Marshal(details)
	if e != nil {
		return uuid.Nil, e
	}
	var operation any
	if op != uuid.Nil {
		operation = op
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_receipts(id,run_id,operation_id,action,outcome,details) VALUES($1,$2,$3,$4,$5,$6)`, id, run, operation, action, outcome, b); e != nil {
		return uuid.Nil, e
	}
	details["receipt_id"] = id
	details["run_id"] = run
	if op != uuid.Nil {
		details["operation_id"] = op
	}
	if e = s.event(ctx, tx, p, project, action, details); e != nil {
		return uuid.Nil, e
	}
	return id, nil
}

func (s *Service) Ready() bool {
	return s != nil && s.db != nil && s.broker != nil && s.authorize != nil && s.audit != nil && s.LockProjectSubjects != nil
}
