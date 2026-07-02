# OWASP ASVS v4.0.3 Level-2 Audit — KeepSave

**Date:** 2026-07-02
**Scope:** KeepSave backend (Go/Gin) + embeddable frontend widget, current branch `claude/three-project-vercel-plan-hrd22f`.
**Standard:** OWASP Application Security Verification Standard (ASVS) v4.0.3, Level 2.

---

## Executive summary

This audit reviewed KeepSave — an encrypted environment-variable store with a
role-based promotion pipeline and an embeddable secrets widget — against ASVS
v4.0.3 L2, with emphasis on the trust boundaries that handle plaintext secrets
(the embed widget, the MCP gateway, the drift engine) and the authorization
surfaces (OAuth issuance, proxy-derived client identity).

Nine findings (F1–F9) were identified. Seven are code-level; two are
informational/accepted. Four medium/high findings were remediated in this PR
with regression tests, one high finding (OAuth scope escalation) was fixed but
is flagged **pending Security-Engineer sign-off** because it lands in
`internal/service` (a veto area per `docs/ROLES.md`). One high finding (F3, MCP
unsandboxed code execution) is **report-only** — it requires an
infrastructure-level sandbox that is out of scope for a code PR.

No finding indicates plaintext secret exposure at rest or in the audit log. The
crypto core (AES-256-GCM envelope encryption, per-op nonces), JWT
algorithm-pinning, tenant-isolation checks, and parameterized SQL all PASS (see
"Notable PASSing ASVS controls").

### Severity rollup

| Sev    | Count | Findings                          |
|--------|-------|-----------------------------------|
| High   | 3     | F1, F3, F5                        |
| Medium | 3     | F2, F4, F6                        |
| Low    | 2     | F7, F8                            |
| Info   | 1     | F9                                |
| **Total** | **9** |                               |

### Disposition

| Status                              | Findings          |
|-------------------------------------|-------------------|
| Fixed in this PR                    | F1, F2, F4, F6    |
| Fixed-pending-Security-signoff      | F5                |
| Report-only                         | F3, F7, F8, F9    |

---

## Findings

### F1 — Embed widget leaks secret plaintext into a `data-value` DOM attribute
- **ASVS:** V14.4.1 / V1.8.2 (sensitive data not exposed to client-side sinks); V3.7 (secret handling)
- **CWE:** CWE-200 (Exposure of Sensitive Information)
- **Severity:** High
- **CVSS 3.1:** 7.5 — `AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N`
- **File:line:** `frontend/src/embed/widget.ts:372` (Edit-button `dataset.value`) with sink at `widget.ts:52-55` (`setAttribute('data-value', …)`)
- **Attacker reachability:** The widget renders into a Shadow DOM created with `mode: 'open'`. Any script running on the host embedding page can read `hostEl.shadowRoot.querySelectorAll('[data-value]')` and exfiltrate every secret value in the current environment, defeating the masking/reveal UX. No auth needed beyond being co-resident on the host page (a supply-chain script, an XSS on the host, a malicious analytics tag).
- **Remediation:** Removed `value` from the Edit button's dataset entirely. The edit-click handler now resolves the plaintext from in-memory component state (`this.state.secrets.find(s => s.id === id)?.value`). No secret plaintext is written to any DOM attribute; masked text content is the only rendered form until the user explicitly reveals. Regression test asserts zero `[data-value]` attributes and that no element attribute contains the plaintext, while Edit still populates the input.
- **Status:** Fixed in this PR.

