package repository

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type AuditRepository struct {
	db      *sql.DB
	dialect Dialect

	// Audit hash chain (ADR-0019). When chainKey is non-nil every Create is a
	// keyed, serialized append: entry_hash = HMAC(chainKey, prev_hash ||
	// canonical(row)), with prev_hash linking to the previous row. lastEntryHash
	// is the in-process chain tip; the mu mutex serializes appends (correct for
	// the single-instance topology, ADR-0016 — multi-instance would add a DB
	// advisory lock). Inert (plain inserts) until SetChainKey is called.
	mu            sync.Mutex
	chainKey      []byte
	lastEntryHash string
}

func NewAuditRepository(db *sql.DB, dialect Dialect) *AuditRepository {
	return &AuditRepository{db: db, dialect: dialect}
}

// SetChainKey enables the tamper-evident hash chain (ADR-0019) and primes the
// in-memory tip from the existing chain (epoch handoff across restarts).
func (r *AuditRepository) SetChainKey(key []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chainKey = key
	r.lastEntryHash = r.computeTip()
}

// computeTip returns the chain tip — the entry_hash that no row references as
// its prev_hash. This is order-independent (it follows the links, not
// created_at, which is too coarse to order same-instant appends). Empty when
// there is no chain yet.
func (r *AuditRepository) computeTip() string {
	rows, err := r.db.Query(Q(r.dialect, `SELECT prev_hash, entry_hash FROM audit_log WHERE entry_hash IS NOT NULL`))
	if err != nil {
		return ""
	}
	defer rows.Close()
	prevs := map[string]bool{}
	var entries []string
	for rows.Next() {
		var prev, entry sql.NullString
		if err := rows.Scan(&prev, &entry); err != nil {
			return ""
		}
		prevs[prev.String] = true
		if entry.Valid {
			entries = append(entries, entry.String)
		}
	}
	for _, e := range entries {
		if !prevs[e] {
			return e
		}
	}
	return ""
}

