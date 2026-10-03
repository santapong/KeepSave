package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

const MaxReadBytes = 4 << 20
const MaxBatchKeys = 100

func ValidKey(key string) bool {
	return key != "" && len(key) <= 255 && !strings.Contains(key, "=") && strings.IndexFunc(key, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) < 0
}

func (s *Service) collectionState(ctx context.Context, tx *sql.Tx, project uuid.UUID, environment string) error {
	var enrolled bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM vault_projects WHERE project_id=$1)`, project).Scan(&enrolled); err != nil {
		return err
	}
	if !enrolled {
		return ErrNotEnrolled
	}
	var id uuid.UUID
	if err := tx.QueryRowContext(ctx, `SELECT id FROM environments WHERE project_id=$1 AND name=$2`, project, environment).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	return nil
}

type BatchResult struct {
	Secrets     []Record `json:"secrets"`
	MissingKeys []string `json:"missing_keys"`
}
type WriteResult struct {
	Created []string `json:"created"`
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
	Records []Record `json:"-"`
}

func (s *Service) collectionPermit(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, environment string) error {
	if s.authorize == nil || s.audit == nil {
		return ErrDenied
	}
	d, err := s.authorize(ctx, tx, p, policy.ReadValue, policy.Resource{ProjectID: project, Environment: environment, Type: "secret_collection"})
	if err != nil {
		return err
	}
	if !d.Allowed {
		return ErrDenied
	}
	return nil
}

// Metadata authorizes the requested action without decrypting a value. A
// write-only key can use WriteSecret; metadata never implies read authority.
func (s *Service) Metadata(ctx context.Context, p policy.Principal, project, id uuid.UUID, action policy.Action) (Record, error) {
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
	if err = s.permit(ctx, tx, p, action, r); err != nil {
		return Record{}, err
	}
	return r, tx.Commit()
}

func (s *Service) readTx(ctx context.Context, tx *sql.Tx, p policy.Principal, r Record) (Record, error) {
	if err := s.permit(ctx, tx, p, policy.ReadValue, r); err != nil {
		return Record{}, err
	}
	if err := s.event(ctx, tx, p, r, "secret.read"); err != nil {
		return Record{}, err
	}
	plain, err := s.readVersion(ctx, tx, r, r.Revision)
	if err != nil {
		return Record{}, err
	}
	r.Value = string(plain)
	for i := range plain {
		plain[i] = 0
	}
	return r, nil
}

// List filters denied keys before decryption. A failed authoritative lookup
// fails the entire read rather than returning a partial apparent success.
func (s *Service) List(ctx context.Context, p policy.Principal, project uuid.UUID, environment string) ([]Record, error) {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.collectionPermit(ctx, tx, p, project, environment); err != nil {
		return nil, err
	}
	if err = s.collectionState(ctx, tx, project, environment); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT v.secret_id FROM vault_entries v JOIN environments e ON e.id=v.environment_id AND e.project_id=v.project_id WHERE v.project_id=$1 AND e.name=$2 AND NOT v.deleted ORDER BY v.secret_key`, project, environment)
	if err != nil {
		return nil, err
	}
	ids := []uuid.UUID{}
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
	result := []Record{}
	size := 0
	for _, id := range ids {
		r, _, err := s.load(ctx, tx, project, id)
		if err != nil {
			return nil, err
		}
		d, e := s.authorize(ctx, tx, p, policy.ReadValue, policy.Resource{ProjectID: project, Environment: r.Environment, ID: r.ID, Key: r.Key, Type: "secret", Revision: r.Revision})
		if e != nil {
			return nil, e
		}
		if !d.Allowed {
			continue
		}
		r, e = s.readTx(ctx, tx, p, r)
		if e != nil {
			return nil, e
		}
		encoded, e := json.Marshal(r)
		if e != nil {
			return nil, e
		}
		size += len(encoded) + 1
		if size > MaxReadBytes {
			return nil, ErrInvalid
		}
		result = append(result, r)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) BatchRead(ctx context.Context, p policy.Principal, project uuid.UUID, environment string, keys []string) (BatchResult, error) {
	result := BatchResult{Secrets: []Record{}, MissingKeys: []string{}}
	if len(keys) == 0 || len(keys) > MaxBatchKeys {
		return result, ErrInvalid
	}
	seen := map[string]bool{}
	ordered := []string{}
	for _, key := range keys {
		if !ValidKey(key) {
			return result, ErrInvalid
		}
		if !seen[key] {
			seen[key] = true
			ordered = append(ordered, key)
		}
	}
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return BatchResult{}, err
	}
	defer tx.Rollback()
	if err = s.collectionPermit(ctx, tx, p, project, environment); err != nil {
		return BatchResult{}, err
	}
	if err = s.collectionState(ctx, tx, project, environment); err != nil {
		return BatchResult{}, err
	}
	size := 0
	for _, key := range ordered {
		var id uuid.UUID
		err = tx.QueryRowContext(ctx, `SELECT v.secret_id FROM vault_entries v JOIN environments e ON e.id=v.environment_id AND e.project_id=v.project_id WHERE v.project_id=$1 AND e.name=$2 AND v.secret_key=$3 AND NOT v.deleted`, project, environment, key).Scan(&id)
		if err == sql.ErrNoRows {
			result.MissingKeys = append(result.MissingKeys, key)
			continue
		}
		if err != nil {
			return BatchResult{}, err
		}
		r, _, err := s.load(ctx, tx, project, id)
		if err != nil {
			return BatchResult{}, err
		}
		d, e := s.authorize(ctx, tx, p, policy.ReadValue, policy.Resource{ProjectID: project, Environment: r.Environment, ID: r.ID, Key: r.Key, Type: "secret", Revision: r.Revision})
		if e != nil {
			return BatchResult{}, e
		}
		if !d.Allowed {
			result.MissingKeys = append(result.MissingKeys, key)
			continue
		}
		r, e = s.readTx(ctx, tx, p, r)
		if e != nil {
			return BatchResult{}, e
		}
		encoded, e := json.Marshal(r)
		if e != nil {
			return BatchResult{}, e
		}
		size += len(encoded) + 1
		if size > MaxReadBytes {
			return BatchResult{}, ErrInvalid
		}
		result.Secrets = append(result.Secrets, r)
	}
	// Include the envelope and missing-key names in the advertised ceiling,
	// not only the permitted record bodies. Check before committing receipts.
	encoded, err := json.Marshal(result)
	if err != nil {
		return BatchResult{}, err
	}
	if len(encoded) > MaxReadBytes {
		return BatchResult{}, ErrInvalid
	}
	if err = tx.Commit(); err != nil {
		return BatchResult{}, err
	}
	return result, nil
}

