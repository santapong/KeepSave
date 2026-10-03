// Package authority implements the database admission barrier shared by vault,
// identity and delegated tool services. It is infrastructure, never a policy
// evaluator: roles and resource capabilities remain in internal/policy.
package authority

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

var ErrDenied = errors.New("resource unavailable")

type Guard struct {
	DB      *sql.DB
	Dialect repository.Dialect
}

func Postgres(db *sql.DB) Guard { return Guard{DB: db, Dialect: &repository.PostgresDialect{}} }

func (g Guard) RequireRecentTx(ctx context.Context, tx *sql.Tx, user, sid uuid.UUID) error {
	if err := g.LockSubjects(ctx, tx, []uuid.UUID{user}, false); err != nil {
		return err
	}
	return g.RequireSession(ctx, tx, policy.Principal{Kind: policy.Human, SubjectID: user, ActorID: user, SessionID: sid}, true)
}

func (g Guard) lock(exclusive bool) string {
	if g.Dialect.DBType() == repository.DBTypeSQLite {
		return ""
	}
	if exclusive {
		return " FOR UPDATE"
	}
	if g.Dialect.DBType() == repository.DBTypeMySQL {
		return " FOR SHARE"
	}
	return " FOR SHARE"
}

// LockSubjects acquires the strongest intended mode once, in UUID order.
// Callers must discover the complete set before calling; lock upgrades and
// subsequent acquisition of a lower UUID are forbidden by the contract.
func (g Guard) LockSubjects(ctx context.Context, tx *sql.Tx, ids []uuid.UUID, exclusive bool) error {
	set := map[uuid.UUID]bool{}
	for _, id := range ids {
		if id == uuid.Nil {
			return ErrDenied
		}
		set[id] = true
	}
	ordered := make([]uuid.UUID, 0, len(set))
	for id := range set {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	for _, id := range ordered {
		var found uuid.UUID
		if err := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT id FROM users WHERE id=$1`+g.lock(exclusive)), id).Scan(&found); err != nil {
			return ErrDenied
		}
	}
	return nil
}

// RequireSession is called after subject, organization/project and membership
// locks. It never upgrades an earlier subject lock.
func (g Guard) RequireSession(ctx context.Context, tx *sql.Tx, p policy.Principal, recent bool) error {
	if p.Kind != policy.Human && p.Kind != policy.OAuthDelegation || p.SubjectID == uuid.Nil || p.SessionID == uuid.Nil || p.ActorID != uuid.Nil && p.ActorID != p.SubjectID {
		return ErrDenied
	}
	q := `SELECT id FROM session_tokens WHERE id=$1 AND user_id=$2 AND revoked=` + g.Dialect.BoolLiteral(false) + ` AND expires_at>` + g.Dialect.Now()
	if recent {
		q += ` AND created_at>` + g.Dialect.IntervalAgo("10 minutes")
	}
	var id uuid.UUID
	if err := tx.QueryRowContext(ctx, repository.Q(g.Dialect, q+g.lock(false)), p.SessionID, p.SubjectID).Scan(&id); err != nil {
		return ErrDenied
	}
	return nil
}

// RequireActiveTx is the session-only adapter used by delegated OAuth. The
// caller must not have acquired any later lock before entering this method.
func (g Guard) RequireActiveTx(ctx context.Context, tx *sql.Tx, user, sid uuid.UUID) error {
	if err := g.LockSubjects(ctx, tx, []uuid.UUID{user}, false); err != nil {
		return err
	}
	return g.RequireSession(ctx, tx, policy.Principal{Kind: policy.Human, SubjectID: user, ActorID: user, SessionID: sid}, false)
}

// RequireOrg expects subjects already locked. It serializes admissions against
// membership removal and authority-epoch changes without granting global roles.
func (g Guard) RequireOrg(ctx context.Context, tx *sql.Tx, p policy.Principal, org uuid.UUID, requiredRole string) error {
	var found uuid.UUID
	if err := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT id FROM organizations WHERE id=$1`+g.lock(false)), org).Scan(&found); err != nil {
		return ErrDenied
	}
	if err := g.member(ctx, tx, org, p.SubjectID, requiredRole); err != nil {
		return err
	}
	return g.RequireSession(ctx, tx, p, false)
}

