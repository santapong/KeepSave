package vault

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"sort"
	"time"
)

// Transaction serializes an authorized application mutation with the journal.
// Adapters retain repository ownership; credential access stays in this package.
func (s *Service) Transaction(ctx context.Context, p policy.Principal, project uuid.UUID, action policy.Action, fn func(*sql.Tx) error) error {
	tx, err := s.begin(ctx, project)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.permit(ctx, tx, p, action, Record{ProjectID: project}); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) EventTx(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, action, environment string) error {
	return s.event(ctx, tx, p, Record{ProjectID: project, Environment: environment}, action)
}
func (s *Service) promotionRecords(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, environment string, keys []string) ([]Record, error) {
	rows, err := tx.QueryContext(ctx, `SELECT v.secret_id FROM vault_entries v JOIN environments e ON e.id=v.environment_id AND e.project_id=v.project_id WHERE v.project_id=$1 AND e.name=$2 AND NOT v.deleted ORDER BY v.secret_key`, project, environment)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	filter := map[string]bool{}
	for _, key := range keys {
		if key == "" || len(key) > 255 || filter[key] {
			return nil, ErrInvalid
		}
		filter[key] = true
	}
	result := []Record{}
	selected := len(filter) > 0
	found := map[string]bool{}
	for _, id := range ids {
		r, _, err := s.load(ctx, tx, project, id)
		if err != nil {
			return nil, err
		}
		if selected && !filter[r.Key] {
			continue
		}
		if err = s.permit(ctx, tx, p, policy.ReadValue, r); err != nil {
			return nil, err
		}
		result = append(result, r)
		found[r.Key] = true
	}
	if selected && len(found) != len(filter) {
		return nil, ErrNotFound
	}
	return result, nil
}

type promotionRevision struct {
	Key      string
	ID       uuid.UUID
	Revision int64
	Deleted  bool
}

