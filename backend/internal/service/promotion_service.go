package service

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

var envOrder = map[string]int{
	"alpha": 0,
	"uat":   1,
	"prod":  2,
}

type PromotionService struct {
	promotionRepo *repository.PromotionRepository
	secretRepo    *repository.SecretRepository
	projectRepo   *repository.ProjectRepository
	envRepo       *repository.EnvironmentRepository
	auditRepo     *repository.AuditRepository
	cryptoSvc     *crypto.Service
}

func NewPromotionService(
	promotionRepo *repository.PromotionRepository,
	secretRepo *repository.SecretRepository,
	projectRepo *repository.ProjectRepository,
	envRepo *repository.EnvironmentRepository,
	auditRepo *repository.AuditRepository,
	cryptoSvc *crypto.Service,
) *PromotionService {
	return &PromotionService{
		promotionRepo: promotionRepo,
		secretRepo:    secretRepo,
		projectRepo:   projectRepo,
		envRepo:       envRepo,
		auditRepo:     auditRepo,
		cryptoSvc:     cryptoSvc,
	}
}

func (s *PromotionService) validateEnvironmentOrder(source, target string) error {
	srcOrder, srcOk := envOrder[source]
	tgtOrder, tgtOk := envOrder[target]
	if !srcOk || !tgtOk {
		return fmt.Errorf("invalid environment name")
	}
	if srcOrder >= tgtOrder {
		return fmt.Errorf("can only promote forward: %s -> %s is not allowed", source, target)
	}
	if tgtOrder-srcOrder > 1 {
		return fmt.Errorf("can only promote to the next environment: %s -> %s skips a stage", source, target)
	}
	return nil
}

func (s *PromotionService) decryptProjectDEK(project *models.Project) ([]byte, error) {
	dek, err := s.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting project DEK: %w", err)
	}
	return dek, nil
}

// Diff computes what would change if secrets were promoted from source to target environment.
func (s *PromotionService) Diff(projectID uuid.UUID, sourceEnv, targetEnv string, keysFilter []string) ([]models.DiffEntry, error) {
	if err := s.validateEnvironmentOrder(sourceEnv, targetEnv); err != nil {
		return nil, err
	}

	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	srcEnv, err := s.envRepo.GetByProjectAndName(projectID, sourceEnv)
	if err != nil {
		return nil, fmt.Errorf("getting source environment: %w", err)
	}

	tgtEnv, err := s.envRepo.GetByProjectAndName(projectID, targetEnv)
	if err != nil {
		return nil, fmt.Errorf("getting target environment: %w", err)
	}

	dek, err := s.decryptProjectDEK(project)
	if err != nil {
		return nil, err
	}

	srcSecrets, err := s.secretRepo.ListByProjectAndEnv(projectID, srcEnv.ID)
	if err != nil {
		return nil, fmt.Errorf("listing source secrets: %w", err)
	}

	tgtSecrets, err := s.secretRepo.ListByProjectAndEnv(projectID, tgtEnv.ID)
	if err != nil {
		return nil, fmt.Errorf("listing target secrets: %w", err)
	}

	// Build key filter set
	filterSet := make(map[string]bool)
	for _, k := range keysFilter {
		filterSet[k] = true
	}

	// Build target map
	tgtMap := make(map[string]models.Secret)
	for _, sec := range tgtSecrets {
		tgtMap[sec.Key] = sec
	}

	var diffs []models.DiffEntry

	for _, srcSec := range srcSecrets {
		if len(filterSet) > 0 && !filterSet[srcSec.Key] {
			continue
		}

		srcValue, err := crypto.Decrypt(dek, srcSec.EncryptedValue, srcSec.ValueNonce)
		if err != nil {
			return nil, fmt.Errorf("decrypting source secret %s: %w", srcSec.Key, err)
		}

		entry := models.DiffEntry{
			Key:          srcSec.Key,
			SourceValue:  string(srcValue),
			SourceExists: true,
		}

		if tgtSec, exists := tgtMap[srcSec.Key]; exists {
			tgtValue, err := crypto.Decrypt(dek, tgtSec.EncryptedValue, tgtSec.ValueNonce)
			if err != nil {
				return nil, fmt.Errorf("decrypting target secret %s: %w", srcSec.Key, err)
			}
			entry.TargetValue = string(tgtValue)
			entry.TargetExists = true

			if string(srcValue) == string(tgtValue) {
				entry.Action = "no_change"
			} else {
				entry.Action = "update"
			}
		} else {
			entry.Action = "add"
		}

		diffs = append(diffs, entry)
	}

	return diffs, nil
}

