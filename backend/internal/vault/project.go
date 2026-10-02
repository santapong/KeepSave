package vault

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

// ArchiveProject retains recoverable ciphertext while removing all active authority.
// The project lock serializes this operation against every journal mutation.
func (s *Service) ArchiveProject(ctx context.Context, p policy.Principal, project uuid.UUID) error {
	tx, err := s.begin(ctx, project)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.permit(ctx, tx, p, policy.ManageProject, Record{ProjectID: project}); err != nil {
		return err
	}
	if err = s.EnrollTx(ctx, tx, p, project); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT secret_id,key_id FROM vault_entries WHERE project_id=$1 AND NOT deleted ORDER BY secret_id`, project)
	if err != nil {
		return err
	}
	type item struct{ id, key uuid.UUID }
	var items []item
	for rows.Next() {
		var i item
		if err = rows.Scan(&i.id, &i.key); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, i := range items {
		r, _, err := s.load(ctx, tx, project, i.id)
		if err != nil {
			return err
		}
		r.Revision++
		r.Deleted = true
		if err = s.append(ctx, tx, p, r, i.key, []byte{}, []byte{}, "delete"); err != nil {
			return err
		}
		if err = s.event(ctx, tx, p, r, "secret.deleted"); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM api_keys WHERE project_id=$1`, project); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM secrets WHERE project_id=$1`, project); err != nil {
		return err
	}
	// Already dispatched external effects are not reclassified as safe retries.
	if _, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',outcome='project_deleted',updated_at=NOW() WHERE payload->>'project_id'=$1 AND status IN ('pending','leased') AND kind NOT LIKE '%.event'`, project.String()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, project); err != nil {
		return err
	}
	if err = s.event(ctx, tx, p, Record{ProjectID: project}, "project.deleted"); err != nil {
		return err
	}
	return tx.Commit()
}

// LockProjectTx exposes the shared project serialization point to authorized
// promotion adapters; it does not grant permission or disclose key material.
func (s *Service) LockProjectTx(ctx context.Context, tx *sql.Tx, project uuid.UUID) error {
	var id uuid.UUID
	if err := tx.QueryRowContext(ctx, `SELECT id FROM projects WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, project).Scan(&id); err != nil {
		return ErrNotFound
	}
	return nil
}