// UpdateCurrent preserves last-write-wins for old clients when expected is nil,
// while clients providing a revision gain an atomic stale-write check.
func (s *Service) UpdateCurrent(ctx context.Context, p policy.Principal, project, id uuid.UUID, value string, expected *int64) (Record, error) {
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
	revision := r.Revision
	if expected != nil {
		revision = *expected
	}
	r.Value = value
	r, err = s.PutTx(ctx, tx, p, r, revision, "")
	if err != nil {
		return Record{}, err
	}
	return r, tx.Commit()
}

func (s *Service) DeleteCurrent(ctx context.Context, p policy.Principal, project, id uuid.UUID, expected *int64) error {
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, _, err := s.load(ctx, tx, project, id)
	if err != nil {
		return err
	}
	if r.Deleted {
		return ErrNotFound
	}
	revision := r.Revision
	if expected != nil {
		revision = *expected
	}
	if err = s.DeleteTx(ctx, tx, p, project, id, revision); err != nil {
		return err
	}
	return tx.Commit()
}

// PutMany backs imports/templates with one project lock and one transaction.
// It never calls independently committing Put in a loop.
func (s *Service) PutMany(ctx context.Context, p policy.Principal, project uuid.UUID, environment string, values map[string]string, overwrite bool, operation string) (WriteResult, error) {
	result := WriteResult{Created: []string{}, Updated: []string{}, Skipped: []string{}, Records: []Record{}}
	if len(values) == 0 || len(values) > 1000 {
		return result, ErrInvalid
	}
	keys := []string{}
	size := 0
	for key, value := range values {
		if !ValidKey(key) || len(value) > 1<<20 {
			return result, ErrInvalid
		}
		keys = append(keys, key)
		size += len(key) + len(value)
	}
	if size > 1<<20 {
		return result, ErrInvalid
	}
	sort.Strings(keys)
	tx, err := s.begin(ctx, p, project)
	if err != nil {
		return WriteResult{}, err
	}
	defer tx.Rollback()
	result, err = s.putManyTx(ctx, tx, p, project, environment, values, overwrite, operation, keys)
	if err != nil {
		return WriteResult{}, err
	}
	return result, tx.Commit()
}

