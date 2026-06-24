# ADR-0017: Promotion engine integrity — atomic execution, idempotent claim, complete rollback

- **Status:** Accepted
- **Date:** 2026-06-21
- **Authors:** Tech Lead
- **Reviewers required:** Tech Lead; Security Engineer (promotion engine)
- **Supersedes:** none
- **Related:** ADR-0003 (promotion engine), ADR-0007 (approver≠requester invariant), `docs/THREAT_MODEL.md` §2, `docs/research/IMPROVEMENT_PLAN_2026-06.md` (P-01..P-04, P-10, DB-08/09)

---

## Context

ADR-0003 chose decrypt-source / re-encrypt-target promotion with snapshot-based
rollback. The implementation has four integrity defects found in the 2026-06
audit:

- **P-01 — no transaction.** `executePromotion` writes each target secret, each
  snapshot, and the final status with separate connections. A failure (or crash)
  mid-loop leaves the target environment half-promoted and the request stuck in
  `pending`/`approved`, with no atomic boundary to recover to.
- **P-02 — approve TOCTOU.** `ApprovePromotion` reads `status == "pending"` then
  later writes `status = "approved"` with no conditional guard. Two approvers (or
  a double-submit) both pass the read and both execute the copy.
- **P-03 / P-04 — incomplete rollback.** Snapshots are written only for
  *overwritten* keys. Keys the promotion *added* (absent in target) have no
  snapshot, so rollback leaves them behind; rollback also never moves the request
  out of `completed`, so it can be replayed.
- **P-10 — SQLite self-approval not enforced at the DB.** The Postgres/MySQL
  `CHECK` (migration 008) has no SQLite equivalent (migration 008 is a `SELECT 1`
  no-op), so the DB-level backstop for ADR-0007 is dialect-dependent.

Constraint: the same project DEK encrypts all environments (ADR-0001/0004), so a
promotion is a same-DEK decrypt→re-encrypt. Promotion is the highest-blast-radius
routine operation; partial application or double application can take PROD down.

## Options considered

### Option A — Single transaction + conditional-claim + prior-state snapshot for every written key

Wrap the whole copy (claim, per-key snapshot, per-key upsert, terminal status)
in one `sql.Tx`. Claim the request with a conditional `UPDATE … WHERE id=? AND
status='pending'` whose `RowsAffected()==1` is the right to proceed; the loser of
a race rolls back. Snapshot the *prior* state of every key the promotion writes —
the old ciphertext when it overwrites, and a `prior_existed=false` marker when it
adds — so rollback can restore overwrites and delete additions, then move the
request to `rolled_back`.

- **Pros:** Atomic (no half-promotions). The conditional claim is a compare-and-set
  that closes the TOCTOU without advisory locks. Rollback is complete and
  single-shot (idempotent via the `completed → rolled_back` guard). Works on all
  three dialects with the existing `database/sql` transaction API.
- **Cons:** Holds a row lock for the duration of the copy (acceptable: promotions
  are short). Adds one column (`prior_existed`) and one migration.

### Option B — Keep separate statements, add `SELECT … FOR UPDATE` advisory locking only

Serialize approvers with a row lock but leave the copy non-transactional.

- **Pros:** Smaller diff; fixes the double-execute race.
- **Cons:** Does **not** fix P-01 (a mid-loop failure still half-applies) or P-03/04
  (rollback still incomplete). `FOR UPDATE` is also not portable to SQLite, which
  the test suite depends on. Rejected.

### Option C — Move promotion to a single-writer queue/worker

Serialize all promotions through one worker so concurrency is impossible.

- **Pros:** No in-DB concurrency reasoning.
- **Cons:** Large new infrastructure (queue, worker, delivery semantics) for a
  problem a transaction solves; still needs the rollback-completeness fix
  independently. Over-engineered for current scale. Rejected.

## Decision

**Option A.** Promotion executes inside one transaction; the request is claimed by
a conditional status update (`pending → completed`) whose single affected row is
the execution right; every written key gets a prior-state snapshot
(`prior_existed` true for overwrite, false for add); rollback restores overwrites,
deletes adds, and transitions `completed → rolled_back` under its own conditional
guard. The SQLite self-approval invariant is enforced by a `BEFORE UPDATE OF
approved_by` trigger at parity with the Postgres/MySQL `CHECK`.

The intermediate `completed` status set at the top of the transaction is never
externally visible: it commits only if the entire copy succeeds, so observers see
an atomic `pending → completed` (or no change on rollback). This refactor also
fixes a latent bug in the old approve path, which overwrote `approved_by` back to
`NULL` on completion — the approver is now preserved.

The transaction uses the `ExecQ`/`*Tx` repository helpers (ADR-less DB-12 fix) so
the out-of-order status update binds correctly on every dialect.

## Rejection rationale

- **Option B** leaves P-01 and P-03/04 unfixed and is not SQLite-portable.
- **Option C** is disproportionate infrastructure for a transaction-shaped problem.

## Consequences

- **Operational:** Promotion holds a per-request row lock for the copy duration;
  measured in ms for typical key counts. No new runbook steps; the kill switch
  (ADR-0003 follow-up) still applies.
- **Security:** Closes the double-execute race (a single compromised approver can
  no longer cause two PROD writes) and makes rollback actually restore PROD to its
  pre-promotion state. Updates `docs/THREAT_MODEL.md` §2.
- **Migration:** `010` adds `secret_snapshots.prior_existed` (default true, so
  existing overwrite-only snapshots remain correct). `011` adds the SQLite
  self-approval trigger (no-op parity rows on Postgres/MySQL).
- **Reversibility:** Fully reversible — the column default and the trigger can be
  dropped without data loss; no ciphertext is rewritten by this change.

## Rollback plan

If the transactional engine misbehaves: revert the service + repo commit (the
migration is additive and inert if unused — `prior_existed` defaults true,
matching the old overwrite-only snapshot semantics, and the trigger only fires on
genuine self-approval). No data migration is required to roll back.

## Open questions

- **Two-of-N approval** remains out of scope (ADR-0003). Revisit under regulatory
  pressure. *Owner: Tech Lead. Due: triage at 90 days.*
- **Snapshot retention / GC** for `rolled_back` promotions: snapshots persist for
  audit; add a retention job if volume warrants. *Owner: Backend. Due: 60 days.*

## References

- `backend/internal/service/promotion_service.go` (runPromotion, ApprovePromotion, Rollback)
- `backend/internal/repository/promotion_repo.go` (WithTx, CompareAndSetStatusTx, MarkRolledBackTx, snapshots)
- `backend/internal/repository/secret_repo.go` (GetByEnvAndKeyTx, UpsertTx, DeleteByEnvAndKeyTx)
- `backend/migrations/{postgres,sqlite,mysql}/010_promotion_rollback_completeness.sql`
- `backend/migrations/{postgres,sqlite,mysql}/011_promotion_self_approval_trigger.sql`
