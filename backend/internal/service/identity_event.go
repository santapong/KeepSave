package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// Identity events contain bounded identifiers and operation metadata, never credentials.
// PostgreSQL is the advertised durable platform; legacy dialects retain audit transactions.
func identityEventTx(ctx context.Context, tx *sql.Tx, d repository.Dialect, actor uuid.UUID, action string, details map[string]string) error {
	if d.DBType() != repository.DBTypePostgres {
		return nil
	}
	payload, err := json.Marshal(map[string]any{"actor_id": actor, "action": action, "details": details})
	if err != nil {
		return err
	}
	return jobs.EnqueueTx(ctx, tx, uuid.New(), "identity.event", payload, "local", 5)
}
