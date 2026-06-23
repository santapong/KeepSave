# KeepSave Threat Model

**Version:** 1.2.1 | **Last review:** 2026-05-15 (audit team sweep delta) | **Previous:** 1.2.0 (2026-05-12)

> **2026-05-15:** 5 new rows + 4 residual-risk updates from audit team sweep (`docs/audits/SECURITY_AUDIT_2026-05-15.md`).

This document is the canonical threat model. Re-baseline cadence: after every Type-1 change and at minimum monthly (`docs/ROLES.md` §6). All STRIDE entries are anchored to file:line refs so they can be re-verified mechanically.

## Trust boundaries

```
[Browser / SDK / MCP client]
   │  TLS (boundary 1: network)
   ▼
[Gin HTTP layer]
   │  Auth middleware: JWT or API key (boundary 2: caller identity)
   │  (backend/internal/api/middleware.go:59-120)
   ▼
[Service layer]
   │
   ├─► [crypto.Service + MasterKeyProvider]
   │     │  (boundary 3: key custody)
   │     │  ← Master key NEVER in DB
   │     ▼
   │   [KMS or env]
   │
   └─► [repository]
         ▼
       [Postgres]   ← all sensitive cols encrypted at rest
```

Assets: master key (KEK), per-project DEKs, secret plaintext, OAuth client secrets, API key hashes, audit log, backup snapshots.

---

## Findings new in v1.2.0 (re-baseline)

The 30-day code audit surfaced gaps not present in v1.1.0. These are listed up front because they invalidate parts of the previous threat model.

### Critical: Secret/Project/APIKey mutations are not audited
- **Evidence:** `backend/internal/service/secret_service.go`, `project_service.go`, `apikey_service.go` do **not** have `auditRepo` fields and emit no audit events on Create/Update/Delete. Handlers (`backend/internal/api/handlers_secret.go:20-137`, etc.) do not write audit rows either.
- **Implication:** "Repudiation" (the R in STRIDE) is currently **not mitigated** for the most important state-mutating endpoints. A compromised user account can create / modify / delete secrets with no in-product trail.
- **Mitigation status:** **Open.** Tracked in `FOLLOWUPS.md` as P0; Backend 30-day work item.

### Critical: No handler-level negative-auth tests
- **Evidence:** Test files in `backend/internal/api/` cover health, rate-limit, highload only (`health_test.go`, `ratelimit_test.go`, `highload_test.go`). No test asserts a wrong-project ID, expired JWT, or revoked API key produces 401/403.
- **Implication:** "Spoofing / Elevation" mitigations exist in middleware (`backend/internal/api/middleware.go:59-100+`) but are not verified by tests. A regression in the middleware would not be caught by CI.
- **Mitigation status:** **Open.** QA + Backend 30-day work item.

### High: Error responses leak DB and crypto details
- **Evidence:** `handlers_auth.go:64`, `handlers_secret.go:35`, `handlers_promotion.go:48`, `handlers_intelligence.go:44, 49, 63, 90` return `err.Error()` directly. Service-layer error wrapping (`fmt.Errorf("decrypting: %w", err)`) preserves the underlying message; `pgx`, `pq`, and `crypto/cipher` errors reach clients.
- **Implication:** "Information disclosure" — internal error text aids attacker enumeration (table names, column types, GCM "message authentication failed" telltales).
- **Mitigation status:** **Open.** Backend 30-day work item.

### High: Embed widget accepts auth from any origin
- **Evidence:** `frontend/src/embed/auth.ts:21-26` listens for `keepsave-auth` messages without checking `ev.origin`. `frontend/src/embed/auth.ts:33` sends to `window.parent` with target origin `'*'`.
- **Implication:** "Spoofing" — a malicious host page (or sibling iframe) can inject a fake auth token into the widget. The widget will use the attacker's credentials, leaking nothing directly but creating a confused-deputy on requests that depend on the widget's identity.
- **Mitigation status:** **Open.** Frontend 30-day work item; full policy in `docs/EMBED_ORIGIN_POLICY.md`.

