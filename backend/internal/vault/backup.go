package vault

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"io"
	"time"
)

const BackupFormat = "keepsave.encrypted-vault.v1"
const maxBundleSize = 64 << 20

// Bundle uses the existing service-secret AES-GCM format. SHA256 detects
// accidental transport corruption; GCM authenticates the encrypted manifest.
type Bundle struct {
	Format     string `json:"format"`
	Ciphertext []byte `json:"ciphertext"`
	Nonce      []byte `json:"nonce"`
	SHA256     string `json:"sha256"`
}
type backupKey struct {
	ID                uuid.UUID `json:"id"`
	Ciphertext, Nonce []byte
}
type backupEntry struct {
	Record        Record    `json:"record"`
	EnvironmentID uuid.UUID `json:"environment_id"`
	KeyID         uuid.UUID `json:"key_id"`
}
type backupRevision struct {
	SecretID, KeyID   uuid.UUID
	Revision          int64
	Ciphertext, Nonce []byte
	Operation         string
	CreatedAt         time.Time
	ActorID           *uuid.UUID
}
type backupSnapshot struct {
	ID, KeyID, PromotionID, EnvironmentID uuid.UUID
	Key                                   string
	Ciphertext, Nonce                     []byte
}
type backupEnvironment struct {
	ID   uuid.UUID
	Name string
}
type manifest struct {
	Format                  string
	ProjectID, CurrentKeyID uuid.UUID
	CreatedAt               time.Time
	Keys                    []backupKey
	Entries                 []backupEntry
	Revisions               []backupRevision
	Snapshots               []backupSnapshot
	Environments            []backupEnvironment
}
type Verification struct {
	ProjectID uuid.UUID `json:"project_id"`
	CreatedAt time.Time `json:"created_at"`
	Entries   int       `json:"entries"`
	Revisions int       `json:"revisions"`
	Keys      int       `json:"keys"`
	Snapshots int       `json:"snapshots"`
}

