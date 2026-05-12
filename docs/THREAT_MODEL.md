# KeepSave Threat Model

**Version:** 1.2.0 | **Last review:** 2026-05-12 (re-baseline during ROLES 30-day) | **Previous:** 1.1.0 (2026-04-19)

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
| I      | Master key exfiltrated via logs              | Master key never logged; only cached in RAM                 | manual review                                                    | Low      |
| D      | KMS throttle stalls startup                  | MasterKeyProvider retries with backoff                      | `keyprovider/env.go` and KMS adapters                            | Medium   |
| E      | Compromised process reads RAM                | Container isolation, minimal image                          | `backend/Dockerfile`                                             | Medium   |

**Open follow-ups:** secure-zero master key in memory on shutdown; verify keyrotation audit emission (likely missing per Critical finding above); hardware attestation for nodes handling master key.

## 2. Authentication (`auth.JWT`, `auth.APIKey`, middleware)

| STRIDE | Threat                                       | Mitigation                                                                 | File:line                                                | Residual |
|--------|----------------------------------------------|----------------------------------------------------------------------------|-----------------------------------------------------------|----------|
| S      | Forged JWT                                   | HS256 with secret only on server; signature verified                        | `auth/auth.go:39`, `api/middleware.go:59-100`             | Low      |
| S      | Forged API key                               | Hashed at rest (SHA-256); compared via constant-time helper                 | `auth/apikey.go:11-26`                                    | Low      |
| T      | Token replay after revocation                | API key revocation = row delete; JWT expires in 24h, **no denylist**        | `auth/auth.go:25-26`                                      | Medium   |
| R      | Login attempts not audited                   | Auth events not in audit log (verify in 30d)                                | `auth_service.go`                                         | Medium   |
| I      | Auth error leaks user existence              | Login wraps `sql.ErrNoRows` as "invalid credentials" — verify             | `auth_service.go:70`                                      | Low      |
| D      | Credential stuffing                          | Per-IP rate limit + exponential backoff                                     | `api/ratelimit*.go`                                       | Low      |
| E      | API key scope escalation                     | Scope is row-bound; per-project / per-env enforced in handlers              | `auth/apikey.go`, `models/models.go:56-59`                | Low      |

## 3. Promotion engine

| STRIDE | Threat                                       | Mitigation                                                       | File:line                                              | Residual |
|--------|----------------------------------------------|------------------------------------------------------------------|---------------------------------------------------------|----------|
| T      | Secret modified between diff and apply       | Transactional apply; diff re-validated                            | `service/promotion_service.go:272-346`                  | Low      |
| R      | Approver identity spoofed                    | Approver re-auths; audit captures `sub`                          | `service/promotion_service.go:215-240`                  | Low      |
| R      | Approval decision merged with execution audit | No distinct `promotion_approved` event before execution (gap)    | `service/promotion_service.go:215-240`                  | Medium   |
| I      | Diff leaks plaintext                         | Diff redacts values; only keys + action shown                    | review needed                                           | Low      |
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

## Change log

- **1.2.0 (2026-05-12):** Re-baselined during 30-day plan. Added Section 4 (embed widget). Added "Findings new in v1.2.0" block with four critical/high open gaps. Added file:line refs throughout. Added "Assumptions" verification cadence and "Out-of-scope" list.
- **1.1.0 (2026-04-19):** STRIDE pass on vault, OAuth, MCP, promotion. Pre-30-day-plan baseline.
