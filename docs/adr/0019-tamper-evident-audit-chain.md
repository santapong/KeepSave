# ADR-0019: Tamper-evident audit log via a keyed hash chain

- **Status:** Accepted (design); implementation tracked as the next Wave-3 task
- **Date:** 2026-06-21
- **Authors:** Tech Lead
- **Reviewers required:** Tech Lead; Security Engineer (audit integrity)
- **Supersedes:** none
- **Related:** ADR-0014 (audit taxonomy), ADR-0015 (safego + audit emission), ADR-0018 (sub-key derivation), `docs/AUDIT_LOG_COVERAGE.md`, `docs/research/IMPROVEMENT_PLAN_2026-06.md` (A-14)

---

## Context

The audit log is described throughout the docs as the product's load-bearing
control, yet `audit_log` rows are stored plain (A-14): no row MAC, no hash
chain, no append-only grant. An actor with DB write access — a compromised app
credential, a malicious insider, or a misused maintenance path — can **alter or
delete** audit rows with no detectable trace. A DB-level `REVOKE DELETE` helps
but collides with retention pruning (`DeleteOlderThan`), which legitimately needs
`DELETE`. A cryptographic integrity layer detects tampering even when a grant is
bypassed, and is independent of the storage engine.

Constraints:
- Must not fail the caller's primary mutation if integrity bookkeeping fails
  (audit emission is best-effort by design — see `emitAudit`).
- Must work across PostgreSQL (prod) and SQLite (tests).
- Must not require a signature key to be handled outside `internal/crypto`
  (ADR-0018) and must not break the ~6 existing `NewAuditRepository(db, dialect)`
  call sites/test harnesses.

## Options considered

### Option A — Per-row keyed hash **chain** (chosen)

Each row stores `prev_hash` (the previous row's `entry_hash`) and
`entry_hash = HMAC-SHA256(chainKey, prev_hash || canonical(row))`. The chain key
is derived from the master key (`crypto.Service.deriveSubKey("keepsave/audit-chain/v1")`)
so it never leaves `internal/crypto` and an attacker who can write rows still
cannot recompute valid hashes. A verifier walks the chain and reports the first
broken link (covering modification, deletion, reordering, and insertion).

- **How it works:** appends are serialized — within a transaction, lock/read the
  current tip (`SELECT … ORDER BY id DESC LIMIT 1 FOR UPDATE` on Postgres;
  SQLite serializes writers natively), compute `entry_hash`, insert. The chain
  key is injected via an optional `SetChainKey([]byte)` (or
  `NewAuditRepositoryWithChainKey`) so the existing `NewAuditRepository(db,
  dialect)` signature is unchanged and the chain is simply inert until a key is
  set (existing tests keep passing; prod wires the key from `main.go`).
- **Pros:** Detects the full tamper set including **deletion** (the primary audit
  threat). Key stays in `internal/crypto`. Additive migration, non-breaking
  call sites. Verifiable offline.
- **Cons:** Serializes audit appends behind one tip lock — a throughput ceiling
  (mitigated: audit volume is modest; the A-06 failure metric already lands so
  regressions are observable). Retention pruning must be reconciled with the
  chain (see Consequences).

### Option B — Per-row MAC, no linkage

Store only `entry_hash = HMAC(chainKey, canonical(row))`.

- **Pros:** No serialization, trivially concurrent, simpler.
- **Cons:** Detects content modification but **not deletion or reordering** —
  exactly what an attacker silencing the log would do. Insufficient for the
  threat that motivates this ADR. Rejected.

### Option C — External append-only / WORM sink

Ship audit events to an immutable external store (e.g. an object-lock bucket or
a managed audit service).

- **Pros:** Strongest durability/immutability; off-box.
- **Cons:** New trust boundary, new dependency, delivery-guarantee complexity,
  and cost — disproportionate for the current stage. Revisit when a compliance
  customer requires it. Rejected for now.

## Decision

**Option A** — a master-key-derived, per-row keyed hash chain with serialized
appends and an offline verifier, keyed via an optional setter so existing call
sites are untouched and the chain is inert until prod configures the key.

Canonical row serialization is a fixed field order
(`user_id|project_id|action|environment|details|ip_address|created_at`) with a
stable encoding, hashed together with `prev_hash`. The genesis row uses an empty
`prev_hash`. The chain has an **epoch**: it begins at the first row written after
the key is configured; pre-existing rows carry NULL hashes and are outside the
verified range (documented, not retroactively forged).

## Rejection rationale

- **Option B** can't detect the deletion/reordering that defines log tampering.
- **Option C** introduces an external trust boundary and delivery semantics we
  don't need yet.

## Consequences

- **Operational:** audit appends serialize on the chain tip; monitor with the
  A-06 counters. Add a `verify-audit-chain` maintenance command/endpoint (admin-only).
- **Security:** tamper-evidence independent of DB grants; updates
  `docs/THREAT_MODEL.md` (repudiation/tamper boundary). The chain key is a new
  piece of key material under the master key (no new root secret).
- **Retention tension (important):** `DeleteOlderThan` breaks a naive chain
  (deleting old rows orphans `prev_hash`). Resolution: pruning becomes a
  **checkpoint** operation — before deleting range `[..cutoff]`, record a signed
  checkpoint row capturing the last pruned `entry_hash`, and re-anchor the next
  live row's `prev_hash` to that checkpoint. The verifier treats checkpoints as
  valid chain joins. (Until implemented, run pruning only with the chain
  disabled, or not at all.)
- **Migration:** additive — `prev_hash`/`entry_hash` nullable columns on
  `audit_log` for all three dialects. No backfill (epoch start).
- **Reversibility:** drop the key (chain goes inert) or drop the columns; rows
  remain fully logged either way.

## Rollback plan

If serialized appends regress throughput unacceptably: unset the chain key —
appends revert to plain inserts (still logged), and the verifier simply covers a
closed epoch. No data is lost; this is a config flip, not a migration.

## Open questions

- **Global vs per-project chain.** Global (one tip) is chosen for simplicity and
  strong total ordering; per-project would parallelize appends but complicates
  cross-table deletion detection and the verifier. Revisit if audit throughput
  becomes a bottleneck. *Owner: Backend. Due: with implementation.*
- **Checkpoint format + signing** for the retention reconciliation above.
  *Owner: Backend/Security. Due: with implementation.*
- **KMS-backed chain key** once ADR-0012 auto-unseal is wired. *Owner: Crypto.*

## References

- `backend/internal/repository/audit_repo.go` (Create, DeleteOlderThan — chain hooks)
- `backend/internal/crypto/crypto.go` (deriveSubKey — chain key)
- `backend/internal/service/audit_helper.go` (emitAudit — best-effort contract)
- `docs/AUDIT_LOG_COVERAGE.md`, `docs/THREAT_MODEL.md`
