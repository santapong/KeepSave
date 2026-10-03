package runs

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/broker"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"time"
)

type ConnectionCheck struct {
	ID                     uuid.UUID `json:"id"`
	ConnectionID           uuid.UUID `json:"connection_id"`
	BindingID              uuid.UUID `json:"binding_id"`
	Provider               string    `json:"provider"`
	Status                 string    `json:"status"`
	RepositoryID           int64     `json:"repository_id"`
	Commit                 string    `json:"commit,omitempty"`
	ExternalRead           bool      `json:"external_read"`
	AuthorizationTokenMint bool      `json:"authorization_token_mint"`
	RepositoryWrite        bool      `json:"repository_write"`
}

// CheckConnection requires explicit acknowledgment of both repository metadata
// reads and short-lived authorization-token creation. It never writes a repo.
func (s *Service) CheckConnection(ctx context.Context, p policy.Principal, project, connectionID, bindingID uuid.UUID, externalRead, tokenMint bool) (ConnectionCheck, error) {
	if !s.flags.Admission || !externalRead || !tokenMint {
		return ConnectionCheck{}, ErrDenied
	}
	tx, e := s.begin(ctx, p, project, policy.ManageConnections, "")
	if e != nil {
		return ConnectionCheck{}, e
	}
	defer tx.Rollback()
	c, target, e := checkBindingTx(ctx, tx, project, connectionID, bindingID)
	if e != nil {
		return ConnectionCheck{}, e
	}
	id := uuid.New()
	deadline := earliest(time.Now().Add(30*time.Second), p.ExpiresAt)
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_connection_checks(id,project_id,connection_id,binding_id,actor_id,session_id,deadline) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, project, connectionID, bindingID, p.ActorID, p.SessionID, deadline); e != nil {
		return ConnectionCheck{}, e
	}
	if e = s.event(ctx, tx, p, project, "tool.connection.check.admitted", models.JSONMap{"check_id": id, "connection_id": connectionID, "binding_id": bindingID, "external_read": true, "authorization_token_mint": true, "repository_write": false}); e != nil {
		return ConnectionCheck{}, e
	}
	if e = tx.Commit(); e != nil {
		return ConnectionCheck{}, e
	}
	call, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	resolved, e := s.broker.ResolveAuthorized(call, c, target, func(ctx context.Context, stage string) error {
		return s.checkAdmission(ctx, p, project, id, connectionID, bindingID, stage)
	})
	status := "succeeded"
	if e != nil {
		status = "failed"
		if errors.Is(e, broker.ErrUncertain) {
			status = "uncertain"
		}
	}
	out := ConnectionCheck{id, connectionID, bindingID, "github_app", status, target.RepositoryID, resolved.Commit, true, true, false}
	finish, e2 := s.begin(ctx, p, project, policy.ManageConnections, "")
	if e2 != nil {
		return ConnectionCheck{}, e2
	}
	defer finish.Rollback()
	if _, _, e2 = checkBindingTx(ctx, finish, project, connectionID, bindingID); e2 != nil {
		return ConnectionCheck{}, e2
	}
	completed, e2 := finish.ExecContext(ctx, `UPDATE tool_connection_checks SET state=$2,finished_at=NOW() WHERE id=$1 AND actor_id=$3 AND session_id=$4 AND deadline>NOW()`, id, status, p.ActorID, p.SessionID)
	if e2 != nil {
		return ConnectionCheck{}, e2
	}
	changed, e2 := completed.RowsAffected()
	if e2 != nil || changed != 1 {
		return ConnectionCheck{}, ErrDenied
	}
	if e2 = s.event(ctx, finish, p, project, "tool.connection.check.completed", models.JSONMap{"check_id": id, "status": status, "repository_id": target.RepositoryID, "commit": resolved.Commit}); e2 != nil {
		return ConnectionCheck{}, e2
	}
	if e2 = finish.Commit(); e2 != nil {
		return ConnectionCheck{}, e2
	}
	return out, nil
}
func checkBindingTx(ctx context.Context, tx *sql.Tx, project, connectionID, bindingID uuid.UUID) (broker.Connection, broker.Target, error) {
	var c broker.Connection
	var t broker.Target
	e := tx.QueryRowContext(ctx, `SELECT c.app_id,c.installation_id,c.ciphertext,c.nonce,b.repository_id,b.owner_name,b.repository_name,b.reference FROM tool_connections c JOIN tool_bindings b ON b.connection_id=c.id AND b.project_id=c.project_id JOIN environments e ON e.id=b.environment_id AND e.project_id=c.project_id WHERE c.id=$1 AND c.project_id=$2 AND b.id=$3 AND c.revoked_at IS NULL AND b.revoked_at IS NULL AND e.name IN('development','alpha','uat') FOR SHARE OF c,b,e`, connectionID, project, bindingID).Scan(&c.AppID, &c.InstallationID, &c.Ciphertext, &c.Nonce, &t.RepositoryID, &t.Owner, &t.Repository, &t.Reference)
	if e != nil {
		return c, t, ErrDenied
	}
	return c, t, nil
}
func (s *Service) checkAdmission(ctx context.Context, p policy.Principal, project, id, conn, binding uuid.UUID, stage string) error {
	tx, e := s.begin(ctx, p, project, policy.ManageConnections, "")
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, _, e = checkBindingTx(ctx, tx, project, conn, binding); e != nil {
		return e
	}
	var calls int
	var state string
	e = tx.QueryRowContext(ctx, `SELECT provider_operations,state FROM tool_connection_checks WHERE id=$1 AND actor_id=$2 AND session_id=$3 AND deadline>NOW() FOR UPDATE`, id, p.ActorID, p.SessionID).Scan(&calls, &state)
	if e != nil || state != "admitted" && state != "dispatched" {
		return ErrDenied
	}
	if stage == "provider_request" {
		if calls >= 3 {
			return ErrDenied
		}
		if _, e = tx.ExecContext(ctx, `UPDATE tool_connection_checks SET provider_operations=provider_operations+1,state='dispatched' WHERE id=$1`, id); e != nil {
			return e
		}
	}
	if e = s.event(ctx, tx, p, project, "tool.connection.check.access", models.JSONMap{"check_id": id, "stage": stage}); e != nil {
		return e
	}
	return tx.Commit()
}
