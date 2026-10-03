package vault

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"encoding/json"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

type Lifecycle struct {
	SecretID          uuid.UUID  `json:"secret_id"`
	ProjectID         uuid.UUID  `json:"project_id"`
	ResponsibleUserID *uuid.UUID `json:"responsible_user_id"`
	DeclaredExpiresAt *time.Time `json:"declared_expires_at"`
	RenewalAt         *time.Time `json:"renewal_at"`
	Provenance        string     `json:"provenance"`
	Revision          int64      `json:"revision"`
	UpdatedAt         time.Time  `json:"updated_at"`
	Known             bool       `json:"known"`
}
type LifecycleChange struct {
	ResponsibleUserID *uuid.UUID `json:"responsible_user_id"`
	DeclaredExpiresAt *time.Time `json:"declared_expires_at"`
	RenewalAt         *time.Time `json:"renewal_at"`
	Provenance        string     `json:"provenance"`
	ExpectedRevision  int64      `json:"expected_revision"`
}

func lifecycleTx(ctx context.Context, tx *sql.Tx, project, secret uuid.UUID) (Lifecycle, error) {
	l := Lifecycle{ProjectID: project, SecretID: secret}
	var owner uuid.NullUUID
	var expiry, renewal sql.NullTime
	err := tx.QueryRowContext(ctx, `SELECT responsible_user_id,declared_expires_at,renewal_at,provenance,revision,updated_at FROM vault_lifecycle WHERE project_id=$1 AND secret_id=$2`, project, secret).Scan(&owner, &expiry, &renewal, &l.Provenance, &l.Revision, &l.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return l, nil
	}
	if err != nil {
		return l, err
	}
	if owner.Valid {
		l.ResponsibleUserID = &owner.UUID
	}
	if expiry.Valid {
		l.DeclaredExpiresAt = &expiry.Time
	}
	if renewal.Valid {
		l.RenewalAt = &renewal.Time
	}
	l.Known = true
	return l, nil
}
func (s *Service) Lifecycle(ctx context.Context, p policy.Principal, project, secret uuid.UUID) (Lifecycle, error) {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return Lifecycle{}, err
	}
	defer tx.Rollback()
	r, _, err := s.load(ctx, tx, project, secret)
	if err != nil || r.Deleted {
		return Lifecycle{}, ErrNotFound
	}
	if err = s.permit(ctx, tx, p, policy.ReadMetadata, r); err != nil {
		return Lifecycle{}, err
	}
	l, err := lifecycleTx(ctx, tx, project, secret)
	if err != nil {
		return l, err
	}
	return l, tx.Commit()
}
func (s *Service) UpdateLifecycle(ctx context.Context, p policy.Principal, project, secret uuid.UUID, c LifecycleChange) (Lifecycle, error) {
	if c.ExpectedRevision < 0 || len(c.Provenance) > 500 || c.ResponsibleUserID != nil && *c.ResponsibleUserID == uuid.Nil {
		return Lifecycle{}, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Lifecycle{}, err
	}
	defer tx.Rollback()
	var extra []uuid.UUID
	if c.ResponsibleUserID != nil {
		extra = append(extra, *c.ResponsibleUserID)
	}
	if err = authority.Postgres(s.db).LockProjectSubjects(ctx, tx, p, project, true, extra); err != nil {
		return Lifecycle{}, ErrDenied
	}
	r, _, err := s.load(ctx, tx, project, secret)
	if err != nil || r.Deleted {
		return Lifecycle{}, ErrNotFound
	}
	if err = s.permit(ctx, tx, p, policy.WriteSecret, r); err != nil {
		return Lifecycle{}, err
	}
	l, err := lifecycleTx(ctx, tx, project, secret)
	if err != nil {
		return l, err
	}
	if l.Revision != c.ExpectedRevision {
		return l, ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO vault_lifecycle(secret_id,project_id,responsible_user_id,declared_expires_at,renewal_at,provenance,revision) VALUES($1,$2,$3,$4,$5,$6,1) ON CONFLICT(secret_id) DO UPDATE SET responsible_user_id=EXCLUDED.responsible_user_id,declared_expires_at=EXCLUDED.declared_expires_at,renewal_at=EXCLUDED.renewal_at,provenance=EXCLUDED.provenance,revision=vault_lifecycle.revision+1,updated_at=NOW()`, secret, project, c.ResponsibleUserID, c.DeclaredExpiresAt, c.RenewalAt, c.Provenance)
	if err != nil {
		return l, err
	}
	l, err = lifecycleTx(ctx, tx, project, secret)
	if err != nil {
		return l, err
	}
	if err = s.eventWithMetadataRevision(ctx, tx, p, r, "secret.lifecycle_updated", l.Revision); err != nil {
		return l, err
	}
	return l, tx.Commit()
}

type Notification struct {
	ID                uuid.UUID  `json:"id"`
	ProjectID         uuid.UUID  `json:"project_id"`
	SecretID          uuid.UUID  `json:"secret_id"`
	Key               string     `json:"key"`
	ThresholdDays     int        `json:"threshold_days"`
	LifecycleRevision int64      `json:"lifecycle_revision"`
	CreatedAt         time.Time  `json:"created_at"`
	ReadAt            *time.Time `json:"read_at"`
}

func (s *Service) Notifications(ctx context.Context, p policy.Principal) ([]Notification, error) {
	if p.Kind != policy.Human {
		return nil, ErrDenied
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	g := authority.Postgres(s.db)
	if err = g.RequireActiveTx(ctx, tx, p.SubjectID, p.SessionID); err != nil {
		return nil, ErrDenied
	}
	// Old reminders disappear after reassignment, deletion or offboarding. No
	// historical ownership snapshot confers current visibility.
	rows, err := tx.QueryContext(ctx, `SELECT n.id,n.project_id,n.secret_id,v.secret_key,n.threshold_days,n.lifecycle_revision,n.created_at,n.read_at FROM vault_notifications n JOIN vault_lifecycle l ON l.secret_id=n.secret_id AND l.project_id=n.project_id AND l.revision=n.lifecycle_revision AND l.responsible_user_id=n.user_id JOIN vault_entries v ON v.secret_id=n.secret_id AND v.project_id=n.project_id AND NOT v.deleted JOIN projects p ON p.id=n.project_id AND p.deleted_at IS NULL WHERE n.user_id=$1 AND ((p.organization_id IS NULL AND p.owner_id=$1) OR EXISTS(SELECT 1 FROM member_authority_state a JOIN organization_members m USING(organization_id,user_id) WHERE a.organization_id=p.organization_id AND a.user_id=$1 AND a.active)) ORDER BY n.created_at DESC,n.id DESC LIMIT 100`, p.SubjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Notification{}
	for rows.Next() {
		var n Notification
		var read sql.NullTime
		if err = rows.Scan(&n.ID, &n.ProjectID, &n.SecretID, &n.Key, &n.ThresholdDays, &n.LifecycleRevision, &n.CreatedAt, &read); err != nil {
			return nil, err
		}
		if read.Valid {
			n.ReadAt = &read.Time
		}
		result = append(result, n)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return result, tx.Commit()
}

// GenerateReminders is a trusted worker operation; it never reads a value and
// rechecks current recipient authority and metadata under the common barrier.
func (s *Service) GenerateReminders(ctx context.Context, now time.Time) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.project_id,l.secret_id,l.responsible_user_id FROM vault_lifecycle l JOIN projects p ON p.id=l.project_id AND p.deleted_at IS NULL JOIN vault_entries v ON v.secret_id=l.secret_id AND v.project_id=l.project_id AND NOT v.deleted WHERE l.responsible_user_id IS NOT NULL AND COALESCE(l.renewal_at,l.declared_expires_at) <= $1::timestamptz + INTERVAL '30 days'
 AND EXISTS(SELECT 1 FROM (VALUES(30),(7),(1)) AS threshold(days) WHERE COALESCE(l.renewal_at,l.declared_expires_at) <= $1::timestamptz + threshold.days*INTERVAL '1 day' AND NOT EXISTS(SELECT 1 FROM vault_notifications n WHERE n.secret_id=l.secret_id AND n.lifecycle_revision=l.revision AND n.user_id=l.responsible_user_id AND n.threshold_days=threshold.days)) ORDER BY l.secret_id LIMIT 100`, now)
	if err != nil {
		return 0, err
	}
	type target struct{ project, secret, user uuid.UUID }
	targets := []target{}
	for rows.Next() {
		var t target
		if err = rows.Scan(&t.project, &t.secret, &t.user); err != nil {
			rows.Close()
			return 0, err
		}
		targets = append(targets, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, t := range targets {
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return count, e
		}
		p := policy.Principal{Kind: policy.Workload, SubjectID: uuid.New(), ActorID: t.user}
		e = authority.Postgres(s.db).LockProject(ctx, tx, p, t.project, true)
		if e != nil {
			tx.Rollback()
			continue
		}
		l, e := lifecycleTx(ctx, tx, t.project, t.secret)
		if e != nil {
			tx.Rollback()
			return count, e
		}
		if l.ResponsibleUserID == nil || *l.ResponsibleUserID != t.user {
			tx.Rollback()
			continue
		}
		due := l.RenewalAt
		if due == nil {
			due = l.DeclaredExpiresAt
		}
		if due == nil {
			tx.Rollback()
			continue
		}
		var active bool
		if e = tx.QueryRowContext(ctx, `SELECT NOT deleted FROM vault_entries WHERE secret_id=$1 AND project_id=$2`, t.secret, t.project).Scan(&active); e != nil || !active {
			tx.Rollback()
			continue
		}
		added := 0
		for _, days := range []int{30, 7, 1} {
			if now.Before(due.Add(-time.Duration(days) * 24 * time.Hour)) {
				continue
			}
			result, e := tx.ExecContext(ctx, `INSERT INTO vault_notifications(id,user_id,project_id,secret_id,lifecycle_revision,threshold_days) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(secret_id,lifecycle_revision,user_id,threshold_days) DO NOTHING`, uuid.New(), t.user, t.project, t.secret, l.Revision, days)
			if e != nil {
				tx.Rollback()
				return count, e
			}
			n, _ := result.RowsAffected()
			added += int(n)
		}
		if added > 0 {
			if s.audit == nil {
				tx.Rollback()
				return count, ErrDenied
			}
			e = s.audit.CreateTx(tx, &t.user, &t.project, "secret.reminder_created", "", models.JSONMap{"secret_id": t.secret.String(), "lifecycle_revision": l.Revision, "count": added}, "")
			if e != nil {
				tx.Rollback()
				return count, e
			}
		}
		if added > 0 {
			payload, _ := json.Marshal(map[string]any{"project_id": t.project, "secret_id": t.secret, "lifecycle_revision": l.Revision})
			if e = jobs.EnqueueTx(ctx, tx, uuid.New(), "vault.event", payload, "local", 5); e != nil {
				tx.Rollback()
				return count, e
			}
		}
		if e = tx.Commit(); e != nil {
			return count, e
		}
		count += added
	}
	return count, nil
}

// LifecycleItem is metadata only; listing it never decrypts current values.
type LifecycleItem struct {
	Lifecycle   Lifecycle `json:"lifecycle"`
	Key         string    `json:"key"`
	Environment string    `json:"environment"`
}

func (s *Service) ListLifecycle(ctx context.Context, p policy.Principal, project uuid.UUID) ([]LifecycleItem, error) {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if s.authorize == nil {
		return nil, ErrDenied
	}
	decision, err := s.authorize(ctx, tx, p, policy.ReadMetadata, policy.Resource{ProjectID: project, Type: "secret_collection"})
	if err != nil || !decision.Allowed {
		return nil, ErrDenied
	}
	rows, err := tx.QueryContext(ctx, `SELECT v.secret_id FROM vault_entries v JOIN environments e ON e.id=v.environment_id AND e.project_id=v.project_id WHERE v.project_id=$1 AND NOT v.deleted ORDER BY e.name,v.secret_key LIMIT 100`, project)
	if err != nil {
		return nil, err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []LifecycleItem{}
	for _, id := range ids {
		r, _, e := s.load(ctx, tx, project, id)
		if e != nil {
			return nil, e
		}
		d, e := s.authorize(ctx, tx, p, policy.ReadMetadata, policy.Resource{ProjectID: project, ID: id, Key: r.Key, Environment: r.Environment, Type: "secret"})
		if e != nil {
			return nil, e
		}
		if !d.Allowed {
			continue
		}
		life, e := lifecycleTx(ctx, tx, project, id)
		if e != nil {
			return nil, e
		}
		out = append(out, LifecycleItem{life, r.Key, r.Environment})
	}
	return out, tx.Commit()
}
