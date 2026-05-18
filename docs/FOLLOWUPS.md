# KeepSave — Follow-ups

This is the single source of truth for tracked technical debt and deferred work. Every entry has an **owner** (a role from `docs/ROLES.md`) and a **due date**. No-owner items are not tracked here — they're either assigned or deleted.

**Cadence:** reviewed at the monthly ADR review (`docs/ROLES.md` §6). Items past due without an explanation get pulled into the next 30-day plan.

---

## Closed (v1.1.0)

- [x] **Nightly audit-log pruner wiring in `main.go`** — closed by `startAuditLogPruner` in `backend/cmd/server/main.go` + `AuditRepository.DeleteOlderThan` + table-driven tests in `backend/internal/repository/audit_repo_test.go`.
- [x] **KeepSave ↔ Seidr regression harness** — closed by `tests/e2e/seidr/` (docker-compose + stdlib-only Go tester).

---

## Closed (Phase 1 deployment sweep — 2026-05-18)

PR #54 / branch `claude/audit-deployment-plan-moSsk`. Cross-references in `docs/audits/AUDIT_2026-05-18_DEPLOYMENT_READINESS.md` §6.

- [x] **#0 Audit logging missing for Secret / Project / API-key mutations** — `emitAudit` wired into `internal/service/{secret,project,apikey,auth}_service.go`; `internal/service/audit_helper.go` is the nil-safe wrapper; `audit_helper_test.go` asserts rows are written.
- [x] **#0a Error responses leak DB / crypto internals** — 125 `err.Error()` leak sites migrated to `httperror.WrapError` per `docs/ERROR_HANDLING_STANDARD.md`; `internal/api/error_leak_test.go` is the AST-walking regression gate.
- [x] **#0b Embed widget accepts auth from any origin** — already landed in PR #50; this PR added the cross-origin CORS test surface that exercises the same boundary (`backend/internal/api/cors_test.go`).
- [x] **#0c No handler-level negative-auth tests** — ~42 cells across `internal/api/negative_auth_test.go`, `internal/service/negative_auth_test.go`, `cors_test.go`, `url_safety_test.go`. Covers the BLOCKER/HIGH surface; full 132-cell matrix remains for Phase 3.
- [x] **#0d / #5 Approver-cannot-be-requester invariant** — app-level `ErrSelfApproval` in `internal/service/promotion_service.go` + DB `CHECK` constraint in `migrations/{postgres,mysql,sqlite}/008_promotion_self_approval_check.sql`.
- [x] **#9 Audit-log assertion coverage gaps** — every new mutating-service test asserts the audit row; covered by the same test sweep as #0.

The following remain **open** because they are explicitly out of Phase 1 scope per the user's plan choices:

- **#1 AWS / GCP KMS adapters** — deferred per ADR-0016 (Vault-only this round). Tracked unchanged.
- **#0e Promotion feature-flag / kill switch** — not in Phase 1 scope.
- **#0f CI permissions block** — already closed earlier.
- **#0g CODEOWNERS** — Phase 3.
- **#0h–0j Frontend follow-ups** — closed earlier in PRs #48/#49.
- **#0k Default expiration on `ks_` keys** — closed in Phase 3 (see below).
- **#0l `MustGet` panic refactor** — closed in Phase 3 (see below).
- **#2, #3, #6, #7, #8, #10** — unchanged.

---

## Closed (Phase 3 medium/low sweep — 2026-05-19)

Commits I–N on the same branch / PR #54. Cross-referenced in
`docs/audits/AUDIT_2026-05-18_DEPLOYMENT_READINESS.md` §7.

- [x] **#0k Default expiration on `ks_` API keys (S-M4)** — ADR-0009 wired
  end-to-end: `internal/service/apikey_service.go::computeEffectiveAPIKeyExpiry`
  defaults to 90 days, caps at 365 days, refuses past times. INSERT now
  writes `expires_at`. Middleware was already enforcing non-NULL expiry.
  Existing NULL rows grandfathered.
- [x] **#0l `MustGet` panic refactor (S-L1)** — 61 of 62 sites migrated
  to `getUserID(c) (uuid.UUID, bool)` helper in
  `internal/api/project_access.go`. Returns 401 + Abort on miss instead
  of panicking. Remaining reference is the helper's own doc comment.

The dead `internal/api/csrf.go` middleware was deleted (S-L2 — KeepSave
is bearer-token only, CSRF moot) and `lucide-react@^1.8.0` was verified
as the current stable line (S-L5 — `npm audit` clean).

---

## Backlog (deferred from PR #54 deployment audit, 2026-05-20)