### Medium: Repository layer is largely untested
- **Evidence:** 20 code files under `backend/internal/repository/`, 1 test file (`audit_repo_test.go`, 4 tests).
- **Implication:** Tampering / Integrity vectors at the DB boundary are not regression-tested. Schema or ORM-mapping regressions could silently corrupt encrypted columns.
- **Mitigation status:** **Open.** QA 60-day work item.

---

## 1. Vault (`crypto.Service` + `MasterKeyProvider`)

| STRIDE | Threat                                       | Mitigation                                                  | File:line                                                       | Residual |
|--------|----------------------------------------------|-------------------------------------------------------------|------------------------------------------------------------------|----------|
| S      | Attacker forges KMS decrypt request          | Provider interface uses IAM role + audited KMS logs         | `crypto/keyprovider/` (AWS/GCP files exist, not yet wired)       | Low      |
| T      | DEK or ciphertext tampered in DB             | AES-GCM auth tag rejects tampered input                     | `crypto/crypto.go:64-90` (decrypt)                               | Low      |
| R      | Key rotation without audit trail             | `keyrotation_service` writes audit entries                  | (verify in 30d) — service exists; check audit emit               | Medium   |
| R      | Audit-log row tamper / taxonomy gaps         | `audit_log` is plain table (no hash-chain/MAC); `role.changed`, `settings.changed` missing from `AUDIT_LOG_COVERAGE.md:21-43`; `AuditEntry` lacks `actor_type` (user vs api-key vs service-account). 2026-05-15: new row — taxonomy + tamper-evidence ADR pending (ADR-0010-adjacent). | `migrations/001_initial_schema.sql:66-77`, `docs/AUDIT_LOG_COVERAGE.md:21-43` | Medium   |
| I      | Master key exfiltrated via logs              | Master key never logged; only cached in RAM                 | manual review                                                    | Low      |
| I      | Webhook SSRF reaches IMDS / RFC1918 hosts    | None today; webhook URL is user-controlled with no allow/deny list. ADR for webhook-SSRF guard pending (Infisical Cand. 1 blocked-pending-SSRF-guard). | `backend/internal/service/webhook_service.go:136,158`            | **High** |
| D      | KMS throttle stalls startup                  | MasterKeyProvider retries with backoff. 2026-05-15: threat is moot today since KMS adapters unwired (`main.go:247-248` bails "not implemented"); restores to Medium when FU#1 lands (ADR-0008/0009-adjacent). | `keyprovider/env.go` and KMS adapters                            | Medium (moot) |
| E      | Compromised process reads RAM                | Container isolation, minimal image                          | `backend/Dockerfile`                                             | Medium   |

**Open follow-ups:** secure-zero master key in memory on shutdown; verify keyrotation audit emission (likely missing per Critical finding above); hardware attestation for nodes handling master key.

## 2. Authentication (`auth.JWT`, `auth.APIKey`, middleware)

| STRIDE | Threat                                       | Mitigation                                                                 | File:line                                                | Residual |
|--------|----------------------------------------------|----------------------------------------------------------------------------|-----------------------------------------------------------|----------|
| S      | Forged JWT                                   | RS256 with per-kid keypair (private key encrypted at rest), verified by kid against JWKS; explicit alg allowlist rejects `none` + algorithm-confusion (RFC 8725); HS256 accepted only during the cutover window (ADR-0008) | `auth/auth.go` (ValidateToken keyfunc), `auth/keystore.go`, `api/middleware.go:59-100` | Low      |
| S      | Forged API key                               | Hashed at rest (SHA-256); compared via constant-time helper                 | `auth/apikey.go:11-26`                                    | Low      |
| T      | Token replay after revocation                | API key revocation = row delete. Agent tokens (jti-bearing) are short-lived (≤15 min) and revocable before expiry via a denylist — by jti or by lease-cascade (ADR-0021). Ordinary user JWTs still expire-only (24h, no jti ⇒ skip denylist). | `auth/auth.go` (Denylister check), `repository/token_denylist_repo.go` | Low (agent) / Medium (user) |
| E      | Agent-token mint widens scope                | Mint exchanges an *active, caller-owned* lease for a token of equal-or-lesser scope and shorter life; project scope enforced before mint; never grants authority the lease lacked (ADR-0021) | `service/agent_token_service.go`, `api/handlers_agent.go` (MintAgentToken) | Low |
| R      | Login attempts not audited                   | Auth events not in audit log (verify in 30d)                                | `auth_service.go`                                         | Medium   |
| I      | Auth error leaks user existence              | Login wraps `sql.ErrNoRows` as "invalid credentials" — verify             | `auth_service.go:70`                                      | Low      |
| D      | Credential stuffing                          | Per-IP rate limit + exponential backoff                                     | `api/ratelimit*.go`                                       | Low      |
| E      | API key scope escalation                     | `EnforceAPIKeyScope` holds keys to their action scope per method and to their environment lock; per-secret scope grammar (`action[:keyGlob]`, ADR-0022) further restricts key-scoped keys to matching keys at create/get/update/delete/list. Legacy bare scopes unchanged. | `api/middleware.go` (EnforceAPIKeyScope, apiKeyScopeAllowsKey), `api/handlers_secret.go` | Low |
| E      | JWT lacks project bind                       | JWT carries `user_id` but no `project_id` claim; any authenticated user can present their valid JWT to any `/projects/:id/*` endpoint and bypass tenant isolation. None today; ADR-0005 (RequireProjectAccess middleware) lands the fix. | `backend/internal/auth/auth.go:11-62`                     | **High** |

