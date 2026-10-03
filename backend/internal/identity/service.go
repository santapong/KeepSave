// Package identity owns verified contacts, purpose-bound proofs and team changes.
// Proof custody and current authority are injected; HTTP handlers do not access SQL.
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

var (
	ErrUnavailable = errors.New("identity platform unavailable")
	ErrDenied      = errors.New("identity authority denied")
	ErrInvalid     = errors.New("invalid identity request")
	ErrProof       = errors.New("invalid or expired proof")
	ErrConflict    = errors.New("identity revision conflict")
	ErrLastMethod  = errors.New("last usable sign-in method")
)

type Auditor interface {
	CreateTx(*sql.Tx, *uuid.UUID, *uuid.UUID, string, string, models.JSONMap, string) error
}
type ProofCustody interface {
	Seal(context.Context, []byte) ([]byte, []byte, error)
	WithOpened(context.Context, []byte, []byte, func([]byte) error) error
}
type Guard interface {
	LockSubjects(context.Context, *sql.Tx, []uuid.UUID, bool) error
	RequireSession(context.Context, *sql.Tx, policy.Principal, bool) error
	RequireOrg(context.Context, *sql.Tx, policy.Principal, uuid.UUID, string) error
}

// RecoveryDelegations owns legacy credential authority. The caller holds the
// exclusive user authority barrier; revocation participates in its transaction.
type RecoveryDelegations interface {
	RevokeDelegationsForRecoveryTx(context.Context, *sql.Tx, uuid.UUID) error
}
type Config struct {
	Enabled                bool
	PostgreSQL             bool
	SMTPConfigured         bool
	ApplicationOrigin      string
	EnabledSocialProviders map[string]bool
}
type Service struct {
	db                  *sql.DB
	config              Config
	audit               Auditor
	custody             ProofCustody
	guard               Guard
	cascade             Cascade
	recoveryDelegations RecoveryDelegations
}

func New(db *sql.DB, cfg Config, audit Auditor, custody ProofCustody, guard Guard) *Service {
	return &Service{db: db, config: cfg, audit: audit, custody: custody, guard: guard}
}
func (s *Service) EnableRecoveryRevocation(r RecoveryDelegations) { s.recoveryDelegations = r }
func (s *Service) Enabled() bool {
	return s != nil && s.config.Enabled && s.config.PostgreSQL && s.db != nil && s.audit != nil && s.guard != nil
}
func (s *Service) transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	return s.transactionOptions(ctx, nil, fn)
}
func (s *Service) transactionOptions(ctx context.Context, options *sql.TxOptions, fn func(*sql.Tx) error) error {
	if !s.Enabled() {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s'; SET LOCAL statement_timeout='10s'`); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) actor(ctx context.Context, tx *sql.Tx, p policy.Principal, targets []uuid.UUID, exclusive, recent bool) error {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil || p.ActorID != p.SubjectID {
		return ErrDenied
	}
	ids := append([]uuid.UUID{p.SubjectID}, targets...)
	if err := s.guard.LockSubjects(ctx, tx, ids, exclusive); err != nil {
		return ErrDenied
	}
	if err := s.guard.RequireSession(ctx, tx, p, recent); err != nil {
		return ErrDenied
	}
	return nil
}
func (s *Service) event(ctx context.Context, tx *sql.Tx, user uuid.UUID, action string, refs map[string]string) error {
	details := models.JSONMap{}
	for key, value := range refs {
		details[key] = value
	}
	if err := s.audit.CreateTx(tx, &user, nil, action, "", details, ""); err != nil {
		return err
	}
	payload, err := json.Marshal(refs)
	if err != nil {
		return err
	}
	return jobs.EnqueueTx(ctx, tx, uuid.New(), "identity.authority_changed", payload, "local", 5)
}
func hashProof(raw string) string { h := sha256.Sum256([]byte(raw)); return hex.EncodeToString(h[:]) }
func randomProof() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func validContact(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return "", ErrInvalid
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return "", ErrInvalid
	}
	return value, nil
}

type ProofRequest struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}
type proofRow struct {
	ID, UserID             uuid.UUID
	SessionID              uuid.NullUUID
	Purpose, Contact, Hash string
	Expires                time.Time
	Consumed, Revoked      bool
	Attempts               int
}

