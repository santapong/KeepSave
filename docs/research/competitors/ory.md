# Competitor Dossier — Ory (Hydra + Kratos + Oathkeeper)

## 1. Header

- **Vendor:** Ory (Hydra, Kratos, Oathkeeper)
- **Category:** self-hosted auth / OIDC + OAuth 2.0 server
- **License:** Apache 2.0
- **Last updated:** 2026-05-15
- **Analyst:** Self-Hosted Auth Analyst
- **Reviewer:** Security Reviewer (accepted 2026-05-15)
- **Status:** accepted
- **Priority:** P0

## 2. One-paragraph overview

Ory ships three OSS servers: **Hydra** (stateless OAuth 2.0 / OIDC, OpenID-certified), **Kratos** (identity / sessions / recovery / registration), and **Oathkeeper** (zero-trust reverse-proxy decision point). Cure53 audited Hydra v1.4 (2019); responsive GitHub security disclosure. KeepSave does not need a federated IdP and is blocked from one by `docs/ROADMAP_NOT.md:22` — we care about Ory as a **reference implementation of three discrete patterns**: RS256 JWKS with key rotation (the empty-JWKS gap at `handlers_oauth.go:217` is the active P0), refresh-token rotation with replay detection, and Kratos's session-token revocation model (informs how the unwired `SessionToken` struct would be plumbed).

## 3. Architecture summary

Three-binary split, each with its own admin/public API and SQL store:

1. **Hydra** — stateless OAuth 2.0 / OIDC; mints access (opaque or JWT) + refresh tokens; exposes `/.well-known/openid-configuration` and `/.well-known/jwks.json`. Keys live in `hydra_jwk`; rotation is a CLI command. Login + consent UI delegated to the integrator.
2. **Kratos** — identity store + session manager; users, Argon2id password hashing, self-service flows, server-tracked `sessions` table. Revocation = row update + cache invalidation.
3. **Oathkeeper** — proxy / decision-point; declarative `access_rules.json` running authenticator / authorizer / mutator chains before upstream forward.

**KeepSave maps to:** Hydra's JWKS + key-rotation surface and refresh-token rotation; Kratos's session-revocation shape. **Ignored:** Kratos identity UI / self-service, Oathkeeper deployment, Ory Cloud. Pattern-borrowing, not binary-deployment.

## 4. Security model

Auth primitives Ory publishes (docs retrieved 2026-05-15 via search summary):

- **Hydra signing:** RS256 default; ES256 / EdDSA opt-in. Keys rotated via `hydra create jwks`; multiple `kid`-keyed keys active simultaneously — publish new, wait for clients, flip signing default, retire old.
- **Refresh-token rotation:** rotates on every refresh exchange and **detects reuse** — replaying invalidates the entire token family (RFC 8725 / OAuth BCP recommendation, implemented).
- **PKCE:** S256 enforced for public clients; `plain` deprecated (RFC 7636).
- **Kratos session revocation:** server-tracked in Postgres; transactional update of `active` column. Bearer is opaque, so no denylist-vs-JWT problem.
- **Trust-boundary stance:** private-key compromise = total compromise; JWKS rotation is the only mitigation.

CVE history (last 24 months, NVD + GHSA, retrieved 2026-05-15):

- **CVE-2025-46337 / GHSA-7q6x-r5pq-pcqh** (Hydra, 2025-05) — open-redirect via crafted `post_logout_redirect_uri`. Tangential, but evidence that redirect-URI validation is a recurring CVE class.
- **GHSA-2grw-9pmq-r7c4** (Kratos, 2024) — account-takeover via login-flow race. Not applicable; we don't adopt Kratos self-service flows.
- **GHSA-9j76-7895-fc6f** (Hydra, 2023) — large-JWT DoS. Informs §7 — token-size guard required.