These items were intentionally **not** addressed by the BLOCKER + HIGH +
MEDIUM + LOW sweep on `claude/audit-deployment-plan-moSsk`. Listed here
so an operator coming in cold sees the explicit remaining surface.

### Deferred by design (audit + plan choice)

- **S-M3 — Full 132-cell negative-auth test matrix.** The Phase 1 sweep
  added ~42 cells covering the BLOCKER + HIGH attack surface
  (`internal/api/negative_auth_test.go`,
  `internal/service/negative_auth_test.go`, `cors_test.go`,
  `url_safety_test.go`). The remaining ~90 cells (per
  `tests/NEGATIVE_AUTH_PLAN.md`) were deferred per the user's
  plan-question-4 choice during Phase 0. Trigger to revisit: any new
  endpoint that handles a project ID, OR a customer security review
  asking for the matrix completion.
- **S-M6 — Audit-log tamper-evident storage.** The original audit
  itself routed this to Phase B. Phase A operators export the
  `audit_log` table nightly to an immutable bucket (GCS Object
  Lock / S3 Object Lock) — see `DEPLOYMENT_PLAN.md` PROD gate G16.
  Hash-chain / MAC inside the row is the Phase B mechanism; revisit
  when a customer SLA names "tamper-evident audit" as a requirement.
- **FOLLOWUPS #1 — AWS / GCP KMS adapters.** Deferred per ADR-0016 /
  user's plan-question-3 choice. Vault provider (already wired) is the
  UAT path. Trigger: production cloud is decided (GCP → wire GCP KMS;
  AWS → wire AWS KMS).

### Still tracked in this file's other sections

- **#0e Promotion feature-flag / kill switch** (Phase A).
- **#0g CODEOWNERS for security-critical paths** (Phase A).
- **#2 Backup tamper-detection test** (Phase A, 60 days).
- **#3 DEK rotation API** (Phase A, 60 days).
- **#6 Phase 15 service unit tests** (Phase A, 60 days).
- **#7 Nonce-collision monitoring per DEK** (Phase A, 90 days).
- **#8 Seidr-runtime boot in E2E harness** (Phase A, 60 days).
- **#10 FIPS-mode evaluation** (Phase A decision).
- All **Phase B** items (per-environment DEK, JWT denylist, fine-grained
  scopes, three-of-N approval, HKDF subkey, ephemeral attested workload
  identity, lease-renewal grammar).

### ADRs still Proposed

- **ADR-0008** (RS256 JWKS rotation).
- **ADR-0014** (Audit log taxonomy extension).
- **ADR-0015** (`safego` helper and audit emission).

ADRs 0005, 0006, 0007, 0009, 0010, 0011, 0012, 0013, 0016 are Accepted
under sponsor authorization. Retroactive Security + Tech Lead sign-off
remains pending per CLAUDE.md §"Decision classes".

---

## Open — Phase A (MVP hardening)

Top-of-list = highest leverage. Order matters — anything blocking a 30-day action in `docs/ROLES_30_60_90.md` goes first.

### 0. Audit logging missing for Secret / Project / API key mutations *(P0)*
- **Status:** **OPEN — discovered during 30-day audit.** `secret_service.go`, `project_service.go`, `apikey_service.go` have no `auditRepo` and emit zero audit events on Create/Update/Delete. Handlers do not write audit rows either.
- **Why it matters:** "Repudiation" mitigation is currently absent for the most important state-mutating endpoints. A compromised account leaves no in-product trail. This is a hole in the feature, not a "future enhancement".
- **Owner:** Backend Engineer + Security Engineer (review).
- **Due:** 30 days (Phase A) — non-negotiable.
- **Related:** `docs/AUDIT_LOG_COVERAGE.md` (the spec), `docs/THREAT_MODEL.md` v1.2.0 "Findings new" §1.

### 0a. Error responses leak DB / crypto internals
- **Status:** **OPEN — discovered during 30-day audit.** Multiple handlers return `err.Error()` directly; `pgx`, `pq`, and `crypto/cipher` errors reach clients.
- **Why it matters:** Information-disclosure. Attacker enumeration is easier when internal error text is echoed.
- **Owner:** Backend Engineer.
- **Due:** 30 days (Phase A).
- **Related:** `docs/ERROR_HANDLING_STANDARD.md`.

### 0b. Embed widget accepts auth from any origin
- **Status:** **OPEN — discovered during 30-day audit.** `frontend/src/embed/auth.ts:21-26` has no `ev.origin` check; outbound `postMessage` uses target origin `'*'`.
- **Why it matters:** A malicious host page can inject a fake auth token — confused-deputy on widget requests.
- **Owner:** Frontend Engineer (allow-list endpoint requires Backend support).
- **Due:** 30 days (Phase A).
- **Related:** `docs/EMBED_ORIGIN_POLICY.md`.

