package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
)

// JWTKeyRepository persists RS256 signing keys (ADR-0008). It implements
// auth.KeyStorage so the auth package stays free of a direct DB dependency.
type JWTKeyRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewJWTKeyRepository(db *sql.DB, dialect Dialect) *JWTKeyRepository {
	return &JWTKeyRepository{db: db, dialect: dialect}
}

func (r *JWTKeyRepository) ListKeys() ([]auth.StoredKey, error) {
	rows, err := QueryQ(r.db, r.dialect, `SELECT kid, alg, public_key, private_key_encrypted, private_key_nonce, status
		 FROM jwt_keys WHERE status != 'retired'`)
	if err != nil {
		return nil, fmt.Errorf("listing jwt keys: %w", err)
	}
	defer rows.Close()
	var out []auth.StoredKey
	for rows.Next() {
		var k auth.StoredKey
		if err := rows.Scan(&k.Kid, &k.Alg, &k.PublicKeyDER, &k.PrivateKeyEncrypted, &k.PrivateKeyNonce, &k.Status); err != nil {
			return nil, fmt.Errorf("scanning jwt key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *JWTKeyRepository) InsertKey(k auth.StoredKey) error {
	_, err := ExecQ(r.db, r.dialect, `INSERT INTO jwt_keys (id, kid, alg, public_key, private_key_encrypted, private_key_nonce, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), k.Kid, k.Alg, k.PublicKeyDER, k.PrivateKeyEncrypted, k.PrivateKeyNonce, k.Status)
	if err != nil {
		return fmt.Errorf("inserting jwt key: %w", err)
	}
	return nil
}

func (r *JWTKeyRepository) SetKeyStatus(kid, status string) error {
	var retiredAt *time.Time
	if status == "retired" {
		now := time.Now().UTC()
		retiredAt = &now
	}
	_, err := ExecQ(r.db, r.dialect, `UPDATE jwt_keys SET status = $2, retired_at = $3 WHERE kid = $1`, kid, status, retiredAt)
	if err != nil {
		return fmt.Errorf("updating jwt key status: %w", err)
	}
	return nil
}
