# P0 Remediation Design — MCP output scrubbing + missing audit emissions

Status: **Proposed / HELD** — Security-Engineer sign-off required before implementation
(crypto/auth/promotion + audit-log mandate fall under the ROLES.md §2.2 veto). This doc carries no
implementation, per the gated process ("open a PR with the design first").

Source: three-project review, re-verified on Opus. All three findings CONFIRMED against current
code. Files/lines below are from that verification.

---

## P0-1 — MCP gateway returns subprocess stdout unscrubbed (NEW-9) — HIGH

**Status: implemented (egress scrubbing).** `scrubSecrets` in
`backend/internal/api/handlers_mcp_gateway.go` redacts every injected secret value from the
subprocess `output` before it is parsed or returned (fix option 1 below); the structured-output
hardening (option 2) and off-env delivery (option 3 / ADR-0010 Phase B) remain tracked follow-ups.

Resolves the open question **ADR-0010 §OQ-2** (secret-in-output handling).

**Defect.** Decrypted secrets are injected into the MCP subprocess env and the subprocess's stdout
is returned to the HTTP caller with no scrubbing:
- `backend/internal/api/handlers_mcp_gateway.go:377` builds `name=<plaintext>` from
  `crypto.Decrypt` (:372); `:421` sets `cmd.Env`.
- `:450` reads stdout raw; `:467-470` / `:474-475` return it to the caller; it reaches the client
  at `:215-219` (`c.JSON(... Result: result)`).
- No `scrub|redact|sanitiz|mask` exists on the success path. The error path is already hardened
  (`:202-213` static message).

A buggy/malicious/diagnostic third-party MCP server that echoes its env leaks plaintext secrets
back over HTTP — a `never-surface-plaintext` violation. (Scope: only secrets the caller is already
authorized for, returned to that caller — so HIGH, not CRITICAL.)

**Proposed fix (for review).**
1. **Scrub on egress (minimum).** Before returning `output`, replace every injected secret *value*
   with a redaction token (`***`). The injected values are known at call time (the same list built
   at `:377`), so this is an exact-string replace over the output buffer — no heuristics.
2. **Treat tool stdout as untrusted (preferred, ADR-0010 direction).** Require structured
   JSON-RPC output and reject non-conforming text rather than passing raw bytes through;
   combine with (1) as defense-in-depth.
3. Decision for sign-off: do we also move secret delivery off the env channel (ADR-0010 Phase B,
   unix-socket) now, or land scrubbing first and keep Phase B tracked? Recommend: land (1)+(2) now,
   keep Phase B as the tracked follow-up.

**Acceptance.** A test MCP server that prints its env yields a response with all secret values
redacted; a conforming server is unaffected.

---

## P0-2 — `MCPService.UninstallServer` emits no audit event (NEW-10) — HIGH

**Defect.** `backend/internal/service/mcp_service.go:150-152` deletes an installation
(`mcpRepo.DeleteInstallation`) and returns with no `emitAudit`, unlike its peers `InstallServer`
(:132) / `UpdateInstallation` (:145) / `DeleteServer` (:93). The handler
(`handlers_mcp.go:301`) returns 204 with no audit; the signature also lacks the `ipAddr` param the
audited peers carry. The canonical taxonomy (`docs/AUDIT_LOG_COVERAGE.md:75-79`) has **no**
installation-uninstall event row.

**Proposed fix (for review).**
- Add a canonical event — `mcp.installation_deleted` (or `…_uninstalled`) — to
  `docs/AUDIT_LOG_COVERAGE.md` following the `entity.action` convention.
- Thread `ipAddr` through `handler → UninstallServer`, emit the event, and add a handler-level test
  asserting the audit row (the CLAUDE.md hard gate).
- Decision for sign-off: confirm the event name and field set.

## P0-3 — `ProjectService.UpdateEmbedConfig` emits no audit event (NEW-11) — HIGH

**Defect.** `backend/internal/service/project_service.go:156-170` mutates the embed origin
allow-list / `embed_policy_enabled` and returns with no `emitAudit` and no `ipAddr`, unlike
`Create`/`Update`/`Delete` siblings. The handler (`handlers_embed.go:94`) passes no
`c.ClientIP()`. The required event **already exists** in the taxonomy
(`docs/AUDIT_LOG_COVERAGE.md:42` — `embed.origins_updated` with `project_id, actor_id, added[],
removed[]`); `auditRepo` is already wired into `ProjectService`.

**Proposed fix (for review).** Thread `ipAddr` through `handler → UpdateEmbedConfig`, emit
`embed.origins_updated` with the added/removed diff, add a handler-level test asserting the row. No
taxonomy or infra change needed — pure compliance fix.

---

## Why HELD

P0-1 changes how decrypted-secret data crosses an output boundary (Security veto, `internal/crypto`
consumers + never-surface-plaintext). P0-2/P0-3 add audit events governed by the audit-log mandate.
Per ROLES.md §2.2 / §3.1 these need Security-Engineer review of the approach (esp. the P0-1 event
name/redaction strategy and the new P0-2 taxonomy entry) before code lands.

## Sign-off requested
- Security Engineer: approve the P0-1 scrubbing strategy + the P0-2 new audit event name.
- Tech Lead: confirm P0-2/P0-3 are compliance fixes (not a new Type-1 ADR).

On sign-off, implementation lands as a follow-up commit on this branch (held draft → ready) with the
required handler-level audit-row tests.