Public audit: **Cure53 audit of Hydra v1.4** (2019). Hydra is **OpenID Certified** for OP Basic / Implicit / Hybrid / Config profiles.

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
| Kratos identity UI (login / registration / recovery / verification / settings) | Out of scope per `docs/ROADMAP_NOT.md:22` — SSO/SAML/OIDC dashboard auth blocked until two customers ask. |
| Kratos Argon2id password-hashing pipeline | Separate work item if it surfaces; not the gap this dossier informs. |
| Oathkeeper as a deployed proxy | Heavyweight for Gin-only KeepSave; `middleware.go:88-120` is already the simpler shape. |
| Ory Cloud multi-tenant projects | Multi-tenancy is Phase B (`FOLLOWUPS.md:155`); won't borrow tenant isolation from a SaaS product before then. |
| Hydra delegated login + consent UI | We are not a third-party OAuth provider; `handlers_oauth.go` serves first-party clients only. |
| Kratos webhook system for flow side-effects | Audit log, not webhooks, is our cross-system signalling channel. |

## 6. Adapt candidates

Three pattern adoptions. None requires deploying Ory.

1. **RS256 + JWKS endpoint with `kid`-keyed key rotation.** Replace `auth.go:39`'s HS256 with RS256; populate `handlers_oauth.go:217` with the real key set. Two keys active at once (`kid` distinguishes); rotation publishes new, waits an expiration window, flips signing default, retires old. Borrowed from Hydra's `hydra_jwk` table + `hydra create jwks` flow.
2. **Refresh-token rotation with replay-detection family invalidation.** A new `refresh_tokens` table tracks `(token_hash, family_id, parent_id, used_at)`. Issuing consumes the row; presenting an already-consumed token invalidates every row in the family. OAuth Security BCP behaviour, implemented in Hydra. KeepSave has no refresh flow today — adopting means *adding* a flow.
3. **Kratos-style session-token denylist with TTL pruning.** Wire the existing `SessionToken` struct (`models.go:352-362`); middleware (`middleware.go:75-80`) consults the table by `jti`. Background sweeper re-uses `startAuditLogPruner` (`FOLLOWUPS.md:11`). RFC 7009 wire shape; Kratos storage shape.

Oathkeeper as a deployed decision-point is **not** an adapt candidate — too heavyweight; `access_rules.json` would replicate, not improve on, `middleware.go:88-120`.

## 7. Pros / cons of adapting

### Candidate 1: RS256 + JWKS rotation

