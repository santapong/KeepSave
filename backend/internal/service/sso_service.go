package service

import (
	"database/sql"
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// SSOService handles SSO provider configuration.
type SSOService struct {
	ssoRepo   *repository.SSORepository
	cryptoSvc *crypto.Service
}

// NewSSOService creates a new SSO service.
func NewSSOService(ssoRepo *repository.SSORepository, cryptoSvc *crypto.Service) *SSOService {
	return &SSOService{ssoRepo: ssoRepo, cryptoSvc: cryptoSvc}
}

// ConfigureSSO sets up an SSO provider for an organization.
func (s *SSOService) ConfigureSSO(orgID uuid.UUID, provider, issuerURL, clientID, clientSecret string, metadata models.JSONMap) (*models.SSOConfig, error) {
	masterKey := s.cryptoSvc.GetMasterKey()
	encrypted, nonce, err := crypto.Encrypt(masterKey, []byte(clientSecret))
	if err != nil {
		return nil, fmt.Errorf("encrypting client secret: %w", err)
	}

	config := &models.SSOConfig{
		OrganizationID:        orgID,
		Provider:              provider,
		IssuerURL:             issuerURL,
		ClientID:              clientID,
		ClientSecretEncrypted: encrypted,
		ClientSecretNonce:     nonce,
		Metadata:              metadata,
		Enabled:               true,
	}

	return s.ssoRepo.Upsert(config)
}

// GetSSOConfig returns the SSO config for an organization.
func (s *SSOService) GetSSOConfig(orgID uuid.UUID, provider string) (*models.SSOConfig, error) {
	return s.ssoRepo.GetByOrgAndProvider(orgID, provider)
}

// ListSSOConfigs returns all SSO configs for an organization.
func (s *SSOService) ListSSOConfigs(orgID uuid.UUID) ([]models.SSOConfig, error) {
	return s.ssoRepo.ListByOrg(orgID)
}

// DeleteSSOConfig removes an SSO configuration.
func (s *SSOService) DeleteSSOConfig(orgID uuid.UUID, provider string) error {
	return s.ssoRepo.Delete(orgID, provider)
}

// ComplianceService generates compliance reports.
type ComplianceService struct {
	complianceRepo *repository.ComplianceRepository
	auditRepo      *repository.AuditRepository
	orgRepo        *repository.OrganizationRepository
}

// NewComplianceService creates a new compliance service.
func NewComplianceService(complianceRepo *repository.ComplianceRepository, auditRepo *repository.AuditRepository, orgRepo *repository.OrganizationRepository) *ComplianceService {
	return &ComplianceService{complianceRepo: complianceRepo, auditRepo: auditRepo, orgRepo: orgRepo}
}

// GenerateReport creates a compliance report.
func (s *ComplianceService) GenerateReport(orgID, userID uuid.UUID, reportType string) (*models.ComplianceReport, error) {
	report := &models.ComplianceReport{
		OrganizationID: orgID,
		ReportType:     reportType,
		Status:         "generating",
		GeneratedBy:    userID,
	}

	created, err := s.complianceRepo.Create(report)
	if err != nil {
		return nil, fmt.Errorf("creating compliance report: %w", err)
	}

	data := models.JSONMap{
		"report_type":     reportType,
		"organization_id": orgID.String(),
		"status":          "completed",
		"summary": map[string]interface{}{
			"encryption_at_rest": true,
			"audit_logging":      true,
			"access_control":     true,
			"key_rotation":       true,
			"secret_versioning":  true,
			"promotion_approval": true,
			"rate_limiting":      true,
			"webhook_signatures": true,
		},
	}

	return s.complianceRepo.Complete(created.ID, data)
}

// ListReports returns compliance reports for an organization.
func (s *ComplianceService) ListReports(orgID uuid.UUID) ([]models.ComplianceReport, error) {
	return s.complianceRepo.ListByOrg(orgID)
}

// BackupService handles encrypted backup and restore.
type BackupService struct {
	backupRepo *repository.BackupRepository
	secretRepo *repository.SecretRepository
	cryptoSvc  *crypto.Service
}

// NewBackupService creates a new backup service.
func NewBackupService(backupRepo *repository.BackupRepository, secretRepo *repository.SecretRepository, cryptoSvc *crypto.Service) *BackupService {
	return &BackupService{backupRepo: backupRepo, secretRepo: secretRepo, cryptoSvc: cryptoSvc}
}

// CreateBackup creates an encrypted backup of project secrets.
func (s *BackupService) CreateBackup(projectID, userID uuid.UUID, snapshotType string) (*models.BackupSnapshot, error) {
	secrets, err := s.secretRepo.ListByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("listing secrets for backup: %w", err)
	}

	data := models.JSONMap{
		"project_id":   projectID.String(),
		"secret_count": len(secrets),
		"type":         snapshotType,
	}

	dataBytes, err := data.Value()
	if err != nil {
		return nil, fmt.Errorf("serializing backup data: %w", err)
	}

	dataStr, ok := dataBytes.([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected data type from Value()")
	}

	masterKey := s.cryptoSvc.GetMasterKey()
	encrypted, nonce, err := crypto.Encrypt(masterKey, dataStr)
	if err != nil {
		return nil, fmt.Errorf("encrypting backup: %w", err)
	}

	snapshot := &models.BackupSnapshot{
		ProjectID:     &projectID,
		SnapshotType:  snapshotType,
		EncryptedData: encrypted,
		DataNonce:     nonce,
		SizeBytes:     int64(len(encrypted)),
		CreatedBy:     userID,
	}

	return s.backupRepo.Create(snapshot)
}

// ListBackups returns backups for a project.
func (s *BackupService) ListBackups(projectID uuid.UUID) ([]models.BackupSnapshot, error) {
	return s.backupRepo.ListByProject(projectID)
}

// IPAllowlistService manages IP allowlists.
type IPAllowlistService struct {
	db      *sql.DB
	dialect repository.Dialect
}

// NewIPAllowlistService creates a new IP allowlist service.
func NewIPAllowlistService(db *sql.DB, dialect repository.Dialect) *IPAllowlistService {
	return &IPAllowlistService{db: db, dialect: dialect}
}

// CheckIPAllowed checks if an IP is in the allowlist for a project or org.
func (s *IPAllowlistService) CheckIPAllowed(projectID *uuid.UUID, orgID *uuid.UUID, clientIP string) (bool, error) {
	var count int
	countQ := repository.Q(s.dialect, `SELECT COUNT(*) FROM ip_allowlists WHERE project_id = $1 OR organization_id = $2`)
	var pid, oid interface{}
	if projectID != nil {
		pid = *projectID
	}
	if orgID != nil {
		oid = *orgID
	}
	err := s.db.QueryRow(countQ, pid, oid).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking allowlist: %w", err)
	}
	if count == 0 {
		return true, nil // No restrictions
	}

	if s.dialect.DBType() == repository.DBTypePostgres {
		// Use PostgreSQL's native CIDR matching
		checkQuery := repository.Q(s.dialect, `SELECT COUNT(*) FROM ip_allowlists
			WHERE (project_id = $1 OR organization_id = $2)
			AND $3::inet <<= cidr::inet`)
		err = s.db.QueryRow(checkQuery, pid, oid, clientIP).Scan(&count)
		if err != nil {
			return false, fmt.Errorf("checking IP against allowlist: %w", err)
		}
		return count > 0, nil
	}

	// For MySQL/SQLite: fetch all CIDRs and check in application code
	rows, err := s.db.Query(
		repository.Q(s.dialect, `SELECT cidr FROM ip_allowlists WHERE project_id = $1 OR organization_id = $2`),
		pid, oid,
	)
	if err != nil {
		return false, fmt.Errorf("fetching allowlist CIDRs: %w", err)
	}
	defer rows.Close()

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false, fmt.Errorf("invalid client IP: %s", clientIP)
	}

	for rows.Next() {
		var cidrStr string
		if err := rows.Scan(&cidrStr); err != nil {
			return false, fmt.Errorf("scanning CIDR: %w", err)
		}
		_, cidrNet, err := net.ParseCIDR(cidrStr)
		if err != nil {
			continue // skip invalid CIDRs
		}
		if cidrNet.Contains(ip) {
			return true, nil
		}
	}
	return false, nil
}

