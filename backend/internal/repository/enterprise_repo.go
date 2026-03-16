package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// SSORepository handles SSO configuration persistence.
type SSORepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewSSORepository(db *sql.DB, dialect Dialect) *SSORepository {
	return &SSORepository{db: db, dialect: dialect}
}

func (r *SSORepository) Upsert(config *models.SSOConfig) (*models.SSOConfig, error) {
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO sso_configs (id, organization_id, provider, issuer_url, client_id, client_secret_encrypted, client_secret_nonce, metadata, enabled)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (organization_id, provider) DO UPDATE SET issuer_url = $4, client_id = $5, client_secret_encrypted = $6, client_secret_nonce = $7, metadata = $8, enabled = $9, updated_at = NOW()
			RETURNING id, created_at, updated_at`,
			id, config.OrganizationID, config.Provider, config.IssuerURL, config.ClientID,
			config.ClientSecretEncrypted, config.ClientSecretNonce, config.Metadata, config.Enabled,
		).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("upserting SSO config: %w", err)
		}
	} else {
		upsertClause := r.dialect.FormatUpsert("organization_id, provider",
			"issuer_url = EXCLUDED.issuer_url, client_id = EXCLUDED.client_id, client_secret_encrypted = EXCLUDED.client_secret_encrypted, client_secret_nonce = EXCLUDED.client_secret_nonce, metadata = EXCLUDED.metadata, enabled = EXCLUDED.enabled, updated_at = "+r.dialect.Now())
		insertQ := Q(r.dialect, `INSERT INTO sso_configs (id, organization_id, provider, issuer_url, client_id, client_secret_encrypted, client_secret_nonce, metadata, enabled)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) `+upsertClause)
		_, err := r.db.Exec(insertQ, id, config.OrganizationID, config.Provider, config.IssuerURL, config.ClientID,
			config.ClientSecretEncrypted, config.ClientSecretNonce, config.Metadata, config.Enabled)
		if err != nil {
			return nil, fmt.Errorf("upserting SSO config: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, created_at, updated_at FROM sso_configs WHERE organization_id = $1 AND provider = $2`)
		err = r.db.QueryRow(selectQ, config.OrganizationID, config.Provider).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading upserted SSO config: %w", err)
		}
	}
	return config, nil
}

func (r *SSORepository) GetByOrgAndProvider(orgID uuid.UUID, provider string) (*models.SSOConfig, error) {
	config := &models.SSOConfig{}
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT id, organization_id, provider, issuer_url, client_id, client_secret_encrypted, client_secret_nonce, metadata, enabled, created_at, updated_at
		FROM sso_configs WHERE organization_id = $1 AND provider = $2`),
		orgID, provider,
	).Scan(&config.ID, &config.OrganizationID, &config.Provider, &config.IssuerURL, &config.ClientID,
		&config.ClientSecretEncrypted, &config.ClientSecretNonce, &config.Metadata, &config.Enabled,
		&config.CreatedAt, &config.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting SSO config: %w", err)
	}
	return config, nil
}

func (r *SSORepository) ListByOrg(orgID uuid.UUID) ([]models.SSOConfig, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, organization_id, provider, issuer_url, client_id, metadata, enabled, created_at, updated_at
		FROM sso_configs WHERE organization_id = $1`), orgID,
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

func (r *SSORepository) Delete(orgID uuid.UUID, provider string) error {
	_, err := r.db.Exec(Q(r.dialect, `DELETE FROM sso_configs WHERE organization_id = $1 AND provider = $2`), orgID, provider)
	if err != nil {
		return fmt.Errorf("deleting SSO config: %w", err)
	}
	return nil
}

// ComplianceRepository handles compliance report persistence.
type ComplianceRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewComplianceRepository(db *sql.DB, dialect Dialect) *ComplianceRepository {
	return &ComplianceRepository{db: db, dialect: dialect}
}

