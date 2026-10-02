package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"strings"
	"unicode/utf8"
)

func (s *TemplateService) EnableSessions(sessions *SessionService) { s.sessions = sessions }
func (s *TemplateService) templateActorTx(ctx context.Context, tx *sql.Tx, p policy.Principal) error {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil || p.ActorID != p.SubjectID {
		return vault.ErrDenied
	}
	if s.sessions != nil {
		return s.sessions.RequireActiveTx(ctx, tx, p.SubjectID, p.SessionID)
	}
	return nil
}
func validTemplate(name, stack string, keys models.JSONMap) bool {
	return utf8.ValidString(name) && strings.TrimSpace(name) != "" && utf8.RuneCountInString(name) <= 255 && strings.TrimSpace(stack) != "" && keys != nil
}
func (s *TemplateService) authorizeTemplateMutationTx(tx *sql.Tx, actor uuid.UUID, t *models.SecretTemplate) error {
	if t.IsGlobal && s.sessions != nil {
		return vault.ErrDenied
	} // builtin/global publication is operator-managed, not signup authority
	if t.OrganizationID != nil {
		ok, err := s.templateRepo.HasOrganizationRoleTx(tx, actor, *t.OrganizationID, "admin")
		if err != nil {
			return err
		}
		if !ok {
			return ErrTemplateNotFound
		}
		return nil
	}
	if t.CreatedBy != actor {
		return ErrTemplateNotFound
	}
	return nil
}
func (s *TemplateService) templateEventTx(ctx context.Context, tx *sql.Tx, actor uuid.UUID, action string, t *models.SecretTemplate, ip string) error {
	if s.auditRepo == nil {
		return vault.ErrDenied
	}
	details := models.JSONMap{"template_id": t.ID.String()}
	if t.OrganizationID != nil {
		details["organization_id"] = t.OrganizationID.String()
	}
	if err := s.auditRepo.CreateTx(tx, &actor, nil, action, "", details, ip); err != nil {
		return err
	}
	if s.templateRepo.Dialect().DBType() != repository.DBTypePostgres {
		return nil
	}
	payload, err := json.Marshal(map[string]any{"actor_id": actor, "action": action, "details": details})
	if err != nil {
		return err
	}
	return jobs.EnqueueTx(ctx, tx, uuid.New(), "template.event", payload, "local", 5)
}
func (s *TemplateService) CreateAuthorized(ctx context.Context, p policy.Principal, name, description, stack string, keys models.JSONMap, org *uuid.UUID, global bool, ip string) (*models.SecretTemplate, error) {
	if !validTemplate(name, stack, keys) {
		return nil, vault.ErrInvalid
	}
	var result *models.SecretTemplate
	err := s.templateRepo.WithTx(func(tx *sql.Tx) error {
		if err := s.templateActorTx(ctx, tx, p); err != nil {
			return err
		}
		if global && s.sessions != nil {
			return vault.ErrDenied
		}
		if org != nil {
			ok, err := s.templateRepo.HasOrganizationRoleTx(tx, p.ActorID, *org, "admin")
			if err != nil {
				return err
			}
			if !ok {
				return vault.ErrDenied
			}
		}
		var err error
		result, err = s.templateRepo.CreateTx(tx, uuid.New(), name, description, stack, keys, p.ActorID, org, global)
		if err != nil {
			return err
		}
		return s.templateEventTx(ctx, tx, p.ActorID, "template.created", result, ip)
	})
	return result, err
}
func (s *TemplateService) UpdateAuthorized(ctx context.Context, p policy.Principal, id uuid.UUID, name, description, stack string, keys models.JSONMap, ip string) (*models.SecretTemplate, error) {
	if !validTemplate(name, stack, keys) {
		return nil, vault.ErrInvalid
	}
	var result *models.SecretTemplate
	err := s.templateRepo.WithTx(func(tx *sql.Tx) error {
		if err := s.templateActorTx(ctx, tx, p); err != nil {
			return err
		}
		stored, err := s.templateRepo.GetByIDTx(tx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTemplateNotFound
		}
		if err != nil {
			return err
		}
		if err = s.authorizeTemplateMutationTx(tx, p.ActorID, stored); err != nil {
			return err
		}
		result, err = s.templateRepo.UpdateTx(tx, id, name, description, stack, keys)
		if err != nil {
			return err
		}
		return s.templateEventTx(ctx, tx, p.ActorID, "template.updated", result, ip)
	})
	return result, err
}
func (s *TemplateService) DeleteAuthorized(ctx context.Context, p policy.Principal, id uuid.UUID, ip string) error {
	return s.templateRepo.WithTx(func(tx *sql.Tx) error {
		if err := s.templateActorTx(ctx, tx, p); err != nil {
			return err
		}
		stored, err := s.templateRepo.GetByIDTx(tx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTemplateNotFound
		}
		if err != nil {
			return err
		}
		if err = s.authorizeTemplateMutationTx(tx, p.ActorID, stored); err != nil {
			return err
		}
		if err = s.templateRepo.DeleteTx(tx, id); err != nil {
			return err
		}
		return s.templateEventTx(ctx, tx, p.ActorID, "template.deleted", stored, ip)
	})
}