// PutManyFromSource authorizes a stored template in the same transaction as all
// resulting revisions. Discovery attributes are revalidated by the source port.
func (s *Service) PutManyFromSource(ctx context.Context, p policy.Principal, project uuid.UUID, environment string, sourceOrganizations []uuid.UUID, source func(*sql.Tx) (map[string]string, error)) (WriteResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WriteResult{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s'; SET LOCAL statement_timeout='15s'`); err != nil {
		return WriteResult{}, err
	}
	if err = authority.Postgres(s.db).LockProjectOrganizations(ctx, tx, p, project, true, sourceOrganizations); err != nil {
		return WriteResult{}, ErrDenied
	}
	values, err := source(tx)
	if err != nil {
		return WriteResult{}, err
	}
	keys := []string{}
	size := 0
	if len(values) == 0 || len(values) > 1000 {
		return WriteResult{}, ErrInvalid
	}
	for k, v := range values {
		if !ValidKey(k) || len(v) > 1<<20 {
			return WriteResult{}, ErrInvalid
		}
		keys = append(keys, k)
		size += len(k) + len(v)
	}
	if size > 1<<20 {
		return WriteResult{}, ErrInvalid
	}
	sort.Strings(keys)
	result, err := s.putManyTx(ctx, tx, p, project, environment, values, true, "template", keys)
	if err != nil {
		return WriteResult{}, err
	}
	return result, tx.Commit()
}
func (s *Service) putManyTx(ctx context.Context, tx *sql.Tx, p policy.Principal, project uuid.UUID, environment string, values map[string]string, overwrite bool, operation string, keys []string) (WriteResult, error) {
	result := WriteResult{Created: []string{}, Updated: []string{}, Skipped: []string{}, Records: []Record{}}
	var err error
	var env uuid.UUID
	if err = tx.QueryRowContext(ctx, `SELECT id FROM environments WHERE project_id=$1 AND name=$2`, project, environment).Scan(&env); err != nil {
		return WriteResult{}, ErrNotFound
	}
	for _, key := range keys {
		r := Record{ProjectID: project, Environment: environment, Key: key, Value: values[key]}
		var id uuid.UUID
		expected := int64(0)
		err = tx.QueryRowContext(ctx, `SELECT secret_id FROM vault_entries WHERE project_id=$1 AND environment_id=$2 AND secret_key=$3 AND NOT deleted`, project, env, key).Scan(&id)
		if err != nil && err != sql.ErrNoRows {
			return WriteResult{}, err
		}
		if err == nil {
			existing, _, e := s.load(ctx, tx, project, id)
			if e != nil {
				return WriteResult{}, e
			}
			if e = s.permit(ctx, tx, p, policy.WriteSecret, existing); e != nil {
				return WriteResult{}, e
			}
			if !overwrite {
				result.Skipped = append(result.Skipped, key)
				continue
			}
			r.ID = id
			expected = existing.Revision
		}
		written, e := s.PutTx(ctx, tx, p, r, expected, operation)
		if e != nil {
			return WriteResult{}, e
		}
		if expected == 0 {
			result.Created = append(result.Created, key)
		} else {
			result.Updated = append(result.Updated, key)
		}
		result.Records = append(result.Records, written)
	}
	return result, nil
}

// CurrentTx provides metadata inside an already project-locked transaction.
func (s *Service) CurrentTx(ctx context.Context, tx *sql.Tx, project uuid.UUID, environment, key string) (Record, error) {
	var id uuid.UUID
	err := tx.QueryRowContext(ctx, `SELECT v.secret_id FROM vault_entries v JOIN environments e ON e.id=v.environment_id AND e.project_id=v.project_id WHERE v.project_id=$1 AND e.name=$2 AND v.secret_key=$3 AND NOT v.deleted`, project, environment, key).Scan(&id)
	if err == sql.ErrNoRows {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	r, _, err := s.load(ctx, tx, project, id)
	return r, err
}

// BindSnapshotKeyTx retains the exact DEK needed by a newly captured snapshot.
func (s *Service) BindSnapshotKeyTx(ctx context.Context, tx *sql.Tx, project, snapshot uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO vault_snapshot_keys(snapshot_id,project_id,key_id) SELECT ss.id,vp.project_id,vp.current_key_id FROM secret_snapshots ss JOIN promotion_requests pr ON pr.id=ss.promotion_id JOIN vault_projects vp ON vp.project_id=pr.project_id WHERE ss.id=$1 AND pr.project_id=$2 AND ss.prior_existed ON CONFLICT(snapshot_id) DO NOTHING`, snapshot, project)
	return err
}
