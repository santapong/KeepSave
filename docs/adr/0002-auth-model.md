# ADR-0002: Authentication model — JWT for humans, API keys for agents

- **Status:** Accepted (backfilled)
- **Date:** 2026-05-12
- **Authors:** Tech Lead (backfilled)
- **Reviewers required:** Tech Lead; Security Engineer
- **Supersedes:** none
- **Related:** ADR-0003 (promotion engine — auth determines who can promote), `docs/THREAT_MODEL.md`

---

## Context

KeepSave has two fundamentally different classes of caller:

1. **Humans** using the dashboard or CLI — interactive, session-bound, can re-authenticate; their access spans a whole user account.
2. **AI agents and CI/CD systems** — non-interactive, long-lived, often running on machines we don't control; their access should be narrow (one project, optionally one environment).

A single unified mechanism is tempting but wrong: human session tokens leaking to an agent's environment file is a class of incident; agent credentials accidentally given a human's full permissions is another. The two populations need different identity primitives with different storage and revocation stories.

Constraints:
- Standard-library only for the cryptographic primitives (consistent with ADR-0001).
- Revocation must be possible without a JWT denylist becoming the bottleneck.
- Stolen credentials must be detectable at rest (hashed in DB so a DB exfiltration doesn't yield usable keys).

## Options considered

### Option A — JWT for humans, API keys for agents (two separate primitives)

Humans authenticate with email/password, receive a short-lived JWT (HS256, 24h expiry). Agents are issued opaque API keys with a `ks_` prefix, scoped to one project and optionally one environment, stored only as a SHA-256 hash. Two separate auth middlewares; an endpoint accepts either or both.

- **Pros:** Each population gets the right primitive. Agents can be scoped narrowly without complicating the JWT claim shape. Hashed-only API keys mean DB exfil doesn't yield usable credentials. Revocation of an API key is a row delete; JWT revocation is bounded by the 24h expiry (acceptable for human sessions).
- **Cons:** Two code paths for callers to understand. Two test surfaces.

### Option B — JWT everywhere, with scope claims for agents

Issue JWTs to both humans and agents; encode agent scope (project, environment) in claims.

- **Pros:** Single mechanism, single middleware.
- **Cons:** Agent JWTs are *long-lived* — they must be, since agents can't re-authenticate interactively. Long-lived JWTs are a JWT anti-pattern: they can't be revoked without a denylist that defeats the point of stateless tokens. JWT secret rotation invalidates all agents at once. Stored in plaintext on agent hosts, they're recoverable from disk; we'd need to rotate constantly.

### Option C — API keys everywhere

Treat humans as if they were agents: issue an API key on login.

- **Pros:** Single primitive. Hashed storage. Cheap revocation.
- **Cons:** Loses the natural expiry of human sessions. Login flow has to mint a new key per session, accumulating rows. No clean way to encode "session" semantics (idle timeout, IP binding) that human apps want.

## Decision

**Option A — two primitives, one auth surface.** Humans use email+password → JWT (HS256, 24h). Agents use `ks_`-prefixed API keys, SHA-256-hashed in storage, scoped to a project and optionally an environment. The API exposes both middlewares; a request may present either.

Implementation: `backend/internal/auth/auth.go:11-15, 25-26, 39` (JWT claim shape, expiry, algorithm), `backend/internal/auth/apikey.go:11-26` (`ks_` prefix, hex format, SHA-256 storage), `backend/internal/auth/password.go:1-19` (bcrypt with default cost), `backend/internal/api/middleware.go:59-120` (API key checked first, then JWT; sets `user_id` and, for API key, `api_key_project_id` / `api_key_scopes` / `api_key_environment` on the request context).

## Rejection rationale

- **Option B** was rejected because long-lived JWTs without a revocation list are a known footgun; with a revocation list, JWT loses its stateless advantage and we have built a worse API-key system.
- **Option C** was rejected because shoehorning interactive human sessions into a long-lived-credential model removes useful properties (expiry, IP binding, session UX) without gain.

## Consequences

- **Operational:** `JWT_SECRET` rotation invalidates *all* live human sessions. This is the right behavior in an incident; in steady state, rotate during a planned window. API keys rotate independently per row.
- **Security:** API key strings are shown to the user *once* at creation (`backend/internal/auth/apikey.go:21-26`). The DB only ever holds the hash. A stolen DB does not yield agent credentials. JWT secret stolen → all live sessions hijackable until `JWT_SECRET` rotation.
- **JWT expiry is fixed at 24h.** No refresh-token flow exists; humans re-authenticate daily. This is deliberate for now (no refresh complexity, smaller attack window); revisit only if UX complaints accumulate.
- **Password hashing:** bcrypt at the library default cost. Acceptable for current scale; revisit cost factor at every major hardware upgrade and at minimum every 18 months.
- **Reversibility:** Auth mechanism changes are migration-heavy but tractable (issue new credentials, deprecate old). Not as irreversible as ADR-0001.

## Rollback plan

If a vulnerability is found in either primitive:

- **JWT:** rotate `JWT_SECRET` — all sessions invalidated instantly. Users re-authenticate; agents unaffected.
- **API keys:** rotate per-key (issue new, deprecate old) or per-project (mass-revoke). Both supported by current schema. No mass-rotation needed if scope is small.
- **Password hash:** if bcrypt itself is broken (unlikely but conceivable), force password reset on next login; re-hash on first authentication with the new algorithm.

## Open questions

- **JWT denylist for compromised tokens before expiry?** Currently no mechanism — a leaked JWT is valid until its 24h expiry. Acceptable now; needed once we have customers with strict revocation SLAs. *Owner: Backend Engineer. Due: 60 days (or sooner if a customer requires it).*
- **API key scopes are coarse:** today an API key gives full read/write for a (project, env). We do not have per-secret or per-action scopes. Add only when a concrete use case appears — don't over-design. *Owner: PM. Due: TBD.*
- **bcrypt cost factor monitoring:** what triggers a re-evaluation? Document in `docs/RUNBOOK.md`. *Owner: Security Engineer. Due: 60 days.*

## References

- `backend/internal/auth/auth.go:11-15, 25-26, 39`
- `backend/internal/auth/apikey.go:11, 15-26, 21`
- `backend/internal/auth/password.go:1-19`
- `backend/internal/api/middleware.go:59-120`
- `backend/internal/models/models.go:56-59` (API key scoping fields)
- RFC 7519 (JWT)
