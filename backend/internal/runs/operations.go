package runs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/broker"
	"github.com/santapong/KeepSave/backend/internal/mcpgateway/catalog"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/runner"
	"time"
)

// Stored delegation is checked under the shared identity barrier before domain
// locks. Rotating a token never widens the authority of already admitted work.
func (s *Service) beginRun(ctx context.Context, p policy.Principal, client string, id uuid.UUID, cap policy.Capability) (*sql.Tx, error) {
	project, subjects, e := s.discoverRun(ctx, id)
	if e != nil {
		return nil, e
	}
	var parent policy.Principal
	var family, token uuid.NullUUID
	e = s.db.QueryRowContext(ctx, `SELECT parent_kind,actor_id,session_id,parent_family,parent_token,authority_expires_at FROM tool_runs WHERE id=$1`, id).Scan(&parent.Kind, &parent.ActorID, &parent.SessionID, &family, &token, &parent.ExpiresAt)
	if e != nil {
		return nil, ErrDenied
	}
	parent.SubjectID = parent.ActorID
	if family.Valid {
		parent.ParentGrantID = family.UUID
	}
	if token.Valid {
		parent.TokenID = token.UUID.String()
	}
	if cap == policy.InspectTeam {
		subjects = nil
	}
	tx, e := s.begin(ctx, p, project, cap, client, subjects...)
	if e != nil {
		return nil, e
	}
	if cap == policy.StartApprovedRun && parent.Kind == policy.OAuthDelegation && (s.RequireGrantTx == nil || s.RequireGrantTx(ctx, tx, parent, client) != nil) {
		tx.Rollback()
		return nil, ErrDenied
	}
	return tx, nil
}

type operationState struct {
	Operation
	Kind            string
	Args            Arguments
	Digest          string
	Parent          policy.Principal
	Fence           int64
	Workload        uuid.NullUUID
	Deadline, Lease sql.NullTime
}

