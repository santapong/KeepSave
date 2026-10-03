package vault

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

// RestoreSelection binds a selected backed-up value to the exact current
// revision. Secret identifiers must still exist and be active in this project.
type RestoreSelection struct {
	SecretID                 uuid.UUID  `json:"secret_id"`
	BackupRevision           int64      `json:"backup_revision"`
	ExpectedRevision         int64      `json:"expected_current_revision"`
	RestoreLifecycle         bool       `json:"restore_lifecycle,omitempty"`
	ExpectedMetadataRevision int64      `json:"expected_metadata_revision,omitempty"`
	MappedResponsibleUserID  *uuid.UUID `json:"mapped_responsible_user_id,omitempty"`
}
type RestoreDiff struct {
	SecretID                uuid.UUID  `json:"secret_id"`
	Environment             string     `json:"environment"`
	Key                     string     `json:"key"`
	BackupRevision          int64      `json:"backup_revision"`
	CurrentRevision         int64      `json:"current_revision"`
	Status                  string     `json:"status"`
	Restorable              bool       `json:"restorable"`
	BackupLifecycle         *Lifecycle `json:"backup_lifecycle,omitempty"`
	CurrentMetadataRevision int64      `json:"current_metadata_revision,omitempty"`
}
type RestorePreview struct {
	ProjectID       uuid.UUID     `json:"project_id"`
	BackupCreatedAt time.Time     `json:"backup_created_at"`
	Records         []RestoreDiff `json:"records"`
}

func verifiedManifest(b Bundle, keys *crypto.Service) (manifest, error) {
	if _, err := VerifyBackup(b, keys); err != nil {
		return manifest{}, ErrInvalid
	}
	m, err := b.decode(keys)
	if err != nil {
		return manifest{}, ErrInvalid
	}
	return m, nil
}

// PreviewRestore is a metadata-only diff. It does not create entries or read
// plaintext into responses. Only administrators can inspect a complete backup.
func (s *Service) PreviewRestore(ctx context.Context, p policy.Principal, project uuid.UUID, b Bundle) (RestorePreview, error) {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return RestorePreview{}, err
	}
	defer tx.Rollback()
	if err = s.permit(ctx, tx, p, policy.ManageProject, Record{ProjectID: project}); err != nil {
		return RestorePreview{}, err
	}
	if err = s.event(ctx, tx, p, Record{ProjectID: project}, "backup.restore.previewed"); err != nil {
		return RestorePreview{}, err
	}
	m, err := verifiedManifest(b, s.crypto)
	if err != nil {
		return RestorePreview{}, err
	}
	if m.ProjectID != project {
		return RestorePreview{}, ErrDenied
	}
	result := RestorePreview{ProjectID: project, BackupCreatedAt: m.CreatedAt, Records: []RestoreDiff{}}
	lifecycle := map[uuid.UUID]Lifecycle{}
	for _, l := range m.Lifecycle {
		lifecycle[l.SecretID] = l
	}
	for _, entry := range m.Entries {
		r := entry.Record
		d := RestoreDiff{SecretID: r.ID, Environment: r.Environment, Key: r.Key, BackupRevision: r.Revision, Status: "missing"}
		if life, ok := lifecycle[r.ID]; ok {
			d.BackupLifecycle = &life
		}
		current, _, e := s.load(ctx, tx, project, r.ID)
		if e != nil && !errors.Is(e, ErrNotFound) {
			return RestorePreview{}, e
		}
		if e == nil {
			life, err := lifecycleTx(ctx, tx, project, r.ID)
			if err != nil {
				return RestorePreview{}, err
			}
			d.CurrentMetadataRevision = life.Revision
			d.CurrentRevision = current.Revision
			d.Status = "changed"
			d.Restorable = !current.Deleted && !r.Deleted && current.Environment == r.Environment && current.Key == r.Key
			if current.Deleted {
				d.Status = "deleted"
			} else if current.Revision == r.Revision {
				d.Status = "same_revision"
			}
		}
		if r.Deleted {
			d.Status = "backup_tombstone"
			d.Restorable = false
		}
		result.Records = append(result.Records, d)
	}
	sort.Slice(result.Records, func(i, j int) bool {
		a, b := result.Records[i], result.Records[j]
		if a.Environment == b.Environment {
			return a.Key < b.Key
		}
		return a.Environment < b.Environment
	})
	if err = tx.Commit(); err != nil {
		return RestorePreview{}, err
	}
	return result, nil
}

