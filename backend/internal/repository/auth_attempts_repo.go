package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// AuthAttemptsRepository persists per-account login-failure counters that
// back the audit S-H7 lockout policy. The table is small (one row per email
// that has ever failed) and writes are infrequent so a single primary-key
// row per email is fine.
type AuthAttemptsRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewAuthAttemptsRepository(db *sql.DB, dialect Dialect) *AuthAttemptsRepository {
	return &AuthAttemptsRepository{db: db, dialect: dialect}
}

// LoginAttempt is the current state of an email's lockout counter.
type LoginAttempt struct {
	Email        string
	FailedCount  int
	LastFailedAt *time.Time
	LockedUntil  *time.Time
}

// Get returns the current attempt row or a zero-value row (no failures yet)
// when the email has no record. sql.ErrNoRows is converted to a zero-value
// so callers don't need to special-case the first-failure path.
func (r *AuthAttemptsRepository) Get(email string) (*LoginAttempt, error) {
	row := r.db.QueryRow(
		Q(r.dialect, `SELECT email, failed_count, last_failed_at, locked_until
		 FROM auth_login_attempts WHERE email = $1`),
		email,
	)
	la := &LoginAttempt{Email: email}
	if err := row.Scan(&la.Email, &la.FailedCount, dbTime(&la.LastFailedAt), dbTime(&la.LockedUntil)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &LoginAttempt{Email: email}, nil
		}
		return nil, fmt.Errorf("get login attempt: %w", err)
	}
	return la, nil
}

// RegisterFailure increments the counter and, when failedCount + 1 crosses
// the threshold, sets locked_until = now + lockFor. Upserts.
func (r *AuthAttemptsRepository) RegisterFailure(email string, threshold int, lockFor time.Duration) error {
	now := time.Now().UTC()
	la, err := r.Get(email)
	if err != nil {
		return err
	}
	newCount := la.FailedCount + 1
	var lockedUntil *time.Time
	if newCount >= threshold {
		t := now.Add(lockFor)
		lockedUntil = &t
	} else if la.LockedUntil != nil && la.LockedUntil.After(now) {
		// preserve an existing lock
		lockedUntil = la.LockedUntil
	}
	return r.upsert(email, newCount, &now, lockedUntil)
}

// Reset clears the failure counter and any lock. Called on successful login.
func (r *AuthAttemptsRepository) Reset(email string) error {
	_, err := r.db.Exec(
		Q(r.dialect, `DELETE FROM auth_login_attempts WHERE email = $1`),
		email,
	)
	if err != nil {
		return fmt.Errorf("reset login attempts: %w", err)
	}
	return nil
}

func (r *AuthAttemptsRepository) upsert(email string, failedCount int, lastFailedAt *time.Time, lockedUntil *time.Time) error {
	// Portable upsert via DELETE-then-INSERT inside a transaction. The table
	// is tiny and writes infrequent, so the simpler dialect-agnostic path
	// beats per-dialect ON CONFLICT / ON DUPLICATE KEY branching.
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(Q(r.dialect, `DELETE FROM auth_login_attempts WHERE email = $1`), email); err != nil {
		return fmt.Errorf("upsert delete: %w", err)
	}
	if _, err := tx.Exec(
		Q(r.dialect, `INSERT INTO auth_login_attempts (email, failed_count, last_failed_at, locked_until)
		 VALUES ($1, $2, $3, $4)`),
		email, failedCount, lastFailedAt, lockedUntil,
	); err != nil {
		return fmt.Errorf("upsert insert: %w", err)
	}
	return tx.Commit()
}