func readOperation(ctx context.Context, tx *sql.Tx, id uuid.UUID, lock bool) (operationState, error) {
	var o operationState
	var b []byte
	var family, token uuid.NullUUID
	q := `SELECT o.id,o.run_id,o.status,o.outcome,o.expires_at,o.cancel_requested,o.kind,o.arguments,o.request_digest,o.parent_kind,o.parent_family,o.parent_token,o.authority_expires_at,r.actor_id,r.session_id,o.fence,o.workload_id,o.deadline,o.lease_until FROM tool_operations o JOIN tool_runs r ON r.id=o.run_id WHERE o.id=$1`
	if lock {
		q += ` FOR UPDATE OF o`
	}
	e := tx.QueryRowContext(ctx, q, id).Scan(&o.ID, &o.RunID, &o.Status, &o.Outcome, &o.ExpiresAt, &o.CancelRequested, &o.Kind, &b, &o.Digest, &o.Parent.Kind, &family, &token, &o.Parent.ExpiresAt, &o.Parent.ActorID, &o.Parent.SessionID, &o.Fence, &o.Workload, &o.Deadline, &o.Lease)
	if e != nil {
		return o, e
	}
	if json.Unmarshal(b, &o.Args) != nil {
		return o, ErrDenied
	}
	o.Parent.SubjectID = o.Parent.ActorID
	if family.Valid {
		o.Parent.ParentGrantID = family.UUID
	}
	if token.Valid {
		o.Parent.TokenID = token.UUID.String()
	}
	return o, nil
}
func (s *Service) discoverOperation(ctx context.Context, id uuid.UUID) (uuid.UUID, policy.Principal, string, error) {
	var run uuid.UUID
	var p policy.Principal
	var family, token uuid.NullUUID
	var client string
	e := s.db.QueryRowContext(ctx, `SELECT o.run_id,o.parent_kind,o.parent_family,o.parent_token,o.authority_expires_at,r.actor_id,r.session_id,r.client_id FROM tool_operations o JOIN tool_runs r ON r.id=o.run_id WHERE o.id=$1`, id).Scan(&run, &p.Kind, &family, &token, &p.ExpiresAt, &p.ActorID, &p.SessionID, &client)
	p.SubjectID = p.ActorID
	if family.Valid {
		p.ParentGrantID = family.UUID
	}
	if token.Valid {
		p.TokenID = token.UUID.String()
	}
	if e != nil {
		return run, p, client, ErrDenied
	}
	return run, p, client, nil
}
func (s *Service) beginOperation(ctx context.Context, p policy.Principal, client string, id uuid.UUID, active bool) (*sql.Tx, runState, grantState, operationState, error) {
	var r runState
	var g grantState
	var o operationState
	run, parent, storedClient, e := s.discoverOperation(ctx, id)
	if e != nil || storedClient != client || parent.ActorID != actor(p) || p.Kind == policy.OAuthDelegation && parent.SessionID != p.SessionID {
		return nil, r, g, o, ErrDenied
	}
	cap := policy.InspectTeam
	if active {
		cap = policy.StartApprovedRun
	}
	tx, e := s.beginRun(ctx, p, client, run, cap)
	if e != nil {
		return nil, r, g, o, e
	}
	fail := func(e error) (*sql.Tx, runState, grantState, operationState, error) {
		tx.Rollback()
		return nil, r, g, o, e
	}
	if active && parent.Kind == policy.OAuthDelegation && (s.RequireGrantTx == nil || s.RequireGrantTx(ctx, tx, parent, client) != nil) {
		return fail(ErrDenied)
	}
	r, g, e = s.runTx(ctx, tx, p, client, run, active)
	if e != nil {
		return fail(e)
	}
	o, e = readOperation(ctx, tx, id, true)
	if e != nil {
		return fail(ErrDenied)
	}
	if active && (o.CancelRequested || !o.ExpiresAt.After(time.Now()) || !o.Parent.ExpiresAt.After(time.Now())) {
		return fail(ErrDenied)
	}
	return tx, r, g, o, nil
}
func validArguments(kind string, args Arguments) bool {
	return kind == "repository_tree" && args.Path == "" || kind == "read_file" && broker.ValidPath(args.Path)
}
func (s *Service) Request(ctx context.Context, p policy.Principal, client string, run uuid.UUID, key, kind string, args Arguments) (Operation, error) {
	if !s.flags.Admission {
		return Operation{}, ErrUnavailable
	}
	if !keyPattern.MatchString(key) || !validArguments(kind, args) {
		return Operation{}, ErrInvalid
	}
	tx, e := s.beginRun(ctx, p, client, run, policy.StartApprovedRun)
	if e != nil {
		return Operation{}, e
	}
	defer tx.Rollback()
	r, g, e := s.runTx(ctx, tx, p, client, run, true)
	if e != nil {
		return Operation{}, e
	}
	if p.Kind == policy.Human {
		p = principalForRun(r)
	}
	digest := runner.RequestDigest(kind, args)
	var id uuid.UUID
	var stored string
	e = tx.QueryRowContext(ctx, `SELECT id,request_digest FROM tool_operations WHERE run_id=$1 AND request_key=$2`, run, key).Scan(&id, &stored)
	if e == nil {
		if stored != digest {
			return Operation{}, ErrConflict
		}
		o, e := readOperation(ctx, tx, id, false)
		return o.Operation, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Operation{}, e
	}
	if r.ProviderOps >= g.Manifest.MaxProviderOperations || r.Bytes >= g.Manifest.MaxTotalBytes {
		return Operation{}, ErrDenied
	}
	id = uuid.New()
	end := earliest(r.ExpiresAt, r.AuthorityExpiry, p.ExpiresAt)
	b, _ := json.Marshal(args)
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_operations(id,run_id,kind,arguments,request_key,request_digest,parent_kind,parent_family,parent_token,authority_expires_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)`, id, run, kind, b, key, digest, p.Kind, familyID(p), tokenID(p), end); e != nil {
		return Operation{}, ErrConflict
	}
	receipt, e := s.receipt(ctx, tx, p, r.ProjectID, run, id, "tool.operation.queued", "queued", models.JSONMap{"request_digest": digest, "kind": kind, "tool_id": catalog.Find(kind).Name, "installation": catalog.Installation, "origin": "keepsave", "schema_digest": catalog.Find(kind).SchemaDigest, "artifact_digest": g.Manifest.ArtifactDigest, "catalog_digest": g.Manifest.CatalogDigest, "connector_digest": g.Image})
	if e != nil {
		return Operation{}, e
	}
	if e = tx.Commit(); e != nil {
		return Operation{}, e
	}
	return Operation{ID: id, RunID: run, Status: "queued", ExpiresAt: end, ReceiptID: receipt}, nil
}
func (s *Service) Status(ctx context.Context, p policy.Principal, client string, id uuid.UUID, wait time.Duration) (Operation, error) {
	if wait < 0 || wait > 2*time.Second {
		return Operation{}, ErrInvalid
	}
	deadline := time.Now().Add(wait)
	for {
		tx, _, _, o, e := s.beginOperation(ctx, p, client, id, false)
		if e != nil {
			return Operation{}, e
		}
		tx.Rollback()
		if !o.ExpiresAt.After(time.Now()) && (o.Status == "queued" || o.Status == "leased") {
			o.Status = "expired"
		}
		if wait == 0 || time.Now().After(deadline) || o.Status != "queued" && o.Status != "leased" && o.Status != "dispatched" {
			o.Result = nil
			return o.Operation, nil
		}
		select {
		case <-ctx.Done():
			return Operation{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
func (s *Service) Result(ctx context.Context, p policy.Principal, client string, id uuid.UUID) (Operation, error) {
	tx, r, _, o, e := s.beginOperation(ctx, p, client, id, true)
	if e != nil {
		return Operation{}, e
	}
	defer tx.Rollback()
	if o.Status != "succeeded" {
		return Operation{}, ErrConflict
	}
	var cipher, nonce []byte
	var end time.Time
	if e = tx.QueryRowContext(ctx, `SELECT ciphertext,nonce,expires_at FROM tool_result_spool WHERE operation_id=$1 FOR SHARE`, id).Scan(&cipher, &nonce, &end); e != nil || !end.After(time.Now()) {
		return Operation{}, ErrDenied
	}
	receipt, e := s.receipt(ctx, tx, p, r.ProjectID, r.ID, id, "tool.result.admitted", "succeeded", models.JSONMap{})
	if e != nil {
		return Operation{}, e
	}
	o.ReceiptID = receipt
	e = s.broker.WithOpened(cipher, nonce, func(b []byte) error {
		if !json.Valid(b) {
			return ErrDenied
		}
		o.Result = append(json.RawMessage(nil), b...)
		serialized, e := json.Marshal(o.Operation)
		if e != nil || len(serialized) > 4<<20 {
			return ErrDenied
		}
		return nil
	})
	if e != nil {
		return Operation{}, ErrDenied
	}
	if e = tx.Commit(); e != nil {
		return Operation{}, e
	}
	return o.Operation, nil
}
func (s *Service) CancelOperation(ctx context.Context, p policy.Principal, client string, id uuid.UUID) error {
	tx, r, _, o, e := s.beginOperation(ctx, p, client, id, false)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	status := o.Status
	if status == "queued" || status == "leased" {
		status = "cancelled"
	}
	if _, e = tx.ExecContext(ctx, `UPDATE tool_operations SET cancel_requested=TRUE,status=$2,updated_at=NOW() WHERE id=$1`, id, status); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE tool_attempts SET state='cancelled',outcome='cancelled',finished_at=NOW() WHERE operation_id=$1 AND state='leased'`, id); e != nil {
		return e
	}
	if _, e = s.receipt(ctx, tx, p, r.ProjectID, r.ID, id, "tool.operation.cancelled", status, models.JSONMap{}); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) CancelRun(ctx context.Context, p policy.Principal, client string, id uuid.UUID) error {
	tx, e := s.beginRun(ctx, p, client, id, policy.InspectTeam)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, _, e := s.runTx(ctx, tx, p, client, id, false)
	if e != nil {
		return e
	}
	if e = cancelRunTx(ctx, tx, id, "cancelled"); e != nil {
		return e
	}
	if _, e = s.receipt(ctx, tx, p, r.ProjectID, id, uuid.Nil, "tool.run.cancelled", "cancelled", models.JSONMap{}); e != nil {
		return e
	}
	return tx.Commit()
}
func cancelRunTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, state string) error {
	if _, e := tx.ExecContext(ctx, `UPDATE tool_runs SET state=$2 WHERE id=$1`, id, state); e != nil {
		return e
	}
	if _, e := tx.ExecContext(ctx, `UPDATE tool_operations SET cancel_requested=TRUE,status=CASE WHEN status IN('queued','leased') THEN 'cancelled' ELSE status END,updated_at=NOW() WHERE run_id=$1`, id); e != nil {
		return e
	}
	_, e := tx.ExecContext(ctx, `UPDATE tool_attempts SET state='cancelled',outcome='cancelled',finished_at=NOW() WHERE operation_id IN(SELECT id FROM tool_operations WHERE run_id=$1) AND state='leased'`, id)
	return e
}