### 0c. No handler-level negative-auth tests anywhere
- **Status:** **OPEN — discovered during 30-day audit.** Auth middleware exists but is not regression-tested. A middleware regression would land green.
- **Why it matters:** "Spoofing / Elevation" mitigations are unverified.
- **Owner:** QA + Backend Engineer.
- **Due:** 30 days (matrix fills); 60 days (CI presence-check gate).
- **Related:** `tests/NEGATIVE_AUTH_PLAN.md`.

### 0d. Approver-cannot-be-requester invariant unverified
- **Status:** ADR-0003 §Open Questions calls it out. Not yet verified whether enforced at DB or only service code.
- **Why it matters:** The multi-party-control linchpin for PROD promotions.
- **Owner:** Backend Engineer + Security Engineer.
- **Due:** 30 days.

### 0e. Promotion feature flag / kill switch
- **Status:** No runtime flag exists to disable `/promote` and `/approve` without redeploy.
- **Why it matters:** ADR-0003 names this as step 1 of the rollback plan.
- **Owner:** Backend Engineer.
- **Due:** 30 days.

### 0f. CI workflow lacked an explicit `permissions:` block
- **Status:** **CLOSED THIS PR.** Top-level `permissions: contents: read` now set in `.github/workflows/ci.yml`. Per-job overrides documented in `docs/CI_PERMISSIONS.md`.

### 0g. CODEOWNERS and commit-message convention for security-critical paths
- **Status:** OPEN. The veto-list audit (`docs/VETO_LIST_AUDIT.md`) identifies the convention; CODEOWNERS file not yet added.
- **Owner:** Tech Lead.
- **Due:** 30 days.

### 0h. Dashboard storage of JWT — key name inconsistency
- **Status:** `frontend/src/pages/HelpPage.tsx` reads `localStorage('jwt')` while the rest of the app uses `localStorage('keepsave_token')`. Likely a bug; HelpPage's auth-bearing call probably returns null in production.
- **Owner:** Frontend Engineer.
- **Due:** 30 days (low-effort fix).

### 0i. Auto-hide timer on revealed secrets
- **Status:** OPEN in both widget and dashboard. No timer; secrets remain revealed indefinitely.
- **Owner:** Frontend Engineer + UX (interim Tech Lead).
- **Due:** 30 days.

### 0j. Destructive actions use `window.confirm` instead of typed confirmation
- **Status:** OPEN. `SecretsPanel.tsx:131`, `PromotionsList.tsx:87`, `ProjectAPIKeysPanel.tsx:97` use the browser native dialog.
- **Owner:** Frontend Engineer + UX (interim).
- **Due:** 30 days.

### ~~0k. Default expiration on `ks_` API keys (no immortal credentials)~~ — **CLOSED in Phase 3 (2026-05-19), see Closed section above**
- **Status:** **OPEN — discovered during competitor synthesis (Vault / SPIFFE / GitHub dossiers converge).** `backend/internal/api/validation.go:33-38` (`CreateAPIKeyRequest` has no `expires_at` field); `backend/internal/service/apikey_service.go:33-60` (`Create` takes no expiry arg); `backend/internal/repository/apikey_repo.go:33-36` (INSERT omits `expires_at`). Result: every `ks_` key minted is valid indefinitely until manual deletion. Middleware at `backend/internal/api/middleware.go:101-105` already honours `ExpiresAt` when non-NULL — only issuance is broken.
- **Why it matters:** A leaked agent / CI key has no time bound. GitHub mandates ≤366d on PATs; SPIFFE recommends ≤15min on JWT-SVID; CircleCI 2023 is the canonical incident shape. Indefinite credentials are below segment baseline.
- **Owner:** Backend Engineer + Security Engineer (review).
- **Due:** 30 days (Phase A). Pattern: GitHub fine-grained PAT mandatory expiration; default 90d, ceiling 365d, sentinel `9999-12-31` for opt-out audited per-key.
- **Related:** `docs/research/competitors/github.md` Candidate 3; `docs/research/competitors/spiffe.md` §5 row JWT-SVID; `docs/research/competitors/vault.md` Candidate 2.

