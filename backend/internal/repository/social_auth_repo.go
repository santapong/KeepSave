package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

var ErrSocialConflict = errors.New("identity requires existing account sign-in")
var ErrSocialFlow = errors.New("invalid or expired sign-in")

type SocialAuthRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewSocialAuthRepository(db *sql.DB, d Dialect) *SocialAuthRepository {
	return &SocialAuthRepository{db, d}
}

func (r *SocialAuthRepository) CreateFlow(ctx context.Context, hash, provider, challenge string, user *uuid.UUID) error {
	return r.CreateFlowForSession(ctx, hash, provider, challenge, user, nil)
}
func (r *SocialAuthRepository) CreateFlowForSession(ctx context.Context, hash, provider, challenge string, user, session *uuid.UUID) error {
	if session != nil && user == nil {
		return ErrSocialFlow
	}
	now := time.Now().Unix()
	if _, err := r.db.ExecContext(ctx, Q(r.dialect, `DELETE FROM social_auth_flows WHERE expires_at <= $1`), now); err != nil {
		return err
	}
	query := `INSERT INTO social_auth_flows (state_hash, provider, challenge, link_user_id, expires_at,link_session_id) SELECT $1, $2, $3, $4, $5,$6 WHERE (SELECT COUNT(*) FROM social_auth_flows) < 10000`
	if r.dialect.DBType() == DBTypeMySQL {
		// MySQL permits INSERT ... SELECT from its target table, but forbids
		// reading that same target through a scalar subquery.
		query = `INSERT INTO social_auth_flows (state_hash, provider, challenge, link_user_id, expires_at,link_session_id) SELECT $1, $2, $3, $4, $5,$6 FROM social_auth_flows HAVING COUNT(*) < 10000`
	}
	result, err := r.db.ExecContext(ctx, Q(r.dialect, query), hash, provider, challenge, user, now+600, session)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrSocialFlow
	}
	return nil
}

func (r *SocialAuthRepository) ConsumeFlow(ctx context.Context, hash, provider, challenge string, user *uuid.UUID) error {
	return r.ConsumeFlowForSession(ctx, hash, provider, challenge, user, nil)
}
func (r *SocialAuthRepository) ConsumeFlowForSession(ctx context.Context, hash, provider, challenge string, user, session *uuid.UUID) error {
	if session != nil && user == nil {
		return ErrSocialFlow
	}
	query := `DELETE FROM social_auth_flows WHERE state_hash=$1 AND provider=$2 AND challenge=$3 AND expires_at > $4 AND link_user_id IS NULL`
	args := []any{hash, provider, challenge, time.Now().Unix()}
	if user != nil {
		query = `DELETE FROM social_auth_flows WHERE state_hash=$1 AND provider=$2 AND challenge=$3 AND expires_at > $4 AND link_user_id=$5`
		args = append(args, *user)
	}
	if session != nil {
		query += ` AND link_session_id=$6`
		args = append(args, *session)
	} else {
		query += ` AND link_session_id IS NULL`
	}
	result, err := r.db.ExecContext(ctx, Q(r.dialect, query), args...)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrSocialFlow
	}
	return nil
}

func (r *SocialAuthRepository) Resolve(ctx context.Context, provider, subject, email string, linkUser *uuid.UUID) (*models.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	user, err := r.ResolveTx(ctx, tx, provider, subject, email, linkUser)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return user, nil
}
func (r *SocialAuthRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}
func (r *SocialAuthRepository) ResolveTx(ctx context.Context, tx *sql.Tx, provider, subject, email string, linkUser *uuid.UUID) (*models.User, error) {
	var id uuid.UUID
	err := tx.QueryRowContext(ctx, Q(r.dialect, `SELECT user_id FROM social_identities WHERE provider=$1 AND subject=$2`), provider, subject).Scan(&id)
	if err == nil {
		if linkUser != nil && id != *linkUser {
			return nil, ErrSocialConflict
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		if linkUser != nil {
			id = *linkUser
		} else {
			var count int
			if err = tx.QueryRowContext(ctx, Q(r.dialect, `SELECT COUNT(*) FROM users WHERE LOWER(TRIM(email))=$1`), CanonicalEmail(email)).Scan(&count); err != nil {
				return nil, err
			}
			if count != 0 {
				return nil, ErrSocialConflict
			}
			id = uuid.New()
			// The sentinel cannot pass bcrypt password verification.
			_, err = tx.ExecContext(ctx, Q(r.dialect, `INSERT INTO users (id,email,password_hash) VALUES ($1,$2,$3)`), id, strings.ToLower(email), "!")
			if isUniqueViolation(err) {
				return nil, ErrSocialConflict
			}
			if err != nil {
				return nil, err
			}
		}
		_, err = tx.ExecContext(ctx, Q(r.dialect, `INSERT INTO social_identities (provider,subject,user_id,email) VALUES ($1,$2,$3,$4)`), provider, subject, id, email)
		if isUniqueViolation(err) {
			return nil, ErrSocialConflict
		}
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	user := &models.User{}
	err = tx.QueryRowContext(ctx, Q(r.dialect, `SELECT id,email,password_hash,created_at,updated_at FROM users WHERE id=$1`), id).Scan(&user.ID, &user.Email, &user.PasswordHash, dbTime(&user.CreatedAt), dbTime(&user.UpdatedAt))
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *SocialAuthRepository) Connections(ctx context.Context, user uuid.UUID) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, Q(r.dialect, `SELECT provider FROM social_identities WHERE user_id=$1 ORDER BY provider`), user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, err
		}
		result = append(result, provider)
	}
	return result, rows.Err()
}

func (r *SocialAuthRepository) Dialect() Dialect { return r.dialect }