func (r *AuditRepository) Create(userID, projectID *uuid.UUID, action, environment string, details models.JSONMap, ipAddress string) error {
	dv, err := details.Value()
	if err != nil {
		return fmt.Errorf("marshaling audit details: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.chainKey == nil {
		// Unchained (pre-ADR-0019 / tests): plain insert, DB-default id + created_at.
		_, err = r.db.Exec(
			Q(r.dialect, `INSERT INTO audit_log (user_id, project_id, action, environment, details, ip_address)
			 VALUES ($1, $2, $3, $4, $5, $6)`),
			userID, projectID, action, environment, dv, ipAddress,
		)
		if err != nil {
			return fmt.Errorf("creating audit entry: %w", err)
		}
		return nil
	}

	// Chained append: link to the in-memory tip and stamp this row's hash.
	prevHash := r.lastEntryHash
	entryHash := computeEntryHash(r.chainKey, prevHash,
		uuidStr(userID), uuidStr(projectID), action, environment,
		canonicalizeJSON(toBytes(dv)), ipAddress)
	id := uuid.New()
	_, err = ExecQ(r.db, r.dialect, `INSERT INTO audit_log (id, user_id, project_id, action, environment, details, ip_address, prev_hash, entry_hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, userID, projectID, action, environment, dv, ipAddress, prevHash, entryHash)
	if err != nil {
		return fmt.Errorf("creating audit entry: %w", err)
	}
	r.lastEntryHash = entryHash
	return nil
}

func (r *AuditRepository) ListByProjectID(projectID uuid.UUID, limit int) ([]models.AuditEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(
		Q(r.dialect, `SELECT id, user_id, project_id, action, environment, details, ip_address, created_at
		 FROM audit_log WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2`),
		projectID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()

	var entries []models.AuditEntry
	for rows.Next() {
		var e models.AuditEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.ProjectID, &e.Action, &e.Environment, &e.Details, &e.IPAddress, dbTime(&e.CreatedAt)); err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// DeleteOlderThan removes audit_log rows older than the retention window. The
// cutoff is computed in Go (UTC) so the WHERE clause is dialect-independent.
//
// When the hash chain is active (ADR-0019) a plain DELETE would orphan the
// chain and make VerifyChain report a break at the deletion boundary. Instead
// this performs a chain-aware prune + re-anchor (see pruneChained): it deletes
// the oldest rows and RE-ANCHORS the chain from the new earliest surviving row
// so integrity still verifies from that anchor forward. Pruned rows are gone by
// design (retention = intended data loss); tamper-evidence holds from the
// retained anchor onward, not before it (which no longer exists).
func (r *AuditRepository) DeleteOlderThan(days int) (int64, error) {
	if days <= 0 {
		return 0, fmt.Errorf("days must be a positive integer, got %d", days)
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.chainKey != nil {
		return r.pruneChained(cutoff)
	}

	res, err := r.db.Exec(
		Q(r.dialect, `DELETE FROM audit_log WHERE created_at < $1`),
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("pruning audit entries: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("counting pruned rows: %w", err)
	}
	return rows, nil
}

// pruneChained deletes chained rows older than cutoff and re-anchors the chain
// from the new earliest surviving row, so VerifyChain still holds. It MUST be
// called with r.mu held. It walks the chain from genesis (prev_hash=="") in
// link order, deletes the leading run of rows whose created_at < cutoff, sets
// the first surviving row's prev_hash to "" (new genesis), and recomputes
// entry_hash for every surviving row forward — updating r.lastEntryHash to the
// new tip so subsequent appends stay contiguous.
func (r *AuditRepository) pruneChained(cutoff time.Time) (int64, error) {
	rows, err := r.db.Query(Q(r.dialect, `SELECT id, user_id, project_id, action, environment, details, ip_address, prev_hash, entry_hash, created_at
		 FROM audit_log WHERE entry_hash IS NOT NULL`))
	if err != nil {
		return 0, fmt.Errorf("reading audit chain for prune: %w", err)
	}

	type chainRow struct {
		id                             uuid.UUID
		userID, projectID, environment string
		action, ipAddress              string
		prevHash, entryHash            string
		details                        []byte
		createdAt                      time.Time
	}
	byPrev := map[string]*chainRow{}
	for rows.Next() {
		var id uuid.UUID
		var userID, projectID, environment, prevHash, entryHash sql.NullString
		var action, ipAddress string
		var detailsRaw []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &userID, &projectID, &action, &environment, &detailsRaw, &ipAddress, &prevHash, &entryHash, dbTime(&createdAt)); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scanning audit chain row for prune: %w", err)
		}
		byPrev[prevHash.String] = &chainRow{
			id: id, userID: userID.String, projectID: projectID.String,
			environment: environment.String, action: action, ipAddress: ipAddress,
			prevHash: prevHash.String, entryHash: entryHash.String,
			details: detailsRaw, createdAt: createdAt,
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// Materialize the chain in link order from genesis.
	var ordered []*chainRow
	cur := ""
	for {
		row, ok := byPrev[cur]
		if !ok {
			break
		}
		ordered = append(ordered, row)
		cur = row.entryHash
	}

	// The delete set is the leading run older than cutoff. Audit is append-only
	// so the oldest rows are the earliest links; stop at the first survivor.
	deleteCount := 0
	for _, row := range ordered {
		if row.createdAt.UTC().Before(cutoff) {
			deleteCount++
		} else {
			break
		}
	}
	if deleteCount == 0 {
		return 0, nil
	}

	// Delete the old prefix AND re-anchor the survivors ATOMICALLY: a partial
	// prune would leave the tamper-evident chain broken (VerifyChain failing at
	// the boundary). r.lastEntryHash is updated only after the tx commits, so the
	// in-memory tip can never diverge from the persisted chain.
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin prune tx: %w", err)
	}
	for _, row := range ordered[:deleteCount] {
		if _, err := tx.Exec(Q(r.dialect, `DELETE FROM audit_log WHERE id = $1`), row.id); err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("deleting pruned audit row: %w", err)
		}
	}

	// Re-anchor: recompute prev_hash/entry_hash for every surviving row, starting
	// the new genesis at prev_hash="". No survivors => empty chain, new tip "".
	prev := ""
	for _, row := range ordered[deleteCount:] {
		entry := computeEntryHash(r.chainKey, prev, row.userID, row.projectID, row.action, row.environment, canonicalizeJSON(row.details), row.ipAddress)
		if _, err := tx.Exec(
			Q(r.dialect, `UPDATE audit_log SET prev_hash = $1, entry_hash = $2 WHERE id = $3`),
			prev, entry, row.id,
		); err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("re-anchoring audit row: %w", err)
		}
		prev = entry
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit prune tx: %w", err)
	}
	r.lastEntryHash = prev
	return int64(deleteCount), nil
}

// VerifyChain follows the prev_hash links from genesis and recomputes each
// row's hash. It returns the id of the first row that fails — a content hash
// mismatch, or a row left unreachable by a deletion/insertion — or (nil, nil)
// when the whole chain verifies. Order-independent. Requires the chain key.
func (r *AuditRepository) VerifyChain() (*uuid.UUID, error) {
	r.mu.Lock()
	key := r.chainKey
	r.mu.Unlock()
	if key == nil {
		return nil, fmt.Errorf("audit chain key is not configured")
	}

	rows, err := r.db.Query(Q(r.dialect, `SELECT id, user_id, project_id, action, environment, details, ip_address, prev_hash, entry_hash
		 FROM audit_log WHERE entry_hash IS NOT NULL`))
	if err != nil {
		return nil, fmt.Errorf("reading audit chain: %w", err)
	}
	defer rows.Close()

	type chainRow struct {
		id                             uuid.UUID
		userID, projectID, environment string
		action, ipAddress, entryHash   string
		details                        []byte
	}
	byPrev := map[string]chainRow{}
	for rows.Next() {
		var id uuid.UUID
		var userID, projectID, environment, prevHash, entryHash sql.NullString
		var action, ipAddress string
		var detailsRaw []byte
		if err := rows.Scan(&id, &userID, &projectID, &action, &environment, &detailsRaw, &ipAddress, &prevHash, &entryHash); err != nil {
			return nil, fmt.Errorf("scanning audit chain row: %w", err)
		}
		byPrev[prevHash.String] = chainRow{
			id: id, userID: userID.String, projectID: projectID.String,
			environment: environment.String, action: action, ipAddress: ipAddress,
			entryHash: entryHash.String, details: detailsRaw,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	total := len(byPrev)
	cur := ""
	seen := 0
	for {
		row, ok := byPrev[cur]
		if !ok {
			break
		}
		want := computeEntryHash(key, cur, row.userID, row.projectID, row.action, row.environment, canonicalizeJSON(row.details), row.ipAddress)
		if want != row.entryHash {
			broken := row.id
			return &broken, nil
		}
		delete(byPrev, cur)
		cur = row.entryHash
		seen++
	}
	if seen != total {
		// Rows unreachable from genesis: a deletion/insertion/fork. Report one.
		for _, row := range byPrev {
			broken := row.id
			return &broken, nil
		}
	}
	return nil, nil
}

// computeEntryHash returns the hex HMAC-SHA256 over the previous hash and this
// row's canonical fields, length-framed so field boundaries are unambiguous.
func computeEntryHash(key []byte, prevHash, userID, projectID, action, environment string, canonicalDetails []byte, ipAddress string) string {
	mac := hmac.New(sha256.New, key)
	frame(mac, []byte(prevHash))
	frame(mac, []byte(userID))
	frame(mac, []byte(projectID))
	frame(mac, []byte(action))
	frame(mac, []byte(environment))
	frame(mac, canonicalDetails)
	frame(mac, []byte(ipAddress))
	return hex.EncodeToString(mac.Sum(nil))
}

func frame(w io.Writer, b []byte) {
	var l [8]byte
	binary.BigEndian.PutUint64(l[:], uint64(len(b)))
	_, _ = w.Write(l[:])
	_, _ = w.Write(b)
}

func uuidStr(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}

func toBytes(v interface{}) []byte {
	switch b := v.(type) {
	case []byte:
		return b
	case string:
		return []byte(b)
	default:
		return nil
	}
}

// canonicalizeJSON re-marshals JSON to a stable form (Go sorts map keys, compact)
// so the hash is reproducible even after a PostgreSQL JSONB round-trip reorders
// keys or restyles whitespace. Non-JSON input is hashed as-is.
func canonicalizeJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("null")
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}
