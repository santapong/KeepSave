package repository

import (
	"context"

	"github.com/santapong/KeepSave/backend/internal/policy"
)

// AuthorizeLegacy preserves scoped SQLite/MySQL adapters while the journal
// remains PostgreSQL-only. Authority is loaded from live stored records.
func (r *ProjectRepository) AuthorizeLegacy(ctx context.Context, p policy.Principal, action policy.Action, resource policy.Resource) (policy.Decision, error) {
	return (policy.Evaluator{Store: AuthorityStore{DB: r.db, Dialect: r.dialect}}).Authorize(ctx, p, action, resource)
}
