package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type AuditRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewAuditRepository(db *sql.DB, dialect Dialect) *AuditRepository {
	return &AuditRepository{db: db, dialect: dialect}
}

func (r *AuditRepository) Create(userID, projectID *uuid.UUID, action, environment string, details models.JSONMap, ipAddress string) error {
	detailsBytes, err := details.Value()
	if err != nil {
		return fmt.Errorf("marshaling audit details: %w", err)
	}
	_, err = r.db.Exec(
		Q(r.dialect, `INSERT INTO audit_log (user_id, project_id, action, environment, details, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6)`),
		userID, projectID, action, environment, detailsBytes, ipAddress,
	)
	if err != nil {
		return fmt.Errorf("creating audit entry: %w", err)
	}
	return nil
}

func (r *AuditRepository) ListByProjectID(projectID uuid.UUID, limit int) ([]models.AuditEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, user_id, project_id, action, environment, details, ip_address, created_at
		 FROM audit_log WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2`),
		projectID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()

	var entries []models.AuditEntry
	for rows.Next() {
		var e models.AuditEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.ProjectID, &e.Action, &e.Environment, &e.Details, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
