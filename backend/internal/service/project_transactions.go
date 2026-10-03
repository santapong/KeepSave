package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

func (s *ProjectService) EnableSessions(sessions *SessionService) { s.sessions = sessions }
func (s *ProjectService) GetByIDAuthorized(ctx context.Context, p policy.Principal, id uuid.UUID) (*models.Project, error) {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil || p.ActorID != p.SubjectID {
		return nil, vault.ErrDenied
	}
	var result *models.Project
	err := s.projectRepo.WithTx(func(tx *sql.Tx) error {
		if s.projectRepo.Dialect().DBType() == repository.DBTypePostgres || s.sessions != nil {
			if e := (authority.Guard{Dialect: s.projectRepo.Dialect()}).LockProject(ctx, tx, p, id, true); e != nil {
				return vault.ErrDenied
			}
		}
		decision, err := (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: s.projectRepo.Dialect(), RequireHumanSession: s.sessions != nil}}).Authorize(ctx, p, policy.ReadMetadata, policy.Resource{Type: "project", ProjectID: id, ID: id})
		if err != nil {
			return err
		}
		if !decision.Allowed {
			return vault.ErrDenied
		}
		result, err = s.projectRepo.GetByIDTx(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (s *ProjectService) UpdateAuthorized(ctx context.Context, p policy.Principal, id uuid.UUID, name, description, ip string) (*models.Project, error) {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil || p.ActorID != p.SubjectID {
		return nil, vault.ErrDenied
	}
	var result *models.Project
	err := s.projectRepo.WithTx(func(tx *sql.Tx) error {
		if s.projectRepo.Dialect().DBType() == repository.DBTypePostgres || s.sessions != nil {
			if e := (authority.Guard{Dialect: s.projectRepo.Dialect()}).LockProject(ctx, tx, p, id, true); e != nil {
				return vault.ErrDenied
			}
		}
		if s.sessions != nil {
			if e := s.sessions.RequireActiveTx(ctx, tx, p.SubjectID, p.SessionID); e != nil {
				return e
			}
		}
		if e := s.projectRepo.RequireOwnedTx(ctx, tx, id, p.SubjectID); e != nil {
			return e
		}
		if e := s.projectRepo.UpdateMetadataTx(ctx, tx, id, name, description); e != nil {
			return e
		}
		if e := s.projectEventTx(ctx, tx, p, id, "project.updated", models.JSONMap{"name": name}, ip); e != nil {
			return e
		}
		var e error
		result, e = s.projectRepo.GetByIDTx(ctx, tx, id)
		return e
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (s *ProjectService) projectEventTx(ctx context.Context, tx *sql.Tx, p policy.Principal, id uuid.UUID, action string, details models.JSONMap, ip string) error {
	if err := s.auditRepo.CreateTx(tx, &p.ActorID, &id, action, "", details, ip); err != nil {
		return err
	}
	if s.projectRepo.Dialect().DBType() == repository.DBTypePostgres {
		payload, _ := json.Marshal(map[string]string{"project_id": id.String(), "action": action})
		return jobs.EnqueueTx(ctx, tx, uuid.New(), "project.event", payload, "local", 5)
	}
	return nil
}
func (s *ProjectService) UpdateEmbedConfigAuthorized(ctx context.Context, p policy.Principal, id uuid.UUID, origins []string, enabled bool, ip string) error {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil || p.ActorID != p.SubjectID {
		return vault.ErrDenied
	}
	for _, origin := range origins {
		if origin == "*" {
			return ErrWildcardOriginNotPermitted
		}
	}
	return s.projectRepo.WithTx(func(tx *sql.Tx) error {
		if s.projectRepo.Dialect().DBType() == repository.DBTypePostgres || s.sessions != nil {
			if e := (authority.Guard{Dialect: s.projectRepo.Dialect()}).LockProject(ctx, tx, p, id, true); e != nil {
				return vault.ErrDenied
			}
		}
		if s.sessions != nil {
			if e := s.sessions.RequireActiveTx(ctx, tx, p.SubjectID, p.SessionID); e != nil {
				return e
			}
		}
		if e := s.projectRepo.RequireOwnedTx(ctx, tx, id, p.SubjectID); e != nil {
			return e
		}
		previous, e := s.projectRepo.EmbedOriginsTx(ctx, tx, id)
		if e != nil {
			return e
		}
		before, after := map[string]bool{}, map[string]bool{}
		for _, origin := range previous {
			before[origin] = true
		}
		for _, origin := range origins {
			after[origin] = true
		}
		added, removed := []string{}, []string{}
		for _, origin := range origins {
			if !before[origin] {
				added = append(added, origin)
			}
		}
		for _, origin := range previous {
			if !after[origin] {
				removed = append(removed, origin)
			}
		}
		if e = s.projectRepo.UpdateEmbedConfigTx(ctx, tx, id, origins, enabled); e != nil {
			return e
		}
		return s.projectEventTx(ctx, tx, p, id, "embed.origins_updated", models.JSONMap{"allowed_origins": origins, "added": added, "removed": removed}, ip)
	})
}
