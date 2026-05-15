# ADR-0008: RS256 JWT signing with JWKS + `kid` rotation

- **Status:** Proposed
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (with Self-Hosted Auth Analyst dossier input)
- **Reviewers required:** Tech Lead; Security Engineer (Type-1, auth + crypto-adjacent — veto applies per `docs/ROLES.md` §3)
- **Supersedes:** none
- **Related:** ADR-0001 (envelope encryption), ADR-0002 (auth model — extended by this ADR), ADR-0004 (key hierarchy — reuses master KEK), `docs/THREAT_MODEL.md` §2 rows S+T, `docs/research/competitors/ory.md` §10 Cand. 1, `docs/research/PATTERN_MATRIX.md` Row 1, `docs/audits/SECURITY_AUDIT_2026-05-15.md` §A07-F4

---

## Context

KeepSave's JWT path today is HS256 with a single shared `JWT_SECRET` (`backend/internal/auth/auth.go:11-62`: signing `:39`, validation `:47-62`). The OIDC discovery document at `backend/internal/api/handlers_oauth.go:233` advertises a `jwks_uri`, but the JWKS endpoint at `:216-218` returns `{"keys": [], "note": "...RS256 support is planned."}`. The Ory dossier (`docs/research/competitors/ory.md` §10 Cand. 1) classifies this as a live P0; Pattern Matrix Row 1 (`docs/research/PATTERN_MATRIX.md`) recommends **adopt-now** for RS256 + JWKS with `kid` rotation.

HS256 has two operational consequences. Rotation requires a synchronous `JWT_SECRET` change across every replica with no overlap window — all live sessions die at cutover. The shared secret also cannot be safely distributed to external verifiers (Phase B SSO under `docs/ROADMAP_NOT.md` §2, future MCP gateway) because anyone who can verify a token can forge one. The discovery document already misrepresents JWKS availability.

This ADR extends ADR-0002. The two-primitive model (JWT for humans, API keys for agents) is unchanged; only signing material changes. STRIDE row S "Forged JWT" (`docs/THREAT_MODEL.md:84`, residual Low) stays Low — private key remains server-only; no new entity enters the trust boundary (boundary **narrows**, since external verifiers no longer need the signing secret). Row T "Token replay after revocation" (`:86`, residual Medium) is **not** addressed here; that remains the trigger for the future Phase B refresh/denylist ADR (Pattern Matrix Row 2). The audit's separately-recommended `§2/E` "JWT lacks project bind" finding is **orthogonal** and belongs to ADR-0005's project-access middleware — this ADR does not touch claim scope.

## Options considered

### Option A — RS256 with `kid`-keyed JWKS rotation (recommended)

Generate RSA-2048 on first startup, persist in a new `jwt_keys` table with a `kid`, encrypt the private key under the existing master KEK (ADR-0001/0004), serve public keys via JWKS, and support multiple simultaneously-active keys for overlap. One row is `signing` (mints tokens); zero-or-more are `verifying` (validate older tokens only); `retired` keys are dropped from JWKS.

- **Pros:** Closes the empty-JWKS gap that `OpenIDConfiguration` already advertises. Private key never leaves the process; external verifiers verify without signing material. Rotation has an overlap window. Hydra is OpenID-certified for this exact shape (`ory.md` §4). Reuses ADR-0001 envelope encryption — no new custody primitive.
- **Cons:** Two algorithms must be accepted during cutover (HS256 + RS256). Algorithm-confusion is a known CVE class — mitigated by explicit `alg`-per-`kid` allowlist, no generic `jwk.Parse` fallback. RSA-2048 signatures are ~256 bytes vs HS256's ~32. New rotation runbook and on-call burden.

### Option B — Keep HS256, add `kid` for rotation only

Stay symmetric but tag tokens with a `kid` and allow multiple shared secrets simultaneously.

- **Pros:** Smallest code change. Rotation gets an overlap window.
- **Cons:** Does not fix the empty JWKS — symmetric secrets cannot be published. Does not enable external verifiers. Discovery document remains misleading. Leaves Pattern Matrix Row 1 P0 open.

