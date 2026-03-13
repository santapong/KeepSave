package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(userID, projectID *uuid.UUID, action, environment string, details models.JSONMap, ipAddress string) error {
	detailsBytes, err := details.Value()
	if err != nil {
		return fmt.Errorf("marshaling audit details: %w", err)
	}
	_, err = r.db.Exec(
		`INSERT INTO audit_log (user_id, project_id, action, environment, details, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, projectID, action, environment, detailsBytes, ipAddress,
	)
	if err != nil {
		return fmt.Errorf("creating audit entry: %w", err)
	}
	return nil
}
