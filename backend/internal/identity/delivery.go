package identity

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

type proofMessage struct {
	Contact string `json:"contact"`
	URL     string `json:"url"`
	Purpose string `json:"purpose"`
}
type DeliveryOutcome struct {
	State  string `json:"state"`
	Reason string `json:"reason_code"`
}

// Sender never returns SMTP response text, which can contain recipient metadata.
type Sender interface {
	Send(context.Context, string, string, string) DeliveryOutcome
}
type SMTPConfig struct{ Address, Hostname, Username, Password, From string }
type SMTP struct{ config SMTPConfig }

func NewSMTP(cfg SMTPConfig) (*SMTP, error) {
	host, port, err := net.SplitHostPort(cfg.Address)
	if err != nil || host == "" || port == "" || cfg.Hostname != host || cfg.Username == "" || cfg.Password == "" {
		return nil, ErrInvalid
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil || from.Address != cfg.From || strings.ContainsAny(cfg.From+cfg.Username, "\r\n") {
		return nil, ErrInvalid
	}
	return &SMTP{config: cfg}, nil
}

// SMTP requires STARTTLS and certificate verification. Errors after handing a
// DATA body to the server are uncertain; callers cannot automatically resend.
func (s *SMTP) Send(ctx context.Context, to, subject, body string) DeliveryOutcome {
	fail := DeliveryOutcome{State: "failed", Reason: "smtp_unavailable"}
	if _, err := validContact(to); err != nil || strings.ContainsAny(subject, "\r\n") {
		return DeliveryOutcome{State: "failed", Reason: "invalid_message"}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.config.Address)
	if err != nil {
		return fail
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return fail
	}
	client, err := smtp.NewClient(conn, s.config.Hostname)
	if err != nil {
		return fail
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return DeliveryOutcome{State: "failed", Reason: "smtp_tls_required"}
	}
	if err = client.StartTLS(&tls.Config{ServerName: s.config.Hostname, MinVersion: tls.VersionTLS12}); err != nil {
		return fail
	}
	if s.config.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Hostname)); err != nil {
			return fail
		}
	}
	if err = client.Mail(s.config.From); err != nil {
		return fail
	}
	if err = client.Rcpt(to); err != nil {
		return fail
	}
	writer, err := client.Data()
	if err != nil {
		return fail
	}
	uncertain := DeliveryOutcome{State: "uncertain", Reason: "smtp_acceptance_unknown"}
	message := "From: " + s.config.From + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body + "\r\n"
	if _, err = writer.Write([]byte(message)); err != nil {
		return uncertain
	}
	if err = writer.Close(); err != nil {
		return uncertain
	}
	// DATA acceptance confirms this message. A subsequent QUIT failure does not
	// turn acceptance into a retryable delivery.
	_ = client.Quit()
	return DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}
}

