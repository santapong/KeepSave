package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// SSORepository handles SSO configuration persistence.
type SSORepository struct {
	db *sql.DB
}

// NewSSORepository creates a new SSO repository.
func NewSSORepository(db *sql.DB) *SSORepository {
	return &SSORepository{db: db}
}

// Upsert creates or updates an SSO configuration.
func (r *SSORepository) Upsert(config *models.SSOConfig) (*models.SSOConfig, error) {
	err := r.db.QueryRow(
		`INSERT INTO sso_configs (organization_id, provider, issuer_url, client_id, client_secret_encrypted, client_secret_nonce, metadata, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (organization_id, provider) DO UPDATE SET issuer_url = $3, client_id = $4, client_secret_encrypted = $5, client_secret_nonce = $6, metadata = $7, enabled = $8, updated_at = NOW()
		RETURNING id, created_at, updated_at`,
		config.OrganizationID, config.Provider, config.IssuerURL, config.ClientID,
		config.ClientSecretEncrypted, config.ClientSecretNonce, config.Metadata, config.Enabled,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting SSO config: %w", err)
	}
	return config, nil
}

// GetByOrgAndProvider returns an SSO config by org and provider.
func (r *SSORepository) GetByOrgAndProvider(orgID uuid.UUID, provider string) (*models.SSOConfig, error) {
	config := &models.SSOConfig{}
	err := r.db.QueryRow(
		`SELECT id, organization_id, provider, issuer_url, client_id, client_secret_encrypted, client_secret_nonce, metadata, enabled, created_at, updated_at
		FROM sso_configs WHERE organization_id = $1 AND provider = $2`,
		orgID, provider,
	).Scan(&config.ID, &config.OrganizationID, &config.Provider, &config.IssuerURL, &config.ClientID,
		&config.ClientSecretEncrypted, &config.ClientSecretNonce, &config.Metadata, &config.Enabled,
		&config.CreatedAt, &config.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting SSO config: %w", err)
	}
	return config, nil
}

