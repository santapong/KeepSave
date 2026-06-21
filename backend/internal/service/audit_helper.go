package service

import (
	"log"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// auditObserver, when set, is notified of every audit emission attempt so the
// audit metrics (A-06) can be recorded without the service layer importing the
// metrics package. ok=false means the write failed. Installed once at startup.
var auditObserver func(action string, ok bool)

// SetAuditObserver installs the audit-emission observer. main.go wires it to the
// keepsave_audit_events_total / keepsave_audit_emit_failed_total counters.
func SetAuditObserver(fn func(action string, ok bool)) { auditObserver = fn }

// emitAudit writes an audit_log row using the canonical action taxonomy
// in docs/AUDIT_LOG_COVERAGE.md. A nil repo is a no-op so tests that
// construct services without an audit repo continue to work.
//
// Failure is logged and surfaced to the audit metric (A-06) but never
// returned: a transient audit-DB failure must not fail the caller's primary
// mutation. Stronger guarantees (single transaction with the mutation) are
// deferred to a follow-up PR.
func emitAudit(repo *repository.AuditRepository, actorID, projectID *uuid.UUID, action, environment string, details models.JSONMap, ipAddr string) {
	if repo == nil {
		return
	}
	err := repo.Create(actorID, projectID, action, environment, details, ipAddr)
	if obs := auditObserver; obs != nil {
		obs(action, err == nil)
	}
	if err != nil {
		log.Printf("audit emit failed action=%s err=%v", action, err)
	}
}
