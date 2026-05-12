# ADR-0001: Envelope encryption with AES-256-GCM

- **Status:** Accepted (backfilled)
- **Date:** 2026-05-12
- **Authors:** Tech Lead (backfilled)
- **Reviewers required:** Tech Lead; Security Engineer
- **Supersedes:** none
- **Related:** ADR-0004 (key hierarchy), `docs/THREAT_MODEL.md`

---

## Context

KeepSave stores other people's secrets. The data-at-rest threat model assumes the database can be exfiltrated (backup compromise, insider, cloud-provider misconfiguration). Plaintext-at-rest is therefore not an option. We need an AEAD scheme that:

- Is implementable from the Go standard library only (no third-party crypto deps; the standard library is already a large enough attack surface).
- Provides authenticated encryption — ciphertext tampering must fail to decrypt, not silently return garbage.
- Allows per-project blast-radius isolation: a leaked or mis-shared envelope must not give access to all secrets across all projects.
- Survives master-key rotation in a future ADR without rewriting every ciphertext.

## Options considered

### Option A — AES-256-GCM with a two-level envelope

Master key (32 bytes) encrypts a per-project Data Encryption Key (DEK, 32 bytes). The DEK encrypts each individual secret value. Nonces are 96-bit random per encryption operation. Master key comes from an environment variable or KMS provider; DEK is generated at project creation and stored encrypted alongside the project row.

- **Pros:** Standard-library only (`crypto/aes`, `crypto/cipher`, `crypto/rand`). AEAD properties from GCM. Per-project key gives blast-radius isolation without per-secret key sprawl. Two levels makes master-key rotation tractable (re-wrap DEKs, leave secrets alone).
- **Cons:** Random nonces over many encryptions under the same DEK approach the GCM birthday bound (~2^32 messages per key before collision risk becomes meaningful). For our scale (per-project DEK, thousands of secrets max per project) this is many orders of magnitude away from the danger zone.

### Option B — AES-256-CBC + HMAC-SHA256 (Encrypt-then-MAC)

Construct AEAD manually: encrypt with CBC mode, authenticate with a separate HMAC key derived from the master.

- **Pros:** No GCM birthday-bound concern. CBC is well-studied.
- **Cons:** Two algorithms means two ways to get it wrong (padding oracle if MAC is checked after decrypt, wrong order, key reuse between MAC and cipher). Hand-rolled AEAD is a historical source of CVEs in other projects. We gain nothing GCM doesn't already give us.

### Option C — XChaCha20-Poly1305

Modern AEAD with a 192-bit nonce, eliminating any practical nonce-collision concern.

- **Pros:** Larger nonce space removes the birthday bound. Generally easier to use safely than GCM.
- **Cons:** Not in Go's standard library (`golang.org/x/crypto` is a separate module). Adds a third-party dependency for the most security-critical code path. Performance and audit-tool ecosystem (e.g., FIPS-mode constraints) is weaker. Customers in regulated industries may require AES specifically.

## Decision

**AES-256-GCM with a two-level envelope (Option A).** Master key holds a key-encrypting key (KEK); each project owns a randomly-generated DEK encrypted under the master key. Each secret value is encrypted under its project's DEK with a fresh 96-bit random nonce.

Implementation: `backend/internal/crypto/crypto.go:11-30` (key/DEK setup, both 32 bytes), `crypto.go:64-72` (encrypt with random nonce from `crypto/rand.Reader`). Master key sourced via `keyprovider.Provider` interface, current default `EnvProvider` (`backend/internal/crypto/keyprovider/env.go:9-30`) reading base64 from `MASTER_KEY` env var.

## Rejection rationale

- **Option B** was rejected because manual AEAD construction is a known CVE generator and offered no benefit GCM does not already provide at our scale.
- **Option C** was rejected for being outside the standard library — for the most security-critical code path we accept GCM's slightly tighter nonce budget in exchange for fewer dependencies. If we ever cross 10^7 secrets per project, this ADR is re-opened.

## Consequences

- **Operational:** `MASTER_KEY` must be 32 bytes (base64) and managed outside the database (env var, KMS, sealed secret). Loss = total data loss. Backup of the master key is part of the deployment runbook.
- **Security:** Per-project DEK means leaking one project's ciphertext + DEK does not affect other projects. Master-key compromise = total compromise; we accept that, mitigated via key-source choices in ADR-0004 follow-ups.
- **Migration:** Ciphertexts are not algorithm-tagged on disk (`encrypted_value`, `value_nonce` are raw `BYTEA` per `backend/migrations/001_initial_schema.sql`). A future scheme migration will require either a version byte prefix (additive migration) or a separate column.
- **Reversibility:** Hard. Once secrets are encrypted under GCM-with-this-envelope, changing the scheme requires decrypting and re-encrypting every secret. The forward path is the only path.

## Rollback plan

True rollback is not possible: we cannot recover secrets if the master key is lost. **Forward-fix only.** If a vulnerability is found in the scheme:

1. Add a version byte prefix to new ciphertexts via additive migration.
2. Re-encrypt existing ciphertexts in a background job, project by project.
3. Both versions readable during transition; new writes only in the new version.
4. After verification, schema migration removes the version-1 column.

## Open questions

- **Nonce-collision monitoring:** at the current implementation we do not count encryptions per DEK. Add a counter and alert when any DEK crosses 2^28 encryptions (well below the 2^32 safety boundary). *Owner: Backend Engineer. Due: end of Phase A.*
- **FIPS mode:** do any current or near-term customers require FIPS 140-2 / 140-3 validated crypto? If yes, we must use `crypto/aes`'s FIPS-compliant code path or a vetted FIPS module. *Owner: Product Manager (interim Tech Lead). Due: 30 days.*

## References

- `backend/internal/crypto/crypto.go:11, 18, 25-30, 64-72`
- `backend/internal/crypto/keyprovider/env.go:9-30`
- `backend/internal/models/models.go:26-27` (Project.EncryptedDEK, Project.DEKNonce)
- `backend/migrations/001_initial_schema.sql` (storage shape)
- NIST SP 800-38D (GCM)
