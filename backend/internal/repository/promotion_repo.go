package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type PromotionRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewPromotionRepository(db *sql.DB, dialect Dialect) *PromotionRepository {
	return &PromotionRepository{db: db, dialect: dialect}
}

func (r *PromotionRepository) Create(
	projectID uuid.UUID,
	sourceEnv, targetEnv string,
	requestedBy uuid.UUID,
	keysFilter []string,
	overridePolicy, notes string,
) (*models.PromotionRequest, error) {
	p := &models.PromotionRequest{}
	id := uuid.New()
	keysParam := r.arrayParam(keysFilter)

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO promotion_requests (id, project_id, source_environment, target_environment, requested_by, keys_filter, override_policy, notes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING id, project_id, source_environment, target_environment, status, requested_by, approved_by, keys_filter, override_policy, notes, created_at, completed_at`,
			id, projectID, sourceEnv, targetEnv, requestedBy, pq.Array(keysFilter), overridePolicy, notes,
		).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, pq.Array(&p.KeysFilter), &p.OverridePolicy, &p.Notes, &p.CreatedAt, &p.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("creating promotion request: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO promotion_requests (id, project_id, source_environment, target_environment, requested_by, keys_filter, override_policy, notes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`)
		_, err := r.db.Exec(insertQ, id, projectID, sourceEnv, targetEnv, requestedBy, keysParam, overridePolicy, notes)
		if err != nil {
			return nil, fmt.Errorf("creating promotion request: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, project_id, source_environment, target_environment, status, requested_by, approved_by, keys_filter, override_policy, notes, created_at, completed_at FROM promotion_requests WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, &p.KeysFilter, &p.OverridePolicy, &p.Notes, &p.CreatedAt, &p.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("reading created promotion request: %w", err)
		}
	}
	return p, nil
}

func (r *PromotionRepository) GetByID(id uuid.UUID) (*models.PromotionRequest, error) {
	p := &models.PromotionRequest{}
	if r.dialect.DBType() == DBTypePostgres {
		err := r.db.QueryRow(
			`SELECT id, project_id, source_environment, target_environment, status, requested_by, approved_by, keys_filter, override_policy, notes, created_at, completed_at
			 FROM promotion_requests WHERE id = $1`,
			id,
		).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, pq.Array(&p.KeysFilter), &p.OverridePolicy, &p.Notes, &p.CreatedAt, &p.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("getting promotion request: %w", err)
		}
	} else {
		err := r.db.QueryRow(
			Q(r.dialect, `SELECT id, project_id, source_environment, target_environment, status, requested_by, approved_by, keys_filter, override_policy, notes, created_at, completed_at
			 FROM promotion_requests WHERE id = $1`),
			id,
		).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, &p.KeysFilter, &p.OverridePolicy, &p.Notes, &p.CreatedAt, &p.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("getting promotion request: %w", err)
		}
	}
	return p, nil
}

func (r *PromotionRepository) ListByProjectID(projectID uuid.UUID) ([]models.PromotionRequest, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, project_id, source_environment, target_environment, status, requested_by, approved_by, keys_filter, override_policy, notes, created_at, completed_at
		 FROM promotion_requests WHERE project_id = $1 ORDER BY created_at DESC`),
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing promotion requests: %w", err)
	}
	defer rows.Close()

	var promotions []models.PromotionRequest
	for rows.Next() {
		var p models.PromotionRequest
		if r.dialect.DBType() == DBTypePostgres {
			if err := rows.Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, pq.Array(&p.KeysFilter), &p.OverridePolicy, &p.Notes, &p.CreatedAt, &p.CompletedAt); err != nil {
				return nil, fmt.Errorf("scanning promotion request: %w", err)
			}
		} else {
			if err := rows.Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, &p.KeysFilter, &p.OverridePolicy, &p.Notes, &p.CreatedAt, &p.CompletedAt); err != nil {
				return nil, fmt.Errorf("scanning promotion request: %w", err)
			}
		}
		promotions = append(promotions, p)
	}
	return promotions, rows.Err()
}

func (r *PromotionRepository) UpdateStatus(id uuid.UUID, status string, approvedBy *uuid.UUID) error {
	var completedAt *time.Time
	if status == "completed" || status == "rejected" {
		now := time.Now()
		completedAt = &now
	}

	_, err := r.db.Exec(
		Q(r.dialect, `UPDATE promotion_requests SET status = $2, approved_by = $3, completed_at = $4 WHERE id = $1`),
		id, status, approvedBy, completedAt,
	)
	if err != nil {
		return fmt.Errorf("updating promotion status: %w", err)
	}
	return nil
}

func (r *PromotionRepository) CreateSnapshot(promotionID, environmentID uuid.UUID, key string, encryptedValue, valueNonce []byte) error {
	_, err := r.db.Exec(
		Q(r.dialect, `INSERT INTO secret_snapshots (promotion_id, environment_id, key, encrypted_value, value_nonce)
		 VALUES ($1, $2, $3, $4, $5)`),
		promotionID, environmentID, key, encryptedValue, valueNonce,
	)
	if err != nil {
		return fmt.Errorf("creating secret snapshot: %w", err)
	}
	return nil
}

func (r *PromotionRepository) GetSnapshotsByPromotionID(promotionID uuid.UUID) ([]models.SecretSnapshot, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, promotion_id, environment_id, key, encrypted_value, value_nonce, created_at
		 FROM secret_snapshots WHERE promotion_id = $1 ORDER BY key`),
		promotionID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []models.SecretSnapshot
	for rows.Next() {
		var s models.SecretSnapshot
		if err := rows.Scan(&s.ID, &s.PromotionID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning snapshot: %w", err)
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, rows.Err()
}

func (r *PromotionRepository) arrayParam(val []string) interface{} {
	if r.dialect.DBType() == DBTypePostgres {
		return pq.Array(val)
	}
	p, _ := r.dialect.ArrayParam(val)
	return p
}