// ListByOrg returns all SSO configs for an organization.
func (r *SSORepository) ListByOrg(orgID uuid.UUID) ([]models.SSOConfig, error) {
	rows, err := r.db.Query(
		`SELECT id, organization_id, provider, issuer_url, client_id, metadata, enabled, created_at, updated_at
		FROM sso_configs WHERE organization_id = $1`, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing SSO configs: %w", err)
	}
	defer rows.Close()

	var configs []models.SSOConfig
	for rows.Next() {
		var c models.SSOConfig
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Provider, &c.IssuerURL, &c.ClientID,
			&c.Metadata, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning SSO config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, nil
}

// Delete removes an SSO configuration.
func (r *SSORepository) Delete(orgID uuid.UUID, provider string) error {
	_, err := r.db.Exec(`DELETE FROM sso_configs WHERE organization_id = $1 AND provider = $2`, orgID, provider)
	if err != nil {
		return fmt.Errorf("deleting SSO config: %w", err)
	}
	return nil
}

// ComplianceRepository handles compliance report persistence.
type ComplianceRepository struct {
	db *sql.DB
}

// NewComplianceRepository creates a new compliance repository.
func NewComplianceRepository(db *sql.DB) *ComplianceRepository {
	return &ComplianceRepository{db: db}
}

// Create creates a new compliance report.
func (r *ComplianceRepository) Create(report *models.ComplianceReport) (*models.ComplianceReport, error) {
	err := r.db.QueryRow(
		`INSERT INTO compliance_reports (organization_id, report_type, status, generated_by)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		report.OrganizationID, report.ReportType, report.Status, report.GeneratedBy,
	).Scan(&report.ID, &report.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating compliance report: %w", err)
	}
	return report, nil
}

// Complete marks a report as completed with data.
func (r *ComplianceRepository) Complete(id uuid.UUID, data models.JSONMap) (*models.ComplianceReport, error) {
	report := &models.ComplianceReport{}
	err := r.db.QueryRow(
		`UPDATE compliance_reports SET status = 'completed', data = $2, completed_at = NOW()
		WHERE id = $1
		RETURNING id, organization_id, report_type, status, data, generated_by, created_at, completed_at`,
		id, data,
	).Scan(&report.ID, &report.OrganizationID, &report.ReportType, &report.Status, &report.Data,
		&report.GeneratedBy, &report.CreatedAt, &report.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("completing compliance report: %w", err)
	}
	return report, nil
}

// ListByOrg returns compliance reports for an organization.
func (r *ComplianceRepository) ListByOrg(orgID uuid.UUID) ([]models.ComplianceReport, error) {
	rows, err := r.db.Query(
		`SELECT id, organization_id, report_type, status, data, generated_by, created_at, completed_at
		FROM compliance_reports WHERE organization_id = $1 ORDER BY created_at DESC`, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing compliance reports: %w", err)
	}
	defer rows.Close()

	var reports []models.ComplianceReport
	for rows.Next() {
		var r models.ComplianceReport
		if err := rows.Scan(&r.ID, &r.OrganizationID, &r.ReportType, &r.Status, &r.Data,
			&r.GeneratedBy, &r.CreatedAt, &r.CompletedAt); err != nil {
			return nil, fmt.Errorf("scanning compliance report: %w", err)
		}
		reports = append(reports, r)
	}
	return reports, nil
}

// BackupRepository handles backup snapshot persistence.
type BackupRepository struct {
	db *sql.DB
}

// NewBackupRepository creates a new backup repository.
func NewBackupRepository(db *sql.DB) *BackupRepository {
	return &BackupRepository{db: db}
}

// Create creates a new backup snapshot.
func (r *BackupRepository) Create(snapshot *models.BackupSnapshot) (*models.BackupSnapshot, error) {
	err := r.db.QueryRow(
		`INSERT INTO backup_snapshots (project_id, snapshot_type, encrypted_data, data_nonce, size_bytes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		snapshot.ProjectID, snapshot.SnapshotType, snapshot.EncryptedData, snapshot.DataNonce,
		snapshot.SizeBytes, snapshot.CreatedBy,
	).Scan(&snapshot.ID, &snapshot.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating backup snapshot: %w", err)
	}
	return snapshot, nil
}

// ListByProject returns backups for a project.
func (r *BackupRepository) ListByProject(projectID uuid.UUID) ([]models.BackupSnapshot, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, snapshot_type, size_bytes, created_by, created_at
		FROM backup_snapshots WHERE project_id = $1 ORDER BY created_at DESC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing backups: %w", err)
	}
	defer rows.Close()

	var snapshots []models.BackupSnapshot
	for rows.Next() {
		var s models.BackupSnapshot
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.SnapshotType, &s.SizeBytes, &s.CreatedBy, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning backup: %w", err)
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

// SecurityEventRepository logs security events.
type SecurityEventRepository struct {
	db *sql.DB
}

// NewSecurityEventRepository creates a new security event repository.
func NewSecurityEventRepository(db *sql.DB) *SecurityEventRepository {
	return &SecurityEventRepository{db: db}
}

// Log records a security event.
func (r *SecurityEventRepository) Log(event *models.SecurityEvent) error {
	_, err := r.db.Exec(
		`INSERT INTO security_events (event_type, user_id, ip_address, user_agent, details, severity)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		event.EventType, event.UserID, event.IPAddress, event.UserAgent, event.Details, event.Severity,
	)
	if err != nil {
		return fmt.Errorf("logging security event: %w", err)
	}
	return nil
}

// ListRecent returns recent security events.
func (r *SecurityEventRepository) ListRecent(limit int) ([]models.SecurityEvent, error) {
	rows, err := r.db.Query(
		`SELECT id, event_type, user_id, ip_address, user_agent, details, severity, created_at
		FROM security_events ORDER BY created_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing security events: %w", err)
	}
	defer rows.Close()

	var events []models.SecurityEvent
	for rows.Next() {
		var e models.SecurityEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.UserID, &e.IPAddress, &e.UserAgent,
			&e.Details, &e.Severity, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning security event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}
