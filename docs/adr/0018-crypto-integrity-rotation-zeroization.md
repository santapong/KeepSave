# ADR-0018: Crypto integrity — atomic DEK rotation, service-secret sub-keys, key-material zeroization

- **Status:** Accepted
- **Date:** 2026-06-21
- **Authors:** Tech Lead
- **Reviewers required:** Tech Lead; Security Engineer (internal/crypto)
- **Supersedes:** none
- **Related:** ADR-0001 (envelope encryption), ADR-0004 (key hierarchy), `docs/research/IMPROVEMENT_PLAN_2026-06.md` (C-01, C-02, H-04)

---

## Context

Three crypto-layer defects from the 2026-06 audit:

- **C-01 — DEK rotation is non-atomic.** `RotateProjectKey` re-encrypts each
  secret with a separate `UPDATE` and writes the new project DEK *last*, with no
  transaction. A crash or error mid-loop leaves some secrets under the new DEK,
  some under the old, and the project DEK in either state — an unrecoverable
  split where no single key decrypts the whole project.
- **C-02 — the raw master key escapes the crypto package.** `GetMasterKey()` is
  called by `sso_service.go` to encrypt SSO client secrets and backup blobs
  *directly* under the KEK, bypassing the envelope. The raw root key is handled
  outside `internal/crypto`, and those blobs are bound to the raw KEK rather than
  a domain-separated key.
- **H-04 — no key-material zeroization.** Decrypted DEKs and plaintext secret
  values linger in heap memory for the GC to eventually reclaim; nothing wipes
  them after use.

Constraint: production runs PostgreSQL; rotation must stay correct on all three
dialects. The DB-12 fix (ExecQ) already made the underlying `UPDATE`s actually
persist on MySQL/SQLite — rotation was previously a silent no-op there.

## Options considered

### Option A — Transaction around rotation; HMAC-derived service sub-key; best-effort zeroization

Wrap the entire re-encrypt loop **and** the project-DEK swap in one transaction
(`ProjectRepository.WithTx` + `UpdateValueTx`/`UpdateDEKTx`). Encrypt service
secrets under a sub-key derived from the master key via HMAC-SHA256 with a
domain-separation label (`EncryptServiceSecret`/`DecryptServiceSecret`), with a
legacy fallback that still decrypts blobs written under the raw master key;
unexport `GetMasterKey`. Add `crypto.SecureZero` and wipe decrypted DEKs and
plaintext values after use in rotation and promotion.

- **Pros:** Rotation is all-or-nothing — a failure rolls back to the consistent
  pre-rotation state. The master key never leaves `internal/crypto`; service
  secrets are domain-separated and the cutover is non-breaking (fallback).
  Zeroization shortens key-material lifetime at near-zero cost.
- **Cons:** Holds a transaction for the duration of a project's rotation (bounded
  by key count; acceptable). `SecureZero` is best-effort under a managed runtime
  (the GC may have copied a value before wiping) — it reduces, not eliminates,
  exposure.

### Option B — Dual-DEK read window (per-secret key version)

Tag each secret with which DEK version encrypts it; rotation writes new rows then
flips a pointer. Avoids a long transaction.

- **Pros:** No long lock; resumable.
- **Cons:** Schema change (key-version column), a dual-key decrypt path
  everywhere, and GC of old versions. Large surface for a problem a transaction
  solves at current scale. Rejected.

### Option C — Wrap service secrets under a per-org DEK

Give SSO/backup their own per-org envelope instead of a derived sub-key.

- **Pros:** Symmetric with project DEKs.
- **Cons:** SSO config exists before any project/DEK; introduces a new key
  hierarchy and rotation surface for a handful of blobs. Rejected as
  disproportionate.

## Decision

**Option A.** Rotation runs in one transaction; service secrets move to an
HMAC-SHA256-derived sub-key with a legacy decrypt fallback; `GetMasterKey` is
unexported; `SecureZero` wipes DEKs and plaintext after use.

The derived key is `HMAC-SHA256(masterKey, "keepsave/service-secret/v1")` — a
standard one-step KDF for a single fixed-length key that keeps the root key
internal and domain-separates service secrets from DEK wrapping. We accept
`SecureZero` as best-effort: under Go's managed heap it cannot guarantee no copy
survives, but it meaningfully shortens the window and documents intent.

## Rejection rationale

- **Option B** adds a schema + dual-key decrypt path the current scale doesn't need.
- **Option C** stands up a whole key hierarchy for a few service blobs.

## Consequences

- **Operational:** Rotation latency is one transaction per project (was N
  autocommits). No new runbook steps.
- **Security:** Master key confined to `internal/crypto`; service secrets
  domain-separated; rotation can no longer leave a project half-encrypted.
  Updates `docs/THREAT_MODEL.md` (key-handling boundary).
- **Migration:** None required. New service secrets use the derived key;
  pre-existing blobs decrypt via the fallback and are upgraded on next write.
- **Reversibility:** The sub-key derivation and zeroization are reversible in
  code. The legacy fallback means no data is stranded if reverted.

## Rollback plan

Revert the commit. The service-secret fallback still reads any blob written under
either scheme, so no SSO/backup data is stranded. Rotation reverts to the
(now DB-12-correct) per-statement form — still correct, just non-atomic — until
re-applied.

## Open questions

- **Nonce-collision monitoring (FOLLOWUPS #7):** add a per-DEK encryption counter
  + birthday-bound alert. Tracked separately; not required for this ADR.
  *Owner: Backend/Crypto. Due: 60 days.*
- **KMS-backed service-secret keys:** when KMS auto-unseal (ADR-0012) is wired,
  consider deriving the service sub-key inside the KMS. *Owner: Crypto. Due: with INF-4.*

## References

- `backend/internal/crypto/crypto.go` (SecureZero, EncryptServiceSecret, DecryptServiceSecret)
- `backend/internal/service/keyrotation_service.go` (transactional RotateProjectKey)
- `backend/internal/service/sso_service.go` (service-secret usage)
- `backend/internal/repository/project_repo.go`, `secret_repo.go` (Tx variants)