### ~~0l. `c.MustGet("user_id").(uuid.UUID)` panic surface (58 sites) → `safego` + helper refactor~~ — **CLOSED in Phase 3 (2026-05-19), see Closed section above**
- **Status:** **OPEN — discovered during 2026-05-15 audit team sweep (`docs/audits/BACKEND_CRASH_RISKS.md` §Risk 11).** Pattern `c.MustGet("user_id").(uuid.UUID)` is repeated verbatim **58 times** across `backend/internal/api/handlers_*.go`. Gin's `MustGet` panics on missing key; bare type-assertion panics on type mismatch. Gin's global `gin.Recovery()` at `backend/internal/api/router.go:29` catches handler-thread panics today, but the pattern is a forward-looking footgun — the moment a future PR mounts any handler outside the `JWTAuthMiddleware` group, the handler panics on every request.
- **Why it matters:** Defense in depth. Also a prerequisite for ADR-0015 (per-use API-key audit emission) which needs the helper as its call point. The audit's BACKEND_CRASH_RISKS.md report names this as the single most valuable hardening pass alongside `defer recover()` on goroutines.
- **Owner:** Backend Engineer (single PR; pure refactor).
- **Due:** 60 days (Phase A → Phase B bridge).
- **Pattern:** new `getUserID(c *gin.Context) (uuid.UUID, bool)` helper that does `c.Get` + comma-ok type assertion + 401 on miss. Replace all 58 sites. Also introduce `safego.Launch(ctx, fn)` wrapper for the 9 production goroutines per the crash audit (ADR-0010 part C draft codifies this).
- **Related:** `docs/audits/BACKEND_CRASH_RISKS.md` §Risk 11; ADR-0010 (proposed) part C; ADR-0015 (proposed) — this FU is its prerequisite.

### 1. AWS / GCP KMS adapters wired into `main.go`
- **Status:** Code exists (`kms_aws.go`, `kms_gcp.go` in `backend/internal/crypto/keyprovider/`); blocked on `go mod tidy` to add `aws-sdk-go-v2/service/kms` and `cloud.google.com/go/kms/apiv1` to `go.sum`.
- **Why it matters:** ADR-0004 calls `EnvProvider` development-only; production deployments need KMS. Without this, the runbook tells customers "use a KMS" but the binary doesn't support one yet.
- **Owner:** DevOps + Backend Engineer (pair).
- **Due:** 30 days (Phase A).
- **Related:** ADR-0004 §Open Questions.

### 2. Backup tamper-detection test
- **Status:** Pending. Need a test that mutates a row's ciphertext or nonce in a backup-style copy of the DB and asserts the AEAD-auth tag check rejects it cleanly.
- **Why it matters:** Formalizes the AEAD-auth guarantee in ADR-0001. Today, the guarantee is in the algorithm; the test proves we've wired it correctly.
- **Owner:** QA / Test Engineer.
- **Due:** 60 days (Phase A).
- **Related:** ADR-0001, `docs/PENTEST_CHECKLIST.md`.

### 3. DEK rotation API
- **Status:** No code path exists. ADR-0004 acknowledges DEK rotation isn't built.
- **Why it matters:** Without DEK rotation, a leaked project DEK has no in-product mitigation — only project re-creation. That's an unacceptable posture for a production secrets product.
- **Owner:** Backend Engineer.
- **Due:** 60 days (Phase A).
- **Related:** ADR-0004 §Open Questions; ROLES_30_60_90 §3 "Backend 90d".

### 4. Promotion feature flag / kill switch
- **Status:** No runtime flag to disable `/promote` and `/approve`.
- **Why it matters:** ADR-0003 names this as the first step in the rollback plan; today it requires a code change + deploy, which is the wrong shape for an incident.
- **Owner:** Backend Engineer.
- **Due:** 30 days (Phase A).
- **Related:** ADR-0003 §Open Questions.

### 5. Approver-cannot-be-requester invariant
- **Status:** Unclear whether enforced at the DB layer or only in service code. ADR-0003 flags it.
- **Why it matters:** A single-account compromise would otherwise let the attacker request *and* approve a PROD promotion. This is the multi-party-control linchpin.
- **Owner:** Backend Engineer + Security Engineer (review).
- **Due:** 30 days (Phase A).
- **Related:** ADR-0003 §Open Questions.

### 6. Phase 15 service unit tests (drift, anomaly, usage-analytics, recommendation, nlp-query)
- **Status:** Feature-complete code; tests missing. Writing them needs a SQLite test harness that doesn't exist in the repo yet.
- **Why it matters:** These services are part of the dashboard. Untested code that ships is a bug factory.
- **Owner:** Backend Engineer.
- **Due:** 60 days (Phase A) — build SQLite test harness first (30 days), then tests.
- **Related:** ROLES_30_60_90 §3 "Backend 60d".