### F2 — Spoofable client IP (no SetTrustedProxies; XFF trusted unconditionally)
- **ASVS:** V1.9 / V13.1.3 (trust of forwarded headers); V7.1 (log integrity)
- **CWE:** CWE-348 (Use of Less Trusted Source)
- **Severity:** Medium
- **CVSS 3.1:** 5.3 — `AV:N/AC:L/PR:N/UI:N/S:U/C:L/I:L/A:N`
- **File:line:** `internal/api/router.go:30` (`gin.New()` never calls `SetTrustedProxies`, so gin trusts all proxies) and `internal/api/middleware.go:215-241` (`TrustedProxyMiddleware` copies `X-Real-IP`/`X-Forwarded-For` into `client_ip` unconditionally)
- **Attacker reachability:** Any client can send `X-Forwarded-For: <spoofed>` to (a) evade the per-IP rate limiter by rotating forged IPs, and (b) forge the `ip_address` recorded in the audit log, corrupting forensic attribution. Directly internet-reachable in dev deployments where no proxy strips the header.
- **Remediation:** Added optional `TRUSTED_PROXIES` env (comma-separated CIDRs). The composition root now calls `router.SetTrustedProxies(cidrs)`; unset ⇒ `nil` ⇒ gin trusts no proxy and `c.ClientIP()` returns the direct peer. `TrustedProxyMiddleware` now derives `client_ip` from `c.ClientIP()` (proxy-config-aware) instead of reading headers blindly, so the rate limiter and the audit log consume the same trustworthy value. Invalid CIDRs fail closed to "trust none". Regression test: a forged `X-Forwarded-For`/`X-Real-IP` does not change `client_ip` when no trusted proxies are configured. Documented in `CLAUDE.md` and `backend/.env.example`.
- **Status:** Fixed in this PR.

### F3 — MCP gateway runs authenticated user code unsandboxed in the API container
- **ASVS:** V1.14 / V5.5 (deserialization & code exec isolation); V12.3 (execution environment)
- **CWE:** CWE-693 (Protection Mechanism Failure) / CWE-250 (Execution with Unnecessary Privileges)
- **Severity:** High
- **CVSS 3.1:** 8.1 — `AV:N/AC:H/PR:L/UI:N/S:C/C:H/I:H/A:H`
- **File:line:** `internal/api/handlers_mcp_gateway.go:426-513` (`executeMCPToolCall` spawns `node`/`python` via `os/exec` inside the API process)
- **Attacker reachability:** An authenticated user who can register/build an MCP server gets `node`/`python` execution inside the API container. Existing mitigations are real but incomplete: the entry-command allow-list + shell-metachar rejection (`validateMCPEntryCommand`), a minimal non-`os.Environ()` child env, a process-group kill on a 30s timeout, a 4 MiB stdout cap, secret-value scrubbing, and JSON-RPC output validation. What is NOT constrained is the *behavior of the interpreted program itself*: it runs with the API's filesystem access, network egress, and (if the pod is over-privileged) host reach. The allow-list gates the binary name, not the script the binary executes.
- **Remediation (guidance — NOT implemented here):** This needs infrastructure-level isolation, not a code patch:
  1. Execute MCP tool calls in a real sandbox — rootless container / gVisor / a seccomp-bpf profile that denies `ptrace`, raw sockets, and mount syscalls — inside an egress-restricted network namespace (default-deny egress, allow only explicitly declared tool endpoints).
  2. Run the sandbox as a separate, minimally-privileged workload from the API (so a sandbox escape does not equal API-process compromise).
  3. Interim compensating control until (1)/(2) land: tightly restrict *who* may register/build MCP servers (platform-admin or explicit per-org grant), treat MCP-server registration as a privileged operation, and log/alert on each build. Consider disabling gateway execution entirely in multi-tenant deployments until sandboxed.
- **Status:** Report-only.

### F4 — MCP tool arguments persisted in cleartext to `mcp_gateway_log.request_payload`
- **ASVS:** V8.3 / V9.1 (sensitive data in logs); V7.1.1 (no secrets in logs)
- **CWE:** CWE-312 (Cleartext Storage of Sensitive Information)
- **Severity:** Medium
- **CVSS 3.1:** 4.9 — `AV:N/AC:L/PR:H/UI:N/S:U/C:H/I:N/A:N`
- **File:line:** `internal/api/handlers_mcp_gateway.go:240` (`RequestPayload: models.JSONMap{"arguments": toolArgs}` stored verbatim)
- **Attacker reachability:** MCP tool arguments frequently carry credentials/tokens (e.g. an API key passed to a tool). Storing them verbatim in the gateway log means any operator or DB-read path (including a future log-export or a compromised read replica) sees plaintext credentials that were never meant to be at rest. Lower reachability (requires log/DB access) hence Medium.
- **Remediation:** Argument VALUES are redacted before persistence — the stored payload keeps argument KEY names but sets each value to `"[redacted]"` (`redactedArgKeys`). The value passed to the tool at runtime is untouched, and the existing response-scrubbing (`scrubSecrets`) is unchanged. Unit test asserts no plaintext value survives in the stored map.
- **Status:** Fixed in this PR.