func (r *ComplianceRepository) Create(report *models.ComplianceReport) (*models.ComplianceReport, error) {
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO compliance_reports (id, organization_id, report_type, status, generated_by)
			VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
			id, report.OrganizationID, report.ReportType, report.Status, report.GeneratedBy,
		).Scan(&report.ID, &report.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating compliance report: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO compliance_reports (id, organization_id, report_type, status, generated_by) VALUES ($1, $2, $3, $4, $5)`)
		_, err := r.db.Exec(insertQ, id, report.OrganizationID, report.ReportType, report.Status, report.GeneratedBy)
		if err != nil {
			return nil, fmt.Errorf("creating compliance report: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, created_at FROM compliance_reports WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&report.ID, &report.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading created compliance report: %w", err)
		}
	}
	return report, nil
}

func (r *ComplianceRepository) Complete(id uuid.UUID, data models.JSONMap) (*models.ComplianceReport, error) {
	report := &models.ComplianceReport{}

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			Q(r.dialect, `UPDATE compliance_reports SET status = 'completed', data = $2, completed_at = NOW()
			WHERE id = $1
			RETURNING id, organization_id, report_type, status, data, generated_by, created_at, completed_at`),
			id, data,
		).Scan(&report.ID, &report.OrganizationID, &report.ReportType, &report.Status, &report.Data,
			&report.GeneratedBy, &report.CreatedAt, &report.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("completing compliance report: %w", err)
		}
	} else {
		updateQ := Q(r.dialect, `UPDATE compliance_reports SET status = 'completed', data = $2, completed_at = `+r.dialect.Now()+` WHERE id = $1`)
		_, err := r.db.Exec(updateQ, id, data)
		if err != nil {
			return nil, fmt.Errorf("completing compliance report: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, organization_id, report_type, status, data, generated_by, created_at, completed_at FROM compliance_reports WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&report.ID, &report.OrganizationID, &report.ReportType, &report.Status, &report.Data,
			&report.GeneratedBy, &report.CreatedAt, &report.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("reading completed compliance report: %w", err)
		}
	}
	return report, nil
}

func (r *ComplianceRepository) ListByOrg(orgID uuid.UUID) ([]models.ComplianceReport, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, organization_id, report_type, status, data, generated_by, created_at, completed_at
		FROM compliance_reports WHERE organization_id = $1 ORDER BY created_at DESC`), orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing compliance reports: %w", err)
	}
	defer rows.Close()

	var reports []models.ComplianceReport
	for rows.Next() {
		var rp models.ComplianceReport
		if err := rows.Scan(&rp.ID, &rp.OrganizationID, &rp.ReportType, &rp.Status, &rp.Data,
			&rp.GeneratedBy, &rp.CreatedAt, &rp.CompletedAt); err != nil {
			return nil, fmt.Errorf("scanning compliance report: %w", err)
		}
		reports = append(reports, rp)
	}
	return reports, nil
}

// BackupRepository handles backup snapshot persistence.
type BackupRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewBackupRepository(db *sql.DB, dialect Dialect) *BackupRepository {
	return &BackupRepository{db: db, dialect: dialect}
}

func (r *BackupRepository) Create(snapshot *models.BackupSnapshot) (*models.BackupSnapshot, error) {
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO backup_snapshots (id, project_id, snapshot_type, encrypted_data, data_nonce, size_bytes, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`,
			id, snapshot.ProjectID, snapshot.SnapshotType, snapshot.EncryptedData, snapshot.DataNonce,
			snapshot.SizeBytes, snapshot.CreatedBy,
		).Scan(&snapshot.ID, &snapshot.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating backup snapshot: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO backup_snapshots (id, project_id, snapshot_type, encrypted_data, data_nonce, size_bytes, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7)`)
		_, err := r.db.Exec(insertQ, id, snapshot.ProjectID, snapshot.SnapshotType, snapshot.EncryptedData, snapshot.DataNonce,
			snapshot.SizeBytes, snapshot.CreatedBy)
		if err != nil {
			return nil, fmt.Errorf("creating backup snapshot: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, created_at FROM backup_snapshots WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&snapshot.ID, &snapshot.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading created backup snapshot: %w", err)
		}
	}
	return snapshot, nil
}

func (r *BackupRepository) ListByProject(projectID uuid.UUID) ([]models.BackupSnapshot, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, project_id, snapshot_type, size_bytes, created_by, created_at
		FROM backup_snapshots WHERE project_id = $1 ORDER BY created_at DESC`), projectID,
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
	db      *sql.DB
	dialect Dialect
}

func NewSecurityEventRepository(db *sql.DB, dialect Dialect) *SecurityEventRepository {
	return &SecurityEventRepository{db: db, dialect: dialect}
}

func (r *SecurityEventRepository) Log(event *models.SecurityEvent) error {
	_, err := r.db.Exec(
		Q(r.dialect, `INSERT INTO security_events (event_type, user_id, ip_address, user_agent, details, severity)
		VALUES ($1, $2, $3, $4, $5, $6)`),
		event.EventType, event.UserID, event.IPAddress, event.UserAgent, event.Details, event.Severity,
	)
	if err != nil {
		return fmt.Errorf("logging security event: %w", err)
	}
	return nil
}

func (r *SecurityEventRepository) ListRecent(limit int) ([]models.SecurityEvent, error) {
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, event_type, user_id, ip_address, user_agent, details, severity, created_at
		FROM security_events ORDER BY created_at DESC LIMIT $1`), limit,
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
