# Error Handling Standard (Backend 30-day)

Today's pattern returns `err.Error()` directly to clients from several handlers, leaking DB column types, GCM auth-tag failure messages, and validation-library output. This document is the standard going forward.

This is Backend Engineer 30-day work item §3 from `docs/ROLES_30_60_90.md`.

---

## Two error layers

```
   handler  →  client      (sanitized: a code + a safe message)
       │
       ▼
   service  →  handler     (rich: wrapped errors with internal context)
       │
       ▼
   repo / crypto  →  service  (raw: pgx, sql, crypto/cipher errors)
```

**Rule:** raw errors from the bottom layer never reach the client. Sanitization happens in *one* place — a small `httperror` package — not in each handler.

## The `httperror` package (to be added at `backend/internal/api/httperror/`)

```go
package httperror

type Error struct {
    Code    string // stable machine-readable, e.g. "INVALID_INPUT", "NOT_FOUND"
    Status  int    // HTTP status
    Message string // safe message for end users
    cause   error  // internal; never serialized
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.cause }
```

Sentinel errors that handlers and services build on:

```go
var (
    ErrInvalidInput   = &Error{Code: "INVALID_INPUT",   Status: 400, Message: "request was invalid"}
    ErrUnauthorized   = &Error{Code: "UNAUTHORIZED",    Status: 401, Message: "authentication required"}
    ErrForbidden      = &Error{Code: "FORBIDDEN",       Status: 403, Message: "access denied"}
    ErrNotFound       = &Error{Code: "NOT_FOUND",       Status: 404, Message: "not found"}
    ErrConflict       = &Error{Code: "CONFLICT",        Status: 409, Message: "conflict with current state"}
    ErrRateLimited    = &Error{Code: "RATE_LIMITED",    Status: 429, Message: "too many requests"}
    ErrInternal       = &Error{Code: "INTERNAL",        Status: 500, Message: "internal error"}
)

// Wrap returns an Error that wraps cause but does NOT serialize the cause text.
func Wrap(base *Error, cause error) *Error { /* ... */ }
```

## Handler pattern

```go
func (h *SecretHandler) Create(c *gin.Context) {
    var req CreateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        respondError(c, httperror.Wrap(httperror.ErrInvalidInput, err))
        return
    }
    secret, err := h.svc.Create(c.Request.Context(), toInput(req))
    if err != nil {
        respondError(c, err) // svc returns *httperror.Error or an unrecognized error
        return
    }
    c.JSON(201, secret)
}

func respondError(c *gin.Context, err error) {
    var he *httperror.Error
    if errors.As(err, &he) {
        // log the chain (with cause) on the server side
        slog.Error("request error",
            "code", he.Code,
            "status", he.Status,
            "cause", he.Unwrap(),  // internal-only
            "path",  c.FullPath(),
            "user_id", c.GetString("user_id"),
        )
        c.JSON(he.Status, gin.H{"error": gin.H{"code": he.Code, "message": he.Message}})
        return
    }
    // Unrecognized error → treat as internal. Never echo the message.
    slog.Error("unrecognized error", "err", err, "path", c.FullPath())
    c.JSON(500, gin.H{"error": gin.H{"code": "INTERNAL", "message": "internal error"}})
}
```

## Service pattern

Services return either an `*httperror.Error` or a raw error. Handlers can rely on the helper above to handle both.

```go
func (s *SecretService) Create(ctx context.Context, in CreateInput) (*Secret, error) {
    if err := s.repo.ProjectExists(ctx, in.ProjectID); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, httperror.Wrap(httperror.ErrNotFound, err) // "project not found"
        }
        return nil, fmt.Errorf("checking project: %w", err) // becomes 500
    }
    ct, nonce, err := s.cryptoSvc.Encrypt(ctx, in.ProjectID, []byte(in.Value))
    if err != nil {
        return nil, fmt.Errorf("encrypting secret: %w", err)  // becomes 500; client sees "internal error"
    }
    // …
}
```

## What MUST NOT happen

- `c.JSON(500, gin.H{"error": err.Error()})` — bare `err.Error()` in the response body is the bug we are fixing. Linter / test rejects.
- `c.AbortWithError(500, err)` — same problem (Gin serializes the error).
- Returning `pgx`, `pq`, `database/sql`, `crypto/cipher` errors directly from a handler.
- Returning the `validator.v10` raw error string (e.g., `"Key: 'X.Field' Error:..."`).

## Enforcement

A small test in `backend/internal/api/error_leak_test.go` walks all handlers and verifies (with table-driven test cases per endpoint):

- 400/401/403/404/409/429 responses have body shape `{"error":{"code":"...","message":"..."}}`.
- 500 responses always have body `{"error":{"code":"INTERNAL","message":"internal error"}}` regardless of underlying cause.
- No response body contains substrings `"pq: "`, `"pgx: "`, `"sql: "`, `"crypto/cipher"`, `"validator:"`.

Linter rule (lightweight, custom — or via `semgrep`): forbid `err.Error()` inside `c.JSON(...)` and `c.AbortWithError(...)` calls. Run on PR.

## Migration plan (30 days)

1. **Day 1-3:** Add `backend/internal/api/httperror/` package and `respondError` helper.
2. **Day 3-5:** Add the leak test (`error_leak_test.go`) — initially it documents the *current* broken state. PRs that fix endpoints flip the assertions from `expect_leak` to `expect_safe`.
3. **Day 5-15:** Migrate handlers in this order: `handlers_auth.go` → `handlers_secret.go` → `handlers_project.go` → `handlers_apikey.go` → `handlers_promotion.go` → `handlers_intelligence.go` (the inconsistent-style one). One PR per handler.
4. **Day 15-20:** Migrate service-layer error returns to wrap with `httperror.Wrap` where appropriate.
5. **Day 20-25:** Tighten the linter to fail CI on any `err.Error()` inside `c.JSON`/`c.AbortWithError`.
6. **Day 25-30:** Smoke test in staging: deliberately trigger crypto / DB errors, confirm responses are sanitized.

## Out of scope

- **Internationalization** of error messages. Single English for now; revisit when a customer requires.
- **Field-level validation messages** for forms. Front-end maps `INVALID_INPUT` to per-field UX on its own — we don't echo validator strings.
- **Stack traces in error responses.** Never. Always server-side log only.

## References

- Leak examples documented in `docs/THREAT_MODEL.md` v1.2.0 "Findings new" §3.
- `backend/internal/api/handlers_auth.go:22, 64`
- `backend/internal/api/handlers_secret.go:35`
- `backend/internal/api/handlers_promotion.go:48`
- `backend/internal/api/handlers_intelligence.go:44, 49, 63, 90`
- `backend/internal/service/secret_service.go:96`
