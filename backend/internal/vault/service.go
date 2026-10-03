// Package vault owns plaintext credential access, encrypted history and recovery.
// PostgreSQL only until the platform's other database contracts are verified.
package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

var (
	ErrDenied      = errors.New("vault access denied")
	ErrConflict    = errors.New("vault revision conflict")
	ErrNotFound    = errors.New("vault record not found")
	ErrNotEnrolled = errors.New("project not enrolled in versioned vault")
	ErrInvalid     = errors.New("invalid vault request")
)

type AuthorizeTx func(context.Context, *sql.Tx, policy.Principal, policy.Action, policy.Resource) (policy.Decision, error)
type Auditor interface {
	CreateTx(*sql.Tx, *uuid.UUID, *uuid.UUID, string, string, models.JSONMap, string) error
}
type Service struct {
	db        *sql.DB
	crypto    *crypto.Service
	authorize AuthorizeTx
	audit     Auditor
	archive   ProjectArchive
}
type ProjectArchive interface {
	ArchiveProjectTx(context.Context, *sql.Tx, uuid.UUID) error
}

func (s *Service) EnableProjectArchive(h ProjectArchive) { s.archive = h }

func New(db *sql.DB, c *crypto.Service, authorize AuthorizeTx, audit Auditor) *Service {
	return &Service{db: db, crypto: c, authorize: authorize, audit: audit}
}