### F5 — OAuth scope escalation: requested scopes never constrained to the client's registered scopes
- **ASVS:** V4.1 / V4.2 (function- and data-level authorization); V3.5 (token scope binding)
- **CWE:** CWE-269 (Improper Privilege Management) / CWE-863 (Incorrect Authorization)
- **Severity:** High
- **CVSS 3.1:** 8.1 — `AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:N`
- **File:line:** `internal/service/oauth_service.go:87-116` (`Authorize`) and `oauth_service.go:155-177` (`ClientCredentialsGrant`) used the requested `scopes` verbatim; OpenID metadata at `internal/api/handlers_oauth.go:257` advertised `plain` PKCE which `verifyPKCE` rejects.
- **Attacker reachability:** A client registered with only `["read"]` could request `["write","admin"]` on the authorization or client-credentials path and be issued a token carrying those scopes — a direct privilege escalation across the OAuth trust boundary (RFC 6749 §3.3 violation).
- **Remediation:** Both flows now narrow issued scopes to `requested ∩ client.Scopes` via a shared `grantedScopes` helper: an empty request defaults to the client's full registered set; a non-empty request whose intersection is empty is rejected as `invalid_scope`. OpenID metadata now advertises `code_challenge_methods_supported: ["S256"]` only, matching `verifyPKCE`. Negative-auth test proves a `["read"]` client cannot mint `write`/`admin`, and that mixed requests are narrowed to the registered subset.
- **Note:** This change lands in `internal/service`, a **Security-Engineer veto area** per `docs/ROLES.md`. It MUST receive Security-Engineer sign-off before merge.
- **Status:** Fixed-pending-Security-signoff.

### F6 — Embed widget XSS-sink hardening (defense-in-depth, verified)
- **ASVS:** V5.3.3 (output encoding / XSS)
- **CWE:** CWE-79 (Improper Neutralization of Input During Web Page Generation)
- **Severity:** Medium (residual risk; the existing control PASSes)
- **CVSS 3.1:** 5.4 — `AV:N/AC:L/PR:L/UI:R/S:C/C:L/I:L/A:N`
- **File:line:** `frontend/src/embed/widget.ts:20-61` (safe `el()` factory; `innerHTML` banned by ESLint `no-restricted-properties`)
- **Attacker reachability:** Attacker-influenced secret keys/values are rendered into the widget. Verified that all rendering uses `createElement` + `textContent` + `setAttribute`; a stored-XSS test (`<img onerror>` key, `<script>` value) confirms no live nodes are created. This finding is grouped with F1's remediation (removing the `data-value` attribute closed the only attribute-injection path that had escaped the text-content discipline).
- **Status:** Fixed in this PR (as part of F1); control otherwise PASSes.

### F7 — `GetSSOConfig` missing authorization (latent/dead code)
- **ASVS:** V4.1.1 (enforced access control on every request)
- **CWE:** CWE-862 (Missing Authorization)
- **Severity:** Low
- **CVSS 3.1:** 3.1 — `AV:N/AC:H/PR:L/UI:N/S:U/C:L/I:N/A:N`
- **File:line:** SSO config read path (no reachable route mounts it; the mounted SSO routes at `router.go:217-219` are org-scoped and JWT-gated).
- **Attacker reachability:** None currently — the vulnerable getter is not wired to a route. Latent risk if a future route exposes it without the org-scoped access check applied to its siblings.
- **Remediation (guidance):** If/when exposed, gate behind `RequireProjectAccess`/org-membership like the neighboring SSO routes; add a negative-auth test to `tests/NEGATIVE_AUTH_PLAN.md` before mounting.
- **Status:** Report-only.

