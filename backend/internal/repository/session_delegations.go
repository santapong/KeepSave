package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
)

// RevokeRecoveryDelegationsTx reduces authority without deleting its recorded
// identity. The caller holds the exclusive subject barrier, which also covers
// legacy key/lease issuance and protected use. All row sets are locked in stable
// order before mutation; no network effect or token cache is updated here.
func (r *SessionRepository) RevokeRecoveryDelegationsTx(ctx context.Context, tx *sql.Tx, user uuid.UUID) error {
	if r.dialect.DBType() != DBTypePostgres || tx == nil || user == uuid.Nil {
		return auth.ErrSessionUnavailable
	}
	queries := []string{
		`SELECT id::text FROM api_keys WHERE user_id=$1 ORDER BY id FOR UPDATE`,
		`SELECT l.id::text FROM secret_leases l JOIN api_keys k ON k.id=l.api_key_id WHERE k.user_id=$1 ORDER BY l.id FOR UPDATE OF l`,
		`SELECT i.jti FROM agent_token_issuance i WHERE i.user_id=$1 OR EXISTS (SELECT 1 FROM secret_leases l JOIN api_keys k ON k.id=l.api_key_id WHERE l.id=i.lease_id AND k.user_id=$1) ORDER BY i.jti FOR UPDATE OF i`,
	}
	for _, query := range queries {
		rows, err := tx.QueryContext(ctx, query, user)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE api_keys SET expires_at=LEAST(COALESCE(expires_at,NOW()),NOW()) WHERE user_id=$1`, user); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE secret_leases l SET revoked=TRUE,revoked_at=COALESCE(l.revoked_at,NOW()) FROM api_keys k WHERE k.id=l.api_key_id AND k.user_id=$1`, user); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO token_denylist(jti,lease_id,expires_at,reason)
		SELECT i.jti,i.lease_id,i.expires_at,'password_recovery' FROM agent_token_issuance i
		WHERE i.user_id=$1 OR EXISTS (SELECT 1 FROM secret_leases l JOIN api_keys k ON k.id=l.api_key_id WHERE l.id=i.lease_id AND k.user_id=$1)
		ON CONFLICT(jti) DO UPDATE SET expires_at=GREATEST(token_denylist.expires_at,EXCLUDED.expires_at)`, user)
	return err
}
