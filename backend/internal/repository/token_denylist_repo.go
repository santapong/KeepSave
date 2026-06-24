package repository

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TokenDenylistRepository implements auth.Denylister (ADR-0021). A token is
// revoked when either:
//   - its jti is in the denylist table (explicit, single-token revocation), or
//   - the lease it was minted from is revoked/expired/missing (lease-cascade).
//
// The explicit jti set is cached in memory (positive cache) so the common
// "not revoked" path costs no DB round-trip; the cache is seeded at boot,
// written through on Revoke, and refreshed periodically (RefreshCache) to pick
// up revocations performed by other instances. The lease-cascade leg always
// reads the live lease row, so it is never stale.
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
	if leaseID == nil {
		return false, nil
	}
	// Lease-cascade: the token is revoked unless an active (non-revoked,
	// non-expired) lease with this ID still exists.
	q := Q(r.dialect, `SELECT 1 FROM secret_leases WHERE id = $1 AND revoked = `+
		r.dialect.BoolLiteral(false)+` AND expires_at > `+r.dialect.Now())
	var one int
	err := r.db.QueryRow(q, leaseID.String()).Scan(&one)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking lease state for token revocation: %w", err)
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
