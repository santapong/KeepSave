# ADR-0021: Short-lived agent tokens with a JWT denylist

- **Status:** Accepted
- **Date:** 2026-06-23
- **Authors:** Tech Lead
- **Reviewers required:** Security Engineer (auth / token lifecycle — veto); Tech Lead
- **Supersedes:** none
- **Related:** ADR-0002 (auth model), ADR-0008 (RS256/JWKS — tokens are now RS256-signed), ADR-0009 (API-key expiration), `lease_service.go` (JIT leases), `docs/THREAT_MODEL.md` §1/§4, `docs/research/IMPROVEMENT_PLAN_2026-06.md` (O5/O10, AUTH-08)

---

## Context

Agents authenticate today with **long-lived API keys** (`X-API-Key`) or 24-hour
user JWTs. Both are bearer credentials with no fast kill switch: a leaked API key
is valid until someone deletes it, and a JWT is valid until it expires — there is
no revocation path for an issued JWT at all. The JIT **lease** mechanism
(`lease_service.go`) grants time-boxed access to specific secret keys, but nothing
binds a *token* to a lease, and the secret-read path does **not** re-check the
lease, so the lease's `revoked`/`expires_at` columns do not actually gate a token
that was minted against it.

We want agents to exchange a lease for a **short-lived, narrowly-scoped JWT** that:
- expires in minutes, not hours (blast-radius reduction on leak);
- can be **revoked before expiry** — both individually (a single leaked token) and
  in bulk (revoke the owning lease ⇒ all its tokens die promptly);
- is RS256-signed (ADR-0008) so verifiers need only the public JWKS.

Constraints:
- Revocation must be **prompt** — within seconds, not "wait for expiry."
- The denylist check must **not** add a DB round-trip to the hot path for ordinary
  user tokens (the overwhelming majority of traffic).
- The `auth` package must stay **free of a direct DB dependency** (mirrors the
  `KeyStorage` seam from ADR-0008): revocation state is reached through an
  interface implemented in `repository`.
- Backward compatible: existing user JWTs (no `jti`, no `lease_id`) keep working
  unchanged.

## Options considered

### Option A — Stateless short TTL only, no denylist
Mint a 5-minute token; rely on expiry.
- **Pros:** zero revocation infrastructure.
- **Cons:** no kill switch — a leaked token is usable for its full (short) life,
  and revoking the owning lease does nothing. Fails the "prompt revocation"
  requirement. Rejected.

### Option B — Per-request lease re-validation (no separate denylist)
Every agent request re-loads the lease and checks `revoked`/`expires_at`.
- **Pros:** lease is the single source of truth; no new table.
- **Cons:** cannot revoke a *single* leaked token without killing the whole lease;
  a DB hit per agent request with no way to cache positively (revocation is the
  rare case). Rejected as the sole mechanism, but its lease-state check is folded
  into Option C.

### Option C — Short TTL + denylist keyed by `jti`, with lease-cascade (chosen)
Mint a short-lived RS256 token carrying a random `jti` and the `lease_id` it was
minted from. Revocation is reachable two ways, both surfaced through one interface
method `IsTokenRevoked(jti, leaseID)`:
1. **explicit** — a revoke endpoint inserts the `jti` into a `token_denylist`
   table (kills one leaked token);
2. **lease-cascade** — revoking the lease flips the lease row; the checker treats
   any token whose `lease_id` is revoked/expired as revoked (kills all tokens from
   that lease at once) without having to enumerate their `jti`s.

The check runs **only when the token carries a `jti`** (i.e. agent tokens). User
JWTs have no `jti`, so they skip the denylist entirely — no added hot-path cost.
A small in-memory positive cache of revoked `jti`s (seeded at boot, write-through
on revoke, periodically refreshed) avoids a DB hit on the common "not revoked"
path; the lease-state leg is a keyed lookup incurred only by agent tokens.

- **Pros:** prompt single-token *and* bulk (per-lease) revocation; user tokens
  unaffected; `auth` stays DB-free behind `Denylister`; RS256 reuse from ADR-0008.
- **Cons:** a new table + a periodic refresh; multi-instance propagation of an
  explicit `jti` revoke is bounded by the refresh interval (mitigation below).

## Decision

**Option C.** Concretely:

- **Claims** gain `TokenType` (`"agent"`) and `LeaseID *uuid.UUID` (both
  `omitempty`); the `jti` is `RegisteredClaims.ID`, a random UUID set for agent
  tokens only.
- **Mint**: `JWTService.GenerateAgentToken(userID, email, leaseID, projectID,
  environment, secretKeys, ttl, leaseRemaining)` caps `ttl` at `maxAgentTokenTTL`
  (15 min) and never exceeds the lease's remaining lifetime; signs RS256
  (ADR-0008); returns `(token, jti)` so the caller can audit the `jti`. The
  lease's `projectID`, `environment` and `secretKeys` are **embedded in the
  token claims** — they are the token's entire authority (see Amendment 2026-06-24).
- **Denylist seam**: `auth.Denylister.IsTokenRevoked(jti string, leaseID *uuid.UUID)
  (bool, error)`; `JWTService.EnableDenylist(d)`. `ValidateToken` consults it iff
  `claims.ID != ""`.