## 3. Promotion engine

| STRIDE | Threat                                       | Mitigation                                                       | File:line                                              | Residual |
|--------|----------------------------------------------|------------------------------------------------------------------|---------------------------------------------------------|----------|
| T      | Secret modified between diff and apply       | Transactional apply; diff re-validated                            | `service/promotion_service.go:272-346`                  | Low      |
| R      | Approver identity spoofed                    | Approver re-auths; audit captures `sub`                          | `service/promotion_service.go:215-240`                  | Low      |
| R      | Approval decision merged with execution audit | No distinct `promotion_approved` event before execution (gap). 2026-05-15: A04-F1 approver=requester gap (separate row below) reinforces this; see ADR-0007 (approval audit split). | `service/promotion_service.go:215-240`                  | Medium   |
| I      | Diff leaks plaintext                         | Diff redacts values; only keys + action shown                    | review needed                                           | Low      |
| I      | Plaintext leak via `/promote/diff` response   | None today. Diff endpoint returns full plaintext `SourceValue` and `TargetValue`, contradicting the design intent that diffs are redacted; ADR for diff-redact pending. | `backend/internal/models/models.go:146-147`, `backend/internal/service/promotion_service.go:131,140` | **High** |
| D      | Flapping promotions saturate DB              | Per-project rate limit                                            | `api/ratelimit*.go`                                     | Low      |
| E      | Non-approver promotes to PROD                | PROD requires `promote` scope; pending record + separate approval | `service/promotion_service.go:186-195`                  | Low      |
| E      | Requester self-approves                      | **Invariant not currently enforced at DB layer** (gap)            | open follow-up                                          | Medium   |

## 4. Embed widget (browser trust domain)

| STRIDE | Threat                                                          | Mitigation                                                                | File:line                                | Residual |
|--------|------------------------------------------------------------------|---------------------------------------------------------------------------|------------------------------------------|----------|
| S      | Malicious host page injects fake `keepsave-auth` postMessage    | **None today** — listener has no origin check                              | `frontend/src/embed/auth.ts:21-26`       | **High** |
| T      | Malicious page modifies revealed-secret DOM                     | Shadow DOM isolates widget                                                | `frontend/src/embed/keepsave-widget.ts`  | Low      |
| I      | Secret exfiltration via postMessage to `*`                      | **No outbound message currently carries a secret;** code-review forbidden | policy + review                          | Low      |
| I      | Secret persisted to localStorage / IndexedDB                    | Embed code does not use storage (verified)                                | `frontend/src/embed/` (no storage refs)  | Low      |
| E      | Privilege escalation via attribute injection                    | `project-id` / `api-key` attributes are host-trusted by construction      | `keepsave-widget.ts:61, 65`              | Low      |

## 5. MCP Gateway (carried from v1.1.0; unchanged)