// Deliver is trusted worker work. A durable dispatch barrier prevents two
// workers or retries from sending the same encrypted proof again.
func (s *Service) Deliver(ctx context.Context, id uuid.UUID, sender Sender) (DeliveryOutcome, error) {
	if sender == nil || s.custody == nil {
		return DeliveryOutcome{}, ErrUnavailable
	}
	var cipher, nonce []byte
	var user uuid.UUID
	var proofID uuid.UUID
	var result DeliveryOutcome
	var expectedContact, expectedPurpose, expectedHash string
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		if e := tx.QueryRowContext(ctx, `SELECT p.user_id,d.proof_id FROM identity_proof_deliveries d JOIN identity_proofs p ON p.id=d.proof_id WHERE d.id=$1`, id).Scan(&user, &proofID); e != nil {
			return ErrProof
		}
		if e := s.guard.LockSubjects(ctx, tx, []uuid.UUID{user}, false); e != nil {
			return e
		}
		row, e := loadProof(ctx, tx, proofID, false)
		if e != nil {
			return e
		}
		expectedContact, expectedPurpose, expectedHash = row.Contact, row.Purpose, row.Hash
		inactive := false
		if row.SessionID.Valid {
			principal := policy.Principal{Kind: policy.Human, SubjectID: user, ActorID: user, SessionID: row.SessionID.UUID}
			if row.Purpose == "invitation" {
				var org uuid.UUID
				if e = tx.QueryRowContext(ctx, `SELECT organization_id FROM identity_invitations WHERE id=$1 AND NOT revoked`, proofID).Scan(&org); e != nil {
					inactive = true
				} else if e = s.guard.RequireOrg(ctx, tx, principal, org, "admin"); e != nil {
					inactive = true
				}
			} else if e = s.guard.RequireSession(ctx, tx, principal, false); e != nil {
				inactive = true
			}
		}
		var state string
		if e = tx.QueryRowContext(ctx, `SELECT state,ciphertext,nonce FROM identity_proof_deliveries WHERE id=$1 FOR UPDATE`, id).Scan(&state, &cipher, &nonce); e != nil {
			return e
		}
		if state != "pending" {
			return ErrConflict
		}
		if inactive || row.Consumed || row.Revoked || !row.Expires.After(time.Now()) {
			result = DeliveryOutcome{State: "cancelled", Reason: "proof_inactive"}
			_, e = tx.ExecContext(ctx, `UPDATE identity_proof_deliveries SET state='cancelled',reason_code='proof_inactive',ciphertext=''::bytea,nonce=''::bytea,completed_at=NOW() WHERE id=$1`, id)
			if e != nil {
				return e
			}
			return s.audit.CreateTx(tx, &user, nil, "identity.delivery_cancelled", "", models.JSONMap{"delivery_id": id.String(), "proof_id": proofID.String()}, "")
		}
		if _, e = tx.ExecContext(ctx, `UPDATE identity_proof_deliveries SET state='dispatched',dispatched_at=NOW() WHERE id=$1`, id); e != nil {
			return e
		}
		return s.audit.CreateTx(tx, &user, nil, "identity.delivery_dispatched", "", models.JSONMap{"delivery_id": id.String(), "proof_id": proofID.String()}, "")
	})
	if err != nil {
		return result, err
	}
	if result.State == "cancelled" {
		return result, nil
	}
	// No database locks survive into custody callback or SMTP network I/O.
	err = s.custody.WithOpened(ctx, cipher, nonce, func(raw []byte) error {
		var message proofMessage
		if e := json.Unmarshal(raw, &message); e != nil {
			return ErrInvalid
		}
		if message.Contact != expectedContact || message.Purpose != expectedPurpose {
			return ErrInvalid
		}
		parsed, e := url.Parse(message.URL)
		if e != nil {
			return ErrInvalid
		}
		origin, e := url.Parse(s.config.ApplicationOrigin)
		if e != nil || parsed.Scheme != origin.Scheme || parsed.Host != origin.Host || parsed.Path != "/identity/confirm" || parsed.RawQuery != "" {
			return ErrInvalid
		}
		fragment, e := url.ParseQuery(parsed.Fragment)
		if e != nil || fragment.Get("id") != proofID.String() || fragment.Get("purpose") != expectedPurpose || len(fragment.Get("proof")) != 43 || hashProof(fragment.Get("proof")) != expectedHash {
			return ErrInvalid
		}
		result = sender.Send(ctx, message.Contact, "KeepSave account confirmation", fmt.Sprintf("Use this single-use link for %s. It expires and grants no workspace role by itself.\n\n%s\n", message.Purpose, message.URL))
		return nil
	})
	clear(cipher)
	clear(nonce)
	if err != nil {
		result = DeliveryOutcome{State: "failed", Reason: "message_unavailable"}
	}
	if result.State != "sent" && result.State != "failed" && result.State != "uncertain" {
		result = DeliveryOutcome{State: "uncertain", Reason: "smtp_acceptance_unknown"}
	}
	if e := s.transaction(ctx, func(tx *sql.Tx) error {
		if e := s.guard.LockSubjects(ctx, tx, []uuid.UUID{user}, false); e != nil {
			return e
		}
		changed, e := tx.ExecContext(ctx, `UPDATE identity_proof_deliveries SET state=$1,reason_code=$2,ciphertext=''::bytea,nonce=''::bytea,completed_at=NOW() WHERE id=$3 AND state='dispatched'`, result.State, result.Reason, id)
		if e != nil {
			return e
		}
		count, e := changed.RowsAffected()
		if e != nil {
			return e
		}
		if count != 1 {
			return ErrConflict
		}
		return s.audit.CreateTx(tx, &user, nil, "identity.delivery_completed", "", models.JSONMap{"delivery_id": id.String(), "state": result.State, "reason_code": result.Reason}, "")
	}); e != nil {
		return DeliveryOutcome{State: "uncertain", Reason: "outcome_persistence_failed"}, e
	}
	return result, nil
}
func (s *Service) ProofStatus(ctx context.Context, p policy.Principal, id uuid.UUID) (DeliveryOutcome, error) {
	var result DeliveryOutcome
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		if e := s.actor(ctx, tx, p, nil, false, false); e != nil {
			return e
		}
		return tx.QueryRowContext(ctx, `SELECT d.state,d.reason_code FROM identity_proof_deliveries d JOIN identity_proofs p ON p.id=d.proof_id WHERE p.id=$1 AND p.user_id=$2`, id, p.SubjectID).Scan(&result.State, &result.Reason)
	})
	return result, err
}

