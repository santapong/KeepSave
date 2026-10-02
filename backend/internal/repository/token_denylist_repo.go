package repository

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

// TokenDenylistRepository implements auth.Denylister (ADR-0021). A token is
// revoked when either:
//   - its jti is in the denylist table (explicit, single-token revocation), or
//   - the lease it was minted from is revoked/expired/missing (lease-cascade).
//
// Positive cache entries can deny early. Every cache miss queries authoritative
// database state, so another instance's committed revocation is immediate.
// The lease-cascade leg also checks the current parent key and its scope.
type TokenDenylistRepository struct {
	db      *sql.DB
	dialect Dialect

	mu      sync.RWMutex
	revoked map[string]struct{}
}

func NewTokenDenylistRepository(db *sql.DB, dialect Dialect) *TokenDenylistRepository {
	return &TokenDenylistRepository{db: db, dialect: dialect, revoked: map[string]struct{}{}}
}

// IsTokenRevoked reports whether the agent token identified by jti (minted from
// leaseID, which may be nil) has been revoked. Fail-closed: a DB error on the
// lease check surfaces as an error so ValidateToken rejects the token.
func (r *TokenDenylistRepository) IsTokenRevoked(jti string, leaseID *uuid.UUID) (bool, error) {
	r.mu.RLock()
	_, denied := r.revoked[jti]
	r.mu.RUnlock()
	if denied {
		return true, nil
	}
	// A cache miss is not evidence of authority: another API instance may
	// have committed a revocation since this process last refreshed.
	var count int
	if err := r.db.QueryRow(Q(r.dialect, `SELECT COUNT(*) FROM token_denylist WHERE jti = $1 AND expires_at > `+r.dialect.Now()), jti).Scan(&count); err != nil {
		return false, fmt.Errorf("checking token revocation: %w", err)
	}
	if count != 0 {
		return true, nil
	}
	if leaseID == nil {
		return false, nil
	}
	// Lease-cascade: the token is revoked unless an active (non-revoked,
	// non-expired) lease with this ID still exists.
	q := Q(r.dialect, `SELECT api_key_id, project_id, environment, secret_keys, expires_at FROM secret_leases WHERE id = $1 AND revoked = `+
		r.dialect.BoolLiteral(false)+` AND expires_at > `+r.dialect.Now())
	var parentID, projectID uuid.UUID
	var environment string
	var keys models.StringList
	var expires time.Time
	err := r.db.QueryRow(q, leaseID.String()).Scan(&parentID, &projectID, &environment, &keys, dbTime(&expires))
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking lease state for token revocation: %w", err)
	}
	if !expires.After(time.Now()) {
		return true, nil
	}
	parent, err := NewAPIKeyRepository(r.db, r.dialect).GetByID(parentID)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking parent authority: %w", err)
	}
	if parent.ProjectID != projectID || (parent.Environment != nil && *parent.Environment != environment) ||
		!policy.LeaseKeysAllowed(parent.Scopes, keys) ||
		(parent.ExpiresAt != nil && (!parent.ExpiresAt.After(time.Now()) || expires.After(*parent.ExpiresAt))) {
		return true, nil
	}
	return false, nil
}

// Revoke adds a jti to the denylist (explicit single-token revocation) and the
// in-memory cache. expiresAt is the token's natural expiry so the row can be
// pruned afterwards.
func (r *TokenDenylistRepository) Revoke(jti string, leaseID *uuid.UUID, expiresAt time.Time, reason string) error {
	var leaseVal interface{}
	if leaseID != nil {
		leaseVal = leaseID.String()
	}
	_, err := ExecQ(r.db, r.dialect,
		`INSERT INTO token_denylist (jti, lease_id, expires_at, reason) VALUES ($1, $2, $3, $4)`,
		jti, leaseVal, expiresAt.UTC(), reason)
	if err != nil {
		return fmt.Errorf("revoking token: %w", err)
	}
	r.mu.Lock()
	r.revoked[jti] = struct{}{}
	r.mu.Unlock()
	return nil
}

// RefreshCache rebuilds the in-memory revoked-jti set from the table, dropping
// entries that have already expired. Call at boot and on a timer.
func (r *TokenDenylistRepository) RefreshCache() error {
	q := Q(r.dialect, `SELECT jti FROM token_denylist WHERE expires_at > `+r.dialect.Now())
	rows, err := r.db.Query(q)
	if err != nil {
		return fmt.Errorf("refreshing denylist cache: %w", err)
	}
	defer rows.Close()
	next := map[string]struct{}{}
	for rows.Next() {
		var jti string
		if err := rows.Scan(&jti); err != nil {
			return fmt.Errorf("scanning denylist jti: %w", err)
		}
		next[jti] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	r.revoked = next
	r.mu.Unlock()
	return nil
}

// DeleteExpired prunes denylist rows whose tokens have already expired (and are
// therefore unusable regardless of the list).
func (r *TokenDenylistRepository) DeleteExpired() (int64, error) {
	res, err := ExecQ(r.db, r.dialect, `DELETE FROM token_denylist WHERE expires_at <= `+r.dialect.Now())
	if err != nil {
		return 0, fmt.Errorf("pruning denylist: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