- **Storage**: migration `014_token_denylist.sql` (`jti` PK, `lease_id`,
  `expires_at`, `revoked_at`, `reason`). `repository.TokenDenylistRepository`
  implements `Denylister`: positive `jti` cache (boot-seed + write-through +
  refresh) **and** a lease-state check (`secret_leases.revoked` /
  `expires_at`) when `leaseID != nil`. `DeleteExpired` prunes rows past
  `expires_at` (pruned by the existing background loop in `main.go`).
- **Endpoints** (`handlers_agent.go`, under an authenticated, project-scoped route):
  - `POST /projects/:id/agent-token` — body `{lease_id, duration_minutes?}` → mints
    a token bound to an *active* lease the caller owns in that project; audits
    `agent.token.minted`.
  - `POST /projects/:id/agent-token/revoke` — body `{jti}` → denylist insert;
    audits `agent.token.revoked`.
- **Lease-cascade**: no extra write on `RevokeLease` — the checker reads the live
  lease state, so an already-revoked/expired lease invalidates its tokens for free.

## Consequences

- **Security (positive):** leaked agent tokens have a ≤15-minute window and a kill
  switch; revoking a lease now actually kills its tokens. New revocation taxonomy
  events (`agent.token.minted`, `agent.token.revoked`) are auditable.
- **Trust boundary:** the mint endpoint converts a lease (project-scoped authority)
  into a bearer token of equal-or-lesser scope and shorter life — it never widens
  scope. `docs/THREAT_MODEL.md` §1/§4 updated (token issuance + revocation path).
- **Performance:** user-token validation is unchanged (no `jti` ⇒ no check). Agent
  tokens incur a cache hit plus, when `lease_id` is present, one keyed lease lookup.
- **Multi-instance staleness:** an explicit `jti` revoke propagates to other
  instances within the refresh interval (default 30 s). Lease-cascade revokes are
  **not** stale (read live). Acceptable because tokens are short-lived; a DB-backed
  pub/sub or per-request lookup is a documented follow-up if the window must shrink.
- **Migration:** additive table; no change to existing tokens or rows.

## Amendment 2026-06-24 — scope enforcement (security fix)

The original implementation honored the *revocation* legs (Option C) but **not**
the scope-narrowing invariant claimed above. `MintToken` minted a token whose
`UserID` claim was the lease's owning user, and `JWTAuthMiddleware` set only
`user_id`/`email` from it. Because nothing on the request path re-derived
authority from the lease (Context noted "the secret-read path does not re-check
the lease"), a presented agent token inherited the **owning user's full access** —
every project, environment and secret the user could reach — directly
contradicting "a bearer token of equal-or-lesser scope … it never widens scope."
A scoped, environment-locked API key could thus mint a 15-minute token with
strictly *greater* authority than the key itself (HIGH-severity privilege
escalation).

Fix (this PR):
- **Embed the lease scope in the token.** `Claims` gains `ProjectID`,
  `Environment`, `SecretKeys`; `GenerateAgentToken` records them. Safe to embed
  because the lease is immutable for the token's ≤15-min life, and
  revocation/expiry remain enforced by the denylist in `ValidateToken`.
- **Default-deny confinement at the single chokepoint.** `JWTAuthMiddleware`
  detects `TokenType=="agent"` and (a) rejects the request unless the matched
  route is on a minimal allowlist — `GET /projects/:id/secrets[/:secretId]` only
  (`agentTokenRouteAllowed`), which also blocks the lease/agent-token endpoints
  themselves (no privilege re-delegation), promotion, org/admin, applications,
  and every write method; (b) installs the lease scope as API-key-equivalent
  context (`api_key_project_id`, `api_key_environment`, and `read:<key>` scopes
  via `agentReadScopes`) plus an `is_agent_token` marker. Authorization is then
  enforced by the **existing** `RequireProjectAccess` + `EnforceAPIKeyScope` +
  `APIKeyScopeAllowsKey` path, so an agent token behaves as exactly a read-only,
  environment-locked, key-globbed API key for its lease — never the user.

Net effect: an agent token can do **no more** than read the specific secrets its
lease names, in the lease's project and environment. This restores the
equal-or-lesser-scope invariant. Covered by `agent_token_confine_test.go`
(end-to-end cross-project/cross-environment/write/promotion denial) and the
round-trip scope-claim assertions in `auth/agent_token_test.go`.

## Rollback plan
Do not call `EnableDenylist` and do not mint agent tokens; `ValidateToken` then
behaves exactly as before (the `jti` branch is dead). The table can be dropped;
no existing data depends on it.

## Open questions
- **Cross-instance immediacy** for explicit `jti` revokes — refresh interval vs.
  pub/sub vs. per-request lookup. *Owner: Backend. Due: if a sub-30 s SLA appears.*
- **Refresh-token-style rotation** for agents (mint a new token before expiry
  without a fresh lease round-trip). *Owner: Backend. Due: on demand.*

## References
- `backend/internal/auth/auth.go` (Claims, GenerateAgentToken, Denylister, ValidateToken)
- `backend/internal/repository/token_denylist_repo.go`
- `backend/internal/service/agent_token_service.go`
- `backend/internal/api/handlers_agent.go` (MintAgentToken, RevokeAgentToken)
- `backend/migrations/*/014_token_denylist.sql`
