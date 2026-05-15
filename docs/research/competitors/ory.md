# Competitor Dossier — Ory (Hydra + Kratos + Oathkeeper)

## 1. Header

- **Vendor:** Ory (Hydra, Kratos, Oathkeeper)
- **Category:** self-hosted auth / OIDC + OAuth 2.0 server
- **License:** Apache 2.0
- **Last updated:** 2026-05-15
- **Analyst:** Self-Hosted Auth Analyst
- **Reviewer:** Security Reviewer (pending)
- **Status:** draft
- **Priority:** P0

## 2. One-paragraph overview

Ory ships three independently-deployable OSS servers: **Hydra** (a stateless OAuth 2.0 / OIDC certified server), **Kratos** (identity, sessions, recovery / registration flows), and **Oathkeeper** (a zero-trust reverse-proxy decision point). They publish a Cure53 audit (Hydra v1.4, 2019) and maintain a responsive security disclosure programme on GitHub. KeepSave does not need a federated IdP and is explicitly blocked from one by `docs/ROADMAP_NOT.md:22` — we care about Ory as a **reference implementation of three discrete patterns**: an RS256 JWKS endpoint with key rotation (the empty-JWKS gap at `handlers_oauth.go:217` is the active P0), refresh-token rotation with replay detection (informs FU #5), and Kratos's session-token revocation model (informs how the unwired `SessionToken` struct would be plumbed).

## 3. Architecture summary

Three-binary split, each with its own admin/public API and SQL store:

1. **Hydra** — stateless OAuth 2.0 / OIDC. Mints access tokens (opaque or JWT) and refresh tokens. Exposes `/.well-known/openid-configuration` and `/.well-known/jwks.json`. Keys live in the `hydra_jwk` table; rotation is a CLI command. Login + consent flows are delegated to an integrator-supplied UI.
2. **Kratos** — identity store + session manager. Owns users, password hashing (Argon2id), self-service flows, and the `sessions` table. Sessions are server-tracked; revocation is a row update + cache invalidation.
3. **Oathkeeper** — proxy / decision-point. Reads declarative `access_rules.json`, runs authenticators (cookie / JWT / anonymous), authorizers, and mutators before forwarding upstream.

**What KeepSave maps to:** Hydra's JWKS + key-rotation surface and refresh-token rotation logic; Kratos's session-revocation pattern. **What we ignore:** Kratos identity UI and self-service flows, Oathkeeper deployment, Ory's SaaS layer. Pattern-borrowing, not binary-deployment.

## 4. Security model

Auth primitives Ory publishes (docs retrieved 2026-05-15 via search summary):

- **Hydra signing:** RS256 default; ES256 / EdDSA opt-in. Keys rotated via `hydra create jwks`. Multiple keys active simultaneously (`kid`-keyed) — rotation invariant: publish new key, wait for clients, flip signing default, retire old.
- **Refresh-token rotation:** Hydra rotates the refresh token on every refresh exchange and **detects reuse** — presenting the same token twice invalidates the entire token family. This is the RFC 8725 / OAuth 2.0 Security BCP recommendation, implemented.
- **PKCE:** S256 enforced for public clients; `plain` deprecated (RFC 7636).
- **Kratos session revocation:** server-tracked sessions in Postgres; revocation is a transactional update of the `active` column. Bearer is opaque, so no denylist-vs-JWT problem.
- **Trust-boundary stance:** private key compromise = total compromise; JWKS rotation is the only mitigation.

CVE history (last 24 months, NVD + GHSA, retrieved 2026-05-15):

- **CVE-2025-46337 / GHSA-7q6x-r5pq-pcqh** (Hydra, 2025-05) — open-redirect via crafted `post_logout_redirect_uri`. Tangential to KeepSave but evidence that redirect-URI validation is a recurring CVE class.
- **GHSA-2grw-9pmq-r7c4** (Kratos, 2024) — account-takeover via login-flow race. Not applicable; we are not adopting Kratos self-service flows.
- **GHSA-9j76-7895-fc6f** (Hydra, 2023) — DoS via large JWT. Informs §7: token-size guard required in any validator we ship.

Public audit: **Cure53 audit of Hydra v1.4** (2019). Hydra is **OpenID Certified** for OP Basic / Implicit / Hybrid / Config profiles (`openid.net/certification/`).

## 5. KeepSave-comparable surface

| Ory concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Hydra `/oauth/token` (authorization_code / refresh_token / client_credentials) | OAuth 2.0 token endpoint | `backend/internal/api/handlers_oauth.go:217` (JWKS handler in same file; full endpoint set declared at `:220-239`) |
| Hydra `/.well-known/jwks.json` with rotating RS256 keys | **Empty JWKS, HS256-only signing** | `backend/internal/api/handlers_oauth.go:216-218`; signing at `backend/internal/auth/auth.go:39` |
| Hydra refresh-token rotation w/ family invalidation | **No refresh-token rotation, no replay detection** — `ValidateToken` validates HMAC only | `backend/internal/auth/auth.go:47-62` |
| Hydra `hydra_jwk` DB-backed key store + `hydra create jwks` rotation CLI | No JWK store; HMAC secret comes from `JWT_SECRET` env var (single key, no rotation) | `backend/internal/auth/auth.go:22-27` |
| Kratos server-tracked sessions with `active`/`revoked` column | `SessionToken` struct exists but is **unwired** — no repository, no service references | `backend/internal/models/models.go:352-362` (no callers per grep 2026-05-15) |
| Oathkeeper access-rule authenticator chain (JWT / cookie / anonymous) | Two-strategy middleware: API key first, JWT fallback (code-resident, not declarative) | `backend/internal/api/middleware.go:88-120` |
| Hydra signed-JWT access token verified via JWKS | API key path uses SHA-256 hashed lookup; JWT path uses shared HMAC | `backend/internal/auth/apikey.go:11-26`; `backend/internal/auth/auth.go:47-62` |

**Concepts deliberately not adopted** (Ory has substantial surface irrelevant to KeepSave — we adopt patterns, not deploy Ory):

| Their concept | Reason we don't adopt |
|---|---|
| Kratos identity-management UI (login / registration / recovery / verification / settings flows) | Out of scope per `docs/ROADMAP_NOT.md:22` — SSO/SAML/OIDC dashboard auth blocked until two customers ask. |
| Kratos Argon2id password-hashing pipeline | Separate work item if it surfaces; not the gap this dossier informs. |
| Oathkeeper as a deployed proxy | Heavyweight for Gin-only, small-scale KeepSave; `middleware.go:88-120` is already the simpler shape. |
| Ory Cloud multi-tenant projects | KeepSave multi-tenancy is Phase B (`FOLLOWUPS.md:155`); won't borrow tenant isolation from a SaaS product before then. |
| Hydra delegated login + consent UI handshake | We are not a third-party OAuth provider; `handlers_oauth.go` serves first-party clients only. |
| Kratos webhook system for flow side-effects | Audit log, not webhooks, is our cross-system signalling channel. |

## 6. Adapt candidates

Three pattern adoptions. None requires deploying Ory.

1. **RS256 + JWKS endpoint with `kid`-keyed key rotation.** Replace `auth.go:39`'s HS256 with RS256; populate `handlers_oauth.go:217` with the real key set. Two keys active at once (`kid` distinguishes); rotation publishes the new key, waits one expiration window, flips signing default, retires old key. Borrowed directly from Hydra's `hydra_jwk` table + `hydra create jwks` flow.
2. **Refresh-token rotation with replay-detection family invalidation.** A new `refresh_tokens` table tracks (token_hash, family_id, parent_id, used_at). Issuing `refresh_token` consumes the row; presenting an already-consumed token invalidates every row sharing its `family_id`. This is OAuth Security BCP behaviour, implemented in Hydra. KeepSave today has no refresh flow at all — adopting this means *adding* a flow, not modifying one.
3. **Kratos-style session-token denylist with TTL pruning.** Wire the existing `SessionToken` struct (`models.go:352-362`): on logout / forced revocation, mark `revoked=true`; middleware (`middleware.go:75-80`) consults the table by `jti`. A background sweeper (re-using `startAuditLogPruner` from `FOLLOWUPS.md:11`) deletes rows past expiry. RFC 7009 (OAuth Token Revocation) is the wire-protocol shape; Kratos's server-tracked session table is the storage shape.

Oathkeeper as a deployed authorization decision-point was considered and is **not** an adapt candidate — too heavyweight for KeepSave's scale; declarative `access_rules.json` would replicate, not improve on, the middleware at `middleware.go:88-120`.

## 7. Pros / cons of adapting

### Candidate 1: RS256 + JWKS rotation

- **Pros:** Closes the live P0 at `handlers_oauth.go:217` (endpoint advertised in `OpenIDConfiguration` at `:233` but serves empty `keys: []`). Enables verifier separation: MCP gateway and AI agents can validate without holding `JWT_SECRET`. Hydra is OpenID-certified for this shape.
- **Cons (operational):** Adds a private-key custody requirement (one more KMS-managed asset alongside the master key per ADR-0004). `RUNBOOK.md` rotation procedure required before adoption. Tokens issued under HS256 must remain valid through a transition window — `ValidateToken` at `auth.go:47-62` needs to accept both algorithms during cutover.
- **Cons (security):** Algorithm-mixed validation is the source of multiple historical CVEs (`none`-algorithm and `HS256-with-RSA-public-key` classes). Mitigation: explicit allowlist of `alg` per `kid`, no `jwk.Parse` fallback. CVE-2025-46337 confirms even well-trodden OAuth surfaces ship CVEs.

### Candidate 2: Refresh-token rotation with replay detection

- **Pros:** Enables shorter-lived access tokens (today's JWT is 24h per `auth.go:25` — long enough that "no denylist" is a live concern). Family invalidation is the OAuth Security BCP recommendation; Hydra is reference-quality. Reduces residual on `docs/THREAT_MODEL.md:84` row T from Medium toward Low.
- **Cons (operational):** New table, new endpoint behaviour, new rate-limit surface (refresh attempts must be throttled per-family to prevent next-token harvest). Every existing client must be taught to refresh.
- **Cons (security):** Family-invalidation is itself a DoS vector — an attacker who briefly sees a refresh token can deliberately replay it to lock out the legitimate user. Mitigation requires audit-logged replay events and a recovery path. Net residual lower than today; tradeoff is real.

### Candidate 3: Session-token denylist with TTL pruning

- **Pros:** Directly informs FU #5 (`docs/FOLLOWUPS.md:160`). `SessionToken` struct already exists (`models.go:352-362`) — schema work is minimal. Pruner pattern already in production for audit log (`FOLLOWUPS.md:11`).
- **Cons (operational):** Every authenticated request consumes one extra Postgres lookup. In-memory LRU cache keyed by `jti` is mandatory or middleware p99 doubles. Pruner adds a new background goroutine and a new failure mode.
- **Cons (security):** Stale cache means a revoked token continues to work for cache TTL. Mitigation: cache TTL ≤ 60s, explicit invalidation on revoke via Postgres LISTEN/NOTIFY, or accept the bound.

## 8. Validation evidence

Mix of RFC, audit, CVE, and OpenID certification. No load-bearing vendor-blog claims.

- **RFC 7517 (JWKS)**, **RFC 7515 (JWS)**, **RFC 7519 (JWT)** — wire formats for Candidate 1.
- **RFC 7636 (PKCE)** and **RFC 8252** — OAuth 2.0 for native apps; PKCE S256 enforcement.
- **RFC 7009 (OAuth Token Revocation)** — load-bearing for Candidate 3's revocation endpoint shape.
- **draft-ietf-oauth-security-topics / RFC 8725** — refresh-token rotation + replay-detection recommendation (Candidate 2's load-bearing source — not the vendor doc).
- **Cure53 audit of Ory Hydra v1.4** (2019) — `ory.sh/blog/hydra-cure53-security-audit` retrieved 2026-05-15. Establishes maturity of the disclosure programme. Pointer to public audit; the audit itself is the load-bearing artefact.
- **OpenID Foundation Certified Implementations** — `openid.net/certification/` retrieved 2026-05-15. Hydra is certified for OP Basic, OP Implicit, OP Hybrid, OP Config.
- **CVE-2025-46337** (Hydra open-redirect, 2025-05) — `github.com/ory/hydra/security/advisories/GHSA-7q6x-r5pq-pcqh` retrieved 2026-05-15. Establishes recurring CVE class on redirect-URI validation.
- **GHSA-2grw-9pmq-r7c4** (Kratos account-takeover race, 2024) — confirms Kratos self-service flows have non-trivial residual; reinforces our §5 decision *not* to adopt them.
- **GHSA-9j76-7895-fc6f** (Hydra large-JWT DoS, 2023) — informs §7 Candidate 1 cons (token-size guard in validator).

Vendor docs cited as supplementary (not load-bearing alone): `ory.sh/docs/hydra/concepts/before-oauth2`, `ory.sh/docs/hydra/jwks`, `ory.sh/docs/kratos/concepts/sessions` — all retrieved 2026-05-15 via search summary. Direct WebFetch not attempted; if a reviewer re-verifies and finds vendor docs `(unreachable via tooling)`, the load-bearing claims still rest on the RFCs and CVEs above.

## 9. Threat-model implications

Two STRIDE rows in `docs/THREAT_MODEL.md` §2 (Authentication):

- **Row S "Forged JWT"** at `docs/THREAT_MODEL.md:82`:

  > *"HS256 with secret only on server; signature verified … Residual: Low"*

  Adopting Candidate 1 (RS256 + JWKS) **does not widen** the trust boundary — the private key remains server-only. It *narrows* a different boundary: external verifiers (MCP gateway, AI agents) can now check the signature without holding `JWT_SECRET`. Residual stays Low; the verifier-distribution surface improves.

- **Row T "Token replay after revocation"** at `docs/THREAT_MODEL.md:84`:

  > *"API key revocation = row delete; JWT expires in 24h, **no denylist**. Residual: Medium"*

  Adopting Candidates 2 + 3 directly attacks this row. Candidate 2 (rotation + replay detection) replaces 24-hour standing access with short-lived access + family-invalidated refresh; Candidate 3 (denylist) gives pre-expiry revocation. Combined residual → Low.

No new entity enters the trust boundary. The `refresh_tokens` and `session_tokens` tables are internal Postgres state, governed by the existing repository / audit invariants.

## 10. Verdict

**Candidate 1 (RS256 + JWKS rotation): `adopt-now`.**

The trigger has already fired: `handlers_oauth.go:217` returns empty `keys: []` while `OpenIDConfiguration` at `:233` advertises a `jwks_uri`. This is a live P0 in `docs/research/README.md:11`:

> *"the OAuth 2.0 server in `backend/internal/api/handlers_oauth.go` exposes an empty JWKS and no RS256"*

Adoption requires a Type-1 ADR (auth-adjacent) per `CLAUDE.md` Decision-classes table. Security Engineer veto applies per `docs/ROLES.md`.

**Candidate 2 (refresh-token rotation): `adopt-when-trigger-fires`.**

Trigger quoted verbatim from `docs/FOLLOWUPS.md:160`:

> *"**JWT denylist for pre-expiry revocation** (ADR-0002 §Open Questions). Build when a customer SLA requires it."*

Refresh-token rotation is the *upstream* of that denylist conversation: shortening access-token lifetime is the precondition that makes a denylist tractable (Candidate 3) versus essential. Adopt when FU #5 is reopened.

**Candidate 3 (session denylist): `adopt-when-trigger-fires`.**

Same trigger as Candidate 2 — `docs/FOLLOWUPS.md:160`. Candidate 2 and Candidate 3 should land in the same ADR.

**Oathkeeper-style deployed decision point: `reject`.** Too heavyweight for KeepSave's scale; `middleware.go:88-120` is the right shape. Recorded here so the question doesn't get re-asked.

## 11. Rollback if adopted

**Candidate 1 (RS256 + JWKS):** Mixed-algorithm validation is the rollback shape. Cutover keeps the HS256 verifier alive while publishing RS256; flipping back sets signing default to HS256 and continues to verify both. True rollback (deleting RS256-signed tokens) is **not possible** — holders retain them until natural expiry (≤ 24h per `auth.go:25`). Runbook step: `kubectl rollout` with `JWT_ALG=HS256` override; clients re-issued at next login. KMS private key retained for forensic re-verification.

**Candidate 2 (refresh-token rotation):** Drop `refresh_tokens` table writes; existing rows become inert. `/oauth/token` `grant_type=refresh_token` returns `unsupported_grant_type`. Clients fall back to re-login. No data destroyed.

**Candidate 3 (session denylist):** Stop reading the table in middleware; revoked sessions resurrect for their remaining TTL. Security regression of finite duration (≤ 24h JWT TTL); runbook must surface this explicitly. Pruner can be safely stopped; table growth resumes but is bounded.

## 12. Won't-break-our-system claim

Integrator-facing contracts: (a) `Authorization: Bearer <jwt>` from the dashboard, (b) `X-API-Key: ks_...` from AI agents. Verified by reading `middleware.go:88-120` — API-key path runs first and short-circuits before JWT validation.

- **Invariant 1 — API-key auth untouched.** `middleware.go:88-115` runs SHA-256 hashed-lookup before falling through to `JWTAuthMiddleware` at `:118`. API keys never transit the JWT validator. Verified by code-reading.
- **Invariant 2 — JWT issuance contract unchanged.** `JWTService.GenerateToken` at `auth.go:29-45` returns a string token; callers do not inspect `alg`. RS256 changes the signature bytes, not the envelope. Only `Set("user_id", ...)` consumer is `middleware.go:82` reading `Claims.UserID`, unaffected by algorithm. Verified by code-reading.
- **Invariant 3 — Existing tests green during dual-algorithm window.** `ValidateToken` at `auth.go:47-62` would accept HS256 and RS256 per Candidate 1's cutover. The `*jwt.SigningMethodHMAC` type assertion at `auth.go:50-52` widens to an allowlist. Existing fixtures exercise HS256 only; they continue to pass. Verified by code-reading.
- **Invariant 4 — `SessionToken` is dormant; wiring is additive.** `models.go:352-362` has no callers (grep 2026-05-15). Adding a repository + middleware lookup modifies no existing code path. Verified by grep across `backend/internal/`.
- **Invariant 5 — Audit taxonomy extends cleanly.** Refresh-token issuance and denylist events are new types; `docs/AUDIT_LOG_COVERAGE.md` accepts new rows without breaking existing assertions. Verified by reading the taxonomy.

Security Reviewer veto applies. Suggested checks: (a) algorithm-confusion guard is an explicit allowlist keyed by `kid`, not generic `jwk.Parse`; (b) refresh-token replay handler emits an audit event and surfaces a recoverable error; (c) denylist cache TTL ≤ 60s with explicit invalidation on revoke.

## 13. References

- RFC 7515 (JWS): `https://www.rfc-editor.org/rfc/rfc7515` — retrieved 2026-05-15.
- RFC 7517 (JWKS): `https://www.rfc-editor.org/rfc/rfc7517` — retrieved 2026-05-15.
- RFC 7519 (JWT): `https://www.rfc-editor.org/rfc/rfc7519` — retrieved 2026-05-15.
- RFC 7636 (PKCE): `https://www.rfc-editor.org/rfc/rfc7636` — retrieved 2026-05-15.
- RFC 8252 (OAuth 2.0 for Native Apps): `https://www.rfc-editor.org/rfc/rfc8252` — retrieved 2026-05-15.
- RFC 7009 (OAuth 2.0 Token Revocation): `https://www.rfc-editor.org/rfc/rfc7009` — retrieved 2026-05-15.
- RFC 8725 / OAuth 2.0 Security Best Current Practice: `https://www.rfc-editor.org/rfc/rfc8725` — retrieved 2026-05-15.
- CVE-2025-46337 / GHSA-7q6x-r5pq-pcqh (Hydra open-redirect): `https://github.com/ory/hydra/security/advisories/GHSA-7q6x-r5pq-pcqh` — retrieved 2026-05-15.
- GHSA-2grw-9pmq-r7c4 (Kratos login race): `https://github.com/ory/kratos/security/advisories` — retrieved 2026-05-15.
- GHSA-9j76-7895-fc6f (Hydra large-JWT DoS): `https://github.com/ory/hydra/security/advisories` — retrieved 2026-05-15.
- Cure53 audit of Ory Hydra (2019): `https://www.ory.sh/blog/hydra-cure53-security-audit` — retrieved 2026-05-15 (vendor blog as pointer; the audit PDF is the load-bearing artefact).
- OpenID Foundation Certified Implementations: `https://openid.net/certification/` — retrieved 2026-05-15.
- Ory Hydra docs — JWKS / sessions / before-OAuth2: `https://www.ory.sh/docs/hydra/jwks`, `…/concepts/before-oauth2`, `…/kratos/concepts/sessions` — retrieved 2026-05-15 (via search summary).
- Ory source repos: `https://github.com/ory/hydra`, `https://github.com/ory/kratos`, `https://github.com/ory/oathkeeper` — retrieved 2026-05-15.