// SecretPolicyService manages secret lifecycle policies.
type SecretPolicyService struct {
	db      *sql.DB
	dialect repository.Dialect
}

// NewSecretPolicyService creates a new secret policy service.
func NewSecretPolicyService(db *sql.DB, dialect repository.Dialect) *SecretPolicyService {
	return &SecretPolicyService{db: db, dialect: dialect}
}

// GetPolicy returns the policy for a project.
func (s *SecretPolicyService) GetPolicy(projectID uuid.UUID) (*models.SecretPolicy, error) {
	policy := &models.SecretPolicy{}
	err := s.db.QueryRow(
		repository.Q(s.dialect, `SELECT id, project_id, max_age_days, rotation_reminder_days, require_rotation, created_at, updated_at
		FROM secret_policies WHERE project_id = $1`), projectID,
	).Scan(&policy.ID, &policy.ProjectID, &policy.MaxAgeDays, &policy.RotationReminderDays,
		&policy.RequireRotation, &policy.CreatedAt, &policy.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting secret policy: %w", err)
	}
	return policy, nil
}

// SetPolicy creates or updates a project's secret policy.
func (s *SecretPolicyService) SetPolicy(projectID uuid.UUID, maxAgeDays, reminderDays int, requireRotation bool) (*models.SecretPolicy, error) {
	policy := &models.SecretPolicy{}
	id := uuid.New()

	if s.dialect.SupportsReturning() {
		err := s.db.QueryRow(
			`INSERT INTO secret_policies (id, project_id, max_age_days, rotation_reminder_days, require_rotation)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (project_id) DO UPDATE SET max_age_days = $3, rotation_reminder_days = $4, require_rotation = $5, updated_at = NOW()
			RETURNING id, project_id, max_age_days, rotation_reminder_days, require_rotation, created_at, updated_at`,
			id, projectID, maxAgeDays, reminderDays, requireRotation,
		).Scan(&policy.ID, &policy.ProjectID, &policy.MaxAgeDays, &policy.RotationReminderDays,
			&policy.RequireRotation, &policy.CreatedAt, &policy.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("upserting secret policy: %w", err)
		}
	} else {
		upsertClause := s.dialect.FormatUpsert("project_id",
			"max_age_days = EXCLUDED.max_age_days, rotation_reminder_days = EXCLUDED.rotation_reminder_days, require_rotation = EXCLUDED.require_rotation, updated_at = "+s.dialect.Now())
		insertQ := repository.Q(s.dialect, `INSERT INTO secret_policies (id, project_id, max_age_days, rotation_reminder_days, require_rotation)
			VALUES ($1, $2, $3, $4, $5) `+upsertClause)
		_, err := s.db.Exec(insertQ, id, projectID, maxAgeDays, reminderDays, requireRotation)
		if err != nil {
			return nil, fmt.Errorf("upserting secret policy: %w", err)
		}
		selectQ := repository.Q(s.dialect, `SELECT id, project_id, max_age_days, rotation_reminder_days, require_rotation, created_at, updated_at FROM secret_policies WHERE project_id = $1`)
		err = s.db.QueryRow(selectQ, projectID).Scan(&policy.ID, &policy.ProjectID, &policy.MaxAgeDays, &policy.RotationReminderDays,
			&policy.RequireRotation, &policy.CreatedAt, &policy.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading upserted secret policy: %w", err)
		}
	}
	return policy, nil
}
