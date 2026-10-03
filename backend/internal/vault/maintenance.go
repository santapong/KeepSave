package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Maintenance runs only on the trusted control host with its private storage.
// It claims only local events and encrypted backup jobs, never connectors or webhooks.
// Its Vault is wired by the operator command with stored ownership authority.
type Maintenance struct {
	Vault            *Service
	Directory        string
	WorkerID         uuid.UUID
	RecoveryVerified bool
}
type BackupMetadata struct {
	ID         uuid.UUID `json:"id"`
	ProjectID  uuid.UUID `json:"project_id"`
	SHA256     string    `json:"sha256"`
	Bytes      int64     `json:"bytes"`
	Scheduled  bool      `json:"scheduled"`
	State      string    `json:"state"`
	VerifiedAt time.Time `json:"verified_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (m *Maintenance) Validate() error {
	if m.Vault == nil || m.WorkerID == uuid.Nil || !filepath.IsAbs(m.Directory) {
		return ErrInvalid
	}
	if err := os.MkdirAll(m.Directory, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(m.Directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("backup directory must be private")
	}
	return nil
}
func (m *Maintenance) Schedule(ctx context.Context, now time.Time) error {
	if !m.RecoveryVerified {
		return nil
	}
	now = now.UTC()
	if now.Hour() < 2 {
		return nil
	}
	rows, err := m.Vault.db.QueryContext(ctx, `SELECT p.id,COALESCE(o.owner_id,p.owner_id) FROM projects p LEFT JOIN organizations o ON o.id=p.organization_id WHERE p.deleted_at IS NULL AND EXISTS(SELECT 1 FROM vault_projects WHERE project_id=p.id) ORDER BY p.id`)
	if err != nil {
		return err
	}
	type target struct{ id, owner uuid.UUID }
	var targets []target
	for rows.Next() {
		var item target
		if err = rows.Scan(&item.id, &item.owner); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	day := now.Format("2006-01-02")
	for _, item := range targets {
		p := policy.Principal{Kind: policy.Human, SubjectID: item.owner, ActorID: item.owner}
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("keepsave/daily-backup/"+item.id.String()+"/"+day))
		err = m.Vault.Transaction(ctx, p, item.id, policy.ManageProject, func(tx *sql.Tx) error {
			var exists bool
			if e := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE id=$1)`, id).Scan(&exists); e != nil {
				return e
			}
			if exists {
				return nil
			}
			if e := m.Vault.audit.CreateTx(tx, &item.owner, &item.id, "backup.scheduled", "", models.JSONMap{"job_id": id.String(), "day": day, "source": "operator_schedule"}, ""); e != nil {
				return e
			}
			payload, _ := json.Marshal(map[string]string{"project_id": item.id.String(), "actor_id": item.owner.String(), "source_day": day})
			return jobs.EnqueueTx(ctx, tx, id, "vault.backup", payload, "local", 5)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func (m *Maintenance) Step(ctx context.Context) error {
	q := jobs.Queue{DB: m.Vault.db}
	job, err := q.ClaimKinds(ctx, m.WorkerID, 5*time.Minute, []string{"vault.event", "identity.event", "org.event", "project.event", "template.event", "backup.event", "identity.authority_changed", "mcp.event", "tool.event", "audit.event", "vault.backup"})
	if err != nil {
		return err
	}
	if job.Kind != "vault.backup" {
		return q.Ack(ctx, *job)
	}
	err = m.processBackup(ctx, *job)
	if err != nil {
		if failure := q.Fail(ctx, *job, false); failure != nil {
			return failure
		}
		return err
	}
	return q.Ack(ctx, *job)
}
func (m *Maintenance) processBackup(ctx context.Context, job jobs.Job) error {
	var payload struct {
		ProjectID uuid.UUID `json:"project_id"`
		ActorID   uuid.UUID `json:"actor_id"`
		SourceDay string    `json:"source_day"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return ErrInvalid
	}
	if payload.ProjectID == uuid.Nil || payload.ActorID == uuid.Nil {
		return ErrInvalid
	}
	// Recheck project ownership before touching storage, including reused files.
	var owner uuid.UUID
	if err := m.Vault.db.QueryRowContext(ctx, `SELECT COALESCE(o.owner_id,p.owner_id) FROM projects p LEFT JOIN organizations o ON o.id=p.organization_id WHERE p.id=$1 AND p.deleted_at IS NULL`, payload.ProjectID).Scan(&owner); err != nil || owner != payload.ActorID {
		return ErrDenied
	}
	p := policy.Principal{Kind: policy.Human, SubjectID: owner, ActorID: owner}
	filename := job.ID.String() + ".json"
	path := filepath.Join(m.Directory, filename)
	var bundle Bundle
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 || info.Size() > 90<<20 {
			return ErrInvalid
		}
		file, e := os.Open(path)
		if e != nil {
			return e
		}
		decoder := json.NewDecoder(io.LimitReader(file, 90<<20))
		decoder.DisallowUnknownFields()
		e = decoder.Decode(&bundle)
		if e == nil {
			var extra any
			if decoder.Decode(&extra) != io.EOF {
				e = ErrInvalid
			}
		}
		file.Close()
		if e != nil {
			return ErrInvalid
		}
	} else if errors.Is(err, os.ErrNotExist) {
		bundle, err = m.Vault.Backup(ctx, p, payload.ProjectID)
		if err != nil {
			return err
		}
		raw, e := json.Marshal(bundle)
		if e != nil {
			return e
		}
		file, e := os.CreateTemp(m.Directory, ".backup-")
		if e != nil {
			return e
		}
		temp := file.Name()
		defer os.Remove(temp)
		if e = file.Chmod(0600); e == nil {
			_, e = file.Write(raw)
		}
		if e == nil {
			e = file.Sync()
		}
		closeErr := file.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
		// Link is atomic and never replaces another worker's durable object.
		if e = os.Link(temp, path); e != nil {
			return e
		}
		dir, e := os.Open(m.Directory)
		if e != nil {
			return e
		}
		e = dir.Sync()
		dir.Close()
		if e != nil {
			return e
		}
	} else {
		return err
	}
	verified, err := m.Vault.VerifyBundle(ctx, p, payload.ProjectID, bundle)
	if err != nil || verified.ProjectID != payload.ProjectID {
		return ErrInvalid
	}
	info, err = os.Stat(path)
	if err != nil {
		return err
	}
	err = m.Vault.Transaction(ctx, p, payload.ProjectID, policy.ManageProject, func(tx *sql.Tx) error {
		var active bool
		if e := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE id=$1 AND worker_id=$2 AND fence=$3 AND status='leased' AND lease_until>NOW())`, job.ID, job.WorkerID, job.Fence).Scan(&active); e != nil {
			return e
		}
		if !active {
			return jobs.ErrFenced
		}
		var day any
		if payload.SourceDay != "" {
			parsed, e := time.Parse("2006-01-02", payload.SourceDay)
			if e != nil {
				return ErrInvalid
			}
			day = parsed
		}
		result, e := tx.ExecContext(ctx, `INSERT INTO backup_catalog(id,project_id,filename,bundle_sha256,byte_size,scheduled,source_day,verified_at) VALUES($1,$2,$3,$4,$5,$6,$7,NOW()) ON CONFLICT(id) DO NOTHING`, job.ID, payload.ProjectID, filename, bundle.SHA256, info.Size(), payload.SourceDay != "", day)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			return nil
		}
		if e = m.Vault.audit.CreateTx(tx, &owner, &payload.ProjectID, "backup.stored", "", models.JSONMap{"backup_id": job.ID.String(), "sha256": bundle.SHA256, "bytes": info.Size(), "scheduled": payload.SourceDay != ""}, ""); e != nil {
			return e
		}
		metadata, _ := json.Marshal(map[string]string{"project_id": payload.ProjectID.String(), "backup_id": job.ID.String(), "action": "backup.stored"})
		return jobs.EnqueueTx(ctx, tx, uuid.New(), "backup.event", metadata, "local", 5)
	})
	if err != nil {
		return err
	}
	return m.Retain(ctx, p, payload.ProjectID, 30)
}

