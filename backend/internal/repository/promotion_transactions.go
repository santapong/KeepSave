package repository

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

func (r *PromotionRepository) CreateTx(ctx context.Context, tx *sql.Tx, project uuid.UUID, source, target string, actor uuid.UUID, keys []string, override, notes string) (*models.PromotionRequest, error) {
	id := uuid.New()
	_, err := tx.ExecContext(ctx, Q(r.dialect, `INSERT INTO promotion_requests(id,project_id,source_environment,target_environment,requested_by,keys_filter,override_policy,notes) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`), id, project, source, target, actor, r.arrayParam(keys), override, notes)
	if err != nil {
		return nil, err
	}
	return r.GetByIDTx(ctx, tx, id, project)
}
func (r *PromotionRepository) GetByIDTx(ctx context.Context, tx *sql.Tx, id, project uuid.UUID) (*models.PromotionRequest, error) {
	p := &models.PromotionRequest{}
	err := tx.QueryRowContext(ctx, Q(r.dialect, `SELECT id,project_id,source_environment,target_environment,status,requested_by,approved_by,keys_filter,override_policy,notes,created_at,completed_at FROM promotion_requests WHERE id=$1 AND project_id=$2`), id, project).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, &p.KeysFilter, &p.OverridePolicy, &p.Notes, dbTime(&p.CreatedAt), dbTime(&p.CompletedAt))
	return p, err
}