### 7. Nonce-collision monitoring per DEK
- **Status:** No counter. ADR-0001 names the 2^32 GCM safety boundary.
- **Why it matters:** We are nowhere near 2^32 today, but "nowhere near" without measurement is wishful thinking. Add a counter and an alert at 2^28.
- **Owner:** Backend Engineer.
- **Due:** End of Phase A (90 days).
- **Related:** ADR-0001 §Open Questions.

### 8. Seidr-runtime boot in E2E harness
- **Status:** Tester mimics `KeepSaveSecretProvider.Get` HTTP contract; needs real Seidr container image published to a registry.
- **Why it matters:** Mimicked contracts drift from real ones. The mimic catches obvious regressions but won't catch protocol-level changes.
- **Owner:** DevOps (image publishing) + QA (harness).
- **Due:** 60 days (Phase A).
- **Related:** `tests/e2e/seidr/`.

### 9. Audit-log assertion coverage gaps
- **Status:** Spotty — many secret-mutating handlers don't have a test asserting the audit row exists.
- **Why it matters:** Audit log is the feature. If we ever ship a handler that mutates without auditing, we won't know until a customer notices.
- **Owner:** Backend Engineer.
- **Due:** 30 days (Phase A).
- **Related:** ROLES_30_60_90 §3 "Backend 30d".

### 10. FIPS-mode evaluation
- **Status:** Open question — do any current/near-term customers require FIPS validation?
- **Why it matters:** If yes, ADR-0001 needs revisiting. If no, we ignore until the question changes.
- **Owner:** PM (interim Tech Lead).
- **Due:** 30 days (decision, not implementation).
- **Related:** ADR-0001 §Open Questions.

---

## Open — Phase B (multi-tenant, deferred)

Captured here so they're not lost, **not** to be worked on until Phase B starts. If something on this list becomes urgent, it gets promoted to Phase A with a written reason and an explicit trade (something else gets pushed down).

- **Per-environment DEK** (Option C in ADR-0004). Reconsider once we have a customer or compliance requirement.
- **JWT denylist for pre-expiry revocation** (ADR-0002 §Open Questions). Build when a customer SLA requires it.
- **Fine-grained API key scopes** (per-secret or per-action; ADR-0002). Build when a use case appears, not before.
- **Three-of-N approval for PROD** (ADR-0003). Build if regulatory pressure or a customer commitment forces it.
- **Subkey derivation (HKDF) for auxiliary purposes** (audit-log MAC, etc.; ADR-0004). Only with a new ADR.
- **Ephemeral attested workload identity for AI agents (SPIFFE-shaped SVIDs).** Today `ks_` keys are issued with no expiry (`backend/internal/api/validation.go:33-38` — `CreateAPIKeyRequest` has no `expires_at`; `backend/internal/service/apikey_service.go:33` — `Create` takes no expiry param), so a leaked agent key is valid until manual deletion. **Trigger:** (a) a customer with an MCP / AI-agent workflow asks for short-lived agent credentials, OR (b) a CVE-class incident in the field shows long-lived agent keys being exfiltrated and abused, OR (c) MedQCNN / Nexus integration (`docs/medqcnn_integration.md`, `docs/nexus_integration.md`) reaches multi-tenant agent deployment. Research input: `docs/research/competitors/spiffe.md`; leapfrog framing: `docs/research/BEYOND.md` §2.5.
- **Lease-renewal grammar for `SecretLease`** (`backend/internal/models/models.go:367-377`). Today `SecretLease` has `ExpiresAt` + `Revoked` but no renewal endpoint; long-running agents must re-issue and lose audit-trail continuity across a single session. **Trigger:** first customer agent runs longer than current lease max-TTL, OR first complaint about lease-ID churn fragmenting audit search. Pattern source: HashiCorp Vault lease-renewal grammar — see `docs/research/competitors/vault.md` Candidate 5. Owner: Backend Engineer (when trigger fires).

---

## Process notes

- **Adding an item:** open a PR that adds a numbered entry under "Open — Phase A" (or Phase B). Include an owner and a due date. PR is reviewed by the named owner — if they refuse, the item goes back to its proposer or is killed.
- **Closing an item:** move it under the "Closed" section with a one-line description of the artifact that closes it (commit SHA, file path, or PR number). Don't delete — historical follow-ups are useful in post-mortems.
- **Slipping a due date:** allowed *once*, with a written reason here. Second slip triggers a re-plan of that role's 30/60/90.

Once Phase A items close, `SECURITY_AUDIT.md` v1.1.0 can be re-stamped v1.1.1 and the "Known follow-ups" block removed there.
