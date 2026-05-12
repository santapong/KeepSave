# ADR-0004: Two-level key hierarchy — master KEK + per-project DEK

- **Status:** Accepted (backfilled)
- **Date:** 2026-05-12
- **Authors:** Tech Lead (backfilled)
- **Reviewers required:** Tech Lead; Security Engineer
- **Supersedes:** none
- **Related:** ADR-0001 (envelope encryption), ADR-0003 (promotion), `docs/THREAT_MODEL.md`, `docs/RUNBOOK.md`

---

## Context

ADR-0001 commits us to envelope encryption. This ADR fixes the *shape* of the key tree: how many levels, what each level encrypts, how keys are sourced and rotated. The shape determines our blast radius for key compromise and our operational cost for rotation.

Constraints:
- Master key must live outside the database (in env, in a KMS, in a sealed secret). Storing it in the DB defeats the point of encrypting the DB.
- Key compromise at any single level must not trivially compromise all data.
- Rotation must be possible without re-encrypting every secret (the most expensive operation).
- Source of the master key must be pluggable — different deployments use env vars, AWS KMS, GCP KMS, Vault, etc.

## Options considered

### Option A — Two levels: master KEK + per-project DEK

Single master key (32 bytes) acts as a key-encrypting key. Each project at creation generates a 32-byte DEK with `crypto/rand`, encrypts the DEK under the master key, and stores the encrypted DEK on the project row. All secrets in that project are encrypted under that DEK.

- **Pros:** Master-key rotation requires re-wrapping N DEKs (one per project), not re-encrypting every secret. Per-project isolation: leaking one project's DEK doesn't affect others. Simple and well-understood (the same shape used by AWS KMS data keys, Vault transit, etc.). Standard library plus a small `Provider` interface for the master source.
- **Cons:** Master-key compromise = all DEKs compromised = full breach. A leaked Alpha DEK exposes the project's PROD secrets too (per ADR-0003 cons).

### Option B — Three levels: master KEK + project KEK + per-environment DEK

Add a per-project KEK that re-wraps per-environment DEKs.

- **Pros:** A leaked Alpha environment DEK does not compromise PROD. Promotion has to re-wrap under a different DEK, removing a class of nonce-reuse risk (covered separately in ADR-0003).
- **Cons:** Triples the key-management surface. Three rotation paths instead of two. Significant complication for a benefit that today's threat model does not demand. Adds latency to every secret read (two unwraps instead of one).

### Option C — Per-secret DEK (one DEK per secret)

Each secret has its own randomly-generated DEK, wrapped under the project KEK.

- **Pros:** Maximum blast-radius isolation: leaking one secret's DEK exposes one secret.
- **Cons:** DEK storage scales with secret count, not project count. Read path now requires DEK unwrap per secret. Rotation requires touching every DEK row. Vastly more rows to back up and audit.

## Decision

**Option A — two-level hierarchy: master KEK + per-project DEK.** The master key is sourced via the `keyprovider.Provider` interface (current default: `EnvProvider` reading base64 from `MASTER_KEY` env var); DEKs are generated per-project at creation time, encrypted under the master key, and stored on the project row.

Implementation:
- Master key sourcing: `backend/internal/crypto/keyprovider/env.go:9-30, 40-42` (the env provider; `Rotate()` returns `ErrUnsupported`).
- Master-key plumbing into the service: `backend/cmd/server/main.go:234-252`.
- DEK generation per project: `backend/internal/service/project_service.go:90-100`.
- DEK storage: `backend/internal/models/models.go:26-27` (`Project.EncryptedDEK`, `Project.DEKNonce`).
- No KDF — both master key and DEK are 32 random bytes used directly (`backend/internal/crypto/crypto.go:25-30`).

## Rejection rationale

- **Option B** was rejected because it triples operational cost without addressing a real attacker we know of today. Reconsider via a future ADR if either (a) a customer requires per-environment isolation, or (b) we ship cross-tenant features in Phase B that change the trust model.
- **Option C** was rejected because storage and operational cost scale with secret count, which is the wrong unit; the natural blast-radius boundary in KeepSave is the project, not the secret.

## Consequences

- **Operational:** Master-key handling is the single most critical operational concern. `MASTER_KEY` must be 32 base64-decoded bytes; lost = total data loss; leaked = total compromise. Production deployments should source from a KMS, not env vars; the `EnvProvider` is acceptable for development only.
- **Rotation:** Master-key rotation is the responsibility of the underlying provider (AWS KMS, GCP KMS, Vault have native rotation). The `EnvProvider` does not support rotation (`env.go:40-42` returns `ErrUnsupported`); switching deployments to env-provider in production is therefore a footgun and must be called out in `docs/RUNBOOK.md`.
- **DEK rotation:** There is *no code path* for DEK rotation today. A leaked DEK requires either (a) re-keying the entire project (decrypt all, generate new DEK, re-encrypt all, atomic swap) or (b) accepting the leak until rotation is built. Tracked as a follow-up.
- **No KDF:** keys are used directly. This is fine for AES-GCM (which accepts 32-byte keys), but means we cannot derive purpose-bound subkeys (e.g., a separate key for audit-log MAC) without an ADR change.
- **Reversibility:** Going from two levels to three (Option B) is migrate-able but painful. Going to per-environment DEK requires a flag-day re-encryption.

## Rollback plan

If the two-level model has a discovered flaw:
- **Master-key compromise:** assume total breach; rotate master, generate fresh DEKs per project, decrypt all secrets under old DEKs, re-encrypt under new DEKs. This is the worst-case runbook and **must be drilled** (Security 60-day item).
- **Single-DEK compromise:** if rotation code does not exist (current state), the only mitigation is project re-creation (export plaintexts from a separate trusted environment, recreate). This is unacceptable as a long-term posture — build DEK rotation as a 60-day Backend item.

## Open questions

- **DEK rotation API:** build a `POST /api/v1/projects/:id/rotate-dek` that atomically re-encrypts a project. *Owner: Backend Engineer. Due: 60 days.*
- **KMS adapters (AWS, GCP) wired into `main.go`:** interface and stub files exist (`kms_aws.go`, `kms_gcp.go` per `FOLLOWUPS.md`); blocked on `go mod tidy` adding SDK deps. *Owner: DevOps + Backend. Due: 30 days.*
- **Subkey derivation for auxiliary uses** (e.g., audit-log integrity MAC distinct from secret encryption): if/when we add an audit-log MAC, write a new ADR adding HKDF or a separate provider-issued key. *Owner: Tech Lead. Due: as needed.*
- **Drill cadence:** master-key rotation table-top exercise quarterly; live in staging twice a year. Capture in `docs/RUNBOOK.md`. *Owner: Security Engineer. Due: 60 days.*

## References

- `backend/internal/crypto/keyprovider/env.go:9-30, 40-42`
- `backend/internal/crypto/crypto.go:11, 18, 25-30`
- `backend/internal/service/project_service.go:90-100`
- `backend/internal/models/models.go:26-27`
- `backend/cmd/server/main.go:234-252`
- `docs/FOLLOWUPS.md` (AWS/GCP KMS adapters)
