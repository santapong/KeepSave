# ADR-0003: Promotion engine — decrypt-and-rewrap, with PROD approval gate

- **Status:** Accepted (backfilled)
- **Date:** 2026-05-12
- **Authors:** Tech Lead (backfilled)
- **Reviewers required:** Tech Lead; Security Engineer
- **Supersedes:** none
- **Related:** ADR-0001 (envelope encryption), ADR-0004 (key hierarchy), `docs/THREAT_MODEL.md`

---

## Context

Secrets must move from Alpha → UAT → PROD without (a) leaking in flight, (b) being silently overwritten, or (c) being promoted by someone who shouldn't. Promotion is the highest-blast-radius routine operation in the system: a botched promotion in PROD can take a customer offline.

Constraints:
- The same project DEK encrypts all environments of that project (per ADR-0001 / ADR-0004). Cross-environment secrets are *all* under one envelope.
- Audit is non-negotiable — every promotion event must be reconstructable from the audit log.
- Rollback must be possible without prior planning (someone realizes mid-incident they need to undo).
- PROD must be promotable only with explicit approval; non-PROD must not be.

## Options considered

### Option A — Decrypt source, re-encrypt target with fresh nonce, write through; PROD requires approval

For each promoted key: decrypt under project DEK, generate a new random nonce, re-encrypt under the same project DEK, write to the target environment. Snapshot the previous target value (if any) for rollback. Audit the full operation. PROD promotions create a pending record requiring a second user to approve before execution; non-PROD promotions execute immediately.

- **Pros:** Each target ciphertext has its own nonce — no nonce reuse across environments even though the DEK is shared. Approval gate scoped to PROD only matches risk. Snapshot enables rollback without prior coordination.
- **Cons:** Plaintext exists in process memory for the duration of the promotion (decrypt → re-encrypt). Two encryptions per promoted key (decryption + re-encryption) instead of one.

### Option B — Copy ciphertext verbatim

Since the DEK is shared across environments, the source ciphertext + nonce already decrypts correctly in any environment. Just copy the bytes.

- **Pros:** No plaintext in memory during promotion. Strictly faster (one DB write, no crypto ops).
- **Cons:** The source nonce is now also a target nonce. If the same key is promoted twice (idempotent re-promotion), we'd be reusing a nonce under the same DEK with the same plaintext — collision is fine in that case, but the moment plaintext differs (e.g., the source was edited, then promoted) we have a (nonce, key) reuse with different plaintexts, which **catastrophically breaks GCM confidentiality and authenticity.** This is exactly the GCM footgun ADR-0001 acknowledged.
- **Cons:** Tightly couples the on-disk format across environments. A future per-environment DEK (ADR-0004 follow-up) would force a flag-day re-encryption.

### Option C — Per-environment DEK; promotion truly re-wraps under a different key

Each (project, environment) gets its own DEK. Promotion = decrypt under source DEK, encrypt under target DEK.

- **Pros:** Cleanest blast-radius isolation: a leaked Alpha DEK doesn't affect PROD. No shared-DEK nonce concerns across environments.
- **Cons:** Triples the DEK count and the key-management surface. Rotation has to handle three keys per project. Significant migration from the current model (today every project has one DEK).

## Decision

**Option A — decrypt source, re-encrypt target with fresh nonce, with a PROD-only approval gate and a snapshot-based rollback.**

Implementation: `backend/internal/service/promotion_service.go`:
- Fetch and decrypt project DEK (lines 272-291).
- List secrets in source environment (lines 293-296).
- Apply override policy (skip vs. overwrite, lines 319-322).
- Snapshot previous target value if overwriting (lines 325-330).
- Decrypt source value, re-encrypt with new random nonce under the same DEK (lines 332-341).
- Upsert into target (lines 343-346).
- Update promotion status (lines 352-354).
- Audit (lines 357-364).

PROD approval gate: `promotion_service.go:186-195` — PROD promotions are persisted as `pending` with a `promotion_requested` audit event; a separate `POST /api/v1/projects/:id/promotions/:promotionId/approve` (`handlers_promotion.go:125-142`) executes the promotion. Non-PROD promotions execute inline.

Audit events: `promotion_requested` (PROD only), `promotion_completed` (with `promoted_keys`, `skipped_keys`, `override_policy`), `promotion_rollback` (with `restored_keys`).

## Rejection rationale

- **Option B** was rejected because verbatim ciphertext copying creates a nonce-reuse class of bug as soon as a source secret is edited between promotions — and that *will* happen in practice. The "performance" win is dwarfed by the security cliff.
- **Option C** was rejected for now because it triples key management without delivering a benefit our current threat model demands. Reconsider via a future ADR if (a) we get a customer who requires per-environment key isolation, or (b) a Phase B multi-tenant model needs it anyway.

## Consequences

- **Operational:** Promotion latency scales linearly with the number of keys (one decrypt + one encrypt per key). For projects with thousands of keys, this is still <1s but should be measured and tracked (Backend 60-day baseline action).
- **Security:** Plaintext exists in process memory during promotion. We accept this; the alternative (Option B) is worse.
- **Audit:** Three distinct event types make promotion history reconstructable. The `metadata` JSON on `audit_log` carries the promoted/skipped key list so a reader can know *what* changed, not just *that* something changed.
- **PROD approval is one approver other than the requester.** Two-of-N approval is not implemented; revisit if regulatory pressure appears.
- **Reversibility:** Promotion is reversible by `promotion_rollback` (restores the snapshotted previous target values). Approval gating is reversible — we can lower or raise the bar in a follow-up ADR without code archaeology.

## Rollback plan

If the promotion engine has a bug:
1. **Disable PROD promotion endpoint immediately** via feature flag or 503 short-circuit in middleware. (Today: no flag exists; add as 30-day follow-up.)
2. Manual rollback via `promotion_rollback` for any in-flight or recently-completed promotions.
3. Patch and redeploy.

If a promotion was performed incorrectly (correct code, bad input):
- Use the existing `promotion_rollback` flow to restore snapshots. Already implemented.

## Open questions

- **Feature flag for promotion:** today there is no kill switch. Add a runtime flag that disables `/promote` and `/approve` endpoints without redeploy. *Owner: Backend Engineer. Due: 30 days.*
- **Approver-cannot-be-requester invariant:** is this currently enforced at the DB or only in service code? Verify and add a test. *Owner: Backend Engineer. Due: 30 days.*
- **Performance baseline:** measure p50/p95/p99 for N=10/100/1000 keys; record in `docs/PERF_BASELINE.md`. *Owner: Backend Engineer. Due: 60 days.*
- **PROD definition:** today "PROD" is matched by environment name. If we add per-project environment configuration, the "is this PROD?" check must move into config, not a string compare. *Owner: Tech Lead. Due: triage at 60 days.*

## References

- `backend/internal/api/handlers_promotion.go:21-58, 125-142`
- `backend/internal/service/promotion_service.go:186-195, 272-291, 293-346, 357-364, 399-404`
- `backend/internal/models/models.go:63-72` (audit_log fields)
