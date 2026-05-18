package service

import (
	"log"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// emitAudit writes an audit_log row using the canonical action taxonomy
// in docs/AUDIT_LOG_COVERAGE.md. A nil repo is a no-op so tests that
// construct services without an audit repo continue to work.
//
// Failure is logged but never returned: a transient audit-DB failure must
// not fail the caller's primary mutation. Stronger guarantees (single
// transaction with the mutation) are deferred to a follow-up PR.
func emitAudit(repo *repository.AuditRepository, actorID, projectID *uuid.UUID, action, environment string, details models.JSONMap, ipAddr string) {
	if repo == nil {
		return
	}
	if err := repo.Create(actorID, projectID, action, environment, details, ipAddr); err != nil {
		log.Printf("audit emit failed action=%s err=%v", action, err)
	}
}
