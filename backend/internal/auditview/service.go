// Package auditview owns authorized, safe projections of the immutable journal.
package auditview

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

var ErrDenied = errors.New("resource unavailable")
var ErrInvalid = errors.New("invalid audit request")

type AuthorizeTx func(context.Context, *sql.Tx, policy.Principal, policy.Action, policy.Resource) (policy.Decision, error)
type Auditor interface {
	CreateTx(*sql.Tx, *uuid.UUID, *uuid.UUID, string, string, models.JSONMap, string) error
}
type Service struct {
	db        *sql.DB
	authorize AuthorizeTx
	audit     Auditor
}

func New(db *sql.DB, authorize AuthorizeTx, audit Auditor) *Service {
	return &Service{db, authorize, audit}
}

type Entry struct {
	ID          uuid.UUID         `json:"id"`
	ActorID     *uuid.UUID        `json:"actor_id,omitempty"`
	ProjectID   uuid.UUID         `json:"project_id"`
	Action      string            `json:"action"`
	Environment string            `json:"environment,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	References  map[string]string `json:"references"`
}
type Filter struct {
	Action      string
	Environment string
	From, To    time.Time
	Cursor      string
	Limit       int
}
type Page struct {
	Entries    []Entry `json:"entries"`
	NextCursor string  `json:"next_cursor,omitempty"`
}
type cursor struct {
	CreatedAt           time.Time `json:"t"`
	ID                  uuid.UUID `json:"id"`
	Project             uuid.UUID `json:"project"`
	Action, Environment string
}

func (s *Service) admit(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID) error {
	if p.Kind != policy.Human || s.authorize == nil || s.audit == nil {
		return ErrDenied
	}
	if err := authority.Postgres(s.db).LockProject(ctx, tx, p, project, false); err != nil {
		return ErrDenied
	}
	d, err := s.authorize(ctx, tx, p, policy.ReadMetadata, policy.Resource{ProjectID: project, Type: "audit"})
	if err != nil || !d.Allowed {
		return ErrDenied
	}
	return nil
}
func validate(f Filter) error {
	if f.Limit < 1 || f.Limit > 100 || len(f.Action) > 100 || len(f.Environment) > 50 || len(f.Cursor) > 1024 || strings.ContainsAny(f.Action+f.Environment, "\r\n\x00") {
		return ErrInvalid
	}
	if f.From.IsZero() || f.To.IsZero() || !f.To.After(f.From) || f.To.Sub(f.From) > 31*24*time.Hour {
		return ErrInvalid
	}
	return nil
}
func safeReferences(m models.JSONMap) map[string]string {
	refs := map[string]string{}
	for _, key := range []string{"secret_id", "project_id", "run_id", "operation_id", "approval_id", "promotion_id", "connection_id", "binding_id", "session_id", "profile_id", "artifact_id"} {
		if value, ok := m[key].(string); ok {
			if id, err := uuid.Parse(value); err == nil && id != uuid.Nil {
				refs[key] = id.String()
			}
		}
	}
	return refs
}
func query(ctx context.Context, tx *sql.Tx, project uuid.UUID, f Filter, max int) ([]Entry, error) {
	var beforeTime any
	var beforeID any
	if f.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(f.Cursor)
		if err != nil {
			return nil, ErrInvalid
		}
		var c cursor
		if err = json.Unmarshal(raw, &c); err != nil || c.ID == uuid.Nil || c.Project != project || c.Action != f.Action || c.Environment != f.Environment {
			return nil, ErrInvalid
		}
		beforeTime = c.CreatedAt
		beforeID = c.ID
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,user_id,project_id,action,environment,created_at,details FROM audit_log WHERE project_id=$1 AND created_at >= $2 AND created_at < $3 AND ($4='' OR action=$4) AND ($5='' OR environment=$5) AND ($6::timestamptz IS NULL OR (created_at,id)<($6::timestamptz,$7::uuid)) ORDER BY created_at DESC,id DESC LIMIT $8`, project, f.From, f.To, f.Action, f.Environment, beforeTime, beforeID, max)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []Entry{}
	for rows.Next() {
		var e Entry
		var actor uuid.NullUUID
		var env sql.NullString
		var details models.JSONMap
		if err = rows.Scan(&e.ID, &actor, &e.ProjectID, &e.Action, &env, &e.CreatedAt, &details); err != nil {
			return nil, err
		}
		if actor.Valid {
			e.ActorID = &actor.UUID
		}
		e.Environment = env.String
		e.References = safeReferences(details)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
func (s *Service) Search(ctx context.Context, p policy.Principal, project uuid.UUID, f Filter) (Page, error) {
	if err := validate(f); err != nil {
		return Page{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback()
	if err = s.admit(ctx, tx, p, project); err != nil {
		return Page{}, err
	}
	entries, err := query(ctx, tx, project, f, f.Limit+1)
	if err != nil {
		return Page{}, err
	}
	page := Page{Entries: entries}
	if len(entries) > f.Limit {
		page.Entries = entries[:f.Limit]
		last := page.Entries[len(page.Entries)-1]
		raw, _ := json.Marshal(cursor{CreatedAt: last.CreatedAt, ID: last.ID, Project: project, Action: f.Action, Environment: f.Environment})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, tx.Commit()
}

type Export struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Status    string    `json:"status"`
	Rows      int       `json:"rows"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Service) CreateExport(ctx context.Context, p policy.Principal, project uuid.UUID, f Filter) (Export, error) {
	f.Limit = 100
	f.Cursor = ""
	if err := validate(f); err != nil {
		return Export{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return Export{}, err
	}
	defer tx.Rollback()
	if err = s.admit(ctx, tx, p, project); err != nil {
		return Export{}, err
	}
	entries, err := query(ctx, tx, project, f, 10001)
	if err != nil {
		return Export{}, err
	}
	if len(entries) > 10000 {
		return Export{}, ErrInvalid
	}
	payload, err := json.Marshal(struct {
		Format  string  `json:"format"`
		Entries []Entry `json:"entries"`
	}{"keepsave.safe-audit.v1", entries})
	if err != nil || len(payload) > 20<<20 {
		return Export{}, ErrInvalid
	}
	out := Export{ID: uuid.New(), ProjectID: project, Status: "pending", Rows: len(entries), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_exports(id,project_id,user_id,from_time,to_time,expires_at,row_count,payload,session_id,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending')`, out.ID, project, p.SubjectID, f.From, f.To, out.ExpiresAt, len(entries), payload, p.SessionID); err != nil {
		return Export{}, err
	}
	if err = s.audit.CreateTx(tx, &p.SubjectID, &project, "audit.export_created", "", models.JSONMap{"export_id": out.ID.String(), "rows": len(entries)}, ""); err != nil {
		return Export{}, err
	}
	refs, _ := json.Marshal(map[string]string{"export_id": out.ID.String(), "project_id": project.String()})
	if err = jobs.EnqueueTx(ctx, tx, uuid.New(), "audit.export.publish", refs, "local", 5); err != nil {
		return Export{}, err
	}
	return out, tx.Commit()
}
func (s *Service) Download(ctx context.Context, p policy.Principal, project, id uuid.UUID) ([]byte, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.admit(ctx, tx, p, project); err != nil {
		return nil, err
	}
	var payload []byte
	if err = tx.QueryRowContext(ctx, `SELECT payload FROM audit_exports WHERE id=$1 AND project_id=$2 AND user_id=$3 AND state='ready' AND expires_at>NOW()`, id, project, p.SubjectID).Scan(&payload); err != nil {
		return nil, ErrDenied
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return payload, nil
}