func loadProof(ctx context.Context, tx *sql.Tx, id uuid.UUID, lock bool) (proofRow, error) {
	row := proofRow{}
	q := `SELECT id,user_id,session_id,purpose,contact,proof_hash,expires_at,consumed,revoked,attempts FROM identity_proofs WHERE id=$1`
	if lock {
		q += ` FOR UPDATE`
	}
	err := tx.QueryRowContext(ctx, q, id).Scan(&row.ID, &row.UserID, &row.SessionID, &row.Purpose, &row.Contact, &row.Hash, &row.Expires, &row.Consumed, &row.Revoked, &row.Attempts)
	return row, err
}
func (s *Service) createProof(ctx context.Context, tx *sql.Tx, user uuid.UUID, session uuid.UUID, purpose, contact string) (ProofRequest, error) {
	if !s.config.SMTPConfigured || s.custody == nil {
		return ProofRequest{}, ErrUnavailable
	}
	origin, err := url.Parse(s.config.ApplicationOrigin)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" {
		return ProofRequest{}, ErrUnavailable
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_proofs WHERE user_id=$1 AND purpose=$2 AND created_at>NOW()-INTERVAL '1 hour'`, user, purpose).Scan(&count); err != nil {
		return ProofRequest{}, err
	}
	if count >= 5 {
		return ProofRequest{}, ErrUnavailable
	}
	raw, err := randomProof()
	if err != nil {
		return ProofRequest{}, err
	}
	id, delivery := uuid.New(), uuid.New()
	// The outbox carries only a delivery ID. Plaintext proof exists only briefly.
	message, err := json.Marshal(proofMessage{Contact: contact, URL: strings.TrimRight(s.config.ApplicationOrigin, "/") + "/identity/confirm#id=" + id.String() + "&purpose=" + purpose + "&proof=" + raw, Purpose: purpose})
	if err != nil {
		return ProofRequest{}, err
	}
	encrypted, nonce, err := s.custody.Seal(ctx, message)
	clear(message)
	if err != nil {
		return ProofRequest{}, err
	}
	var sid any
	if session != uuid.Nil {
		sid = session
	}
	if purpose != "invitation" {
		if _, err = tx.ExecContext(ctx, `UPDATE identity_proofs SET revoked=TRUE WHERE user_id=$1 AND purpose=$2 AND NOT consumed AND NOT revoked`, user, purpose); err != nil {
			return ProofRequest{}, err
		}
	}
	lifetime := 15 * time.Minute
	if purpose == "invitation" {
		lifetime = 24 * time.Hour
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO identity_proofs(id,user_id,session_id,purpose,contact,proof_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6,NOW()+$7*INTERVAL '1 second')`, id, user, sid, purpose, contact, hashProof(raw), int64(lifetime.Seconds())); err != nil {
		return ProofRequest{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO identity_proof_deliveries(id,proof_id,ciphertext,nonce) VALUES($1,$2,$3,$4)`, delivery, id, encrypted, nonce); err != nil {
		return ProofRequest{}, err
	}
	if err = s.audit.CreateTx(tx, &user, nil, "identity.proof_requested", "", models.JSONMap{"proof_id": id.String(), "purpose": purpose}, ""); err != nil {
		return ProofRequest{}, err
	}
	payload, _ := json.Marshal(map[string]string{"delivery_id": delivery.String()})
	if err = jobs.EnqueueTx(ctx, tx, uuid.New(), "identity.proof_delivery", payload, "external", 1); err != nil {
		return ProofRequest{}, err
	}
	return ProofRequest{ID: id, Status: "pending"}, nil
}
func (s *Service) RequestContact(ctx context.Context, p policy.Principal, contact string) (ProofRequest, error) {
	contact, err := validContact(contact)
	if err != nil {
		return ProofRequest{}, err
	}
	var result ProofRequest
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		if err := s.actor(ctx, tx, p, nil, true, true); err != nil {
			return err
		}
		var e error
		result, e = s.createProof(ctx, tx, p.SubjectID, p.SessionID, "contact_verify", contact)
		return e
	})
	return result, err
}

// Recovery initiation always returns a random nonsecret ID for unknown contacts.
// Only a previously verified contact can select an existing immutable account.
func (s *Service) RequestRecovery(ctx context.Context, contact string) (ProofRequest, error) {
	contact, err := validContact(contact)
	if err != nil {
		return ProofRequest{}, ErrInvalid
	}
	result := ProofRequest{ID: uuid.New(), Status: "accepted"}
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		var user uuid.UUID
		e := tx.QueryRowContext(ctx, `SELECT user_id FROM identity_verified_contacts WHERE contact=$1 AND revoked_at IS NULL`, contact).Scan(&user)
		if errors.Is(e, sql.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if e = s.guard.LockSubjects(ctx, tx, []uuid.UUID{user}, true); e != nil {
			return e
		}
		// Recheck after the authority barrier; a revoked/reassigned contact cannot recover.
		var current uuid.UUID
		if e = tx.QueryRowContext(ctx, `SELECT user_id FROM identity_verified_contacts WHERE contact=$1 AND user_id=$2 AND revoked_at IS NULL FOR UPDATE`, contact, user).Scan(&current); e != nil {
			return nil
		}
		created, e := s.createProof(ctx, tx, user, uuid.Nil, "password_reset", contact)
		if errors.Is(e, ErrUnavailable) {
			return nil
		}
		if e != nil {
			return e
		}
		result.ID = created.ID
		return nil
	})
	return result, err
}
func (s *Service) ConfirmContact(ctx context.Context, p policy.Principal, id uuid.UUID, raw string) error {
	return s.consume(ctx, p, id, raw, "contact_verify", func(tx *sql.Tx, row proofRow) error {
		var other uuid.UUID
		err := tx.QueryRowContext(ctx, `SELECT user_id FROM identity_verified_contacts WHERE contact=$1 AND revoked_at IS NULL`, row.Contact).Scan(&other)
		if err == nil && other != row.UserID {
			return ErrConflict
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO identity_verified_contacts(user_id,contact,verified_at) VALUES($1,$2,NOW()) ON CONFLICT(user_id,contact) DO UPDATE SET verified_at=NOW(),revoked_at=NULL`, row.UserID, row.Contact)
		return err
	})
}
func (s *Service) ResetPassword(ctx context.Context, id uuid.UUID, raw, password string) error {
	if s.recoveryDelegations == nil {
		return ErrUnavailable
	}
	if len(password) > 72 {
		return ErrInvalid
	}
	if err := auth.DefaultPasswordPolicy().Validate(password); err != nil {
		return ErrInvalid
	}
	hashed, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	return s.consume(ctx, policy.Principal{}, id, raw, "password_reset", func(tx *sql.Tx, row proofRow) error {
		var verified int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_verified_contacts WHERE user_id=$1 AND contact=$2 AND revoked_at IS NULL`, row.UserID, row.Contact).Scan(&verified); err != nil {
			return err
		}
		if verified != 1 {
			return ErrProof
		}
		if err := s.recoveryDelegations.RevokeDelegationsForRecoveryTx(ctx, tx, row.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=$1,updated_at=NOW() WHERE id=$2`, hashed, row.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE session_tokens SET revoked=TRUE WHERE user_id=$1`, row.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM social_auth_flows WHERE link_user_id=$1`, row.UserID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE identity_proofs SET revoked=TRUE WHERE user_id=$1 AND id<>$2 AND NOT consumed`, row.UserID, row.ID)
		return err
	})
}
func (s *Service) consume(ctx context.Context, p policy.Principal, id uuid.UUID, raw, purpose string, apply func(*sql.Tx, proofRow) error) error {
	if id == uuid.Nil || len(raw) != 43 {
		return ErrProof
	}
	var rejected bool
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		row, err := loadProof(ctx, tx, id, false)
		if err != nil {
			return ErrProof
		}
		if purpose == "contact_verify" {
			if err = s.actor(ctx, tx, p, []uuid.UUID{row.UserID}, true, false); err != nil {
				return err
			}
			if p.SubjectID != row.UserID || !row.SessionID.Valid || p.SessionID != row.SessionID.UUID {
				return ErrProof
			}
		} else if err = s.guard.LockSubjects(ctx, tx, []uuid.UUID{row.UserID}, true); err != nil {
			return err
		}
		row, err = loadProof(ctx, tx, id, true)
		if err != nil {
			return ErrProof
		}
		if row.Purpose != purpose || row.Consumed || row.Revoked || !row.Expires.After(time.Now()) || row.Attempts >= 5 {
			return ErrProof
		}
		if subtle.ConstantTimeCompare([]byte(hashProof(raw)), []byte(row.Hash)) != 1 {
			rejected = true
			_, err = tx.ExecContext(ctx, `UPDATE identity_proofs SET attempts=attempts+1,revoked=(attempts+1>=5) WHERE id=$1`, id)
			if err != nil {
				return err
			}
			return s.event(ctx, tx, row.UserID, "identity.proof_rejected", map[string]string{"proof_id": id.String(), "purpose": purpose})
		}
		if err = apply(tx, row); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE identity_proofs SET consumed=TRUE,consumed_at=NOW() WHERE id=$1`, id); err != nil {
			return err
		}
		return s.event(ctx, tx, row.UserID, "identity.proof_consumed", map[string]string{"proof_id": id.String(), "purpose": purpose, "user_id": row.UserID.String()})
	})
	if err == nil && rejected {
		return ErrProof
	}
	return err
}

