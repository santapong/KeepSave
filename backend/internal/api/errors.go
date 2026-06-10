package api

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse is the wire shape returned for every non-2xx response.
// The numeric Code field mirrors the HTTP status (kept for back-compat with
// existing clients); ErrorCode carries the stable machine-readable symbol
// described in docs/ERROR_HANDLING_STANDARD.md.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
}

// HTTPError is the typed error carried up from services to handlers. Handlers
// pass it to WrapError, which sends only the safe fields (Status, Message,
// Symbol) to the client and logs the underlying cause server-side. The cause
// is never serialized.
type HTTPError struct {
	Symbol  string
	Status  int
	Message string
	cause   error
}

func (e *HTTPError) Error() string { return e.Message }
func (e *HTTPError) Unwrap() error { return e.cause }

// Sentinel errors. Use Wrap(<sentinel>, cause) when you have an internal
// error to log; use the sentinel directly when the cause is already a safe
// summary (e.g. "missing field foo").
var (
	ErrInvalidInput = &HTTPError{Symbol: "INVALID_INPUT", Status: http.StatusBadRequest, Message: "request was invalid"}
	ErrUnauthorized = &HTTPError{Symbol: "UNAUTHORIZED", Status: http.StatusUnauthorized, Message: "authentication required"}
	ErrForbidden    = &HTTPError{Symbol: "FORBIDDEN", Status: http.StatusForbidden, Message: "forbidden"}
	ErrNotFound     = &HTTPError{Symbol: "NOT_FOUND", Status: http.StatusNotFound, Message: "not found"}
	ErrConflict     = &HTTPError{Symbol: "CONFLICT", Status: http.StatusConflict, Message: "conflict with current state"}
	ErrRateLimited  = &HTTPError{Symbol: "RATE_LIMITED", Status: http.StatusTooManyRequests, Message: "too many requests"}
	ErrInternal     = &HTTPError{Symbol: "INTERNAL", Status: http.StatusInternalServerError, Message: "internal error"}

	// ErrServiceUnavailable is for features deliberately switched off at
	// runtime (e.g. the promotion kill switch, FOLLOWUPS #0e) — not an
	// authz decision, so 403 would mislead clients.
	ErrServiceUnavailable = &HTTPError{Symbol: "SERVICE_UNAVAILABLE", Status: http.StatusServiceUnavailable, Message: "service temporarily unavailable"}
)

// Wrap returns a new HTTPError that inherits Symbol/Status/Message from base
// and attaches cause for server-side logging.
func Wrap(base *HTTPError, cause error) *HTTPError {
	return &HTTPError{
		Symbol:  base.Symbol,
		Status:  base.Status,
		Message: base.Message,
		cause:   cause,
	}
}

// WrapMessage is like Wrap but overrides the safe end-user message. Use when
// the sentinel's default message is too generic to be useful in context.
func WrapMessage(base *HTTPError, message string, cause error) *HTTPError {
	return &HTTPError{
		Symbol:  base.Symbol,
		Status:  base.Status,
		Message: message,
		cause:   cause,
	}
}

// RespondError preserves the legacy shape so existing safe call-sites that
// pass a hardcoded message (e.g. "missing field") keep working without
// migration. New code should prefer WrapError so the typed sentinel drives
// the response and any internal cause is logged out-of-band.
func RespondError(c *gin.Context, code int, message string) {
	c.JSON(code, ErrorResponse{
		Error: ErrorDetail{Code: code, Message: message},
	})
}

// WrapError is the safe sink for any error reaching a handler. If err is or
// wraps an *HTTPError, the sentinel's Status + Message are sent to the
// client and the cause (if any) is logged. sql.ErrNoRows is mapped to 404
// to collapse repetitive boilerplate in repository chains. Anything else is
// reported as 500 INTERNAL with the cause logged for triage; the client
// never sees the raw error string.
func WrapError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	var he *HTTPError
	if errors.As(err, &he) {
		if he.cause != nil {
			log.Printf("api error code=%s status=%d path=%s cause=%v", he.Symbol, he.Status, c.FullPath(), he.cause)
		}
		c.JSON(he.Status, ErrorResponse{
			Error: ErrorDetail{Code: he.Status, Message: he.Message, ErrorCode: he.Symbol},
		})
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("api error code=%s status=%d path=%s cause=%v", ErrNotFound.Symbol, ErrNotFound.Status, c.FullPath(), err)
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: ErrorDetail{Code: http.StatusNotFound, Message: ErrNotFound.Message, ErrorCode: ErrNotFound.Symbol},
		})
		return
	}
	log.Printf("api error code=%s status=%d path=%s cause=%v", ErrInternal.Symbol, ErrInternal.Status, c.FullPath(), err)
	c.JSON(http.StatusInternalServerError, ErrorResponse{
		Error: ErrorDetail{Code: http.StatusInternalServerError, Message: ErrInternal.Message, ErrorCode: ErrInternal.Symbol},
	})
}