| STRIDE | Threat | Mitigation | Residual |
|---|---|---|---|
| S | Malicious MCP server impersonates legit | Registry binds server to owner + signature | Medium |
| T | Tool response tampered in transit | TLS to registered server; response schema check | Low |
| I | Secret injected into wrong tool | Injection scoped per project + environment | Low |
| D | Slow MCP server stalls gateway | Per-call 10s timeout + circuit breaker | Low |
| E | Tool call bypasses auth | Gateway enforces JWT/API key before routing | Low |

## 6. MCP tool execution (new, 2026-05-15)

Covers the execution path that §5 does not: command spawning by the gateway and build/install jobs by the registry. §5 covers transport and routing; §6 covers what happens when the gateway *runs* a registered server.

| STRIDE | Threat                                                                                                        | Mitigation                                                                                  | File:line                                                                | Residual |
|--------|---------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------|---------------------------------------------------------------------------|----------|
| T      | Authenticated user registers MCP server with malicious `EntryCommand`; gateway then execs the stored command with decrypted secrets in env. | None today; ADR-0010 (MCP command-execution hardening) pending.                              | `backend/internal/api/handlers_mcp_gateway.go:327-339`                    | **High** (authenticated RCE primitive) |
| D      | `RegisterServer` / `RebuildServer` spawn unbounded goroutines running `git clone` + `npm/pip install` + `go build` with no timeout. | None today.                                                                                  | `backend/internal/api/handlers_mcp.go:47, :163`                           | **High** (user-triggered DoS) |

## 7. Secret-version retention (new, 2026-05-15)

| STRIDE | Threat                                                                                              | Mitigation                                                                | File:line                                                                              | Residual |
|--------|-----------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------|-----------------------------------------------------------------------------------------|----------|
| I      | Deleted secrets retain their value via `SecretVersion` history; if a version was leaked and the secret is later "deleted", the leaked value still grants access. | None today — separate ADR for shred-on-delete required.                    | `backend/migrations/003_secret_versions.sql:2-16`, `backend/internal/repository/version_repo.go` | Medium   |

---

## Assumptions (verify at each re-baseline)

- Kubernetes or equivalent container runtime with network policies.
- Postgres with `sslmode=require`, offline encrypted backups.
- Master key managed by KMS in production (`EnvProvider` is dev-only per ADR-0004).
- Operators keep host OS and container images patched.
- HTTPS termination in front of the API; TLS not terminated at the Go process (verify per environment via `docs/SECRET_SOURCES.md`).

## Out-of-scope