type Method struct {
	Name   string `json:"name"`
	Usable bool   `json:"usable"`
}
type Contact struct {
	Contact    string    `json:"contact"`
	VerifiedAt time.Time `json:"verified_at"`
}

func (s *Service) Contacts(ctx context.Context, p policy.Principal) ([]Contact, error) {
	result := []Contact{}
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		if e := s.actor(ctx, tx, p, nil, false, false); e != nil {
			return e
		}
		rows, e := tx.QueryContext(ctx, `SELECT contact,verified_at FROM identity_verified_contacts WHERE user_id=$1 AND revoked_at IS NULL ORDER BY contact LIMIT 20`, p.SubjectID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var row Contact
			if e = rows.Scan(&row.Contact, &row.VerifiedAt); e != nil {
				return e
			}
			result = append(result, row)
		}
		return rows.Err()
	})
	return result, err
}
func (s *Service) methodsTx(ctx context.Context, tx *sql.Tx, user uuid.UUID) ([]Method, error) {
	var hash string
	if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=$1`, user).Scan(&hash); err != nil {
		return nil, err
	}
	methods := []Method{{Name: "password", Usable: strings.HasPrefix(hash, "$2")}}
	rows, err := tx.QueryContext(ctx, `SELECT provider FROM social_identities WHERE user_id=$1 ORDER BY provider`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		methods = append(methods, Method{Name: name, Usable: s.config.EnabledSocialProviders[name]})
	}
	return methods, rows.Err()
}
func (s *Service) Methods(ctx context.Context, p policy.Principal) ([]Method, error) {
	var result []Method
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		if e := s.actor(ctx, tx, p, nil, false, false); e != nil {
			return e
		}
		var e error
		result, e = s.methodsTx(ctx, tx, p.SubjectID)
		return e
	})
	return result, err
}
func (s *Service) RemoveMethod(ctx context.Context, p policy.Principal, name string) error {
	if name != "password" && name != "google" && name != "github" {
		return ErrInvalid
	}
	return s.transaction(ctx, func(tx *sql.Tx) error {
		if e := s.actor(ctx, tx, p, nil, true, true); e != nil {
			return e
		}
		methods, e := s.methodsTx(ctx, tx, p.SubjectID)
		if e != nil {
			return e
		}
		var authenticatedMethod string
		if e = tx.QueryRowContext(ctx, `SELECT auth_method FROM session_tokens WHERE id=$1 AND user_id=$2`, p.SessionID, p.SubjectID).Scan(&authenticatedMethod); e != nil {
			return e
		}
		remains, found, proved := false, false, false
		for _, m := range methods {
			if m.Name == name {
				found = true
			} else if m.Usable {
				remains = true
				if m.Name == authenticatedMethod {
					proved = true
				}
			}
		}
		if !found {
			return ErrInvalid
		}
		if !remains {
			return ErrLastMethod
		}
		if !proved {
			return ErrDenied
		}
		if name == "password" {
			_, e = tx.ExecContext(ctx, `UPDATE users SET password_hash='!',updated_at=NOW() WHERE id=$1`, p.SubjectID)
		} else {
			_, e = tx.ExecContext(ctx, `DELETE FROM social_identities WHERE user_id=$1 AND provider=$2`, p.SubjectID, name)
		}
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE session_tokens SET revoked=TRUE WHERE user_id=$1 AND id<>$2`, p.SubjectID, p.SessionID); e != nil {
			return e
		}
		return s.event(ctx, tx, p.SubjectID, "identity.method_removed", map[string]string{"method": name, "user_id": p.SubjectID.String()})
	})
}