// A retry is explicit and allowed only when no provider dispatch took place.
func (s *Service) RetryOperation(ctx context.Context, p policy.Principal, client string, id uuid.UUID) (Operation, error) {
	tx, r, _, o, e := s.beginOperation(ctx, p, client, id, true)
	if e != nil {
		return Operation{}, e
	}
	defer tx.Rollback()
	if o.Status != "failed" || o.Outcome != "pre_dispatch_failure" {
		return Operation{}, ErrConflict
	}
	var dispatched int
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tool_attempts WHERE operation_id=$1 AND state IN('dispatched','succeeded','uncertain')`, id).Scan(&dispatched); e != nil || dispatched != 0 {
		return Operation{}, ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `UPDATE tool_operations SET status='queued',outcome='',workload_id=NULL,lease_until=NULL,deadline=NULL,updated_at=NOW() WHERE id=$1`, id); e != nil {
		return Operation{}, e
	}
	receipt, e := s.receipt(ctx, tx, p, r.ProjectID, r.ID, id, "tool.operation.retried", "queued", models.JSONMap{})
	if e != nil {
		return Operation{}, e
	}
	if e = tx.Commit(); e != nil {
		return Operation{}, e
	}
	o.Status = "queued"
	o.Outcome = ""
	o.ReceiptID = receipt
	return o.Operation, nil
}

func (s *Service) Claim(ctx context.Context, certificate, image string) (Ticket, error) {
	if !s.Enabled() || !s.flags.Dispatch {
		return Ticket{}, ErrUnavailable
	}
	if !digestPattern.MatchString(certificate) || !runner.ValidImage(image) {
		return Ticket{}, ErrDenied
	}
	var workload uuid.UUID
	if e := s.db.QueryRowContext(ctx, `SELECT id FROM tool_workloads WHERE certificate_sha256=$1 AND image_digest=$2 AND revoked_at IS NULL`, certificate, image).Scan(&workload); e != nil {
		return Ticket{}, ErrDenied
	}
	rows, e := s.db.QueryContext(ctx, `SELECT o.id FROM tool_operations o JOIN tool_runs r ON r.id=o.run_id JOIN tool_grants g ON g.id=r.grant_id WHERE g.workload_id=$1 AND o.status IN('queued','leased','dispatched') ORDER BY o.created_at,o.id LIMIT 100`, workload)
	if e != nil {
		return Ticket{}, ErrUnavailable
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return Ticket{}, ErrUnavailable
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return Ticket{}, ErrUnavailable
	}
	for _, id := range ids {
		ticket, e := s.claimOne(ctx, certificate, image, workload, id)
		if e == nil {
			return ticket, nil
		}
		if errors.Is(e, ErrUnavailable) {
			return Ticket{}, e
		}
	}
	return Ticket{}, ErrNoWork
}
func (s *Service) claimOne(ctx context.Context, certificate, image string, workload, id uuid.UUID) (Ticket, error) {
	_, p, client, e := s.discoverOperation(ctx, id)
	if e != nil {
		return Ticket{}, e
	}
	tx, r, g, o, e := s.beginOperation(ctx, p, client, id, true)
	if e != nil {
		return Ticket{}, e
	}
	defer tx.Rollback()
	if g.WorkloadID != workload || g.Image != image {
		return Ticket{}, ErrDenied
	}
	var stored string
	if e = tx.QueryRowContext(ctx, `SELECT certificate_sha256 FROM tool_workloads WHERE id=$1 AND revoked_at IS NULL`, workload).Scan(&stored); e != nil || stored != certificate {
		return Ticket{}, ErrDenied
	}
	now := time.Now()
	if o.Status == "dispatched" && o.Deadline.Valid && !o.Deadline.Time.After(now) {
		if e = s.finishAttemptTx(ctx, tx, p, r, o, "uncertain", "deadline_elapsed", nil); e != nil {
			return Ticket{}, e
		}
		if e = tx.Commit(); e != nil {
			return Ticket{}, e
		}
		return Ticket{}, ErrNoWork
	}
	if o.Status == "leased" && o.Lease.Valid && !o.Lease.Time.After(now) {
		if _, e = tx.ExecContext(ctx, `UPDATE tool_attempts SET state='failed',outcome='pre_dispatch_failure',finished_at=NOW() WHERE operation_id=$1 AND fence=$2 AND state='leased'`, id, o.Fence); e != nil {
			return Ticket{}, e
		}
		o.Status = "queued"
	}
	if o.Status != "queued" {
		return Ticket{}, ErrNoWork
	}
	var current int
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tool_operations WHERE run_id=$1 AND status IN('leased','dispatched') AND deadline>NOW()`, r.ID).Scan(&current); e != nil {
		return Ticket{}, e
	}
	if current >= g.Manifest.MaxConcurrent || r.ProviderOps >= g.Manifest.MaxProviderOperations || r.Bytes >= g.Manifest.MaxTotalBytes {
		return Ticket{}, ErrNoWork
	}
	token, e := randomToken()
	if e != nil {
		return Ticket{}, e
	}
	var nonceBytes [32]byte
	if _, e = rand.Read(nonceBytes[:]); e != nil {
		return Ticket{}, e
	}
	end := earliest(o.ExpiresAt, p.ExpiresAt, r.ExpiresAt, time.Now().Add(time.Duration(g.Manifest.ToolSeconds)*time.Second))
	t := Ticket{TicketID: uuid.New(), OperationID: id, RunID: r.ID, GrantID: g.ID, Attempt: o.Fence + 1, Fence: o.Fence + 1, ImageDigest: image, RequestDigest: o.Digest, Nonce: hex.EncodeToString(nonceBytes[:]), ExpiresAt: end, Kind: o.Kind, Arguments: o.Args, Token: token}
	if _, e = tx.ExecContext(ctx, `UPDATE tool_operations SET status='leased',fence=$2,workload_id=$3,lease_until=$4,deadline=$4,updated_at=NOW() WHERE id=$1`, id, t.Fence, workload, end); e != nil {
		return Ticket{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_tickets(id,operation_id,workload_id,token_hash,fence,request_digest,image_digest,nonce,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, t.TicketID, id, workload, hash(token), t.Fence, o.Digest, image, t.Nonce, end); e != nil {
		return Ticket{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO tool_attempts(operation_id,fence,ticket_id,workload_id,state,deadline) VALUES($1,$2,$3,$4,'leased',$5)`, id, t.Fence, t.TicketID, workload, end); e != nil {
		return Ticket{}, e
	}
	if _, e = s.receipt(ctx, tx, p, r.ProjectID, r.ID, id, "tool.attempt.leased", "leased", models.JSONMap{"fence": t.Fence, "workload_id": workload}); e != nil {
		return Ticket{}, e
	}
	if e = tx.Commit(); e != nil {
		return Ticket{}, e
	}
	return t, nil
}
func (s *Service) ticketOperation(ctx context.Context, certificate string, id uuid.UUID, token string) (uuid.UUID, error) {
	if !digestPattern.MatchString(certificate) || len(token) != 43 {
		return uuid.Nil, ErrDenied
	}
	var op uuid.UUID
	e := s.db.QueryRowContext(ctx, `SELECT t.operation_id FROM tool_tickets t JOIN tool_workloads w ON w.id=t.workload_id WHERE t.id=$1 AND t.token_hash=$2 AND w.certificate_sha256=$3 AND w.revoked_at IS NULL AND t.expires_at>NOW()`, id, hash(token), certificate).Scan(&op)
	if e != nil {
		return uuid.Nil, ErrDenied
	}
	return op, nil
}
func (s *Service) ticketTx(ctx context.Context, tx *sql.Tx, certificate string, req runner.ExecuteRequest, r runState, g grantState, o operationState, consume bool) error {
	var tokenHash, image, nonce, digest, cert string
	var workload uuid.UUID
	var fence int64
	var end time.Time
	var consumed sql.NullTime
	e := tx.QueryRowContext(ctx, `SELECT t.token_hash,t.image_digest,t.nonce,t.request_digest,t.workload_id,t.fence,t.expires_at,t.consumed_at,w.certificate_sha256 FROM tool_tickets t JOIN tool_workloads w ON w.id=t.workload_id WHERE t.id=$1 AND t.operation_id=$2 AND w.revoked_at IS NULL FOR UPDATE OF t`, req.TicketID, o.ID).Scan(&tokenHash, &image, &nonce, &digest, &workload, &fence, &end, &consumed, &cert)
	if e != nil || cert != certificate || tokenHash != hash(req.Token) || workload != g.WorkloadID || image != g.Image || fence != o.Fence || !end.After(time.Now()) || consume && consumed.Valid {
		return ErrDenied
	}
	if consume {
		expected := runner.OperationRequest{OperationID: o.ID, RunID: r.ID, GrantID: g.ID, Attempt: fence, Fence: fence, ImageDigest: image, RequestDigest: digest, Nonce: nonce, Kind: o.Kind, Arguments: o.Args}
		if req.Request != expected || req.Request.Validate() != nil || o.Status != "leased" {
			return ErrDenied
		}
		if _, e = tx.ExecContext(ctx, `UPDATE tool_tickets SET consumed_at=NOW() WHERE id=$1`, req.TicketID); e != nil {
			return e
		}
	} else if o.Status != "leased" && o.Status != "dispatched" {
		return ErrDenied
	}
	return nil
}
func (s *Service) TicketStatus(ctx context.Context, certificate string, req runner.TicketStatusRequest) (runner.TicketStatusResponse, error) {
	id, e := s.ticketOperation(ctx, certificate, req.TicketID, req.Token)
	if e != nil {
		return runner.TicketStatusResponse{}, e
	}
	_, p, client, e := s.discoverOperation(ctx, id)
	if e != nil {
		return runner.TicketStatusResponse{}, e
	}
	tx, r, g, o, e := s.beginOperation(ctx, p, client, id, true)
	if e != nil {
		return runner.TicketStatusResponse{}, e
	}
	defer tx.Rollback()
	e = s.ticketTx(ctx, tx, certificate, runner.ExecuteRequest{TicketID: req.TicketID, Token: req.Token}, r, g, o, false)
	return runner.TicketStatusResponse{Active: e == nil}, e
}
func (s *Service) Execute(ctx context.Context, certificate string, req runner.ExecuteRequest) (runner.ExecuteResponse, error) {
	if !s.Enabled() || !s.flags.Dispatch {
		return runner.ExecuteResponse{}, ErrUnavailable
	}
	if req.Request.Validate() != nil {
		return runner.ExecuteResponse{}, ErrInvalid
	}
	id, e := s.ticketOperation(ctx, certificate, req.TicketID, req.Token)
	if e != nil || id != req.Request.OperationID {
		return runner.ExecuteResponse{}, ErrDenied
	}
	_, p, client, e := s.discoverOperation(ctx, id)
	if e != nil {
		return runner.ExecuteResponse{}, e
	}
	tx, r, g, o, e := s.beginOperation(ctx, p, client, id, true)
	if e != nil {
		return runner.ExecuteResponse{}, e
	}
	defer tx.Rollback()
	if e = s.ticketTx(ctx, tx, certificate, req, r, g, o, true); e != nil {
		return runner.ExecuteResponse{}, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE tool_operations SET status='dispatched',updated_at=NOW() WHERE id=$1`, id); e != nil {
		return runner.ExecuteResponse{}, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE tool_attempts SET state='dispatched' WHERE operation_id=$1 AND fence=$2`, id, o.Fence); e != nil {
		return runner.ExecuteResponse{}, e
	}
	if _, e = s.receipt(ctx, tx, p, r.ProjectID, r.ID, id, "tool.attempt.dispatched", "dispatched", models.JSONMap{"fence": o.Fence}); e != nil {
		return runner.ExecuteResponse{}, e
	}
	if e = tx.Commit(); e != nil {
		return runner.ExecuteResponse{}, e
	}
	callCtx, cancel := context.WithDeadline(ctx, o.Deadline.Time)
	defer cancel()
	payloadLimit := (g.Manifest.MaxResponseBytes - 65536) / 2
	if remaining := g.Manifest.MaxTotalBytes - r.Bytes; remaining < int64(payloadLimit) {
		payloadLimit = int(remaining)
	}
	result, e := s.broker.ExecuteAuthorized(callCtx, g.Connection, g.Target, o.Kind, o.Args.Path, payloadLimit, func(c context.Context, stage string) error {
		return s.operationAdmission(c, p, client, id, o.Fence, stage)
	})
	if e != nil {
		state, outcome := "failed", "provider_failed"
		if errors.Is(e, broker.ErrUncertain) {
			state, outcome = "uncertain", "provider_uncertain"
		}
		s.finishOperation(context.WithoutCancel(ctx), p, client, id, o.Fence, state, outcome, nil)
		return runner.ExecuteResponse{}, ErrDenied
	}
	receipt, e := s.finishOperation(ctx, p, client, id, o.Fence, "succeeded", "succeeded", result)
	if e != nil {
		return runner.ExecuteResponse{}, e
	}
	return runner.ExecuteResponse{Result: result, Outcome: "succeeded", ReceiptID: receipt}, nil
}
func (s *Service) operationAdmission(ctx context.Context, p policy.Principal, client string, id uuid.UUID, fence int64, stage string) error {
	tx, r, g, o, e := s.beginOperation(ctx, p, client, id, true)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if o.Status != "dispatched" || o.Fence != fence || !o.Deadline.Valid || !o.Deadline.Time.After(time.Now()) {
		return ErrDenied
	}
	if stage == "provider_request" {
		if r.ProviderOps >= g.Manifest.MaxProviderOperations {
			return ErrDenied
		}
		if _, e = tx.ExecContext(ctx, `UPDATE tool_runs SET provider_operations=provider_operations+1 WHERE id=$1`, r.ID); e != nil {
			return e
		}
	}
	if _, e = s.receipt(ctx, tx, p, r.ProjectID, r.ID, id, "tool.broker.admitted", stage, models.JSONMap{"fence": fence}); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) finishAttemptTx(ctx context.Context, tx *sql.Tx, p policy.Principal, r runState, o operationState, status, outcome string, result []byte) error {
	if _, e := tx.ExecContext(ctx, `UPDATE tool_operations SET status=$2,outcome=$3,updated_at=NOW() WHERE id=$1`, o.ID, status, outcome); e != nil {
		return e
	}
	_, e := tx.ExecContext(ctx, `UPDATE tool_attempts SET state=$3,outcome=$4,finished_at=NOW() WHERE operation_id=$1 AND fence=$2`, o.ID, o.Fence, status, outcome)
	if e != nil {
		return e
	}
	_, e = s.receipt(ctx, tx, p, r.ProjectID, r.ID, o.ID, "tool.attempt.completed", status, models.JSONMap{"fence": o.Fence, "outcome": outcome})
	return e
}
func (s *Service) finishOperation(ctx context.Context, p policy.Principal, client string, id uuid.UUID, fence int64, status, outcome string, result []byte) (uuid.UUID, error) {
	tx, r, g, o, e := s.beginOperation(ctx, p, client, id, true)
	if e != nil {
		return uuid.Nil, e
	}
	defer tx.Rollback()
	if o.Status != "dispatched" || o.Fence != fence || !o.Deadline.Valid || !o.Deadline.Time.After(time.Now()) {
		return uuid.Nil, ErrDenied
	}
	if status == "succeeded" {
		if !json.Valid(result) || len(result) > (g.Manifest.MaxResponseBytes-65536)/2 || r.Bytes+int64(len(result)) > g.Manifest.MaxTotalBytes {
			return uuid.Nil, ErrDenied
		}
		cipher, nonce, e := s.broker.Seal(result)
		if e != nil {
			return uuid.Nil, e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO tool_result_spool(operation_id,ciphertext,nonce,expires_at) VALUES($1,$2,$3,$4)`, id, cipher, nonce, o.ExpiresAt); e != nil {
			return uuid.Nil, e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE tool_runs SET result_bytes=result_bytes+$2 WHERE id=$1`, r.ID, len(result)); e != nil {
			return uuid.Nil, e
		}
	}
	if e = s.finishAttemptTx(ctx, tx, p, r, o, status, outcome, result); e != nil {
		return uuid.Nil, e
	}
	var receipt uuid.UUID
	if e = tx.QueryRowContext(ctx, `SELECT id FROM tool_receipts WHERE operation_id=$1 AND action='tool.attempt.completed' ORDER BY created_at DESC,id DESC LIMIT 1`, id).Scan(&receipt); e != nil {
		return uuid.Nil, e
	}
	if e = tx.Commit(); e != nil {
		return uuid.Nil, e
	}
	return receipt, nil
}