- **Pros:** Closes the live P0 at `handlers_oauth.go:217` (endpoint advertised in `OpenIDConfiguration` at `:233` but serves empty `keys: []`). Enables verifier separation — MCP gateway and AI agents validate without holding `JWT_SECRET`. Hydra is OpenID-certified for this shape.
- **Cons (operational):** Adds a private-key custody requirement (one more KMS-managed asset alongside ADR-0004's master key). `RUNBOOK.md` rotation procedure required. `ValidateToken` (`auth.go:47-62`) must accept both algorithms during cutover.
- **Cons (security):** Algorithm-mixed validation is the source of multiple historical CVEs (`none`-algorithm, `HS256-with-RSA-public-key`). Mitigation: explicit `alg`-per-`kid` allowlist, no `jwk.Parse` fallback. CVE-2025-46337 confirms even well-trodden OAuth surfaces ship CVEs.

### Candidate 2: Refresh-token rotation with replay detection

- **Pros:** Enables shorter-lived access tokens (today's JWT is 24h per `auth.go:25` — long enough that "no denylist" is a live concern). Family invalidation is the OAuth Security BCP recommendation. Reduces residual on `THREAT_MODEL.md:84` row T from Medium toward Low.
- **Cons (operational):** New table, new endpoint, new rate-limit surface (refresh attempts must be throttled per-family to prevent next-token harvest). Every existing client must learn to refresh.
- **Cons (security):** Family-invalidation is itself a DoS vector — an attacker who briefly sees a refresh token can replay it to lock out the legitimate user. Requires audit-logged replay events + recovery path. Net residual lower than today; tradeoff is real.

### Candidate 3: Session-token denylist with TTL pruning

- **Pros:** Directly informs the Phase B JWT-denylist trigger (`FOLLOWUPS.md:167`). `SessionToken` struct exists (`models.go:352-362`); schema work is minimal. Pruner pattern is already in production for audit log (`FOLLOWUPS.md:11`).
- **Cons (operational):** Every authenticated request consumes one extra Postgres lookup. In-memory LRU cache keyed by `jti` is mandatory or middleware p99 doubles. Pruner adds a new background goroutine + failure mode.
- **Cons (security):** Stale cache means a revoked token works for cache TTL. Mitigation: cache TTL ≤ 60s + explicit invalidation on revoke (LISTEN/NOTIFY), or accept the bound.

## 8. Validation evidence

Mix of RFC, audit, CVE, and OpenID certification. No load-bearing vendor-blog claims.

- **RFC 7515 / 7517 / 7519** — wire formats for Candidate 1 (JWS, JWKS, JWT).
- **RFC 7636 + RFC 8252** — PKCE S256 enforcement for native / public clients.
- **RFC 7009** — Candidate 3's revocation endpoint shape.
- **RFC 8725 / OAuth Security BCP** — refresh rotation + replay detection (Candidate 2's load-bearing source).
- **Cure53 audit of Ory Hydra v1.4** (2019, `ory.sh/blog/hydra-cure53-security-audit`, retrieved 2026-05-15) — pointer to public audit; the audit PDF is the load-bearing artefact.
- **OpenID Foundation Certified Implementations** (`openid.net/certification/`, retrieved 2026-05-15) — Hydra OP Basic / Implicit / Hybrid / Config.
- **CVE-2025-46337 / GHSA-7q6x-r5pq-pcqh** (Hydra open-redirect, 2025-05) — recurring CVE class on redirect-URI validation.
- **GHSA-2grw-9pmq-r7c4** (Kratos race, 2024) — reinforces §5 decision not to adopt self-service flows.
- **GHSA-9j76-7895-fc6f** (Hydra large-JWT DoS, 2023) — informs §7 Candidate 1 cons (token-size guard).

Vendor docs supplementary: `ory.sh/docs/hydra/{before-oauth2,jwks}`, `…/kratos/concepts/sessions` — retrieved 2026-05-15 via search summary. Load-bearing claims rest on the RFCs / CVEs above, not vendor docs.

## 9. Threat-model implications

Two STRIDE rows in `docs/THREAT_MODEL.md` §2 (Authentication):

- **Row S "Forged JWT"** at `docs/THREAT_MODEL.md:82`: *"HS256 with secret only on server; signature verified … Residual: Low"*. Candidate 1 (RS256 + JWKS) **does not widen** the trust boundary — private key remains server-only. It *narrows* a different one: external verifiers (MCP gateway, AI agents) can validate without holding `JWT_SECRET`. Residual stays Low; verifier-distribution surface improves.
- **Row T "Token replay after revocation"** at `docs/THREAT_MODEL.md:84`: *"API key revocation = row delete; JWT expires in 24h, **no denylist**. Residual: Medium"*. Candidates 2+3 directly attack this. Candidate 2 replaces 24-hour standing access with short-lived access + family-invalidated refresh; Candidate 3 gives pre-expiry revocation. Combined residual → Low.

No new entity enters the trust boundary. `refresh_tokens` and `session_tokens` are internal Postgres state under existing repository / audit invariants.

## 10. Verdict

**Candidate 1 (RS256 + JWKS rotation): `adopt-now`.** Trigger fired — `handlers_oauth.go:217` serves empty `keys: []` while `:233` advertises a `jwks_uri`. Live P0 per `docs/research/README.md:11`: *"the OAuth 2.0 server in `backend/internal/api/handlers_oauth.go` exposes an empty JWKS and no RS256"*. Adoption requires a Type-1 ADR (auth-adjacent) per `CLAUDE.md` Decision-classes table. Security Engineer veto applies.

**Candidate 2 (refresh-token rotation): `adopt-when-trigger-fires`.** Trigger quoted verbatim from `docs/FOLLOWUPS.md:167`:

> *"**JWT denylist for pre-expiry revocation** (ADR-0002 §Open Questions). Build when a customer SLA requires it."*

Refresh-token rotation is the *upstream* of that denylist conversation. Adopt when the Phase B JWT-denylist item (under section header at `FOLLOWUPS.md:162`) is reopened.

**Candidate 3 (session denylist): `adopt-when-trigger-fires`.** Same trigger as Candidate 2 — `docs/FOLLOWUPS.md:167`. Candidates 2 and 3 land in the same ADR.

**Oathkeeper-style deployed decision point: `reject`.** Too heavyweight for KeepSave's scale; `middleware.go:88-120` is the right shape. Recorded so the question isn't re-asked.

## 11. Rollback if adopted

**Candidate 1 (RS256 + JWKS):** Mixed-algorithm validation is the rollback shape — cutover keeps HS256 verifier alive while RS256 is published; flipping back sets signing default to HS256 and continues to verify both. True rollback (deleting RS256-signed tokens) is **not possible** — holders retain them until natural expiry (≤ 24h, `auth.go:25`). Runbook: `kubectl rollout` with `JWT_ALG=HS256`; KMS private key retained for forensic verification.

**Candidate 2 (refresh-token rotation):** Drop `refresh_tokens` writes; rows become inert; `/oauth/token` `grant_type=refresh_token` returns `unsupported_grant_type`. Clients fall back to re-login. No data destroyed.

**Candidate 3 (session denylist):** Stop reading the table in middleware; revoked sessions resurrect for their remaining TTL (≤ 24h regression). Pruner stops cleanly; table growth resumes but is bounded.

## 12. Won't-break-our-system claim

Integrator-facing contracts: (a) `Authorization: Bearer <jwt>` from the dashboard, (b) `X-API-Key: ks_...` from AI agents. API-key path runs first and short-circuits before JWT validation (`middleware.go:88-120`).

- **Invariant 1 — API-key auth untouched.** `middleware.go:88-115` runs SHA-256 hashed-lookup before falling through to JWT at `:118`. API keys never transit the JWT validator. Verified by code-reading.
- **Invariant 2 — JWT issuance contract unchanged.** `GenerateToken` (`auth.go:29-45`) returns a string token; callers don't inspect `alg`. RS256 changes signature bytes, not envelope. Sole `Claims.UserID` consumer at `middleware.go:82` is algorithm-agnostic. Verified by code-reading.
- **Invariant 3 — Tests green during dual-algorithm window.** `ValidateToken` (`auth.go:47-62`) accepts both algorithms during Candidate 1 cutover; the type assertion at `:50-52` widens to an allowlist. Existing HS256 fixtures continue to pass. Verified by code-reading.
- **Invariant 4 — `SessionToken` is dormant; wiring is additive.** `models.go:352-362` has no callers (grep 2026-05-15). New repository + middleware lookup modifies no existing path. Verified by grep across `backend/internal/`.
- **Invariant 5 — Audit taxonomy extends cleanly.** New refresh-token and denylist events don't break existing assertions in `docs/AUDIT_LOG_COVERAGE.md`. Verified by reading the taxonomy.

Security Reviewer veto applies. Suggested checks: (a) algorithm-confusion guard is explicit `alg`-per-`kid` allowlist, not generic `jwk.Parse`; (b) refresh-token replay handler emits an audit event and surfaces a recoverable error; (c) denylist cache TTL ≤ 60s with explicit invalidation on revoke.

## 13. References

All retrieved 2026-05-15.

- RFCs: 7515 (JWS), 7517 (JWKS), 7519 (JWT), 7636 (PKCE), 8252 (OAuth Native Apps), 7009 (Token Revocation), 8725 (OAuth BCP) — `https://www.rfc-editor.org/rfc/rfc<NNNN>`.
- CVE-2025-46337 / GHSA-7q6x-r5pq-pcqh (Hydra open-redirect): `https://github.com/ory/hydra/security/advisories/GHSA-7q6x-r5pq-pcqh`.
- GHSA-2grw-9pmq-r7c4 (Kratos login race): `https://github.com/ory/kratos/security/advisories`.
- GHSA-9j76-7895-fc6f (Hydra large-JWT DoS): `https://github.com/ory/hydra/security/advisories`.
- Cure53 audit of Hydra v1.4 (2019): `https://www.ory.sh/blog/hydra-cure53-security-audit` (vendor-blog pointer; audit PDF is the load-bearing artefact).
- OpenID Certified Implementations: `https://openid.net/certification/`.
- Ory docs (supplementary, via search summary): `https://www.ory.sh/docs/hydra/jwks`, `…/concepts/before-oauth2`, `…/kratos/concepts/sessions`.
- Source repos: `https://github.com/ory/{hydra,kratos,oathkeeper}`.

## Security Reviewer notes

**Verdict:** accepted-with-changes (retrofitted inline). Veto **not** exercised on §9, §10, or §12. Candidate 1 adoption still requires a Type-1 ADR + Security Engineer sign-off at the ADR stage.

**Refs spot-checked (Read tool, 2026-05-15) — all resolve:**

- `handlers_oauth.go:216-218` — JWKS returns `{"keys": [], "note": "...HS256...RS256 support is planned."}`; `jwks_uri` advertised at `:233`. §5 row 2 and §10 Candidate 1 framing accurate.
- `auth.go:11-62` — `Claims` 11-15; `NewJWTService` 22-27 (24h expiry at 25); `GenerateToken` 29-45 using `SigningMethodHS256` at 39; `ValidateToken` 47-62 with HMAC-only allowlist at 50-52. Every sub-range checks out.
- `middleware.go:88-120` — API-key short-circuit at 91-115 (lookup 93-94, context set 107-112, `c.Next(); return` 113-114); JWT fallback at 118. **§12 Invariant 1 confirmed.**
- `apikey.go:11-26` — SHA-256 hashing as cited.
- `models.go:352-362` — `SessionToken` struct as described; `grep -rn SessionToken backend/internal/` returns only the model definition. §12 Invariant 4 confirmed — wiring is genuinely additive.

**§9 STRIDE rows quoted accurately:**

- `THREAT_MODEL.md:82` (§2/S — Forged JWT): `HS256 with secret only on server; signature verified | Low`. Dossier quote with ellipsis matches. Trust-boundary claim correct — RS256 *removes* HS256 secret from external verifiers; no new entity enters the boundary.
- `THREAT_MODEL.md:84` (§2/T — Token replay after revocation): `API key revocation = row delete; JWT expires in 24h, **no denylist** | Medium`. Verbatim match. Residual Medium → Low under Candidates 2+3 is the correct direction.

**§10 trigger verification — citation corrected:** Original draft cited `FOLLOWUPS.md:160` for the JWT-denylist trigger. Line 160 is the `---` rule separating Phase A from Phase B; the trigger lives at **line 167** under `## Open — Phase B` (header at 162). Corrected inline. Original "FU #5 is reopened" prose was also wrong — FU #5 at line 118 is "Approver-cannot-be-requester", unrelated. Reworded to "Phase B JWT-denylist item is reopened".

**Audit cross-references applied (`docs/audits/SECURITY_AUDIT_2026-05-15.md`):**

Two relevant audit findings carry into the eventual ADR:

1. **Audit §5.2 → new `THREAT_MODEL.md §2/E` row "JWT lacks project bind"** (residual High today). **Orthogonal to Candidates 1-3** — RS256/JWKS, refresh rotation, session denylist do not address project-scope binding. ADR Drafter must not frame Candidate 1 as fixing this; recommend the same ADR adds a `project_ids` / membership claim into the JWT envelope. Audit A01-F2 (P1 Critical) is the API-key-side mirror — equally untouched by Ory patterns.
2. **Audit A04-F4 "OAuth public-client doesn't require PKCE" (P2 High).** §4 already notes Ory enforces PKCE S256 — the canonical fix for `oauth_service.go:109-121` PKCE-bypass and `handlers_oauth.go:238` `plain` advertisement. An honest §6 expansion would add **Candidate 4: enforce PKCE S256 for public OAuth clients** (`adopt-now`). Flagged for next revision; expanding §10 verdicts beyond the analyst's scope exceeds an accept-with-changes pass.

**Inline changes during acceptance:** (1) §1 status `draft` → `accepted`. (2) §10 trigger `FOLLOWUPS.md:160` → `:167` (twice). (3) §10 "FU #5 is reopened" → "Phase B JWT-denylist item is reopened". (4) §2-§5 prose condensed to fit 2700-word cap.

**Non-blocking observations (for ADR Drafter):**

- Algorithm-confusion: ADR must include a test fixture presenting an HS256-signed token using the RS256 public key as HMAC secret, and assert rejection.
- Candidate 3 cache invalidation on revoke must itself be audit-logged per `docs/AUDIT_LOG_COVERAGE.md`; pruner sweeps do not need audit (volume).
- Candidate 1 rollback should mirror Pomerium's sentinel pattern — a `JWT_ALG_VERIFY` allowlist env var cycleable without code change.
- Vendor-doc fallback matches template guidance; load-bearing claims rest on RFCs + Cure53 audit pointer, not vendor blog excerpts.

No reference failed to check out.
