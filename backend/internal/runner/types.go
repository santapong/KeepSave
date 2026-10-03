// Package runner supervises the separately enrolled, rootless Linux connector.
// It has no database, vault or provider credential dependency.
package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalid     = errors.New("invalid runner request")
	ErrDenied      = errors.New("runner operation denied")
	ErrUnavailable = errors.New("runner unavailable")
	ErrNoWork      = errors.New("no runner work")
	ErrConsumed    = errors.New("runner attempt already used")
	imagePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9./:_-]{0,180}@sha256:[a-f0-9]{64}$`)
	digestPattern  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	tokenPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

const (
	MaxRequestBytes    = 16 * 1024
	MaxResultBytes     = 4 * 1024 * 1024
	MaxAttemptDuration = 30 * time.Second
)

type Arguments struct {
	Path string `json:"path,omitempty"`
}

// OperationRequest is the entire connector-visible authority request. There are
// deliberately no URLs, methods, commands, environment or credential fields.
type OperationRequest struct {
	OperationID   uuid.UUID `json:"operation_id"`
	RunID         uuid.UUID `json:"run_id"`
	GrantID       uuid.UUID `json:"grant_id"`
	Attempt       int64     `json:"attempt"`
	Fence         int64     `json:"fence"`
	ImageDigest   string    `json:"image_digest"`
	RequestDigest string    `json:"request_digest"`
	Nonce         string    `json:"nonce"`
	Kind          string    `json:"kind"`
	Arguments     Arguments `json:"arguments"`
}

// Ticket is returned only to the authenticated supervisor. Token must never be
// written to the connector request file, environment, arguments or telemetry.
type Ticket struct {
	TicketID      uuid.UUID `json:"ticket_id"`
	OperationID   uuid.UUID `json:"operation_id"`
	RunID         uuid.UUID `json:"run_id"`
	GrantID       uuid.UUID `json:"grant_id"`
	Attempt       int64     `json:"attempt"`
	Fence         int64     `json:"fence"`
	ImageDigest   string    `json:"image_digest"`
	RequestDigest string    `json:"request_digest"`
	Nonce         string    `json:"nonce"`
	ExpiresAt     time.Time `json:"expires_at"`
	Kind          string    `json:"kind"`
	Arguments     Arguments `json:"arguments"`
	Token         string    `json:"token"`
}

type ClaimRequest struct {
	ImageDigest string `json:"image_digest"`
}
type TicketStatusRequest struct {
	TicketID uuid.UUID `json:"ticket_id"`
	Token    string    `json:"token"`
}
type TicketStatusResponse struct {
	Active bool `json:"active"`
}
type ExecuteRequest struct {
	TicketID uuid.UUID        `json:"ticket_id"`
	Token    string           `json:"token"`
	Request  OperationRequest `json:"request"`
}
type ExecuteResponse struct {
	Result    json.RawMessage `json:"result"`
	Outcome   string          `json:"outcome"`
	ReceiptID uuid.UUID       `json:"receipt_id"`
}

func (t Ticket) Request() OperationRequest {
	return OperationRequest{t.OperationID, t.RunID, t.GrantID, t.Attempt, t.Fence, t.ImageDigest, t.RequestDigest, t.Nonce, t.Kind, t.Arguments}
}

func RequestDigest(kind string, args Arguments) string {
	b, _ := json.Marshal(struct {
		Kind      string    `json:"kind"`
		Arguments Arguments `json:"arguments"`
	}{kind, args})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func ValidImage(image string) bool { return imagePattern.MatchString(image) }
func (r OperationRequest) Validate() error {
	if r.OperationID == uuid.Nil || r.RunID == uuid.Nil || r.GrantID == uuid.Nil || r.Attempt < 1 || r.Attempt != r.Fence || !ValidImage(r.ImageDigest) || !digestPattern.MatchString(r.Nonce) || !digestPattern.MatchString(r.RequestDigest) || r.RequestDigest != RequestDigest(r.Kind, r.Arguments) {
		return ErrInvalid
	}
	switch r.Kind {
	case "repository_tree":
		if r.Arguments.Path != "" {
			return ErrInvalid
		}
	case "read_file":
		p := r.Arguments.Path
		if p == "" || len(p) > 512 || !utf8.ValidString(p) || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00\r\n") {
			return ErrInvalid
		}
		for _, r := range p {
			if unicode.IsControl(r) {
				return ErrInvalid
			}
		}
		for _, part := range strings.Split(p, "/") {
			if part == "" || part == "." || part == ".." {
				return ErrInvalid
			}
		}
	default:
		return ErrInvalid
	}
	return nil
}

func (t Ticket) Validate(image string, now time.Time) error {
	if t.TicketID == uuid.Nil || !tokenPattern.MatchString(t.Token) || t.ImageDigest != image || !t.ExpiresAt.After(now) || t.Request().Validate() != nil {
		return ErrInvalid
	}
	return nil
}
