# ADR-0007: Enforce approver ≠ requester at the database layer for promotion approvals

- **Status:** Proposed
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (Backend Engineer role)
- **Reviewers required:** Tech Lead; Security Engineer (mandatory veto — touches promotion engine, per `docs/ROLES.md` §2.2 and ADR-0003 §Open Questions)
- **Supersedes:** none
- **Related:** ADR-0003 (promotion engine), `docs/THREAT_MODEL.md` §3 row E (line 100), `docs/audits/SECURITY_AUDIT_2026-05-15.md` A04-F1, `docs/FOLLOWUPS.md` 0d (lines 48-52), `docs/research/competitors/github.md` Candidate 1

---

## Context

The promotion engine exists to enforce multi-party control over PROD changes. ADR-0003 §74 commits us to "one approver other than the requester" as the binding invariant, and §Open Questions (line 90) flagged that the enforcement layer was never verified. The 30-day audit verified it: **neither layer enforces it.**

From `docs/audits/SECURITY_AUDIT_2026-05-15.md` finding A04-F1 (P2 High, OWASP A04 Insecure Design):

> *"Approver = requester not enforced — `backend/internal/service/promotion_service.go:215-240` `ApprovePromotion(promotionID, approverID)` updates status without checking `promotion.RequestedBy != approverID`. Confirms the existing FOLLOWUPS #5 / #0d. Repro: Single compromised account can request a PROD promotion (`promotion_service.go:181`) and then approve it themselves. Fix: `if promotion.RequestedBy == approverID { return error }` at the top of `ApprovePromotion`. Add a DB-level CHECK constraint as defense-in-depth."*

Direct reading of `promotion_service.go:214-240` confirms it: `ApprovePromotion` loads the row, checks only `Status == "pending"`, then calls `UpdateStatus(promotionID, "approved", &approverID)`. `promotion.RequestedBy` is never read; `approverID` is never compared. `promotion_repo.go:35-39, 119` show the schema already carries both columns, so the data exists — only the check is missing.

The threat model named this gap. `docs/THREAT_MODEL.md` §3 row E (line 100):

> *"E | Requester self-approves | Invariant not currently enforced at DB layer (gap) | open follow-up | Medium"*

FU 0d (verbatim, `docs/FOLLOWUPS.md:48-52`):

> *"### 0d. Approver-cannot-be-requester invariant unverified  · **Status:** ADR-0003 §Open Questions calls it out. Not yet verified whether enforced at DB or only service code.  · **Why it matters:** The multi-party-control linchpin for PROD promotions.  · **Owner:** Backend Engineer + Security Engineer.  · **Due:** 30 days."*

The GitHub Environments dossier (`docs/research/competitors/github.md` Candidate 1, accepted 2026-05-15) names the pattern:

> *"Approver ≠ requester DB-level invariant — `CHECK (approved_by IS NULL OR approved_by <> requested_by)` on `promotion_requests` + service-layer 422 guard in `ApprovePromotion` before touching DB. Closes FU 0d. Pattern source: Environments' `cannot_approve_own_deployment`."*

Constraints: (1) audit-log emission per `CLAUDE.md` Coding Conventions; (2) PG/SQLite parity per `docs/SECRET_SOURCES.md`; (3) no destruction of legacy rows; (4) ADR-0003's load-bearing claim cannot silently widen. Trust boundary **narrows** — a compromised `promoter` can no longer both request and approve. No new entity, no new sub-system.

## Options considered

### Option A — DB CHECK constraint + service-layer guard (defense in depth)

Add a named CHECK constraint on `promotion_requests` — `CHECK (approved_by IS NULL OR approved_by <> requested_by)` — via `ALTER TABLE ... ADD CONSTRAINT ... NOT VALID`, then `VALIDATE CONSTRAINT` after an audit step confirms no legacy row violates it. Plus a service-layer guard at the top of `ApprovePromotion` that returns a typed 422 error before any DB round-trip, emitting a `promotion.approve_rejected` audit event.