func (s *Service) Backup(ctx context.Context, p policy.Principal, project uuid.UUID) (Bundle, error) {
	tx, err := s.begin(ctx, project)
	if err != nil {
		return Bundle{}, err
	}
	defer tx.Rollback()
	rec := Record{ProjectID: project}
	if err = s.permit(ctx, tx, p, policy.ManageProject, rec); err != nil {
		return Bundle{}, err
	}
	if err = s.event(ctx, tx, p, rec, "backup.created"); err != nil {
		return Bundle{}, err
	}
	m := manifest{Format: BackupFormat, ProjectID: project, CreatedAt: time.Now().UTC()}
	if err = collect(ctx, tx, `SELECT id,name FROM environments WHERE project_id=$1 ORDER BY name`, project, func(rows *sql.Rows) error {
		var e backupEnvironment
		if err := rows.Scan(&e.ID, &e.Name); err != nil {
			return err
		}
		m.Environments = append(m.Environments, e)
		return nil
	}); err != nil {
		return Bundle{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT current_key_id FROM vault_projects WHERE project_id=$1`, project).Scan(&m.CurrentKeyID); err != nil {
		return Bundle{}, err
	}
	if err = collect(ctx, tx, `SELECT id,encrypted_key,nonce FROM vault_keys WHERE project_id=$1 ORDER BY id`, project, func(rows *sql.Rows) error {
		var k backupKey
		if e := rows.Scan(&k.ID, &k.Ciphertext, &k.Nonce); e != nil {
			return e
		}
		m.Keys = append(m.Keys, k)
		return nil
	}); err != nil {
		return Bundle{}, err
	}
	if err = collect(ctx, tx, `SELECT v.secret_id,v.environment_id,e.name,v.secret_key,v.revision,v.deleted,v.key_id,s.created_at,s.updated_at FROM vault_entries v JOIN environments e ON e.id=v.environment_id AND e.project_id=v.project_id LEFT JOIN secrets s ON s.id=v.secret_id AND s.project_id=v.project_id WHERE v.project_id=$1 ORDER BY v.secret_id`, project, func(rows *sql.Rows) error {
		var e backupEntry
		var created, updated sql.NullTime
		e.Record.ProjectID = project
		if err := rows.Scan(&e.Record.ID, &e.EnvironmentID, &e.Record.Environment, &e.Record.Key, &e.Record.Revision, &e.Record.Deleted, &e.KeyID, &created, &updated); err != nil {
			return err
		}
		e.Record.EnvironmentID = e.EnvironmentID
		if created.Valid {
			e.Record.CreatedAt = created.Time
		}
		if updated.Valid {
			e.Record.UpdatedAt = updated.Time
		}
		m.Entries = append(m.Entries, e)
		return nil
	}); err != nil {
		return Bundle{}, err
	}
	if err = collect(ctx, tx, `SELECT secret_id,key_id,revision,encrypted_value,nonce,operation,created_at,actor_id FROM vault_revisions WHERE project_id=$1 ORDER BY secret_id,revision`, project, func(rows *sql.Rows) error {
		var v backupRevision
		if err := rows.Scan(&v.SecretID, &v.KeyID, &v.Revision, &v.Ciphertext, &v.Nonce, &v.Operation, &v.CreatedAt, &v.ActorID); err != nil {
			return err
		}
		m.Revisions = append(m.Revisions, v)
		return nil
	}); err != nil {
		return Bundle{}, err
	}
	if err = collect(ctx, tx, `SELECT ss.id,v.key_id,ss.promotion_id,ss.environment_id,ss.key,ss.encrypted_value,ss.value_nonce FROM vault_snapshot_keys v JOIN secret_snapshots ss ON ss.id=v.snapshot_id WHERE v.project_id=$1 UNION ALL SELECT id,key_id,source_promotion_id,environment_id,secret_key,encrypted_value,nonce FROM vault_recovery_snapshots WHERE project_id=$1 ORDER BY 1`, project, func(rows *sql.Rows) error {
		var v backupSnapshot
		if err := rows.Scan(&v.ID, &v.KeyID, &v.PromotionID, &v.EnvironmentID, &v.Key, &v.Ciphertext, &v.Nonce); err != nil {
			return err
		}
		m.Snapshots = append(m.Snapshots, v)
		return nil
	}); err != nil {
		return Bundle{}, err
	}
	plain, err := json.Marshal(m)
	if err != nil {
		return Bundle{}, err
	}
	defer crypto.SecureZero(plain)
	if len(plain) > maxBundleSize {
		return Bundle{}, errors.New("backup exceeds bounded bundle size")
	}
	cipher, nonce, err := s.crypto.EncryptServiceSecret(plain)
	if err != nil {
		return Bundle{}, err
	}
	sum := sha256.Sum256(cipher)
	b := Bundle{BackupFormat, cipher, nonce, hex.EncodeToString(sum[:])}
	if err = tx.Commit(); err != nil {
		return Bundle{}, err
	}
	return b, nil
}
func collect(ctx context.Context, tx *sql.Tx, query string, project uuid.UUID, scan func(*sql.Rows) error) error {
	rows, err := tx.QueryContext(ctx, query, project)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err = scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
func (c *Bundle) decode(keys *crypto.Service) (manifest, error) {
	if keys == nil || c.Format != BackupFormat || len(c.Ciphertext) > maxBundleSize+32 || len(c.Nonce) != 12 {
		return manifest{}, errors.New("invalid backup format")
	}
	digest := sha256.Sum256(c.Ciphertext)
	if c.SHA256 != hex.EncodeToString(digest[:]) {
		return manifest{}, errors.New("backup integrity failure")
	}
	plain, err := keys.DecryptServiceSecret(c.Ciphertext, c.Nonce)
	if err != nil {
		return manifest{}, errors.New("backup authentication failed")
	}
	defer crypto.SecureZero(plain)
	var m manifest
	dec := json.NewDecoder(bytes.NewReader(plain))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&m); err != nil {
		return m, errors.New("invalid backup manifest")
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return m, errors.New("invalid backup trailing data")
	}
	if m.Format != BackupFormat || m.ProjectID == uuid.Nil {
		return m, errors.New("invalid backup identity")
	}
	return m, nil
}

// VerifyBackup needs only an external bundle and the independently recovered
// master-key service, not the original database. It returns metadata only.
func VerifyBackup(b Bundle, keys *crypto.Service) (Verification, error) {
	m, err := b.decode(keys)
	if err != nil {
		return Verification{}, err
	}
	deks := map[uuid.UUID][]byte{}
	defer func() {
		for _, d := range deks {
			crypto.SecureZero(d)
		}
	}()
	for _, k := range m.Keys {
		if _, exists := deks[k.ID]; exists {
			return Verification{}, errors.New("duplicate backup key")
		}
		d, err := keys.DecryptDEK(k.Ciphertext, k.Nonce)
		if err != nil {
			return Verification{}, errors.New("key recovery failed")
		}
		deks[k.ID] = d
	}
	if deks[m.CurrentKeyID] == nil {
		return Verification{}, errors.New("current key dependency missing")
	}
	entries := map[uuid.UUID]backupEntry{}
	counts := map[uuid.UUID]int64{}
	for _, e := range m.Entries {
		if e.Record.ProjectID != m.ProjectID || e.Record.ID == uuid.Nil || e.Record.Revision < 1 || deks[e.KeyID] == nil {
			return Verification{}, errors.New("invalid backup entry")
		}
		if _, ok := entries[e.Record.ID]; ok {
			return Verification{}, errors.New("duplicate backup entry")
		}
		entries[e.Record.ID] = e
	}
	for _, v := range m.Revisions {
		entry, exists := entries[v.SecretID]
		if !exists || v.Revision != counts[v.SecretID]+1 || v.Revision > entry.Record.Revision {
			return Verification{}, errors.New("backup history discontinuity")
		}
		d := deks[v.KeyID]
		if d == nil {
			return Verification{}, errors.New("historical key dependency missing")
		}
		if v.Operation != "delete" {
			p, e := crypto.Decrypt(d, v.Ciphertext, v.Nonce)
			crypto.SecureZero(p)
			if e != nil {
				return Verification{}, errors.New("historical value recovery failed")
			}
		} else if len(v.Ciphertext) != 0 || len(v.Nonce) != 0 {
			return Verification{}, errors.New("invalid tombstone")
		}
		if v.Revision == entry.Record.Revision && (entry.KeyID != v.KeyID || entry.Record.Deleted != (v.Operation == "delete")) {
			return Verification{}, errors.New("current revision mismatch")
		}
		counts[v.SecretID]++
	}
	for id, e := range entries {
		if counts[id] != e.Record.Revision {
			return Verification{}, errors.New("missing current revision")
		}
	}
	for _, v := range m.Snapshots {
		d := deks[v.KeyID]
		if d == nil {
			return Verification{}, errors.New("snapshot key dependency missing")
		}
		p, e := crypto.Decrypt(d, v.Ciphertext, v.Nonce)
		crypto.SecureZero(p)
		if e != nil {
			return Verification{}, errors.New("snapshot recovery failed")
		}
	}
	return Verification{m.ProjectID, m.CreatedAt, len(m.Entries), len(m.Revisions), len(m.Keys), len(m.Snapshots)}, nil
}
