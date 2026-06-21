package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// diffHashLen is the number of hex chars returned for each diff hash. 16
// hex chars = 64 bits, enough to make collision-driven false-equalities
// vanishingly rare while keeping the response compact and removing any
// realistic preimage exposure (HMAC truncation is safe per RFC 2104 §5).
const diffHashLen = 16

// hashSecretForDiff returns a per-project HMAC-SHA256 prefix of value.
// Using the project DEK as the MAC key means an attacker who observes a
// /promote/diff response cannot match the hash against guessed secret
// values without first stealing the DEK - the very thing the encryption
// scheme protects.
func hashSecretForDiff(dek, value []byte) string {
	m := hmac.New(sha256.New, dek)
	m.Write(value)
	sum := m.Sum(nil)
	return hex.EncodeToString(sum)[:diffHashLen]
}

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
			SourceHash:   hashSecretForDiff(dek, srcValue),
			SourceExists: true,
		}

		if tgtSec, exists := tgtMap[srcSec.Key]; exists {
			tgtValue, err := crypto.Decrypt(dek, tgtSec.EncryptedValue, tgtSec.ValueNonce)
			if err != nil {
				return nil, fmt.Errorf("decrypting target secret %s: %w", srcSec.Key, err)
			}
			entry.TargetHash = hashSecretForDiff(dek, tgtValue)
			entry.TargetExists = true

			// Equality via constant-time compare on the same-keyed HMAC is
			// equivalent to plaintext equality without revealing either value.
			if entry.SourceHash == entry.TargetHash {
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
	if err := s.runPromotion(promotion, userID, nil, ipAddress); err != nil {
		if !errors.Is(err, ErrPromotionNotPending) {
			s.promotionRepo.UpdateStatus(promotion.ID, "rejected", nil)
		}
		return nil, fmt.Errorf("executing promotion: %w", err)
	}

	// Refresh promotion status
	promotion, err = s.promotionRepo.GetByID(promotion.ID)
	if err != nil {
		return nil, fmt.Errorf("refreshing promotion: %w", err)
	}

	return promotion, nil
}

// ErrSelfApproval is returned when the approver is the same user that
// requested the promotion. Per ADR-0007 / audit S-H3, four-eyes is a hard
// invariant: a single compromised account cannot move secrets to PROD.
var ErrSelfApproval = errors.New("requester cannot approve their own promotion")

// ErrPromotionNotPending is returned when the compare-and-set claim finds the
// promotion already moved out of `pending` — i.e. a concurrent approver won the
// race and executed it (ADR-0017, P-02). Callers must NOT mark it rejected.
var ErrPromotionNotPending = errors.New("promotion is no longer pending")

// ApprovePromotion approves and executes a pending PROD promotion.
func (s *PromotionService) ApprovePromotion(promotionID, approverID uuid.UUID, ipAddress string) (*models.PromotionRequest, error) {
	promotion, err := s.promotionRepo.GetByID(promotionID)
	if err != nil {
		return nil, fmt.Errorf("getting promotion: %w", err)
	}

	if promotion.Status != "pending" {
		return nil, fmt.Errorf("promotion is not pending approval (status: %s)", promotion.Status)
	}

	if promotion.RequestedBy == approverID {
		return nil, ErrSelfApproval
	}

	// runPromotion claims (pending -> completed) and executes atomically; the
	// claim is the four-eyes-safe race guard. Record approverID as approved_by.
	if err := s.runPromotion(promotion, approverID, &approverID, ipAddress); err != nil {
		if errors.Is(err, ErrPromotionNotPending) {
			// Another approver won the race and already executed it; do not
			// flip the now-completed promotion to rejected.
			return nil, err
		}
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

// runPromotion executes the secret copy from source to target inside a single
// transaction (ADR-0017). It claims the request with a conditional
// pending -> completed update; the single matched row is the execution right,
// which closes the approve TOCTOU. Every written key is snapshotted with its
// prior state (overwrite vs. add) so Rollback can fully reverse the promotion.
// Any error rolls the whole transaction back, leaving status `pending`.
func (s *PromotionService) runPromotion(promotion *models.PromotionRequest, executorID uuid.UUID, approvedBy *uuid.UUID, ipAddress string) error {
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

	filterSet := make(map[string]bool)
	for _, k := range promotion.KeysFilter {
		filterSet[k] = true
	}

	var promotedKeys, skippedKeys []string

	txErr := s.promotionRepo.WithTx(func(tx *sql.Tx) error {
		// Claim the promotion: pending -> completed. The single affected row is
		// the right to execute; a concurrent approver loses here and rolls back.
		claimed, err := s.promotionRepo.CompareAndSetStatusTx(tx, promotion.ID, "pending", "completed", approvedBy)
		if err != nil {
			return err
		}
		if !claimed {
			return ErrPromotionNotPending
		}

		for _, srcSec := range srcSecrets {
			if len(filterSet) > 0 && !filterSet[srcSec.Key] {
				continue
			}

			existing, err := s.secretRepo.GetByEnvAndKeyTx(tx, tgtEnv.ID, srcSec.Key)
			if err != nil {
				return fmt.Errorf("checking target key %s: %w", srcSec.Key, err)
			}
			if existing != nil && promotion.OverridePolicy == "skip" {
				skippedKeys = append(skippedKeys, srcSec.Key)
				continue
			}

			// Snapshot prior state so rollback can restore (overwrite) or
			// delete (add) this key.
			if existing != nil {
				if err := s.promotionRepo.CreateSnapshotTx(tx, promotion.ID, tgtEnv.ID, existing.Key, existing.EncryptedValue, existing.ValueNonce, true); err != nil {
					return err
				}
			} else {
				if err := s.promotionRepo.CreateSnapshotTx(tx, promotion.ID, tgtEnv.ID, srcSec.Key, []byte{}, []byte{}, false); err != nil {
					return err
				}
			}

			// Decrypt source, re-encrypt with a fresh nonce under the same DEK.
			srcValue, err := crypto.Decrypt(dek, srcSec.EncryptedValue, srcSec.ValueNonce)
			if err != nil {
				return fmt.Errorf("decrypting source secret %s: %w", srcSec.Key, err)
			}
			newEncrypted, newNonce, err := crypto.Encrypt(dek, srcValue)
			if err != nil {
				return fmt.Errorf("encrypting secret %s for target: %w", srcSec.Key, err)
			}
			if err := s.secretRepo.UpsertTx(tx, promotion.ProjectID, tgtEnv.ID, srcSec.Key, newEncrypted, newNonce); err != nil {
				return fmt.Errorf("upserting secret %s: %w", srcSec.Key, err)
			}

			promotedKeys = append(promotedKeys, srcSec.Key)
		}
		return nil
	})
	if txErr != nil {
		return txErr
	}

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

	var restoredKeys, deletedKeys []string

	txErr := s.promotionRepo.WithTx(func(tx *sql.Tx) error {
		for _, snap := range snapshots {
			if snap.PriorExisted {
				if err := s.secretRepo.UpsertTx(tx, promotion.ProjectID, snap.EnvironmentID, snap.Key, snap.EncryptedValue, snap.ValueNonce); err != nil {
					return fmt.Errorf("restoring secret %s: %w", snap.Key, err)
				}
				restoredKeys = append(restoredKeys, snap.Key)
			} else {
				// The promotion added this key; rollback removes it.
				if err := s.secretRepo.DeleteByEnvAndKeyTx(tx, snap.EnvironmentID, snap.Key); err != nil {
					return fmt.Errorf("removing added secret %s: %w", snap.Key, err)
				}
				deletedKeys = append(deletedKeys, snap.Key)
			}
		}
		ok, err := s.promotionRepo.MarkRolledBackTx(tx, promotionID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("promotion was not in completed state at rollback")
		}
		return nil
	})
	if txErr != nil {
		return txErr
	}

	// Audit log
	s.auditRepo.Create(&userID, &promotion.ProjectID, "promotion_rollback", promotion.TargetEnvironment, models.JSONMap{
		"promotion_id":       promotionID.String(),
		"source_environment": promotion.SourceEnvironment,
		"target_environment": promotion.TargetEnvironment,
		"restored_keys":      restoredKeys,
		"deleted_keys":       deletedKeys,
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