- **Pros:** Two independent layers. The DB constraint cannot be bypassed by future code paths that forget the guard. The service guard short-circuits before the DB call, giving a typed error and an audit row. Matches the audit's §6 row 6 prescription verbatim and the GitHub Environments pattern.
- **Cons:** Two layers to keep consistent if the invariant changes. Migration story for any existing self-approved rows.

### Option B — Service-layer guard only

Add `if promotion.RequestedBy == approverID { return httperror.UnprocessableEntity(...) }` at the top of `ApprovePromotion` and stop there.

- **Pros:** Single source of truth; no migration.
- **Cons:** No backstop. A future second approval path (admin override, automation) that forgets the guard silently loses the invariant. No protection against direct DB writes (operator scripts, edited backup restores, compromised service account). The audit's "Insecure Design (A04)" framing specifically recommends DB-level invariants for this class.

### Option C — External policy engine (Vault Sentinel pattern)

Externalize approval policy to an OPA/Sentinel-style evaluator called from the promotion service.

- **Pros:** Generalizes to N-of-M, time windows, role pools — the Phase B work ADR-0003 §74 defers.
- **Cons:** Heavyweight for one rule. New sub-system in the trust boundary. Vault dossier itself verdicts this pattern `adopt-when-trigger-fires` (Phase B).

## Decision

**Option A — DB CHECK constraint + service-layer 422 guard.**

The audit explicitly prescribes defense in depth ("Service-layer check + DB CHECK constraint", `SECURITY_AUDIT_2026-05-15.md` §6 row 6); the cost over Option B is one migration plus four lines; and the DB constraint is the only layer that protects against future code paths that don't yet exist. ADR-0003's load-bearing claim becomes physically enforced rather than aspirational.

Trigger to re-decide: if a future approval policy must allow `requester ∈ approvers` (e.g., as one of N independent approvers), Option C replaces this ADR and the CHECK constraint is dropped.

## Rejection rationale

- **Option B** loses because a service-only guard re-creates the exact failure mode the audit just found. The next person to add an approval path (Phase B reviewer pool, admin override, automated approver) silently bypasses it. The cost of Option A is one migration.
- **Option C** loses for Phase A because one invariant doesn't justify a policy engine; Vault dossier Candidate 1 already deferred this pattern.

## Consequences

- **Operational:** New migration `009_approver_not_requester_check.sql`. Service guard adds <100ns per approve. New audit event `promotion.approve_rejected` added to `docs/AUDIT_LOG_COVERAGE.md`. Runbook gains one line: "If `promotion.approve_rejected` events spike, investigate org-membership compromise."
- **Security:** Narrows `THREAT_MODEL.md` §3 row E from Medium → Low. No new trust boundary; no new entity. Threat-model row E must be updated in the same PR per `CLAUDE.md` governance.
- **Migration:** Existing rows with `approved_by = requested_by` are not destroyed (constraint added `NOT VALID`); a one-shot SELECT in the migration emits `promotion.legacy_self_approved` audit rows for Security Engineer review. `VALIDATE CONSTRAINT` runs in a follow-up migration after triage.
- **Reversibility:** Easy — one `DROP CONSTRAINT`, four-line service revert, no data loss, no re-encryption.
- **Dev-workflow break (intended):** self-request + self-approve fails with 422. Local dev uses two seeded users; documented in `docs/RUNBOOK.md`.

## Rollback plan

1. `ALTER TABLE promotion_requests DROP CONSTRAINT approver_not_requester;` — single statement, no data loss.
2. Revert the 4-line service-layer guard in `promotion_service.go`.
3. Remove `promotion.approve_rejected` from `docs/AUDIT_LOG_COVERAGE.md` (historical rows stay in the table).
4. Revert added tests in `promotion_service_test.go` and `tests/NEGATIVE_AUTH_PLAN.md`.

Single revert PR. No coordinated multi-step rollback.

## Open questions