func (g Guard) member(ctx context.Context, tx *sql.Tx, org, user uuid.UUID, role string) error {
	var actual string
	// Existing compatibility databases do not advertise the new epoch guarantee.
	if g.Dialect.DBType() == repository.DBTypePostgres {
		var active bool
		var epoch int64
		if err := tx.QueryRowContext(ctx, `SELECT active,epoch FROM member_authority_state WHERE organization_id=$1 AND user_id=$2 FOR SHARE`, org, user).Scan(&active, &epoch); err != nil || !active || epoch < 1 {
			return ErrDenied
		}
	}
	if err := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT role FROM organization_members WHERE organization_id=$1 AND user_id=$2`+g.lock(false)), org, user).Scan(&actual); err != nil {
		return ErrDenied
	}
	if role != "" && !policy.RoleAllows(actual, role) {
		return ErrDenied
	}
	return nil
}

// LockProject discovers ownership, locks the complete subject set, then locks
// organization/project and membership before session/parent authority. Callers
// re-evaluate capabilities in their transaction after this barrier.
func (g Guard) LockProject(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, exclusive bool) error {
	return g.LockProjectSubjects(ctx, tx, p, project, exclusive, nil)
}

func (g Guard) LockProjectSubjects(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, exclusive bool, extra []uuid.UUID) error {
	return g.lockProjectOrganizations(ctx, tx, p, project, exclusive, extra, nil)
}

// LockProjectOrganizations includes an authorized source organization before
// project/session locks, for atomic cross-workspace template application.
func (g Guard) LockProjectOrganizations(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, exclusive bool, organizations []uuid.UUID) error {
	return g.lockProjectOrganizations(ctx, tx, p, project, exclusive, nil, organizations)
}
func (g Guard) lockProjectOrganizations(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, exclusive bool, extra []uuid.UUID, organizations []uuid.UUID) error {
	actor := p.SubjectID
	if p.Kind != policy.Human && p.Kind != policy.OAuthDelegation {
		actor = p.ActorID
		if p.Kind == policy.APIKey {
			if err := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT user_id FROM api_keys WHERE id=$1`), p.SubjectID).Scan(&actor); err != nil {
				return ErrDenied
			}
		}
	}
	var owner uuid.UUID
	var org uuid.NullUUID
	if err := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT owner_id,organization_id FROM projects WHERE id=$1 AND deleted_at IS NULL`), project).Scan(&owner, &org); err != nil {
		return ErrDenied
	}
	ids := append([]uuid.UUID{actor, owner}, extra...)
	if err := g.LockSubjects(ctx, tx, ids, false); err != nil {
		return err
	}
	orgs := map[uuid.UUID]bool{}
	for _, id := range organizations {
		if id == uuid.Nil {
			return ErrDenied
		}
		orgs[id] = true
	}
	if org.Valid {
		orgs[org.UUID] = true
	}
	orderedOrgs := []uuid.UUID{}
	for id := range orgs {
		orderedOrgs = append(orderedOrgs, id)
	}
	sort.Slice(orderedOrgs, func(i, j int) bool { return orderedOrgs[i].String() < orderedOrgs[j].String() })
	for _, oid := range orderedOrgs {
		var id uuid.UUID
		if err := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT id FROM organizations WHERE id=$1`+g.lock(false)), oid).Scan(&id); err != nil {
			return ErrDenied
		}
	}
	var currentOwner uuid.UUID
	var currentOrg uuid.NullUUID
	if err := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT owner_id,organization_id FROM projects WHERE id=$1 AND deleted_at IS NULL`+g.lock(exclusive)), project).Scan(&currentOwner, &currentOrg); err != nil || owner != currentOwner || org != currentOrg {
		return ErrDenied
	}
	for _, oid := range orderedOrgs {
		members := []uuid.UUID{actor}
		if org.Valid && oid == org.UUID {
			members = append(members, extra...)
		}
		sort.Slice(members, func(i, j int) bool { return members[i].String() < members[j].String() })
		for _, id := range members {
			if err := g.member(ctx, tx, oid, id, ""); err != nil {
				return err
			}
		}
	}
	if !org.Valid {
		for _, id := range extra {
			if id != owner {
				return ErrDenied
			}
		}
	}
	if (p.Kind == policy.Human || p.Kind == policy.OAuthDelegation) && p.SessionID != uuid.Nil {
		if err := g.RequireSession(ctx, tx, p, false); err != nil {
			return err
		}
	}
	return nil
}

// LockProjectMaintenance holds the same ordered authority barriers without an
// allow decision. It can only reduce authority/erase expired material, including
// resources whose actor or project has already been revoked or tombstoned.
func (g Guard) LockProjectMaintenance(ctx context.Context, tx *sql.Tx, project uuid.UUID, extra []uuid.UUID) error {
	var owner uuid.UUID
	var org uuid.NullUUID
	if e := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT owner_id,organization_id FROM projects WHERE id=$1`), project).Scan(&owner, &org); e != nil {
		return ErrDenied
	}
	ids := append([]uuid.UUID{owner}, extra...)
	if e := g.LockSubjects(ctx, tx, ids, false); e != nil {
		return e
	}
	if org.Valid {
		var id uuid.UUID
		if e := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT id FROM organizations WHERE id=$1`+g.lock(false)), org.UUID).Scan(&id); e != nil {
			return ErrDenied
		}
	}
	var actualOwner uuid.UUID
	var actualOrg uuid.NullUUID
	if e := tx.QueryRowContext(ctx, repository.Q(g.Dialect, `SELECT owner_id,organization_id FROM projects WHERE id=$1`+g.lock(true)), project).Scan(&actualOwner, &actualOrg); e != nil || owner != actualOwner || org != actualOrg {
		return ErrDenied
	}
	if org.Valid {
		sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
		seen := map[uuid.UUID]bool{}
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			var epoch int64
			e := tx.QueryRowContext(ctx, `SELECT epoch FROM member_authority_state WHERE organization_id=$1 AND user_id=$2 FOR SHARE`, org.UUID, id).Scan(&epoch)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return e
			}
		}
	}
	return nil
}