### Option C — ES256 (ECDSA P-256)

- **Pros:** Smaller signatures (~64 bytes), faster verification, same JWKS shape.
- **Cons:** RP/IdP library support is more uneven than RS256 in 2026; Phase B SSO targets (Okta/Auth0/Azure AD) default to RS256. ES256 can be added later under the same `kid`-keyed infrastructure (each row carries `alg`).

## Decision

**Option A — RS256 with `kid`-keyed JWKS rotation.** Migration is forward-only with a bounded HS256-verification window. Keys live in `jwt_keys` (private key encrypted under the master KEK per ADR-0001 envelope shape), served via JWKS, rotated 90-day cadence with 60-day overlap. ES256 is not adopted now; the `jwt_keys.alg` column reserves the slot.

Option B does not close the live P0 because JWKS is intrinsically a public-key surface. Option C is strictly future work the same infrastructure supports, so deferring costs nothing.

## Rejection rationale

- **Option B** loses because publishing a real JWKS is the load-bearing requirement; symmetric keys cannot be published.
- **Option C** loses because RS256 has broader RP-library support today; ES256 can be added later without redoing this work.

## Consequences

- **Operational:** New rotation runbook entry. The `signing` row must always exist exactly once; keystore startup self-heals if missing. Two algorithms accepted during migration (≤ 24h until all HS256 tokens expire per `auth.go:25`).
- **Security:** Signing private key encrypted-at-rest under the master KEK; in-memory only during process lifetime. JWKS exposes only public keys. `THREAT_MODEL.md` §2 row S trust boundary **narrows**. Algorithm-confusion guard hardened: explicit allowlist `["HS256","RS256"]` during cutover, `["RS256"]` after; no `jwk.Parse` of attacker-supplied material as HMAC key.
- **Migration:** New `jwt_keys` table; new `internal/auth/keystore.go`; `auth.go` validation widened to `kid` lookup; JWKS handler returns real keys.
- **Reversibility:** Time-bounded. See §Rollback plan.

## Implementation plan

1. **Schema (`backend/migrations/009_jwt_keys.sql`)**: `jwt_keys (id UUID PK, kid TEXT UNIQUE NOT NULL, alg TEXT NOT NULL, public_key BYTEA, private_key_encrypted BYTEA, private_key_nonce BYTEA, status TEXT CHECK (status IN ('signing','verifying','retired')), created_at TIMESTAMPTZ, retired_at TIMESTAMPTZ)`. Partial unique index `WHERE status='signing'` enforces exactly-one signing key.
2. **Keystore (`backend/internal/auth/keystore.go`, new)**: `LoadOrInit(crypto.Service) (*Keystore, error)` generates RSA-2048 on first startup; wraps the private key via `crypto.Service.GetMasterKey()` + `crypto.Encrypt` (`backend/internal/crypto/crypto.go:49-54`). Methods: `SigningKey() (kid, *rsa.PrivateKey)`, `VerifyKey(kid) (*rsa.PublicKey, alg, error)`, `PublicKeySet() []JWK`, `Rotate(ctx)` (promote new → `signing`, demote old → `verifying`).
3. **JWT issuance + validation (`backend/internal/auth/auth.go:11-62`)**: `GenerateToken` (`:29-45`) signs with `jwt.SigningMethodRS256` and sets `Header["kid"]`. `ValidateToken` (`:47-62`) replaces the HMAC-only type assertion at `:50-52` with an `alg`-per-`kid` allowlist; during cutover both `*jwt.SigningMethodHMAC` and `*jwt.SigningMethodRSA` accepted; after cutover RSA-only.
4. **JWKS endpoint (`backend/internal/api/handlers_oauth.go:216-218`)**: return `Keystore.PublicKeySet()` as RFC 7517 JWKs. `OpenIDConfiguration` at `:233` becomes truthful.
5. **Config**: `JWT_ROTATION_INTERVAL` (default `90d`), `JWT_OVERLAP_WINDOW` (default `60d`), `JWT_ALG_VERIFY` (allowlist; default `RS256,HS256` during migration, then `RS256`). Provides rollback knob without code change (Pomerium-shape sentinel per `ory.md` line 203).
6. **Audit events**: `auth.jwt_key.rotated`, `auth.jwt_key.retired`, `auth.jwt_key.emergency_rotation` per `docs/AUDIT_LOG_COVERAGE.md`.
7. **Tests (`backend/internal/auth/auth_test.go`)**: table-driven — (a) HS256 token issued pre-cutover validates during overlap; (b) RS256 validates; (c) `alg: none` rejected; (d) HS256 token signed using RS256 public key as HMAC secret rejected (algorithm-confusion fixture per `ory.md` line 201); (e) mismatched-`kid` rejected; (f) `retired` `kid` rejected; (g) `verifying` `kid` validates but cannot mint.

