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
		).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, pq.Array(&p.KeysFilter), &p.OverridePolicy, &p.Notes, dbTime(&p.CreatedAt), dbTime(&p.CompletedAt))
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
		err = r.db.QueryRow(selectQ, id).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, &p.KeysFilter, &p.OverridePolicy, &p.Notes, dbTime(&p.CreatedAt), dbTime(&p.CompletedAt))
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
		).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, pq.Array(&p.KeysFilter), &p.OverridePolicy, &p.Notes, dbTime(&p.CreatedAt), dbTime(&p.CompletedAt))
		if err != nil {
			return nil, fmt.Errorf("getting promotion request: %w", err)
		}
	} else {
		err := r.db.QueryRow(
			Q(r.dialect, `SELECT id, project_id, source_environment, target_environment, status, requested_by, approved_by, keys_filter, override_policy, notes, created_at, completed_at
			 FROM promotion_requests WHERE id = $1`),
			id,
		).Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, &p.KeysFilter, &p.OverridePolicy, &p.Notes, dbTime(&p.CreatedAt), dbTime(&p.CompletedAt))
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
			if err := rows.Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, pq.Array(&p.KeysFilter), &p.OverridePolicy, &p.Notes, dbTime(&p.CreatedAt), dbTime(&p.CompletedAt)); err != nil {
				return nil, fmt.Errorf("scanning promotion request: %w", err)
			}
		} else {
			if err := rows.Scan(&p.ID, &p.ProjectID, &p.SourceEnvironment, &p.TargetEnvironment, &p.Status, &p.RequestedBy, &p.ApprovedBy, &p.KeysFilter, &p.OverridePolicy, &p.Notes, dbTime(&p.CreatedAt), dbTime(&p.CompletedAt)); err != nil {
				return nil, fmt.Errorf("scanning promotion request: %w", err)
			}
		}
		promotions = append(promotions, p)
	}
	return promotions, rows.Err()
}

// WithTx runs fn inside a single transaction, committing on success and rolling
// back on any error. Every promotion write (claim, snapshots, upserts, terminal
// status) runs through one tx so a partial failure leaves no half-promoted
// state (ADR-0017, P-01).
func (r *PromotionRepository) WithTx(fn func(*sql.Tx) error) error {
	return runInTx(r.db, fn)
}

// UpdateStatus sets a promotion's status unconditionally — used for the non-prod
// immediate path and for marking a failed promotion rejected.
func (r *PromotionRepository) UpdateStatus(id uuid.UUID, status string, approvedBy *uuid.UUID) error {
	_, err := r.setStatus(r.db, id, status, approvedBy, "")
	return err
}

// CompareAndSetStatusTx atomically transitions a promotion from `from` to `to`
// inside tx, returning true only if exactly one row matched. This is the
// execution claim that closes the approve TOCTOU (ADR-0017, P-02): the loser of
// a concurrent approve sees false and rolls back.
func (r *PromotionRepository) CompareAndSetStatusTx(tx *sql.Tx, id uuid.UUID, from, to string, approvedBy *uuid.UUID) (bool, error) {
	n, err := r.setStatus(tx, id, to, approvedBy, from)
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// MarkRolledBackTx transitions completed -> rolled_back inside tx without
// touching approved_by or completed_at (those record the original completion).
// Returns false if the promotion was not `completed` (idempotency guard).
func (r *PromotionRepository) MarkRolledBackTx(tx *sql.Tx, id uuid.UUID) (bool, error) {
	res, err := ExecQ(tx, r.dialect, `UPDATE promotion_requests SET status = 'rolled_back' WHERE id = $1 AND status = 'completed'`, id)
	if err != nil {
		return false, fmt.Errorf("marking promotion rolled back: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rolled-back rows affected: %w", err)
	}
	return n == 1, nil
}

// setStatus performs the status UPDATE over db (a *sql.DB or *sql.Tx). When
// requireFrom is non-empty it adds an `AND status = requireFrom` guard, and the
// returned count tells the caller whether the row matched.
func (r *PromotionRepository) setStatus(db dbtx, id uuid.UUID, status string, approvedBy *uuid.UUID, requireFrom string) (int64, error) {
	var completedAt *time.Time
	if status == "completed" || status == "rejected" {
		now := time.Now()
		completedAt = &now
	}
	query := `UPDATE promotion_requests SET status = $2, approved_by = $3, completed_at = $4 WHERE id = $1`
	args := []interface{}{id, status, approvedBy, completedAt}
	if requireFrom != "" {
		query += ` AND status = $5`
		args = append(args, requireFrom)
	}
	res, err := ExecQ(db, r.dialect, query, args...)
	if err != nil {
		return 0, fmt.Errorf("updating promotion status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("promotion status rows affected: %w", err)
	}
	return n, nil
}

// CreateSnapshotTx records the prior state of a target key inside tx.
// priorExisted is true for an overwritten value (rollback restores it) and
// false for an added key (rollback deletes it) — see ADR-0017.
func (r *PromotionRepository) CreateSnapshotTx(tx *sql.Tx, promotionID, environmentID uuid.UUID, key string, encryptedValue, valueNonce []byte, priorExisted bool) error {
	id := uuid.New()
	_, err := ExecQ(tx, r.dialect, `INSERT INTO secret_snapshots (id, promotion_id, environment_id, key, encrypted_value, value_nonce, prior_existed)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`, id, promotionID, environmentID, key, encryptedValue, valueNonce, priorExisted)
	if err != nil {
		return fmt.Errorf("creating secret snapshot: %w", err)
	}
	return nil
}

func (r *PromotionRepository) GetSnapshotsByPromotionID(promotionID uuid.UUID) ([]models.SecretSnapshot, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, promotion_id, environment_id, key, encrypted_value, value_nonce, prior_existed, created_at
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
		if err := rows.Scan(&s.ID, &s.PromotionID, &s.EnvironmentID, &s.Key, &s.EncryptedValue, &s.ValueNonce, &s.PriorExisted, dbTime(&s.CreatedAt)); err != nil {
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
