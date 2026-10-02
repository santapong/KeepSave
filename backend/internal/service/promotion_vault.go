package service

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

func (s *PromotionService) EnableVault(v *vault.Service) { s.vault = v }
func (s *PromotionService) DiffAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, source, target string, keys []string) ([]models.DiffEntry, error) {
	if s.vault == nil {
		return s.Diff(project, source, target, keys)
	}
	if p.Kind != policy.Human {
		return nil, vault.ErrDenied
	}
	if err := s.validateEnvironmentOrder(source, target); err != nil {
		return nil, vault.ErrInvalid
	}
	return s.vault.PromotionDiff(ctx, p, project, source, target, keys)
}
func (s *PromotionService) PromoteAuthorized(ctx context.Context, p policy.Principal, project uuid.UUID, source, target string, keys []string, override, notes, ip string) (*models.PromotionRequest, error) {
	if s.vault == nil {
		return s.Promote(project, source, target, keys, override, notes, p.ActorID, ip)
	}
	if p.Kind != policy.Human {
		return nil, vault.ErrDenied
	}
	if err := s.validateEnvironmentOrder(source, target); err != nil {
		return nil, vault.ErrInvalid
	}
	if override == "" {
		override = "skip"
	}
	if override != "skip" && override != "overwrite" {
		return nil, vault.ErrInvalid
	}
	var pr *models.PromotionRequest
	err := s.vault.Transaction(ctx, p, project, policy.WriteSecret, func(tx *sql.Tx) error {
		var err error
		pr, err = s.promotionRepo.CreateTx(ctx, tx, project, source, target, p.ActorID, keys, override, notes)
		if err != nil {
			return err
		}
		if err = s.vault.BindPromotionTx(ctx, tx, p, pr); err != nil {
			return err
		}
		if err = s.vault.EventTx(ctx, tx, p, project, "promotion_requested", target); err != nil {
			return err
		}
		if target == "prod" {
			return nil
		}
		if err = s.vault.ApplyPromotionTx(ctx, tx, p, pr); err != nil {
			return err
		}
		claimed, err := s.promotionRepo.CompareAndSetStatusTx(tx, pr.ID, "pending", "completed", nil)
		if err != nil {
			return err
		}
		if !claimed {
			return vault.ErrConflict
		}
		if err = s.vault.EventTx(ctx, tx, p, project, "promotion_completed", target); err != nil {
			return err
		}
		pr, err = s.promotionRepo.GetByIDTx(ctx, tx, pr.ID, project)
		return err
	})
	return pr, err
}
func (s *PromotionService) ApprovePromotionAuthorized(ctx context.Context, p policy.Principal, project, id uuid.UUID, ip string) (*models.PromotionRequest, error) {
	if s.vault == nil {
		return s.ApprovePromotion(id, p.ActorID, ip)
	}
	if p.Kind != policy.Human {
		return nil, vault.ErrDenied
	}
	var pr *models.PromotionRequest
	err := s.vault.Transaction(ctx, p, project, policy.ApprovePromotion, func(tx *sql.Tx) error {
		var err error
		pr, err = s.promotionRepo.GetByIDTx(ctx, tx, id, project)
		if err != nil {
			return vault.ErrNotFound
		}
		if pr.RequestedBy == p.ActorID {
			return ErrSelfApproval
		}
		if pr.Status != "pending" {
			return vault.ErrConflict
		}
		if err = s.vault.EventTx(ctx, tx, p, project, "promotion_approved", pr.TargetEnvironment); err != nil {
			return err
		}
		if err = s.vault.ApplyPromotionTx(ctx, tx, p, pr); err != nil {
			return err
		}
		claimed, err := s.promotionRepo.CompareAndSetStatusTx(tx, id, "pending", "completed", &p.ActorID)
		if err != nil {
			return err
		}
		if !claimed {
			return vault.ErrConflict
		}
		if err = s.vault.EventTx(ctx, tx, p, project, "promotion_completed", pr.TargetEnvironment); err != nil {
			return err
		}
		pr, err = s.promotionRepo.GetByIDTx(ctx, tx, id, project)
		return err
	})
	return pr, err
}
func (s *PromotionService) RejectPromotionAuthorized(ctx context.Context, p policy.Principal, project, id uuid.UUID, ip string) (*models.PromotionRequest, error) {
	if s.vault == nil {
		return s.RejectPromotion(id, p.ActorID, ip)
	}
	if p.Kind != policy.Human {
		return nil, vault.ErrDenied
	}
	var pr *models.PromotionRequest
	err := s.vault.Transaction(ctx, p, project, policy.ApprovePromotion, func(tx *sql.Tx) error {
		var err error
		pr, err = s.promotionRepo.GetByIDTx(ctx, tx, id, project)
		if err != nil {
			return vault.ErrNotFound
		}
		claimed, err := s.promotionRepo.CompareAndSetStatusTx(tx, id, "pending", "rejected", &p.ActorID)
		if err != nil {
			return err
		}
		if !claimed {
			return vault.ErrConflict
		}
		if err = s.vault.EventTx(ctx, tx, p, project, "promotion_rejected", pr.TargetEnvironment); err != nil {
			return err
		}
		pr, err = s.promotionRepo.GetByIDTx(ctx, tx, id, project)
		return err
	})
	return pr, err
}
func (s *PromotionService) RollbackAuthorized(ctx context.Context, p policy.Principal, project, id uuid.UUID, ip string) error {
	if s.vault == nil {
		return s.Rollback(id, p.ActorID, ip)
	}
	if p.Kind != policy.Human {
		return vault.ErrDenied
	}
	return s.vault.Transaction(ctx, p, project, policy.ApprovePromotion, func(tx *sql.Tx) error {
		pr, err := s.promotionRepo.GetByIDTx(ctx, tx, id, project)
		if err != nil {
			return vault.ErrNotFound
		}
		if pr.Status != "completed" {
			return vault.ErrConflict
		}
		if err = s.vault.EventTx(ctx, tx, p, project, "promotion_rollback", pr.TargetEnvironment); err != nil {
			return err
		}
		if err = s.vault.RollbackPromotionTx(ctx, tx, p, pr); err != nil {
			return err
		}
		ok, err := s.promotionRepo.MarkRolledBackTx(tx, id)
		if err != nil {
			return err
		}
		if !ok {
			return vault.ErrConflict
		}
		return nil
	})
}