## Rollback plan

**Rollback is time-bounded, not free.** Once an RS256-signed JWT is issued, it persists in that client until natural expiry (24h per `auth.go:25`). Reverting within the first 24h is clean: set `JWT_ALG_VERIFY=HS256`, redeploy with HS256 signing forced; no live RS256 token exists yet. **After the first RS256 token issues, true rollback requires up to 24h of forced re-authentication** (RS256 tokens rejected; clients re-login).

Key material is not destroyed: `jwt_keys` rows persist for forensic verification. Operator path: (1) `JWT_ALG_VERIFY=HS256`; (2) env flag to force HS256 signing; (3) deploy; (4) monitor re-auth via existing rate-limit dashboards. True client-side rollback (deleting issued tokens) is not possible — holders retain them. Mirrors Ory dossier §11 Cand. 1.

## Open questions

1. **Master-key source for the encrypted private key — KMS, env, or both?** ADR-0004 marks `EnvProvider` development-only; AWS/GCP KMS adapters exist in `backend/internal/crypto/keyprovider/` but are **unwired in `main.go`** per FU #1 (`docs/FOLLOWUPS.md:90-95`). This ADR reuses whichever provider `crypto.Service` is constructed with, so FU #1 landing picks up automatically. Open: block this ADR's production rollout on FU #1 first, or ship with `EnvProvider` and migrate later? *Owner: Security Engineer + DevOps. Due: 30 days.*
2. **Should the JWKS endpoint require authentication?** OAuth/OIDC convention is public, but enumeration concerns exist if a future per-tenant `kid` namespace leaks tenant identifiers. Decision needed before the endpoint goes live. *Owner: Security Engineer. Due: 14 days.*
3. **Operator UX for emergency rotation (suspected key compromise).** No UI, no CLI, no API today. Options: manual SQL, a `keepsavectl auth rotate-jwt-key` CLI, or an admin UI button. Manual SQL is the runbook escape hatch but encourages bad habits. *Owner: Backend Engineer + DevOps. Due: 30 days (before first 90-day rotation cycle).*

## References

- `backend/internal/auth/auth.go:11-62` — current HS256 implementation (signing `:39`, validation `:47-62`, 24h expiry `:25`).
- `backend/internal/api/handlers_oauth.go:216-218` — empty JWKS stub; `:233` — `jwks_uri` advertised.
- `backend/internal/crypto/crypto.go:17-54` — envelope-encryption helpers reused.
- `docs/THREAT_MODEL.md:84` (§2 row S, residual Low, unchanged); `:86` (row T, residual Medium, **not** addressed here).
- `docs/audits/SECURITY_AUDIT_2026-05-15.md` §A02-F4 (alg-confusion PASS), §A07-F4 (denylist — separate Phase B work).
- `docs/research/competitors/ory.md` §4, §10 Cand. 1, §11, §12 Invariants 2-3.
- `docs/research/PATTERN_MATRIX.md` Row 1 + Footnote ^1.
- `docs/FOLLOWUPS.md:90-95` (FU #1 KMS adapters).
- ADR-0001, ADR-0002 (extended), ADR-0004 (reused).
- RFC 7515 (JWS), 7517 (JWK), 7519 (JWT), 8725 (JWT BCP — alg-confusion guidance).
