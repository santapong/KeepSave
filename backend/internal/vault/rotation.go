package vault

import (
	"context"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

// Rotate retains previous keys while current values move atomically to a fresh
// key. Immutable revisions and promotion snapshots retain their key references.
func (s *Service) Rotate(ctx context.Context, p policy.Principal, project uuid.UUID) (int, error) {
	tx, err := s.begin(ctx, project)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	base := Record{ProjectID: project}
	if err = s.permit(ctx, tx, p, policy.ManageProject, base); err != nil {
		return 0, err
	}
	if err = s.event(ctx, tx, p, base, "key.dek_rotated"); err != nil {
		return 0, err
	}
	_, old, err := s.currentKey(ctx, tx, project)
	if err != nil {
		return 0, err
	}
	crypto.SecureZero(old)
	dek, err := s.crypto.GenerateDEK()
	if err != nil {
		return 0, err
	}
	defer crypto.SecureZero(dek)
	encrypted, nonce, err := s.crypto.EncryptDEK(dek)
	if err != nil {
		return 0, err
	}
	key := uuid.New()
	if _, err = tx.ExecContext(ctx, `INSERT INTO vault_keys(id,project_id,encrypted_key,nonce) VALUES($1,$2,$3,$4)`, key, project, encrypted, nonce); err != nil {
		return 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT secret_id FROM vault_entries WHERE project_id=$1 AND NOT deleted ORDER BY secret_id`, project)
	if err != nil {
		return 0, err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		r, _, err := s.load(ctx, tx, project, id)
		if err != nil {
			return 0, err
		}
		plain, err := s.readVersion(ctx, tx, r, r.Revision)
		if err != nil {
			return 0, err
		}
		c, n, err := crypto.Encrypt(dek, plain)
		crypto.SecureZero(plain)
		if err != nil {
			return 0, err
		}
		r.Revision++
		if err = s.append(ctx, tx, p, r, key, c, n, "rotation"); err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE secrets SET encrypted_value=$1,value_nonce=$2,updated_at=NOW() WHERE id=$3 AND project_id=$4`, c, n, id, project); err != nil {
			return 0, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET encrypted_dek=$1,dek_nonce=$2,updated_at=NOW() WHERE id=$3`, encrypted, nonce, project); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE vault_projects SET current_key_id=$1 WHERE project_id=$2`, key, project); err != nil {
		return 0, err
	}
	return len(ids), tx.Commit()
}