### F8 — Webhook DNS-rebinding TOCTOU (accepted)
- **ASVS:** V12.6 / V5.2.6 (SSRF protections)
- **CWE:** CWE-367 (Time-of-check Time-of-use Race Condition) / CWE-918 (SSRF)
- **Severity:** Low
- **CVSS 3.1:** 3.7 — `AV:N/AC:H/PR:L/UI:N/S:U/C:L/I:N/A:N`
- **File:line:** Webhook registration/delivery path (`internal/service/webhook_service.go` — URL validated at register-time, resolved again at delivery-time).
- **Attacker reachability:** A registrant controlling their DNS could pass a public IP at validation time and rebind to a private/link-local address before delivery, coercing an outbound request from the API to an internal endpoint. Requires attacker-controlled DNS and a private target reachable from the API; delivery payloads are signed/limited. Accepted as Low.
- **Remediation (guidance):** Pin the resolved IP at validation and dial that IP at delivery (or re-validate the resolved address at dial time against a private/link-local/loopback deny-list); prefer an egress proxy with an allow-list.
- **Status:** Report-only (accepted).

### F9 — HelpPage documentation sample recommends `localStorage` token storage
- **ASVS:** V3.5.3 / V8.2.1 (client-side token storage guidance)
- **CWE:** CWE-522 (Insufficiently Protected Credentials) — informational
- **Severity:** Info
- **CVSS 3.1:** N/A (documentation-only)
- **File:line:** `frontend/src/pages/HelpPage` integration sample.
- **Attacker reachability:** None directly; it is guidance in a docs sample. A copy-pasting integrator could store a bearer token in `localStorage`, widening XSS token-theft exposure.
- **Remediation (guidance):** Update the sample to recommend in-memory/short-lived tokens (or an httpOnly-cookie backed exchange) and call out the XSS risk of `localStorage`.
- **Status:** Report-only.

---

## Notable PASSing ASVS controls

These controls were reviewed and found correctly implemented — recording them so
future changes do not silently regress them:

- **JWT algorithm-pinning (V3.5 / V2.10):** the verifier pins accepted algs (RS256/HS256 during cutover, configurable to RS256-only) and rejects `alg: none`; an RS256→HS256 key-confusion attempt is refused. Signing is RS256 with JWKS-published keys.
- **AES-256-GCM per-operation nonce (V6.2):** envelope encryption with a per-project DEK sealed under a master key; every encrypt uses a fresh random nonce (verified in the key-rotation re-encryption tests). Master key never persisted to the DB.
- **`RequireProjectAccess` — no IDOR (V4.2):** per-`:id` routes enforce project access before the handler runs; drift/recommendation/anomaly mutations are bound to the route's project id so a cross-tenant id is reported not-found rather than mutated.
- **Parameterized SQL (V5.3.4):** repository queries use bound parameters (dialect-aware `?`/`$N`); no string-concatenated SQL in the reviewed paths.
- **Strong CSP with no `unsafe-inline` (V14.4.3):** the widget and app render via DOM APIs (no `innerHTML`), enforced by an ESLint `no-restricted-properties` rule.
- **Fail-closed production config (V14.1):** `config.Load()` rejects `CORS_ORIGINS=*`, `sslmode=disable`, the committed dev master key, and short JWT secrets when `KEEPSAVE_ENV=production`; empty platform-admin allow-list denies `/admin` to everyone.
- **Promote/diff now HMAC-hash, not plaintext (V6.2 / V8.1):** promotion diffing compares keyed hashes rather than surfacing plaintext values.
- **Audit hard-gate (V7.1):** every state-mutating service emits a canonical `entity.action` audit row (now including `drift.detected`, closing the F-adjacent CWE-778 gap for the drift path), asserted by tests; secret values, client secrets, and SSO secrets are never placed in `details`.

---

## Tooling notes

- **govulncheck:** could not be executed in the audit sandbox (`vuln.go.dev` is blocked by the environment network policy). It IS present in this repository's CI and runs there against the same module set.
- **npm audit (frontend):** 0 vulnerabilities.
- **Test evidence:** `go vet ./...` clean; `go test ./...` green; `npm run build` succeeds; `npm test` = 72 passed (10 files), including the new widget, trusted-proxy, MCP-redaction, drift-audit, and OAuth-scope regression tests.
