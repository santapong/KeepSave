package repository

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/models"
)

const RecentAuthenticationWindow = 10 * time.Minute

type SessionRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewSessionRepository(db *sql.DB, d Dialect) *SessionRepository { return &SessionRepository{db, d} }

func (r *SessionRepository) CreateTx(ctx context.Context, tx *sql.Tx, id, user uuid.UUID, hash string, expires time.Time, ip, ua string) error {
	if len(ua) > 500 {
		ua = ua[:500]
	}
	if len(ip) > 45 {
		ip = ""
	}
	_, err := tx.ExecContext(ctx, Q(r.dialect, `INSERT INTO session_tokens(id,user_id,token_hash,expires_at,revoked,ip_address,user_agent) VALUES($1,$2,$3,$4,$5,$6,$7)`), id, user, hash, expires, false, ip, ua)
	return err
}

func (r *SessionRepository) Check(ctx context.Context, id, user uuid.UUID, hash string, tokenExpiry time.Time) error {
	var storedHash string
	var expires time.Time
	var revoked bool
	err := r.db.QueryRowContext(ctx, Q(r.dialect, `SELECT s.token_hash,s.expires_at,s.revoked FROM session_tokens s JOIN users u ON u.id=s.user_id WHERE s.id=$1 AND s.user_id=$2`), id, user).Scan(&storedHash, dbTime(&expires), &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.ErrSessionInvalid
	}
	if err != nil {
		return auth.ErrSessionUnavailable
	}
	if revoked || storedHash != hash || !expires.After(time.Now()) || !expires.Equal(tokenExpiry) {
		return auth.ErrSessionInvalid
	}
	return nil
}

// LockActiveTx serializes protected mutation admission with session revocation.
func (r *SessionRepository) LockActiveTx(ctx context.Context, tx *sql.Tx, user, id uuid.UUID, recent bool) error {
	var revoked bool
	var expires, created time.Time
	q := `SELECT revoked,expires_at,created_at FROM session_tokens WHERE id=$1 AND user_id=$2`
	if r.dialect.DBType() != DBTypeSQLite {
		q += ` FOR SHARE`
	}
	err := tx.QueryRowContext(ctx, Q(r.dialect, q), id, user).Scan(&revoked, dbTime(&expires), dbTime(&created))
	if errors.Is(err, sql.ErrNoRows) {
		return auth.ErrSessionInvalid
	}
	if err != nil {
		return auth.ErrSessionUnavailable
	}
	if revoked || !expires.After(time.Now()) {
		return auth.ErrSessionInvalid
	}
	if recent && time.Since(created) > RecentAuthenticationWindow {
		return ErrRecentAuthentication
	}
	return nil
}

var ErrRecentAuthentication = errors.New("recent authentication required")

// LockRevocationTargetsTx acquires the complete exclusive session set once.
// Callers hold the exclusive subject barrier before invoking this method.
func (r *SessionRepository) LockRevocationTargetsTx(ctx context.Context, tx *sql.Tx, user uuid.UUID, ids ...uuid.UUID) error {
	set := map[uuid.UUID]bool{}
	for _, id := range ids {
		if id == uuid.Nil {
			return auth.ErrSessionInvalid
		}
		set[id] = true
	}
	ordered := make([]uuid.UUID, 0, len(set))
	for id := range set {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	query := `SELECT id FROM session_tokens WHERE id=$1 AND user_id=$2`
	if r.dialect.DBType() != DBTypeSQLite {
		query += ` FOR UPDATE`
	}
	for _, id := range ordered {
		var found uuid.UUID
		if err := tx.QueryRowContext(ctx, Q(r.dialect, query), id, user).Scan(&found); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sql.ErrNoRows
			}
			return auth.ErrSessionUnavailable
		}
	}
	return nil
}

func (r *SessionRepository) List(ctx context.Context, user uuid.UUID) ([]models.SessionToken, error) {
	rows, err := r.db.QueryContext(ctx, Q(r.dialect, `SELECT id,user_id,expires_at,revoked,COALESCE(ip_address,''),COALESCE(user_agent,''),created_at FROM session_tokens WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`), user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.SessionToken{}
	for rows.Next() {
		var s models.SessionToken
		if err = rows.Scan(&s.ID, &s.UserID, dbTime(&s.ExpiresAt), &s.Revoked, &s.IPAddress, &s.UserAgent, dbTime(&s.CreatedAt)); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *SessionRepository) RevokeTx(ctx context.Context, tx *sql.Tx, user, id uuid.UUID) (bool, error) {
	var revoked bool
	q := `SELECT revoked FROM session_tokens WHERE id=$1 AND user_id=$2`
	if r.dialect.DBType() != DBTypeSQLite {
		q += ` FOR UPDATE`
	}
	if err := tx.QueryRowContext(ctx, Q(r.dialect, q), id, user).Scan(&revoked); err != nil {
		return false, err
	}
	if revoked {
		return false, nil
	}
	_, err := tx.ExecContext(ctx, Q(r.dialect, `UPDATE session_tokens SET revoked=$1 WHERE id=$2 AND user_id=$3`), true, id, user)
	return err == nil, err
}
