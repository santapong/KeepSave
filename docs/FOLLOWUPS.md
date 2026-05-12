# KeepSave — Follow-ups

This is the single source of truth for tracked technical debt and deferred work. Every entry has an **owner** (a role from `docs/ROLES.md`) and a **due date**. No-owner items are not tracked here — they're either assigned or deleted.

**Cadence:** reviewed at the monthly ADR review (`docs/ROLES.md` §6). Items past due without an explanation get pulled into the next 30-day plan.

---

## Closed (v1.1.0)

- [x] **Nightly audit-log pruner wiring in `main.go`** — closed by `startAuditLogPruner` in `backend/cmd/server/main.go` + `AuditRepository.DeleteOlderThan` + table-driven tests in `backend/internal/repository/audit_repo_test.go`.
- [x] **KeepSave ↔ Seidr regression harness** — closed by `tests/e2e/seidr/` (docker-compose + stdlib-only Go tester).

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

---

## Process notes

- **Adding an item:** open a PR that adds a numbered entry under "Open — Phase A" (or Phase B). Include an owner and a due date. PR is reviewed by the named owner — if they refuse, the item goes back to its proposer or is killed.
- **Closing an item:** move it under the "Closed" section with a one-line description of the artifact that closes it (commit SHA, file path, or PR number). Don't delete — historical follow-ups are useful in post-mortems.
- **Slipping a due date:** allowed *once*, with a written reason here. Second slip triggers a re-plan of that role's 30/60/90.

Once Phase A items close, `SECURITY_AUDIT.md` v1.1.0 can be re-stamped v1.1.1 and the "Known follow-ups" block removed there.