- **(a) Retroactive validation:** does the CHECK constraint apply retroactively (`NOT VALID` then `VALIDATE` after audit of existing rows) or only to new rows? The plan is audit-then-validate; if any customer DB contains pre-ADR self-approved rows they must be rejected, archived to `legacy_promotions`, or grandfathered. *Owner: Security Engineer. Due: before `VALIDATE` runs in prod (within 60 days of constraint deployment).*
- **(b) Error response shape:** is 422 (Unprocessable Entity) right, or should the service guard return 403 (Forbidden)? 422 frames it as a business-rule violation; 403 as authorization refusal. The GitHub Environments precedent (dossier §4) is `422 cannot approve own deployment` — we follow it pending Security Engineer confirmation that 422 doesn't leak less-actionable info than 403. *Owner: Security Engineer. Due: ADR sign-off.*
- **(c) System-actor edge case:** if `requested_by` is a system actor (AI agent, scheduled-job user) and the human approver has a distinct `user_id`, the constraint is vacuously satisfied — yet the *spirit* (two independent humans) may not be. Do we need an `actor_type` column, or a rule that system actors cannot request PROD promotions? *Owner: Tech Lead + Security Engineer. Due: 60 days, pending whether agent-initiated promotions emerge as a use case.*

## Implementation plan

| # | File | Detail |
|---|---|---|
| 1 | `backend/migrations/009_approver_not_requester_check.sql` (next-available; verified via `ls backend/migrations/` showing latest `008_phase15_quotas.sql`) | `ALTER TABLE promotion_requests ADD CONSTRAINT approver_not_requester CHECK (approved_by IS NULL OR approved_by <> requested_by) NOT VALID;` plus a `SELECT id, project_id, requested_by FROM promotion_requests WHERE approved_by = requested_by` step emitting `promotion.legacy_self_approved` rows. Constraint name is dialect-portable across PG/SQLite (dossier §Security Reviewer notes bullet 1). |
| 2 | `backend/internal/service/promotion_service.go:214-240` | After line 220 (status check), insert: `if promotion.RequestedBy == approverID { s.audit.Emit("promotion.approve_rejected", ..., map[string]any{"reason":"approver_equals_requester"}); return nil, httperror.UnprocessableEntity("approver cannot be requester") }`. Audit per `CLAUDE.md`; error shape per `docs/ERROR_HANDLING_STANDARD.md`. |
| 3 | `docs/AUDIT_LOG_COVERAGE.md` | Add `promotion.approve_rejected` with reason `approver_equals_requester`. |
| 4 | `backend/internal/service/promotion_service_test.go` | Table-driven: (a) approver==requester → 422 + audit row + DB unchanged; (b) approver!=requester → success preserved; (c) legacy self-approved row pre-migration → `promotion.legacy_self_approved` emitted, constraint stays `NOT VALID`; (d) approver `nil` (rejection path) → constraint vacuously satisfied. |
| 5 | `tests/NEGATIVE_AUTH_PLAN.md` | New row: "promotion approval where `approverID == requestedBy` → 422; audit row written; status unchanged". |
| 6 | `docs/THREAT_MODEL.md` §3 row E (line 100) | Update mitigation to "DB CHECK `approver_not_requester` + service-layer 422 guard (ADR-0007)"; residual Medium → Low. |
| 7 | `docs/FOLLOWUPS.md` 0d | Mark **CLOSED BY ADR-0007**. |

## References

- `backend/internal/service/promotion_service.go:214-240` — missing-check site
- `backend/internal/repository/promotion_repo.go:35-39, 50-51, 119` — schema columns
- `backend/migrations/002_promotion_tables.sql:15-30` — original `promotion_requests` schema
- `backend/migrations/008_phase15_quotas.sql` — latest; next is `009`
- `docs/adr/0003-promotion-engine.md` §74, §90
- `docs/THREAT_MODEL.md` §3 row E (line 100)
- `docs/audits/SECURITY_AUDIT_2026-05-15.md` A04-F1, §6 row 6
- `docs/FOLLOWUPS.md` 0d (lines 48-52)
- `docs/research/competitors/github.md` Candidate 1 (§6, §10, §12)
- `docs/AUDIT_LOG_COVERAGE.md` — to be amended
- `docs/ERROR_HANDLING_STANDARD.md` — `httperror` package
- OWASP ASVS v4.0.3 V14.1 (separation of duties)