- Compromise of the integrator's own host page (the embed widget cannot defend against an attacker with full DOM control on the host).
- Compromise of the customer-side runtime that holds API keys (we provide rotation; secure runtime storage is the customer's problem).
- Physical compromise of the database or KMS hardware.

## §8. Cross-origin trust boundary (added per ADR-0016, 2026-05-18)

ADR-0016 splits the frontend (Vercel) and backend (container platform) onto distinct origins. This widens the trust boundary — there is now a public-internet hop between the React SPA and the Gin API that previously was a same-origin call.

| Component | Trust | Notes |
|---|---|---|
| Vercel-hosted SPA | Untrusted (any browser may load it; integrity gated by Vercel + SRI) | Build artifacts are public. No secrets in `import.meta.env.VITE_*`. |
| Public internet between SPA and API | Untrusted | Confidentiality + integrity from TLS only. No mTLS today. |
| Container-hosted Gin API | Trusted | Holds master key in memory. CORS allow-list is the boundary control. |

### STRIDE rows for the new boundary

| # | Threat | Mitigation | Residual |
|---|---|---|---|
| §8/S | Attacker hosts a malicious SPA at `https://evil.example` and tricks a user into pasting their bearer token | Bearer tokens stored in `localStorage` are origin-bound by the browser; CORS allow-list refuses requests from non-allowlisted origins; embed widget refuses `postMessage` from non-allowlisted hosts (ADR-0006) | Low |
| §8/T | MITM on the public-internet hop | TLS required (`https://` only; `KEEPSAVE_ENV=production` rejects `sslmode=disable`); HSTS emitted when TLS terminates at the Go process | Low — TLS at managed Vercel/Fly/Cloud Run edges |
| §8/R | Origin spoofing in CORS preflight (the `Origin` header is set by the browser; an attacker tool may forge it) | Reflected `Access-Control-Allow-Origin` is only echoed when the value matches the exact-list or single-glob in `CORS_ORIGINS`; SOP still applies in real browsers | Low — non-browser callers can already use the API directly |
| §8/I | A second Vercel deploy under an attacker-controlled team subdomain matches a too-permissive glob pattern | Glob patterns reject 2+ wildcards (`compileOriginPatterns` in `backend/internal/api/middleware.go`); operators must use the **single-`*`** form and pin the team suffix; runbook documents the convention | Low if convention is followed; Medium if operators use overly broad globs |
| §8/D | Preflight floods aimed at exhausting backend CPU | Existing per-IP rate limiter applies to OPTIONS too; preflight returns 204 cheaply | Low |
| §8/E | Cookie-based session bypass | `Access-Control-Allow-Credentials` is unconditionally false; bearer-token only. Cookie auth is a Type-1 ADR away — do not enable without a new threat-model pass | None today |

### Operational invariants

- `Access-Control-Allow-Credentials` MUST stay `false`. Enabling it without re-doing this section opens up CSRF on cross-origin POSTs.
- `CORS_ORIGINS=*` is forbidden in production (`internal/config/config.go` startup check).
- Glob patterns are single-wildcard only. Multi-wildcard entries are dropped at parse time (`backend/internal/api/middleware.go::compileOriginPatterns`).
- `Vary: Origin` is emitted whenever an allow-list match reflects the origin (prevents cache poisoning by intermediaries).

### Files for verification

- `backend/internal/api/middleware.go` (`CORSMiddleware`, `matchOrigin`, `compileOriginPatterns`)
- `backend/internal/api/cors_test.go` (allow-list matrix + no-credentials assertion)
- `backend/internal/config/config.go:82-87` (production refuses `CORS_ORIGINS=*`)
- `frontend/src/api/client.ts:24-31` (`VITE_API_BASE_URL` resolution)
- `docs/adr/0016-deployment-topology.md` §Consequences/Security
- `docs/DEPLOYMENT_PLAN.md` §4.4 (operator CORS guidance)

## Change log

- **1.2.3 (2026-06-23):** §2 auth refresh for the auth-chain work on PR #63. Updated the *Forged JWT* row for RS256/JWKS + algorithm-confusion guard (ADR-0008) and the *Token replay after revocation* row for the short-lived agent-token denylist (ADR-0021); added a §2/E row for the agent-token mint path (lease→token, no scope widening); rewrote the *API key scope escalation* row (was **High**, now Low) for the enforced action scope + per-secret scope grammar (ADR-0022).
- **1.2.2 (2026-05-18):** Added §8 (cross-origin trust boundary per ADR-0016 acceptance). Updated mitigations columns referencing the audit S-B2/S-B3/S-B4/S-B5/S-B6 fixes that landed in PR #54.
- **1.2.1 (2026-05-15):** Audit team sweep delta (`docs/audits/SECURITY_AUDIT_2026-05-15.md`). 5 new STRIDE rows (§2/E JWT-no-project-bind, §3/I Diff plaintext, §1/I webhook SSRF, §1/R audit-log tamper + taxonomy gaps, §6/T MCP entry-command RCE, §6/D MCP build DoS, §7/I secret-version retention) and 4 residual-risk updates (§2/E API key scope -> High; §3/R approval-merge cross-refs ADR-0007; §1/T KMS throttle noted moot pending FU#1; §1/R audit-log row added). New top-level sections §6 (MCP tool execution) and §7 (Secret-version retention).
- **1.2.0 (2026-05-12):** Re-baselined during 30-day plan. Added Section 4 (embed widget). Added "Findings new in v1.2.0" block with four critical/high open gaps. Added file:line refs throughout. Added "Assumptions" verification cadence and "Out-of-scope" list.
- **1.1.0 (2026-04-19):** STRIDE pass on vault, OAuth, MCP, promotion. Pre-30-day-plan baseline.