// RestoreSelected never replaces a project. Every selected active record is
// restored as a new revision, atomically, after current authority and revision
// checks. Tombstoned or missing entries cannot be resurrected.
func (s *Service) RestoreSelected(ctx context.Context, p policy.Principal, project uuid.UUID, b Bundle, selection []RestoreSelection) ([]Record, error) {
	if len(selection) == 0 || len(selection) > 1000 {
		return nil, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	extra := []uuid.UUID{}
	for _, chosen := range selection {
		if chosen.RestoreLifecycle && chosen.MappedResponsibleUserID != nil {
			extra = append(extra, *chosen.MappedResponsibleUserID)
		}
	}
	if err = authority.Postgres(s.db).LockProjectSubjects(ctx, tx, p, project, true, extra); err != nil {
		tx.Rollback()
		return nil, ErrDenied
	}

	defer tx.Rollback()
	if err = s.permit(ctx, tx, p, policy.ManageProject, Record{ProjectID: project}); err != nil {
		return nil, err
	}
	if err = s.event(ctx, tx, p, Record{ProjectID: project}, "backup.restore.admitted"); err != nil {
		return nil, err
	}
	m, err := verifiedManifest(b, s.crypto)
	if err != nil {
		return nil, err
	}
	if m.ProjectID != project {
		return nil, ErrDenied
	}
	entries := map[uuid.UUID]backupEntry{}
	for _, e := range m.Entries {
		entries[e.Record.ID] = e
	}
	keys := map[uuid.UUID]backupKey{}
	for _, k := range m.Keys {
		keys[k.ID] = k
	}
	versions := map[uuid.UUID]map[int64]backupRevision{}
	for _, v := range m.Revisions {
		if versions[v.SecretID] == nil {
			versions[v.SecretID] = map[int64]backupRevision{}
		}
		versions[v.SecretID][v.Revision] = v
	}
	lifecycle := map[uuid.UUID]Lifecycle{}
	for _, l := range m.Lifecycle {
		lifecycle[l.SecretID] = l
	}
	seen := map[uuid.UUID]bool{}
	result := []Record{}
	for _, chosen := range selection {
		if chosen.SecretID == uuid.Nil || chosen.BackupRevision < 1 || chosen.ExpectedRevision < 1 || seen[chosen.SecretID] {
			return nil, ErrInvalid
		}
		seen[chosen.SecretID] = true
		entry, ok := entries[chosen.SecretID]
		if !ok || entry.Record.Deleted {
			return nil, ErrConflict
		}
		revision, ok := versions[chosen.SecretID][chosen.BackupRevision]
		if !ok || revision.Operation == "delete" {
			return nil, ErrInvalid
		}
		current, _, e := s.load(ctx, tx, project, chosen.SecretID)
		if e != nil {
			return nil, e
		}
		if current.Deleted || current.Revision != chosen.ExpectedRevision || current.Environment != entry.Record.Environment || current.Key != entry.Record.Key {
			return nil, ErrConflict
		}
		if err = s.permit(ctx, tx, p, policy.RestoreSecret, current); err != nil {
			return nil, err
		}
		if chosen.RestoreLifecycle {
			backed, ok := lifecycle[chosen.SecretID]
			if !ok {
				return nil, ErrInvalid
			}
			if backed.ResponsibleUserID != nil && chosen.MappedResponsibleUserID == nil {
				return nil, ErrInvalid
			}
			existing, e := lifecycleTx(ctx, tx, project, chosen.SecretID)
			if e != nil {
				return nil, e
			}
			if existing.Revision != chosen.ExpectedMetadataRevision {
				return nil, ErrConflict
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO vault_lifecycle(secret_id,project_id,responsible_user_id,declared_expires_at,renewal_at,provenance,revision) VALUES($1,$2,$3,$4,$5,$6,1) ON CONFLICT(secret_id) DO UPDATE SET responsible_user_id=EXCLUDED.responsible_user_id,declared_expires_at=EXCLUDED.declared_expires_at,renewal_at=EXCLUDED.renewal_at,provenance=EXCLUDED.provenance,revision=vault_lifecycle.revision+1,updated_at=NOW()`, chosen.SecretID, project, chosen.MappedResponsibleUserID, backed.DeclaredExpiresAt, backed.RenewalAt, backed.Provenance); e != nil {
				return nil, e
			}
			if err = s.event(ctx, tx, p, current, "secret.lifecycle_restored"); err != nil {
				return nil, err
			}
		}
		// Record the admission before unwrapping the backed-up credential key.
		next := current
		next.Revision++
		if err = s.event(ctx, tx, p, next, "secret.restored"); err != nil {
			return nil, err
		}
		key := keys[revision.KeyID]
		dek, e := s.crypto.DecryptDEK(key.Ciphertext, key.Nonce)
		if e != nil {
			return nil, e
		}
		plain, e := crypto.Decrypt(dek, revision.Ciphertext, revision.Nonce)
		crypto.SecureZero(dek)
		if e != nil {
			return nil, e
		}
		current.Value = string(plain)
		crypto.SecureZero(plain)
		written, e := s.PutTx(ctx, tx, p, current, chosen.ExpectedRevision, "backup_restore")
		current.Value = ""
		if e != nil {
			return nil, e
		}
		result = append(result, written)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// RecoverToEmptyProject is an operator-only isolated drill entry point, never
// an HTTP API. It uses independently supplied recovery material and accepts
// only an empty, explicitly selected target project. It copies encrypted vault
// records and history; no account, session, grant or promotion authority moves.
// The caller must keep the target isolated until verification is complete.
func (s *Service) RecoverToEmptyProject(ctx context.Context, p policy.Principal, target uuid.UUID, b Bundle, recoveryKeys *crypto.Service) (Verification, error) {
	return s.RecoverToEmptyProjectWithOwnerMap(ctx, p, target, b, recoveryKeys, nil)
}

// Owner mappings are explicit current-target identities, not recovered authority.
func (s *Service) RecoverToEmptyProjectWithOwnerMap(ctx context.Context, p policy.Principal, target uuid.UUID, b Bundle, recoveryKeys *crypto.Service, owners map[uuid.UUID]uuid.UUID) (Verification, error) {
	if recoveryKeys == nil {
		return Verification{}, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Verification{}, err
	}
	extra := []uuid.UUID{}
	for source, mapped := range owners {
		if source == uuid.Nil || mapped == uuid.Nil {
			tx.Rollback()
			return Verification{}, ErrInvalid
		}
		extra = append(extra, mapped)
	}
	if err = authority.Postgres(s.db).LockProjectSubjects(ctx, tx, p, target, true, extra); err != nil {
		tx.Rollback()
		return Verification{}, ErrDenied
	}

	defer tx.Rollback()
	if err = s.permit(ctx, tx, p, policy.ManageProject, Record{ProjectID: target}); err != nil {
		return Verification{}, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM secrets WHERE project_id=$1)+(SELECT COUNT(*) FROM vault_entries WHERE project_id=$1)+(SELECT COUNT(*) FROM promotion_requests WHERE project_id=$1)`, target).Scan(&count); err != nil {
		return Verification{}, err
	}
	if count != 0 {
		return Verification{}, ErrConflict
	}
	if err = s.event(ctx, tx, p, Record{ProjectID: target}, "backup.isolated_recovery.admitted"); err != nil {
		return Verification{}, err
	}
	m, err := verifiedManifest(b, recoveryKeys)
	if err != nil {
		return Verification{}, err
	}
	if m.ProjectID == target {
		return Verification{}, ErrInvalid
	}
	envs := map[uuid.UUID]uuid.UUID{}
	names := map[string]bool{}
	for _, env := range m.Environments {
		if env.ID == uuid.Nil || env.Name == "" || len(env.Name) > 50 || names[env.Name] || envs[env.ID] != uuid.Nil {
			return Verification{}, ErrInvalid
		}
		names[env.Name] = true
		var id uuid.UUID
		err = tx.QueryRowContext(ctx, `SELECT id FROM environments WHERE project_id=$1 AND name=$2`, target, env.Name).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			id = uuid.New()
			_, err = tx.ExecContext(ctx, `INSERT INTO environments(id,project_id,name) VALUES($1,$2,$3)`, id, target, env.Name)
		}
		if err != nil {
			return Verification{}, err
		}
		envs[env.ID] = id
	}
	for _, entry := range m.Entries {
		if envs[entry.EnvironmentID] == uuid.Nil {
			return Verification{}, ErrInvalid
		}
	}
	for _, snapshot := range m.Snapshots {
		if envs[snapshot.EnvironmentID] == uuid.Nil {
			return Verification{}, ErrInvalid
		}
	}
	var currentCipher, currentNonce []byte
	for _, key := range m.Keys {
		dek, e := recoveryKeys.DecryptDEK(key.Ciphertext, key.Nonce)
		if e != nil {
			return Verification{}, e
		}
		encrypted, nonce, e := s.crypto.EncryptDEK(dek)
		crypto.SecureZero(dek)
		if e != nil {
			return Verification{}, e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO vault_keys(id,project_id,encrypted_key,nonce) VALUES($1,$2,$3,$4)`, key.ID, target, encrypted, nonce); e != nil {
			return Verification{}, e
		}
		if key.ID == m.CurrentKeyID {
			currentCipher = encrypted
			currentNonce = nonce
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO vault_projects(project_id,current_key_id) VALUES($1,$2) ON CONFLICT(project_id) DO UPDATE SET current_key_id=EXCLUDED.current_key_id`, target, m.CurrentKeyID); err != nil {
		return Verification{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET encrypted_dek=$1,dek_nonce=$2,updated_at=NOW() WHERE id=$3`, currentCipher, currentNonce, target); err != nil {
		return Verification{}, err
	}
	for _, entry := range m.Entries {
		r := entry.Record
		if _, err = tx.ExecContext(ctx, `INSERT INTO vault_entries(secret_id,project_id,environment_id,secret_key,revision,key_id,deleted) VALUES($1,$2,$3,$4,$5,$6,$7)`, r.ID, target, envs[entry.EnvironmentID], r.Key, r.Revision, entry.KeyID, r.Deleted); err != nil {
			return Verification{}, err
		}
	}
	for _, l := range m.Lifecycle {
		var mapped any
		if l.ResponsibleUserID != nil {
			owner, ok := owners[*l.ResponsibleUserID]
			if !ok {
				return Verification{}, ErrInvalid
			}
			mapped = owner
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO vault_lifecycle(secret_id,project_id,responsible_user_id,declared_expires_at,renewal_at,provenance,revision,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, l.SecretID, target, mapped, l.DeclaredExpiresAt, l.RenewalAt, l.Provenance, l.Revision, l.UpdatedAt); err != nil {
			return Verification{}, err
		}
	}
	for _, revision := range m.Revisions {
		if _, err = tx.ExecContext(ctx, `INSERT INTO vault_revisions(secret_id,project_id,revision,key_id,encrypted_value,nonce,operation,created_at,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, revision.SecretID, target, revision.Revision, revision.KeyID, revision.Ciphertext, revision.Nonce, revision.Operation, revision.CreatedAt, revision.ActorID); err != nil {
			return Verification{}, err
		}
	}
	currents := map[uuid.UUID]backupRevision{}
	for _, r := range m.Revisions {
		currents[r.SecretID] = r
	}
	for _, entry := range m.Entries {
		if entry.Record.Deleted {
			continue
		}
		r := currents[entry.Record.ID]
		created, updated := entry.Record.CreatedAt, entry.Record.UpdatedAt
		if created.IsZero() {
			created = m.CreatedAt
		}
		if updated.IsZero() {
			updated = m.CreatedAt
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO secrets(id,project_id,environment_id,key,encrypted_value,value_nonce,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, entry.Record.ID, target, envs[entry.EnvironmentID], entry.Record.Key, r.Ciphertext, r.Nonce, created, updated); err != nil {
			return Verification{}, err
		}
	}
	for _, snapshot := range m.Snapshots {
		if _, err = tx.ExecContext(ctx, `INSERT INTO vault_recovery_snapshots(id,project_id,key_id,source_promotion_id,environment_id,secret_key,encrypted_value,nonce) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, snapshot.ID, target, snapshot.KeyID, snapshot.PromotionID, envs[snapshot.EnvironmentID], snapshot.Key, snapshot.Ciphertext, snapshot.Nonce); err != nil {
			return Verification{}, err
		}
	}
	if err = s.event(ctx, tx, p, Record{ProjectID: target}, "backup.isolated_recovery.completed"); err != nil {
		return Verification{}, err
	}
	if err = tx.Commit(); err != nil {
		return Verification{}, err
	}
	return Verification{ProjectID: target, CreatedAt: m.CreatedAt, Entries: len(m.Entries), Revisions: len(m.Revisions), Keys: len(m.Keys), Snapshots: len(m.Snapshots), LifecycleRecords: len(m.Lifecycle)}, nil
}

// VerifyBundle authorizes verification as a project operation. Standalone
// offline operators instead use VerifyBackup with independent recovery keys.
func (s *Service) VerifyBundle(ctx context.Context, p policy.Principal, project uuid.UUID, b Bundle) (Verification, error) {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return Verification{}, err
	}
	defer tx.Rollback()
	if err = s.permit(ctx, tx, p, policy.ManageProject, Record{ProjectID: project}); err != nil {
		return Verification{}, err
	}
	if err = s.event(ctx, tx, p, Record{ProjectID: project}, "backup.verified"); err != nil {
		return Verification{}, err
	}
	metadata, err := VerifyBackup(b, s.crypto)
	if err != nil {
		return Verification{}, ErrInvalid
	}
	if metadata.ProjectID != project {
		return Verification{}, ErrDenied
	}
	if err = tx.Commit(); err != nil {
		return Verification{}, err
	}
	return metadata, nil
}