func revisionDigest(records []Record) string {
	rows := []promotionRevision{}
	for _, r := range records {
		rows = append(rows, promotionRevision{r.Key, r.ID, r.Revision, r.Deleted})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	data, _ := json.Marshal(rows)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func promotionRequestDigest(pr *models.PromotionRequest) string {
	keys := append([]string{}, pr.KeysFilter...)
	sort.Strings(keys)
	data, _ := json.Marshal(struct {
		ID, Project, RequestedBy        uuid.UUID
		Source, Target, Override, Notes string
		Keys                            []string
		PolicyRevision                  int
		ApprovalRequired                bool
	}{pr.ID, pr.ProjectID, pr.RequestedBy, pr.SourceEnvironment, pr.TargetEnvironment, pr.OverridePolicy, pr.Notes, keys, 1, pr.TargetEnvironment == "prod"})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (s *Service) BindPromotionTx(ctx context.Context, tx *sql.Tx, p policy.Principal, pr *models.PromotionRequest) error {
	var envCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM environments WHERE project_id=$1 AND name IN($2,$3)`, pr.ProjectID, pr.SourceEnvironment, pr.TargetEnvironment).Scan(&envCount); err != nil {
		return err
	}
	if envCount != 2 {
		return ErrInvalid
	}
	source, err := s.promotionRecords(ctx, tx, p, pr.ProjectID, pr.SourceEnvironment, pr.KeysFilter)
	if err != nil {
		return err
	}
	if len(source) == 0 {
		return ErrInvalid
	}
	// Bind the whole target environment, including additions/removals, conservatively.
	target, err := s.promotionRecords(ctx, tx, p, pr.ProjectID, pr.TargetEnvironment, nil)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO vault_promotion_bindings(promotion_id,project_id,request_digest,source_digest,target_digest,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, pr.ID, pr.ProjectID, promotionRequestDigest(pr), revisionDigest(source), revisionDigest(target), time.Now().UTC().Add(30*time.Minute))
	return err
}
func (s *Service) CheckPromotionTx(ctx context.Context, tx *sql.Tx, p policy.Principal, pr *models.PromotionRequest) error {
	var requestDigest, sourceDigest, targetDigest string
	var expiry time.Time
	if err := tx.QueryRowContext(ctx, `SELECT request_digest,source_digest,target_digest,expires_at FROM vault_promotion_bindings WHERE promotion_id=$1 AND project_id=$2`, pr.ID, pr.ProjectID).Scan(&requestDigest, &sourceDigest, &targetDigest, &expiry); err != nil {
		return ErrConflict
	}
	if requestDigest != promotionRequestDigest(pr) || !expiry.After(time.Now()) {
		return ErrConflict
	}
	source, err := s.promotionRecords(ctx, tx, p, pr.ProjectID, pr.SourceEnvironment, pr.KeysFilter)
	if err != nil {
		return err
	}
	target, err := s.promotionRecords(ctx, tx, p, pr.ProjectID, pr.TargetEnvironment, nil)
	if err != nil {
		return err
	}
	if sourceDigest != revisionDigest(source) || targetDigest != revisionDigest(target) {
		return ErrConflict
	}
	return nil
}
func (s *Service) ApplyPromotionTx(ctx context.Context, tx *sql.Tx, p policy.Principal, pr *models.PromotionRequest) error {
	if err := s.CheckPromotionTx(ctx, tx, p, pr); err != nil {
		return err
	}
	source, err := s.promotionRecords(ctx, tx, p, pr.ProjectID, pr.SourceEnvironment, pr.KeysFilter)
	if err != nil {
		return err
	}
	for _, src := range source {
		target, err := s.CurrentTx(ctx, tx, pr.ProjectID, pr.TargetEnvironment, src.Key)
		exists := err == nil
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		if exists && pr.OverridePolicy == "skip" {
			continue
		}
		if err = s.permit(ctx, tx, p, policy.WriteSecret, Record{ProjectID: pr.ProjectID, Environment: pr.TargetEnvironment, Key: src.Key}); err != nil {
			return err
		}
		snapshot := uuid.New()
		var env uuid.UUID
		if err = tx.QueryRowContext(ctx, `SELECT id FROM environments WHERE project_id=$1 AND name=$2`, pr.ProjectID, pr.TargetEnvironment).Scan(&env); err != nil {
			return err
		}
		cipher, nonce := []byte{}, []byte{}
		var key uuid.UUID
		if exists {
			if err = tx.QueryRowContext(ctx, `SELECT vr.encrypted_value,vr.nonce,vr.key_id FROM vault_revisions vr WHERE vr.project_id=$1 AND vr.secret_id=$2 AND vr.revision=$3`, pr.ProjectID, target.ID, target.Revision).Scan(&cipher, &nonce, &key); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO secret_snapshots(id,promotion_id,environment_id,key,encrypted_value,value_nonce,prior_existed) VALUES($1,$2,$3,$4,$5,$6,$7)`, snapshot, pr.ID, env, src.Key, cipher, nonce, exists); err != nil {
			return err
		}
		if exists {
			if _, err = tx.ExecContext(ctx, `INSERT INTO vault_snapshot_keys(snapshot_id,project_id,key_id) VALUES($1,$2,$3)`, snapshot, pr.ProjectID, key); err != nil {
				return err
			}
		}
		if err = s.event(ctx, tx, p, src, "secret.read"); err != nil {
			return err
		}
		plain, err := s.readVersion(ctx, tx, src, src.Revision)
		if err != nil {
			return err
		}
		record := Record{ProjectID: pr.ProjectID, Environment: pr.TargetEnvironment, Key: src.Key, Value: string(plain)}
		expected := int64(0)
		if exists {
			record.ID = target.ID
			expected = target.Revision
		}
		updated, err := s.PutTx(ctx, tx, p, record, expected, "promotion")
		crypto.SecureZero(plain)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO vault_promotion_effects(promotion_id,project_id,secret_id,applied_revision) VALUES($1,$2,$3,$4)`, pr.ID, pr.ProjectID, updated.ID, updated.Revision); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) RollbackPromotionTx(ctx context.Context, tx *sql.Tx, p policy.Principal, pr *models.PromotionRequest) error {
	rows, err := tx.QueryContext(ctx, `SELECT ss.key,ss.prior_existed,ss.encrypted_value,ss.value_nonce,k.encrypted_key,k.nonce,ve.secret_id,ve.applied_revision FROM secret_snapshots ss JOIN vault_promotion_effects ve ON ve.promotion_id=ss.promotion_id JOIN vault_entries entry ON entry.secret_id=ve.secret_id AND entry.project_id=ve.project_id AND entry.secret_key=ss.key AND entry.environment_id=ss.environment_id LEFT JOIN vault_snapshot_keys sk ON sk.snapshot_id=ss.id AND sk.project_id=ve.project_id LEFT JOIN vault_keys k ON k.id=sk.key_id AND k.project_id=sk.project_id WHERE ss.promotion_id=$1 AND ve.project_id=$2 ORDER BY ss.key`, pr.ID, pr.ProjectID)
	if err != nil {
		return err
	}
	type item struct {
		key                       string
		exists                    bool
		c, n, encrypted, keyNonce []byte
		id                        uuid.UUID
		revision                  int64
	}
	var items []item
	for rows.Next() {
		var i item
		if err = rows.Scan(&i.key, &i.exists, &i.c, &i.n, &i.encrypted, &i.keyNonce, &i.id, &i.revision); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, i := range items {
		r, _, err := s.load(ctx, tx, pr.ProjectID, i.id)
		if err != nil {
			return err
		}
		if r.Deleted {
			return ErrConflict
		}
		if r.Revision != i.revision {
			var changed int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM vault_revisions WHERE project_id=$1 AND secret_id=$2 AND revision>$3 AND operation<>'rotation'`, pr.ProjectID, r.ID, i.revision).Scan(&changed); err != nil {
				return err
			}
			if changed > 0 {
				return ErrConflict
			}
		}

		if !i.exists {
			if err = s.DeleteTx(ctx, tx, p, pr.ProjectID, i.id, r.Revision); err != nil {
				return err
			}
			continue
		}
		if err = s.permit(ctx, tx, p, policy.RestoreSecret, r); err != nil {
			return err
		}
		next := r
		next.Revision++
		if err = s.event(ctx, tx, p, next, "secret.restored"); err != nil {
			return err
		}
		dek, err := s.crypto.DecryptDEK(i.encrypted, i.keyNonce)
		if err != nil {
			return err
		}
		plain, err := crypto.Decrypt(dek, i.c, i.n)
		crypto.SecureZero(dek)
		if err != nil {
			return err
		}
		r.Value = string(plain)
		_, err = s.PutTx(ctx, tx, p, r, r.Revision, "rollback")
		crypto.SecureZero(plain)
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) PromotionDiff(ctx context.Context, p policy.Principal, project uuid.UUID, source, target string, keys []string) ([]models.DiffEntry, error) {
	result := []models.DiffEntry{}
	err := s.Transaction(ctx, p, project, policy.ReadValue, func(tx *sql.Tx) error {
		if err := s.EventTx(ctx, tx, p, project, "promotion.diff", source); err != nil {
			return err
		}
		sourceRecords, err := s.promotionRecords(ctx, tx, p, project, source, keys)
		if err != nil {
			return err
		}
		_, dek, err := s.currentKey(ctx, tx, project)
		if err != nil {
			return err
		}
		defer crypto.SecureZero(dek)
		hash := func(plain []byte) string {
			h := hmac.New(sha256.New, dek)
			h.Write(plain)
			return hex.EncodeToString(h.Sum(nil))[:16]
		}
		for _, src := range sourceRecords {
			if err = s.event(ctx, tx, p, src, "secret.read"); err != nil {
				return err
			}
			plain, err := s.readVersion(ctx, tx, src, src.Revision)
			if err != nil {
				return err
			}
			entry := models.DiffEntry{Key: src.Key, SourceHash: hash(plain), SourceExists: true, Action: "add"}
			crypto.SecureZero(plain)
			dst, err := s.CurrentTx(ctx, tx, project, target, src.Key)
			if err == nil {
				if err = s.permit(ctx, tx, p, policy.ReadValue, dst); err != nil {
					return err
				}
				if err = s.event(ctx, tx, p, dst, "secret.read"); err != nil {
					return err
				}
				plain, err = s.readVersion(ctx, tx, dst, dst.Revision)
				if err != nil {
					return err
				}
				entry.TargetExists = true
				entry.TargetHash = hash(plain)
				crypto.SecureZero(plain)
				entry.Action = "update"
				if entry.SourceHash == entry.TargetHash {
					entry.Action = "no_change"
				}
			} else if !errors.Is(err, ErrNotFound) {
				return err
			}
			result = append(result, entry)
		}
		return nil
	})
	return result, err
}