// Retain removes only excess verified scheduled bundles; manual/pre-upgrade
// objects are untouched. A failed unlink remains visible and cannot count as a
// successful deletion. The project lock serializes retention across workers.
func (m *Maintenance) Retain(ctx context.Context, p policy.Principal, project uuid.UUID, keep int) error {
	// Turning off verified recovery pauses destructive retention as well as
	// scheduling. Manual jobs may still store a fresh encrypted bundle.
	if !m.RecoveryVerified {
		return nil
	}
	if keep < 2 {
		return ErrInvalid
	}
	type item struct {
		id       uuid.UUID
		filename string
	}
	var items []item
	err := m.Vault.Transaction(ctx, p, project, policy.ManageProject, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT id,filename FROM backup_catalog WHERE project_id=$1 AND state IN('delete_pending','retention_failed') AND scheduled UNION ALL SELECT id,filename FROM (SELECT id,filename FROM backup_catalog WHERE project_id=$1 AND scheduled AND state='verified' ORDER BY created_at DESC,id DESC OFFSET $2) excess`, project, keep)
		if e != nil {
			return e
		}
		for rows.Next() {
			var i item
			if e = rows.Scan(&i.id, &i.filename); e != nil {
				rows.Close()
				return e
			}
			items = append(items, i)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, i := range items {
			if i.filename != i.id.String()+".json" {
				return ErrInvalid
			}
			if _, e = tx.ExecContext(ctx, `UPDATE backup_catalog SET state='delete_pending',updated_at=NOW() WHERE id=$1`, i.id); e != nil {
				return e
			}
			if e = m.Vault.audit.CreateTx(tx, &p.ActorID, &project, "backup.retention.requested", "", models.JSONMap{"backup_id": i.id.String()}, ""); e != nil {
				return e
			}
			metadata, _ := json.Marshal(map[string]string{"project_id": project.String(), "backup_id": i.id.String(), "action": "backup.retention.requested"})
			if e = jobs.EnqueueTx(ctx, tx, uuid.New(), "backup.event", metadata, "local", 5); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, i := range items {
		unlinkErr := os.Remove(filepath.Join(m.Directory, i.filename))
		state, action := "deleted", "backup.retention.deleted"
		if unlinkErr != nil && !errors.Is(unlinkErr, os.ErrNotExist) {
			state, action = "retention_failed", "backup.retention.failed"
		}
		err = m.Vault.Transaction(ctx, p, project, policy.ManageProject, func(tx *sql.Tx) error {
			if _, e := tx.ExecContext(ctx, `UPDATE backup_catalog SET state=$1,updated_at=NOW() WHERE id=$2 AND state='delete_pending'`, state, i.id); e != nil {
				return e
			}
			if e := m.Vault.audit.CreateTx(tx, &p.ActorID, &project, action, "", models.JSONMap{"backup_id": i.id.String()}, ""); e != nil {
				return e
			}
			metadata, _ := json.Marshal(map[string]string{"project_id": project.String(), "backup_id": i.id.String(), "action": action})
			return jobs.EnqueueTx(ctx, tx, uuid.New(), "backup.event", metadata, "local", 5)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) ListBackups(ctx context.Context, p policy.Principal, project uuid.UUID) ([]BackupMetadata, error) {
	result := []BackupMetadata{}
	err := s.Transaction(ctx, p, project, policy.ManageProject, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT id,project_id,bundle_sha256,byte_size,scheduled,state,verified_at,created_at FROM backup_catalog WHERE project_id=$1 ORDER BY created_at DESC LIMIT 100`, project)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var item BackupMetadata
			if e = rows.Scan(&item.ID, &item.ProjectID, &item.SHA256, &item.Bytes, &item.Scheduled, &item.State, &item.VerifiedAt, &item.CreatedAt); e != nil {
				return e
			}
			result = append(result, item)
		}
		return rows.Err()
	})
	return result, err
}