// Promote creates a promotion request and (for non-prod) executes it immediately.
// For prod promotions, it creates a pending request that requires approval.
func (s *PromotionService) Promote(
	projectID uuid.UUID,
	sourceEnv, targetEnv string,
	keysFilter []string,
	overridePolicy string,
	notes string,
	userID uuid.UUID,
	ipAddress string,
) (*models.PromotionRequest, error) {
	if err := s.validateEnvironmentOrder(sourceEnv, targetEnv); err != nil {
		return nil, err
	}

	if overridePolicy == "" {
		overridePolicy = "skip"
	}
	if overridePolicy != "skip" && overridePolicy != "overwrite" {
		return nil, fmt.Errorf("invalid override_policy: must be 'skip' or 'overwrite'")
	}

	// Create promotion request
	promotion, err := s.promotionRepo.Create(projectID, sourceEnv, targetEnv, userID, keysFilter, overridePolicy, notes)
	if err != nil {
		return nil, fmt.Errorf("creating promotion request: %w", err)
	}

	// For PROD promotions, require approval workflow
	if targetEnv == "prod" {
		// Log audit for promotion request
		s.auditRepo.Create(&userID, &projectID, "promotion_requested", targetEnv, models.JSONMap{
			"promotion_id":       promotion.ID.String(),
			"source_environment": sourceEnv,
			"target_environment": targetEnv,
			"status":             "pending_approval",
		}, ipAddress)
		return promotion, nil
	}

	// For non-prod, execute immediately
	if err := s.executePromotion(promotion, userID, ipAddress); err != nil {
		// Mark as rejected on failure
		s.promotionRepo.UpdateStatus(promotion.ID, "rejected", nil)
		return nil, fmt.Errorf("executing promotion: %w", err)
	}

	// Refresh promotion status
	promotion, err = s.promotionRepo.GetByID(promotion.ID)
	if err != nil {
		return nil, fmt.Errorf("refreshing promotion: %w", err)
	}

	return promotion, nil
}

// ApprovePromotion approves and executes a pending PROD promotion.
func (s *PromotionService) ApprovePromotion(promotionID, approverID uuid.UUID, ipAddress string) (*models.PromotionRequest, error) {
	promotion, err := s.promotionRepo.GetByID(promotionID)
	if err != nil {
		return nil, fmt.Errorf("getting promotion: %w", err)
	}

	if promotion.Status != "pending" {
		return nil, fmt.Errorf("promotion is not pending approval (status: %s)", promotion.Status)
	}

	if err := s.promotionRepo.UpdateStatus(promotionID, "approved", &approverID); err != nil {
		return nil, fmt.Errorf("approving promotion: %w", err)
	}

	if err := s.executePromotion(promotion, approverID, ipAddress); err != nil {
		s.promotionRepo.UpdateStatus(promotionID, "rejected", &approverID)
		return nil, fmt.Errorf("executing promotion: %w", err)
	}

	promotion, err = s.promotionRepo.GetByID(promotionID)
	if err != nil {
		return nil, fmt.Errorf("refreshing promotion: %w", err)
	}

	return promotion, nil
}

// RejectPromotion rejects a pending promotion request.
func (s *PromotionService) RejectPromotion(promotionID, rejecterID uuid.UUID, ipAddress string) (*models.PromotionRequest, error) {
	promotion, err := s.promotionRepo.GetByID(promotionID)
	if err != nil {
		return nil, fmt.Errorf("getting promotion: %w", err)
	}

	if promotion.Status != "pending" {
		return nil, fmt.Errorf("promotion is not pending (status: %s)", promotion.Status)
	}

	if err := s.promotionRepo.UpdateStatus(promotionID, "rejected", &rejecterID); err != nil {
		return nil, fmt.Errorf("rejecting promotion: %w", err)
	}

	s.auditRepo.Create(&rejecterID, &promotion.ProjectID, "promotion_rejected", promotion.TargetEnvironment, models.JSONMap{
		"promotion_id":       promotionID.String(),
		"source_environment": promotion.SourceEnvironment,
		"target_environment": promotion.TargetEnvironment,
	}, ipAddress)

	promotion, err = s.promotionRepo.GetByID(promotionID)
	if err != nil {
		return nil, fmt.Errorf("refreshing promotion: %w", err)
	}

	return promotion, nil
}

