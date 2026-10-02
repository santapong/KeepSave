package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"strings"
	"time"
)

func (s *APIKeyService) EnableSessions(sessions *SessionService) { s.sessions = sessions }
func (s *APIKeyService) CreateAuthorized(ctx context.Context, p policy.Principal, name string, project uuid.UUID, scopes []string, environment *string, expires *time.Time, ip string) (*CreateAPIKeyResponse, error) {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil || p.ActorID != p.SubjectID {
		return nil, vault.ErrDenied
	}
	if len(scopes) == 0 {
		scopes = []string{"read"}
	}
	for _, scope := range scopes {
		if !validAPIKeyScope(scope) {
			return nil, vault.ErrInvalid
		}
	}
	expiry, err := computeEffectiveAPIKeyExpiry(time.Now().UTC(), expires)
	if err != nil {
		return nil, err
	}
	raw, hash, err := auth.GenerateAPIKey()
	if err != nil {
		return nil, err
	}
	var k *models.APIKey
	err = s.apikeyRepo.WithTx(func(tx *sql.Tx) error {
		if s.sessions != nil {
			if e := s.sessions.RequireActiveTx(ctx, tx, p.SubjectID, p.SessionID); e != nil {
				return e
			}
		}
		if e := s.projectRepo.RequireOwnedTx(ctx, tx, project, p.SubjectID); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotAuthorized
			}
			return e
		}
		var e error
		k, e = s.apikeyRepo.CreateTx(ctx, tx, name, hash, p.SubjectID, project, scopes, environment, expiry)
		if e != nil {
			return e
		}
		env := ""
		if environment != nil {
			env = *environment
		}
		if e = s.auditRepo.CreateTx(tx, &p.ActorID, &project, "apikey.created", env, models.JSONMap{"key_id": k.ID.String(), "name": name, "scopes": scopes}, ip); e != nil {
			return e
		}
		if s.apikeyRepo.Dialect().DBType() == repository.DBTypePostgres {
			payload, _ := json.Marshal(map[string]string{"project_id": project.String(), "key_id": k.ID.String(), "action": "apikey.created"})
			return jobs.EnqueueTx(ctx, tx, uuid.New(), "identity.event", payload, "local", 5)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &CreateAPIKeyResponse{APIKey: k, RawKey: raw}, nil
}
func (s *APIKeyService) DeleteAuthorized(ctx context.Context, p policy.Principal, id uuid.UUID, ip string) error {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil || p.ActorID != p.SubjectID {
		return vault.ErrDenied
	}
	return s.apikeyRepo.WithTx(func(tx *sql.Tx) error {
		if s.sessions != nil {
			if e := s.sessions.RequireActiveTx(ctx, tx, p.SubjectID, p.SessionID); e != nil {
				return e
			}
		}
		k, e := s.apikeyRepo.GetByIDTx(ctx, tx, id)
		if e != nil || k.UserID != p.SubjectID {
			return vault.ErrNotFound
		}
		if e = s.projectRepo.RequireOwnedTx(ctx, tx, k.ProjectID, p.SubjectID); e != nil {
			return vault.ErrDenied
		}
		if e = s.apikeyRepo.DeleteTx(ctx, tx, id, p.SubjectID); e != nil {
			return e
		}
		if e = s.auditRepo.CreateTx(tx, &p.ActorID, &k.ProjectID, "apikey.deleted", "", models.JSONMap{"key_id": id.String()}, ip); e != nil {
			return e
		}
		if s.apikeyRepo.Dialect().DBType() == repository.DBTypePostgres {
			payload, _ := json.Marshal(map[string]string{"project_id": k.ProjectID.String(), "key_id": id.String(), "action": "apikey.deleted"})
			return jobs.EnqueueTx(ctx, tx, uuid.New(), "identity.event", payload, "local", 5)
		}
		return nil
	})
}

func validAPIKeyScope(scope string) bool {
	action, pattern, qualified := strings.Cut(scope, ":")
	if action != "read" && action != "write" && action != "delete" && action != "promote" {
		return false
	}
	return !qualified || (len(pattern) > 0 && len(pattern) <= 255 && !strings.ContainsAny(pattern, "\x00\r\n"))
}
