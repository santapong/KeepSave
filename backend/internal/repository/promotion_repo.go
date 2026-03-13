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
	db *sql.DB
}

func NewPromotionRepository(db *sql.DB) *PromotionRepository {
	return &PromotionRepository{db: db}
}

func (r *PromotionRepository) Create(
	projectID uuid.UUID,
	sourceEnv, targetEnv string,
	requestedBy uuid.UUID,
	keysFilter []string,
	overridePolicy, notes string,
) (*models.PromotionRequest, error) {
	p := &models.PromotionRequest{}
	err := r.db.QueryRow(
		`INSERT INTO promotion_requests (project_id, source_environment, target_environment, requested_by, keys_filter, override_policy, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, project_id, source_environment, target_environment, status, requested_by, approved_by, keys_filter, override_policy, notes, created_at, completed_at`,
		projectID, sourceEnv, targetEnv, requestedBy, pq.Array(keysFilter), overridePolicy, notes,
	).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, pq.Array(&p.KeysFilter), &p.OverridePolicy, &p.Notes, &p.CreatedAt, &p.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("creating promotion request: %w", err)
	}
	return p, nil
}

func (r *PromotionRepository) GetByID(id uuid.UUID) (*models.PromotionRequest, error) {
	p := &models.PromotionRequest{}
	err := r.db.QueryRow(
		`SELECT id, project_id, source_environment, target_environment, status, requested_by, approved_by, keys_filter, override_policy, notes, created_at, completed_at
		 FROM promotion_requests WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, pq.Array(&p.KeysFilter), &p.OverridePolicy, &p.Notes, &p.CreatedAt, &p.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("getting promotion request: %w", err)
	}
	return p, nil
}

func (r *PromotionRepository) ListByProjectID(projectID uuid.UUID) ([]models.PromotionRequest, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, source_environment, target_environment, status, requested_by, approved_by, keys_filter, override_policy, notes, created_at, completed_at
		 FROM promotion_requests WHERE project_id = $1 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing promotion requests: %w", err)
	}
	defer rows.Close()

	var promotions []models.PromotionRequest
	for rows.Next() {
		var p models.PromotionRequest
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, pq.Array(&p.KeysFilter), &p.OverridePolicy, &p.Notes, &p.CreatedAt, &p.CompletedAt); err != nil {
			return nil, fmt.Errorf("scanning promotion request: %w", err)
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
		`UPDATE promotion_requests SET status = $2, approved_by = $3, completed_at = $4 WHERE id = $1`,
		id, status, approvedBy, completedAt,
	)
	if err != nil {
		return fmt.Errorf("updating promotion status: %w", err)
	}
	return nil
}

func (r *PromotionRepository) CreateSnapshot(promotionID, environmentID uuid.UUID, key string, encryptedValue, valueNonce []byte) error {
	_, err := r.db.Exec(
		`INSERT INTO secret_snapshots (promotion_id, environment_id, key, encrypted_value, value_nonce)
		 VALUES ($1, $2, $3, $4, $5)`,
		promotionID, environmentID, key, encryptedValue, valueNonce,
	)
	if err != nil {
		return fmt.Errorf("creating secret snapshot: %w", err)
	}
	return nil
}

func (r *PromotionRepository) GetSnapshotsByPromotionID(promotionID uuid.UUID) ([]models.SecretSnapshot, error) {
	rows, err := r.db.Query(
		`SELECT id, promotion_id, environment_id, key, encrypted_value, value_nonce, created_at
		 FROM secret_snapshots WHERE promotion_id = $1 ORDER BY key`,
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
