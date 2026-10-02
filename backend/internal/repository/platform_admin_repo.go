package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"strings"
)

type PlatformAdminRepository struct {
	db      *sql.DB
	dialect Dialect
	audit   *AuditRepository
}

func NewPlatformAdminRepository(db *sql.DB, d Dialect, audit *AuditRepository) *PlatformAdminRepository {
	return &PlatformAdminRepository{db, d, audit}
}
func (r *PlatformAdminRepository) IsPlatformAdmin(ctx context.Context, user uuid.UUID) (bool, error) {
	var found bool
	err := r.db.QueryRowContext(ctx, Q(r.dialect, `SELECT EXISTS(SELECT 1 FROM platform_admin_grants g JOIN users u ON u.id=g.user_id WHERE g.user_id=$1 AND g.active=$2)`), user, true).Scan(&found)
	return found, err
}

// SetGrant is operator-only. No HTTP handler exposes this mutation.
func (r *PlatformAdminRepository) SetGrant(ctx context.Context, user uuid.UUID, active bool, operator, reason string) error {
	operator = strings.TrimSpace(operator)
	reason = strings.TrimSpace(reason)
	if user == uuid.Nil || operator == "" || len(operator) > 255 || reason == "" || len(reason) > 500 {
		return fmt.Errorf("operator identity, account ID and bounded reason are required")
	}
	if r.audit == nil {
		return fmt.Errorf("operator audit unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existing uuid.UUID
	if err = tx.QueryRowContext(ctx, Q(r.dialect, `SELECT id FROM users WHERE id=$1`), user).Scan(&existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("account not found")
		}
		return err
	}
	if _, err = tx.ExecContext(ctx, Q(r.dialect, `DELETE FROM platform_admin_grants WHERE user_id=$1`), user); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, Q(r.dialect, `INSERT INTO platform_admin_grants(user_id,active,operator_name,reason) VALUES($1,$2,$3,$4)`), user, active, operator, reason); err != nil {
		return err
	}
	action := "platform.admin_granted"
	if !active {
		action = "platform.admin_revoked"
	}
	if err = r.audit.CreateTx(tx, nil, nil, action, "", models.JSONMap{"user_id": user.String(), "operator": operator, "reason": reason}, ""); err != nil {
		return err
	}
	if r.dialect.DBType() == DBTypePostgres {
		payload, err := json.Marshal(map[string]any{"actor_id": nil, "action": action, "user_id": user, "active": active})
		if err != nil {
			return err
		}
		if err = jobs.EnqueueTx(ctx, tx, uuid.New(), "identity.event", payload, "local", 5); err != nil {
			return err
		}
	}
	return tx.Commit()
}
