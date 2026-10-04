# KeepSave error handling standard

![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

Source reconciled 2026-10-04. The implemented safe sink is
[backend/internal/api/errors.go](../backend/internal/api/errors.go), not a future
`api/httperror` package. This corrects the original Phase A proposal while retaining
its no-leak requirement. See [current status](STATUS.md) and
[threat model](THREAT_MODEL.md); this is a source guide, not a new security audit.

## Public REST contract

The legacy-compatible nested envelope has a **numeric HTTP status** in `code`,
a safe `message`, and an optional symbolic `error_code`:

```json
{
  "error": {
    "code": 503,
    "message": "service temporarily unavailable",
    "error_code": "SERVICE_UNAVAILABLE"
  }
}
```

`WrapError` resolves typed `HTTPError` sentinels and returns safe fields only.
`RespondError` preserves hardcoded safe legacy responses and may omit the symbolic
field. A global promise that every failure includes `error_code` is incorrect.
MCP/OAuth have their own protocol error envelopes; do not wrap them as management
REST merely to make examples uniform. The sole
[OpenAPI source](../backend/internal/api/openapi/core.json) describes maintained
management responses.

| Condition | Source mapping / public behavior |
|---|---|
| Invalid input | 400 `INVALID_INPUT` or an explicit safe validation response |
| Missing/expired/revoked human session | 401 authentication required; reauthenticate before protected diagnostics |
| Authoritative session database unavailable | 503 `SERVICE_UNAVAILABLE`; do not fall back to stale allow |
| Vault/authority denial | 403 safe denial through common sink; resource-aware adapters may conceal foreign/missing resources identically |
| Known missing permitted record | 404 `NOT_FOUND`; do not expose stored cross-tenant ownership |
| Stale expected revision | 409 `CONFLICT`; reload and decide, never silently overwrite |
| Throttled request | 429 `RATE_LIMITED` or safe rate-limit response |
| Deliberately unavailable slice/enrollment | 503; configured metadata is not evidence the feature is enabled |
| Unrecognized internal error | 500 with safe `internal error`; no raw SQL/crypto/provider text |

`HTTPError` carries `Symbol`, `Status`, `Message` and an unexported cause.
`Wrap` attaches a cause to a known safe sentinel; `WrapMessage` allows an explicit
reviewed safe message. Do not derive that message from `err.Error()`, request
payloads, secret values, proof strings or provider output.

## Handler and domain boundaries

Handlers translate requests and call authorized application services. They map
typed domain outcomes to public messages without adding SQL, decryption, credential
logic or permission decisions. New platform handlers use bounded domain-specific
mappers over the shared response helper; unknown errors collapse to a fixed error.
The policy denial projection separately decides which details the caller may see.

Foreign and nonexistent protected resources must remain indistinguishable where
required by their contract. An expired/revoked caller does not obtain resource
hints through diagnostics. Explicit current-authority checks precede historical
value reads, operation result retrieval and audit downloads.

The sink logs internal causes on selected paths. Logging a cause is **not** blanket
permission to log credential-bearing errors. Keep causes, structured references
and safe reason codes free of values, tokens, proof URLs, SQL parameters, recovery
material and raw provider bodies. Public-safe text and logging-safe context need
separate review. Do not expose arbitrary stack traces to the browser.

## Frontend and client behavior

The application client extracts `error.message` from the nested envelope and
retains older string-error compatibility. Protected 401 clears local browser
identity, pending provider proofs and registered secret caches; account-switch
checks reject late responses before publication. A failed server logout/revocation
must be shown as unconfirmed, rather than pretending the server session was revoked.

403 and 404 are not evidence that a foreign resource exists. Show the contract's
safe denial with a next step appropriate to current authority. A 503 can mean an
unavailable feature or authoritative dependency, not successful operation. Do not
label configured, queued or uncertain work completed. External provider outcome
and permission to deliver its result are separate.

The retained [embed API](../frontend/src/embed/api.ts) still throws from
`data.error` directly and does not implement the application's protected-401 reset
path. It may display an unhelpful object-conversion message. This is an explicit
compatibility limitation; do not claim every client has the application mapper or
cache-clear behavior. Improving that adapter needs its own implementation/tests.

## Enforcement and verification

[error_leak_test.go](../backend/internal/api/error_leak_test.go) performs an AST
check across non-test Go files in the API package. It rejects `err.Error()` passed
to `RespondError`, bare `AbortWithError` and direct error strings placed in `gin.H`.
It is a useful regression guard, not proof that every computed string, transitive
log or endpoint is safe. Actual router/OpenAPI tests establish concrete envelopes.

For touched contracts, test invalid input, generic unknown-account login, foreign
and missing resources, expired/revoked sessions, database outage, stale revision
and forced internal/audit failure. Assert safe public output, no misleading success
and no value/proof/token in logs or receipts. Preserve independent Security review
for authorization/custody boundary changes. Dated local evidence and external
qualification remain in the [acceptance ledger](validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).