// Resend creates a fresh proof and invalidates its predecessor. It never retries
// an uncertain SMTP attempt. Invitations have an explicit admin revoke/recreate.
func (s *Service) ResendProof(ctx context.Context, p policy.Principal, id uuid.UUID) (ProofRequest, error) {
	var result ProofRequest
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		if e := s.actor(ctx, tx, p, nil, true, true); e != nil {
			return e
		}
		row, e := loadProof(ctx, tx, id, true)
		if e != nil || row.UserID != p.SubjectID || row.Purpose == "invitation" || row.Consumed {
			return ErrProof
		}
		var state string
		if e = tx.QueryRowContext(ctx, `SELECT state FROM identity_proof_deliveries WHERE proof_id=$1 FOR UPDATE`, id).Scan(&state); e != nil {
			return e
		}
		if state == "dispatched" {
			return ErrConflict
		}
		if state != "failed" && state != "uncertain" && state != "cancelled" {
			return ErrConflict
		}
		sid := p.SessionID
		if row.Purpose == "password_reset" {
			sid = uuid.Nil
		}
		result, e = s.createProof(ctx, tx, p.SubjectID, sid, row.Purpose, row.Contact)
		return e
	})
	return result, err
}

// MarkDeliveryUncertain is recovery after a worker dies during SMTP. It does not
// enqueue another message; an active user/admin must create a new proof.
func (s *Service) MarkDeliveryUncertain(ctx context.Context, id uuid.UUID) error {
	return s.transaction(ctx, func(tx *sql.Tx) error {
		var user uuid.UUID
		if e := tx.QueryRowContext(ctx, `SELECT p.user_id FROM identity_proof_deliveries d JOIN identity_proofs p ON p.id=d.proof_id WHERE d.id=$1`, id).Scan(&user); e != nil {
			return e
		}
		if e := s.guard.LockSubjects(ctx, tx, []uuid.UUID{user}, false); e != nil {
			return e
		}
		changed, e := tx.ExecContext(ctx, `UPDATE identity_proof_deliveries SET state='uncertain',reason_code='worker_outcome_unknown',ciphertext=''::bytea,nonce=''::bytea,completed_at=NOW() WHERE id=$1 AND state='dispatched' AND dispatched_at<NOW()-INTERVAL '1 minute'`, id)
		if e != nil {
			return e
		}
		n, e := changed.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			return ErrConflict
		}
		return s.audit.CreateTx(tx, &user, nil, "identity.delivery_uncertain", "", models.JSONMap{"delivery_id": id.String()}, "")
	})
}