// executePromotion performs the actual secret copy between environments.
func (s *PromotionService) executePromotion(promotion *models.PromotionRequest, executorID uuid.UUID, ipAddress string) error {
	project, err := s.projectRepo.GetByID(promotion.ProjectID)
	if err != nil {
		return fmt.Errorf("getting project: %w", err)
	}

	srcEnv, err := s.envRepo.GetByProjectAndName(promotion.ProjectID, promotion.SourceEnvironment)
	if err != nil {
		return fmt.Errorf("getting source environment: %w", err)
	}

	tgtEnv, err := s.envRepo.GetByProjectAndName(promotion.ProjectID, promotion.TargetEnvironment)
	if err != nil {
		return fmt.Errorf("getting target environment: %w", err)
	}

	dek, err := s.decryptProjectDEK(project)
	if err != nil {
		return err
	}

	srcSecrets, err := s.secretRepo.ListByProjectAndEnv(promotion.ProjectID, srcEnv.ID)
	if err != nil {
		return fmt.Errorf("listing source secrets: %w", err)
	}

	// Build key filter set
	filterSet := make(map[string]bool)
	for _, k := range promotion.KeysFilter {
		filterSet[k] = true
	}

	promotedKeys := []string{}
	skippedKeys := []string{}

	for _, srcSec := range srcSecrets {
		if len(filterSet) > 0 && !filterSet[srcSec.Key] {
			continue
		}

		// Check if target already has this key
		existingTarget, err := s.secretRepo.GetByEnvAndKey(tgtEnv.ID, srcSec.Key)
		if err != nil && err.Error() != fmt.Sprintf("getting secret by env and key: %s", sql.ErrNoRows.Error()) {
			// Key doesn't exist in target - check if it's actually a not-found error
			existingTarget = nil
		}

		if existingTarget != nil && promotion.OverridePolicy == "skip" {
			skippedKeys = append(skippedKeys, srcSec.Key)
			continue
		}

		// Snapshot existing target value before overwrite (for rollback)
		if existingTarget != nil {
			s.promotionRepo.CreateSnapshot(
				promotion.ID, tgtEnv.ID, existingTarget.Key,
				existingTarget.EncryptedValue, existingTarget.ValueNonce,
			)
		}

		// Decrypt source value and re-encrypt for target (same DEK since same project)
		srcValue, err := crypto.Decrypt(dek, srcSec.EncryptedValue, srcSec.ValueNonce)
		if err != nil {
			return fmt.Errorf("decrypting source secret %s: %w", srcSec.Key, err)
		}

		newEncrypted, newNonce, err := crypto.Encrypt(dek, srcValue)
		if err != nil {
			return fmt.Errorf("encrypting secret %s for target: %w", srcSec.Key, err)
		}

		_, err = s.secretRepo.Upsert(promotion.ProjectID, tgtEnv.ID, srcSec.Key, newEncrypted, newNonce)
		if err != nil {
			return fmt.Errorf("upserting secret %s: %w", srcSec.Key, err)
		}

		promotedKeys = append(promotedKeys, srcSec.Key)
	}

	// Mark promotion as completed
	if err := s.promotionRepo.UpdateStatus(promotion.ID, "completed", nil); err != nil {
		return fmt.Errorf("completing promotion: %w", err)
	}

	// Audit log
	s.auditRepo.Create(&executorID, &promotion.ProjectID, "promotion_completed", promotion.TargetEnvironment, models.JSONMap{
		"promotion_id":       promotion.ID.String(),
		"source_environment": promotion.SourceEnvironment,
		"target_environment": promotion.TargetEnvironment,
		"promoted_keys":      promotedKeys,
		"skipped_keys":       skippedKeys,
		"override_policy":    promotion.OverridePolicy,
	}, ipAddress)

	return nil
}

// Rollback restores secrets in the target environment to their pre-promotion state.
func (s *PromotionService) Rollback(promotionID, userID uuid.UUID, ipAddress string) error {
	promotion, err := s.promotionRepo.GetByID(promotionID)
	if err != nil {
		return fmt.Errorf("getting promotion: %w", err)
	}

	if promotion.Status != "completed" {
		return fmt.Errorf("can only rollback completed promotions (status: %s)", promotion.Status)
	}

	snapshots, err := s.promotionRepo.GetSnapshotsByPromotionID(promotionID)
	if err != nil {
		return fmt.Errorf("getting snapshots: %w", err)
	}

	restoredKeys := []string{}

	for _, snap := range snapshots {
		_, err := s.secretRepo.Upsert(
			promotion.ProjectID, snap.EnvironmentID,
			snap.Key, snap.EncryptedValue, snap.ValueNonce,
		)
		if err != nil {
			return fmt.Errorf("restoring secret %s: %w", snap.Key, err)
		}
		restoredKeys = append(restoredKeys, snap.Key)
	}

	// Audit log
	s.auditRepo.Create(&userID, &promotion.ProjectID, "promotion_rollback", promotion.TargetEnvironment, models.JSONMap{
		"promotion_id":       promotionID.String(),
		"source_environment": promotion.SourceEnvironment,
		"target_environment": promotion.TargetEnvironment,
		"restored_keys":      restoredKeys,
	}, ipAddress)

	return nil
}

// ListPromotions returns all promotion requests for a project.
func (s *PromotionService) ListPromotions(projectID uuid.UUID) ([]models.PromotionRequest, error) {
	return s.promotionRepo.ListByProjectID(projectID)
}

// GetPromotion returns a single promotion request.
func (s *PromotionService) GetPromotion(promotionID uuid.UUID) (*models.PromotionRequest, error) {
	return s.promotionRepo.GetByID(promotionID)
}

// ListAuditLog returns audit entries for a project.
func (s *PromotionService) ListAuditLog(projectID uuid.UUID, limit int) ([]models.AuditEntry, error) {
	return s.auditRepo.ListByProjectID(projectID, limit)
}