type Record struct {
	ID            uuid.UUID `json:"id"`
	ProjectID     uuid.UUID `json:"project_id"`
	Environment   string    `json:"environment"`
	Key           string    `json:"key"`
	Revision      int64     `json:"revision"`
	Value         string    `json:"value,omitempty"`
	Deleted       bool      `json:"deleted"`
	EnvironmentID uuid.UUID `json:"environment_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type Revision struct {
	Revision  int64      `json:"revision"`
	Operation string     `json:"operation"`
	CreatedAt time.Time  `json:"created_at"`
	ActorID   *uuid.UUID `json:"actor_id,omitempty"`
}

func (s *Service) begin(ctx context.Context, p policy.Principal, project uuid.UUID) (*sql.Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s'`); err != nil {
		tx.Rollback()
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='15s'`); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err = authority.Postgres(s.db).LockProject(ctx, tx, p, project, true); err != nil {
		tx.Rollback()
		return nil, ErrDenied
	}
	return tx, nil
}

func (s *Service) permit(ctx context.Context, tx *sql.Tx, p policy.Principal, action policy.Action, r Record) error {
	if s.authorize == nil || s.audit == nil {
		return ErrDenied
	}
	d, err := s.authorize(ctx, tx, p, action, policy.Resource{ProjectID: r.ProjectID, Environment: r.Environment, ID: r.ID, Key: r.Key, Type: "secret", Revision: r.Revision})
	if err != nil || !d.Allowed {
		return ErrDenied
	}
	return nil
}
func (s *Service) event(ctx context.Context, tx *sql.Tx, p policy.Principal, r Record, action string) error {
	return s.eventWithMetadataRevision(ctx, tx, p, r, action, 0)
}
func (s *Service) eventWithMetadataRevision(ctx context.Context, tx *sql.Tx, p policy.Principal, r Record, action string, metadataRevision int64) error {
	actor := p.ActorID
	if actor == uuid.Nil && p.Kind == policy.Human {
		actor = p.SubjectID
	}
	if actor == uuid.Nil {
		return ErrDenied
	}
	details := models.JSONMap{"secret_id": r.ID.String(), "revision": r.Revision, "key": r.Key}
	if metadataRevision > 0 {
		details["metadata_revision"] = metadataRevision
	}
	if err := s.audit.CreateTx(tx, &actor, &r.ProjectID, action, r.Environment, details, ""); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"project_id": r.ProjectID, "secret_id": r.ID, "revision": r.Revision, "action": action})
	if err != nil {
		return err
	}
	return jobs.EnqueueTx(ctx, tx, uuid.New(), "vault.event", payload, "local", 5)
}

// Enroll captures existing encrypted values as a labeled baseline. The caller
// must hold administrator authority; enrollment never invents earlier versions.
func (s *Service) Enroll(ctx context.Context, p policy.Principal, project uuid.UUID) error {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.EnrollTx(ctx, tx, p, project); err != nil {
		return err
	}
	return tx.Commit()
}

// EnrollTx joins project creation or an explicitly approved migration transaction.
// Callers must hold the project row lock before invoking it.
func (s *Service) EnrollTx(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID) error {
	var err error
	rec := Record{ProjectID: project}
	if err = s.permit(ctx, tx, p, policy.ManageProject, rec); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM vault_projects WHERE project_id=$1`, project).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	var encrypted, nonce []byte
	if err = tx.QueryRowContext(ctx, `SELECT encrypted_dek,dek_nonce FROM projects WHERE id=$1`, project).Scan(&encrypted, &nonce); err != nil {
		return err
	}
	if err = s.event(ctx, tx, p, rec, "vault.baseline.created"); err != nil {
		return err
	}
	dek, err := s.crypto.DecryptDEK(encrypted, nonce)
	if err != nil {
		return err
	}
	defer crypto.SecureZero(dek)
	// Refuse to label ciphertext with a key that cannot recover it.
	rows, err := tx.QueryContext(ctx, `SELECT encrypted_value,value_nonce FROM secrets WHERE project_id=$1 UNION ALL SELECT ss.encrypted_value,ss.value_nonce FROM secret_snapshots ss JOIN promotion_requests pr ON pr.id=ss.promotion_id WHERE pr.project_id=$1 AND ss.prior_existed`, project)
	if err != nil {
		return err
	}
	for rows.Next() {
		var c, n []byte
		if err = rows.Scan(&c, &n); err != nil {
			rows.Close()
			return err
		}
		plain, e := crypto.Decrypt(dek, c, n)
		crypto.SecureZero(plain)
		if e != nil {
			rows.Close()
			return errors.New("existing ciphertext requires unavailable key; enrollment refused")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	key := uuid.New()
	if _, err = tx.ExecContext(ctx, `INSERT INTO vault_keys(id,project_id,encrypted_key,nonce) VALUES($1,$2,$3,$4)`, key, project, encrypted, nonce); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO vault_projects(project_id,current_key_id) VALUES($1,$2)`, project, key); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO vault_entries(secret_id,project_id,environment_id,secret_key,revision,key_id) SELECT id,project_id,environment_id,key,1,$2 FROM secrets WHERE project_id=$1`, project, key); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO vault_revisions(secret_id,project_id,revision,key_id,encrypted_value,nonce,operation) SELECT id,project_id,1,$2,encrypted_value,value_nonce,'baseline' FROM secrets WHERE project_id=$1`, project, key); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO vault_snapshot_keys(snapshot_id,project_id,key_id) SELECT ss.id,$1,$2 FROM secret_snapshots ss JOIN promotion_requests pr ON pr.id=ss.promotion_id WHERE pr.project_id=$1 AND ss.prior_existed`, project, key); err != nil {
		return err
	}
	return nil
}
func (s *Service) currentKey(ctx context.Context, tx *sql.Tx, project uuid.UUID) (uuid.UUID, []byte, error) {
	var id uuid.UUID
	var c, n []byte
	err := tx.QueryRowContext(ctx, `SELECT k.id,k.encrypted_key,k.nonce FROM vault_projects vp JOIN vault_keys k ON k.id=vp.current_key_id AND k.project_id=vp.project_id WHERE vp.project_id=$1`, project).Scan(&id, &c, &n)
	if errors.Is(err, sql.ErrNoRows) {
		return id, nil, ErrNotEnrolled
	}
	if err != nil {
		return id, nil, err
	}
	dek, err := s.crypto.DecryptDEK(c, n)
	return id, dek, err
}
func (s *Service) load(ctx context.Context, tx *sql.Tx, project, id uuid.UUID) (Record, uuid.UUID, error) {
	r := Record{ID: id, ProjectID: project}
	var env uuid.UUID
	var created, updated sql.NullTime
	err := tx.QueryRowContext(ctx, `SELECT e.name,v.environment_id,v.secret_key,v.revision,v.deleted,s.created_at,s.updated_at FROM vault_entries v JOIN environments e ON e.id=v.environment_id AND e.project_id=v.project_id LEFT JOIN secrets s ON s.id=v.secret_id AND s.project_id=v.project_id WHERE v.secret_id=$1 AND v.project_id=$2`, id, project).Scan(&r.Environment, &env, &r.Key, &r.Revision, &r.Deleted, &created, &updated)
	r.EnvironmentID = env
	if created.Valid {
		r.CreatedAt = created.Time
	}
	if updated.Valid {
		r.UpdatedAt = updated.Time
	}
	if errors.Is(err, sql.ErrNoRows) {
		return r, env, ErrNotFound
	}
	return r, env, err
}
func (s *Service) append(ctx context.Context, tx *sql.Tx, p policy.Principal, r Record, keyID uuid.UUID, cipher, nonce []byte, operation string) error {
	var actor any
	if p.ActorID != uuid.Nil {
		actor = p.ActorID
	} else if p.Kind == policy.Human {
		actor = p.SubjectID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO vault_revisions(secret_id,project_id,revision,key_id,encrypted_value,nonce,operation,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, r.ID, r.ProjectID, r.Revision, keyID, cipher, nonce, operation, actor)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE vault_entries SET revision=$1,key_id=$2,deleted=$3 WHERE secret_id=$4 AND project_id=$5`, r.Revision, keyID, r.Deleted, r.ID, r.ProjectID)
	return err
}

// Put requires the expected current revision (zero means a new key).
func (s *Service) Put(ctx context.Context, p policy.Principal, r Record, expected int64) (Record, error) {
	tx, err := s.begin(ctx, p, r.ProjectID)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback()
	result, err := s.PutTx(ctx, tx, p, r, expected, "")
	if err != nil {
		return Record{}, err
	}
	if err = tx.Commit(); err != nil {
		return Record{}, err
	}
	return result, nil
}

// PutTx is the shared mutation boundary for adapters and promotion. Callers
// hold the project row lock and must commit only after all domain changes.
func (s *Service) PutTx(ctx context.Context, tx *sql.Tx, p policy.Principal, r Record, expected int64, operation string) (Record, error) {
	if (r.ID == uuid.Nil && !ValidKey(r.Key)) || len(r.Value) > 1<<20 {
		return Record{}, ErrInvalid
	}
	var err error
	var env uuid.UUID
	r.Deleted = false
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
		r.Revision = 1
		if expected != 0 {
			return Record{}, ErrConflict
		}
		err = tx.QueryRowContext(ctx, `SELECT id FROM environments WHERE project_id=$1 AND name=$2`, r.ProjectID, r.Environment).Scan(&env)
	} else {
		value := r.Value
		r, env, err = s.load(ctx, tx, r.ProjectID, r.ID)
		r.Value = value
		if err == nil && (r.Deleted || r.Revision != expected) {
			return Record{}, ErrConflict
		}
		r.Revision++
	}
	if err != nil {
		return Record{}, ErrNotFound
	}
	if err = s.permit(ctx, tx, p, policy.WriteSecret, r); err != nil {
		return Record{}, err
	}
	event := "secret.updated"
	if expected == 0 {
		event = "secret.created"
	}
	if err = s.event(ctx, tx, p, r, event); err != nil {
		return Record{}, err
	}
	key, dek, err := s.currentKey(ctx, tx, r.ProjectID)
	if err != nil {
		return Record{}, err
	}
	defer crypto.SecureZero(dek)
	cipher, nonce, err := crypto.Encrypt(dek, []byte(r.Value))
	if err != nil {
		return Record{}, err
	}
	if operation == "" {
		operation = "update"
	}
	if expected == 0 {
		if operation == "update" {
			operation = "create"
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO vault_entries(secret_id,project_id,environment_id,secret_key,revision,key_id) VALUES($1,$2,$3,$4,1,$5)`, r.ID, r.ProjectID, env, r.Key, key); err != nil {
			return Record{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO secrets(id,project_id,environment_id,key,encrypted_value,value_nonce) VALUES($1,$2,$3,$4,$5,$6)`, r.ID, r.ProjectID, env, r.Key, cipher, nonce)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE secrets SET encrypted_value=$1,value_nonce=$2,updated_at=NOW() WHERE id=$3 AND project_id=$4`, cipher, nonce, r.ID, r.ProjectID)
	}
	if err != nil {
		return Record{}, err
	}
	if err = s.append(ctx, tx, p, r, key, cipher, nonce, operation); err != nil {
		return Record{}, err
	}
	r, _, err = s.load(ctx, tx, r.ProjectID, r.ID)
	if err != nil {
		return Record{}, err
	}
	r.Value = ""
	return r, nil
}
func (s *Service) readVersion(ctx context.Context, tx *sql.Tx, r Record, version int64) ([]byte, error) {
	var cipher, nonce, encryptedKey, keyNonce []byte
	err := tx.QueryRowContext(ctx, `SELECT vr.encrypted_value,vr.nonce,k.encrypted_key,k.nonce FROM vault_revisions vr JOIN vault_keys k ON k.id=vr.key_id AND k.project_id=vr.project_id WHERE vr.secret_id=$1 AND vr.project_id=$2 AND vr.revision=$3`, r.ID, r.ProjectID, version).Scan(&cipher, &nonce, &encryptedKey, &keyNonce)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	dek, err := s.crypto.DecryptDEK(encryptedKey, keyNonce)
	if err != nil {
		return nil, err
	}
	defer crypto.SecureZero(dek)
	return crypto.Decrypt(dek, cipher, nonce)
}
func (s *Service) Read(ctx context.Context, p policy.Principal, project, id uuid.UUID, version int64) (Record, error) {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback()
	r, _, err := s.load(ctx, tx, project, id)
	if err != nil {
		return Record{}, err
	}
	if r.Deleted {
		return Record{}, ErrNotFound
	}
	if version == 0 {
		version = r.Revision
	}
	if err = s.permit(ctx, tx, p, policy.ReadValue, r); err != nil {
		return Record{}, err
	}
	admitted := r
	admitted.Revision = version
	if err = s.event(ctx, tx, p, admitted, "secret.read"); err != nil {
		return Record{}, err
	}
	plain, err := s.readVersion(ctx, tx, r, version)
	if err != nil {
		return Record{}, err
	}
	defer crypto.SecureZero(plain)
	if err = tx.Commit(); err != nil {
		return Record{}, err
	}
	r.Value = string(plain)
	r.Revision = version
	return r, nil
}
func (s *Service) Restore(ctx context.Context, p policy.Principal, project, id uuid.UUID, version, expected int64) (Record, error) {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback()
	r, _, err := s.load(ctx, tx, project, id)
	if err != nil {
		return Record{}, err
	}
	if r.Deleted || r.Revision != expected || version < 1 {
		return Record{}, ErrConflict
	}
	if err = s.permit(ctx, tx, p, policy.RestoreSecret, r); err != nil {
		return Record{}, err
	}
	r.Revision++
	if err = s.event(ctx, tx, p, r, "secret.restored"); err != nil {
		return Record{}, err
	}
	plain, err := s.readVersion(ctx, tx, r, version)
	if err != nil {
		return Record{}, err
	}
	defer crypto.SecureZero(plain)
	key, dek, err := s.currentKey(ctx, tx, project)
	if err != nil {
		return Record{}, err
	}
	defer crypto.SecureZero(dek)
	cipher, nonce, err := crypto.Encrypt(dek, plain)
	if err != nil {
		return Record{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE secrets SET encrypted_value=$1,value_nonce=$2,updated_at=NOW() WHERE id=$3 AND project_id=$4`, cipher, nonce, id, project); err != nil {
		return Record{}, err
	}
	if err = s.append(ctx, tx, p, r, key, cipher, nonce, "restore"); err != nil {
		return Record{}, err
	}
	r, _, err = s.load(ctx, tx, project, id)
	if err != nil {
		return Record{}, err
	}
	if err = tx.Commit(); err != nil {
		return Record{}, err
	}
	return r, nil
}
func (s *Service) History(ctx context.Context, p policy.Principal, project, id uuid.UUID) ([]Revision, error) {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	r, _, err := s.load(ctx, tx, project, id)
	if err != nil {
		return nil, err
	}
	if err = s.permit(ctx, tx, p, policy.ReadMetadata, r); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT revision,operation,created_at,actor_id FROM vault_revisions WHERE secret_id=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 100`, id, project)
	if err != nil {
		return nil, err
	}
	result := []Revision{}
	for rows.Next() {
		var v Revision
		if err = rows.Scan(&v.Revision, &v.Operation, &v.CreatedAt, &v.ActorID); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return result, tx.Commit()
}

func (s *Service) Delete(ctx context.Context, p policy.Principal, project, id uuid.UUID, expected int64) error {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.DeleteTx(ctx, tx, p, project, id, expected); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteTx appends a tombstone in the same transaction as an enclosing flow.
func (s *Service) DeleteTx(ctx context.Context, tx *sql.Tx, p policy.Principal, project, id uuid.UUID, expected int64) error {
	r, _, err := s.load(ctx, tx, project, id)
	if err != nil {
		return err
	}
	if r.Deleted || r.Revision != expected {
		return ErrConflict
	}
	if err = s.permit(ctx, tx, p, policy.DeleteSecret, r); err != nil {
		return err
	}
	r.Revision++
	r.Deleted = true
	if err = s.event(ctx, tx, p, r, "secret.deleted"); err != nil {
		return err
	}
	var key uuid.UUID
	if err = tx.QueryRowContext(ctx, `SELECT key_id FROM vault_entries WHERE secret_id=$1 AND project_id=$2`, id, project).Scan(&key); err != nil {
		return err
	}
	if err = s.append(ctx, tx, p, r, key, []byte{}, []byte{}, "delete"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM secrets WHERE id=$1 AND project_id=$2`, id, project); err != nil {
		return err
	}
	return nil
}
