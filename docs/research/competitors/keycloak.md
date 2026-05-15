# Competitor Dossier — Keycloak

## 1. Header

- **Vendor:** Keycloak (CNCF incubation, originally Red Hat)
- **Category:** identity provider / OIDC + SAML
- **License:** Apache 2.0
- **Last updated:** 2026-05-15
- **Analyst:** Identity Provider Analyst
- **Reviewer:** Security Reviewer (pending)
- **Status:** draft
- **Priority:** P0 (tied to dormant `SSOConfig` model and `ROADMAP_NOT.md` #2 trigger readiness)

## 2. One-paragraph overview

Keycloak is the canonical open-source OIDC + SAML identity provider: realms, clients, identity brokering, LDAP/AD federation, REST admin API. We do **not** intend to deploy Keycloak — KeepSave already has its own OAuth server (`handlers_oauth.go`) and HS256 dashboard JWT auth (`auth/auth.go:39`). We care about Keycloak as a **reference shape** for when `docs/ROADMAP_NOT.md:19-22` triggers: KeepSave has a `SSOConfig` model with CRUD scaffolding (`models/models.go:277-290`, `service/sso_service.go`, `migrations/005_phase7_12.sql:4-17`) but **no protocol flow** — no auth-code redirect, no upstream JWKS fetch, no JIT provisioning, no SCIM. This dossier captures what we would adopt the day the trigger fires, so the next engineer does not re-research from cold.

## 3. Architecture summary

Keycloak is a JVM monolith + relational DB exposing: **realms** (isolation boundary; per-realm users / clients / signing keys), **clients** (OIDC RP or SAML SP, public-with-PKCE or confidential), **identity brokering** (delegate auth to upstream IdPs), **user federation** (LDAP/AD), **token mappers** (declarative claim transforms), and an **admin REST API**.

Two flows matter:

- **Authorization code + PKCE (RFC 7636)** — browser → Keycloak login → redirect to `LoginPage.tsx` with `code` → backend exchanges for `id_token` → backend verifies via JWKS at `/.well-known/openid-configuration` (RFC 8414).
- **JIT user provisioning on first login** — `sub`/`email` claims materialised into a `users` row + org membership.

We touch realms (as `Organization`), clients (we are the RP), and the auth-code flow. We ignore identity brokering, user federation, token mappers, and the admin UI — we integrate *to* an external Keycloak (or any RFC-conformant OP), we do not deploy one.

## 4. Security model

Keycloak publishes (docs retrieved 2026-05-15 via search; direct WebFetch returned 403 — §8 triangulation applied):

- **Token signing:** RS256 default; ES256/PS256 optional. Per-realm key rotation; JWKS advertises active + recently-retired `kid`s so verifiers stay green during rotation.
- **Session management:** SSO cookie + refresh-token rotation; per-realm idle/absolute timeouts. Introspection (RFC 7662) available; we would not use it — we want stateless JWT verify.
- **SAML assertion signing/encryption:** XML-DSig per-realm key. **XML signature wrapping** is a recurring CVE class across SAML implementations — load-bearing reason to land OIDC-only in the first wave even if SAML is asked for.
- **Brute-force + password policy:** built-in account lockout, per-IP throttle, declarative password policy (length, history, HIBP plugin).
- **Trust-boundary stance:** integrating as an OIDC RP adds the customer's OP to our trust boundary (we trust its JWKS and `iss`). Compromise of the OP = compromise of all federated identities. See §9.

## 5. KeepSave-comparable surface

Only rows with a real analog (existing, missing, or stub). Required file:line citations.

| Keycloak concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Realm (isolation boundary; users, clients, signing keys) | `Organization` + `OrgMember` (no per-org signing keys) | `backend/internal/models/models.go:219-227` |
| Client (OIDC RP / SAML SP registered to a realm) | `OAuthClient` (we are an OP; clients here are *downstream*) | `backend/internal/models/oauth.go:10-25` |
| Identity provider broker (OIDC/SAML upstream config) | `SSOConfig` — schema + CRUD wired, **protocol flow absent** | `models/models.go:277-290`; `service/sso_service.go:25-60`; `migrations/005_phase7_12.sql:4-17` |
| Role mappers (claims → app roles) | `OrgMember.Role` string; no claim-mapping layer | `backend/internal/models/models.go:224` |
| JWKS endpoint with `kid` rotation | Handler exists, returns empty `keys: []`, HS256-only | `backend/internal/api/handlers_oauth.go:216-218` |
| OIDC discovery (`/.well-known/openid-configuration`) | Discovery handler present, points to our JWKS | `backend/internal/api/handlers_oauth.go:233` |
| Authorization-code + PKCE login flow | LoginPage has an "SSO" tab with disabled placeholder buttons; no backing route | `frontend/src/pages/LoginPage.tsx:104-117, 192-207` |
| Direct password / API-key login | `apiLogin(email, password)` + HS256 JWT in `localStorage` | `LoginPage.tsx:32-33`; `auth/auth.go:29-45` |
| Per-request token validation middleware | JWT validate (no denylist; FU #5) | `auth/auth.go:47-62`; `FOLLOWUPS.md:160` |

**Concepts deliberately not adopted** (Keycloak has substantial surface area irrelevant to KeepSave; we are adopting patterns, not deploying Keycloak):

| Their concept | Reason we don't adopt |
|---|---|
| Keycloak server deployment itself (JVM monolith + DB) | Out of scope. We do not host an IdP; when the trigger fires we integrate *to* a customer-hosted OP (theirs, not ours) as an OIDC client. |
| User federation (LDAP/AD directory connectors) | KeepSave does not federate user directories. The OIDC flow surfaces `sub`/`email` claims; that is sufficient. |
| Token mappers / claim transformation engine | Premature. Role assignment will be either a JIT default (`viewer`) or admin-set after first login; no engine needed in v1. |
| SAML 2.0 protocol support | XML signature wrapping is a recurring CVE class. We would explicitly land OIDC-only in the first wave even if the asking customer wants SAML — and document that constraint in the ADR. |
| Admin REST API for managing realms / clients / users | We integrate *as* an RP; we do not manage Keycloak's realms from KeepSave. |
| Keycloak's own session-cookie issuance | KeepSave already issues its own session JWT after login; the OIDC `id_token` is consumed once, never persisted. |

## 6. Adapt candidates

Pattern adoptions only. These are what we would build the day `docs/ROADMAP_NOT.md:19-22` triggers. Scope is "two customers can log in via their OIDC provider", not "Keycloak feature parity".

1. **OIDC auth-code + PKCE as the only protocol path.** Backend gains `GET /api/v1/auth/sso/:provider/start` (issues `state` + `code_verifier`, redirects to upstream `authorization_endpoint`) and `GET /callback` (exchanges code, verifies `id_token` via upstream JWKS, mints a KeepSave HS256 JWT identical to today's password login). The disabled `LoginPage.tsx:192-207` buttons become live. PKCE mandatory per RFC 8252 §8.1.
2. **`SSOConfig`-keyed JIT discovery.** Keep existing `SSOConfig` shape; add a runtime cache fetching `${issuer_url}/.well-known/openid-configuration` (RFC 8414) on first use per `(org_id, provider)`, refresh JWKS on `kid`-miss. No new schema — migration **already merged** (`005_phase7_12.sql:4-17`).
3. **`id_token` verification with audience + issuer binding.** Verify against discovered JWKS, enforcing `iss == SSOConfig.issuer_url`, `aud == SSOConfig.client_id`, `exp` valid, `nonce` matches `state`-stashed value. CVE-prone code path (§8); test coverage mandatory.
4. **JIT user provisioning bound to org.** On first successful callback: upsert `users` keyed on `(iss, sub)` (not email — email is mutable), insert `OrgMember` with default role `viewer`. Matches Keycloak's "first broker login" minus the broker.

Out of scope even on trigger fire: refresh-token rotation against the upstream OP (we mint our own short-lived JWT, user re-auths); SCIM; role-mapping engine; SAML; identity brokering (we are the RP, not a broker).

## 7. Pros / cons of adapting

### Candidate 1: OIDC code + PKCE flow

- **Pros:** Closes ROADMAP_NOT #2 the day it triggers with the smallest surface. Reuses existing JWT minting (`auth.go:29-45`) and middleware — no second auth path through the rest of the system.
- **Cons (operational):** Two new unauthenticated endpoints (`/start`, `/callback`) must rate-limit per-IP and per-`state` to prevent CSRF / open-redirect abuse. `state` storage needs a TTL'd signed cookie (cheaper than a new table).
- **Cons (security):** `id_token` validation is a known CVE class across all OIDC implementations (Keycloak itself has had `iss`/`aud` bypass entries — §8). Mitigation: negative-auth tests asserting wrong-issuer, wrong-audience, expired, bad-signature, replayed-nonce all return `401` and emit audit events per `docs/AUDIT_LOG_COVERAGE.md`.

### Candidate 2: `SSOConfig`-keyed JIT discovery

- **Pros:** Schema exists multi-DB today (`postgres` / `mysql` / `sqlite` migrations all present). No schema migration at trigger-fire — the point of trigger readiness.
- **Cons (operational):** Misconfigured `issuer_url` could hot-loop discovery fetches. Mitigation: 60s negative cache; audit event on first failure per `(org_id, provider)`.
- **Cons (security):** Org admins supply `issuer_url`. Permitting `http://` or self-signed certs would widen the boundary further. Mitigation: schema-validate to `https://` only at write-time in `handlers_enterprise.go:31-57`; CI lint.

### Candidate 3: `id_token` verification with audience + issuer binding

- **Pros:** RFC 7519 + OIDC Core §3.1.3.7 are unambiguous; small implementation; the security-critical code we get right once and extend to additional providers without re-litigating.
- **Cons (operational):** `none material` — verification is local; only new dependency is the JWKS fetch already in Candidate 2.
- **Cons (security):** JWT library choice dominates risk. We already vendor `github.com/golang-jwt/jwt/v5` (`auth/auth.go:7`); reuse it. Library swap would itself be a Type-1 ADR.

### Candidate 4: JIT user provisioning bound to org

- **Pros:** Keying on `(iss, sub)` instead of email avoids the account-takeover-via-email-change class. Default role `viewer` matches the existing org-membership model (`models.go:224`).
- **Cons (operational):** Multi-org first-login UX needs an org-selection step. Mitigation: defer multi-org-on-login until a customer asks (mirror ROADMAP_NOT discipline inside the feature).
- **Cons (security):** `(iss, sub)` keying assumes **the upstream OP's `sub` is stable and uniquely identifies a human**. Holds for Keycloak / Auth0 / Okta / Google Workspace; may not for a customer-built OP. Mitigation: document the warrant in the ADR; reject changing `issuer_url` on an already-used `SSOConfig` (would orphan all `(iss, sub)`-keyed users).

## 8. Validation evidence

Vendor docs (Keycloak): `https://www.keycloak.org/docs/latest/server_admin/` — direct WebFetch **403 (unreachable via tooling 2026-05-15)**; cited for traceability. Load-bearing claims rest on non-vendor sources.

- **RFC 6749 (OAuth 2.0)** + **OpenID Connect Core 1.0** — normative for Candidate 1's auth-code flow.
- **RFC 7636 (PKCE)** + **RFC 8252 §8.1** — mandate PKCE for public clients.
- **RFC 8414 (Authorization Server Metadata)** — the discovery doc shape used by Candidate 2.
- **OWASP OIDC Cheat Sheet** — codifies `state`/`nonce`/`iss`/`aud` validation that Candidate 3 implements.
- **Keycloak CVEs (last 24 months, relevance-picked):**
  - **CVE-2024-1132** (CVSS 8.1) — path-traversal in admin endpoint, fixed 24.0.3. Tangential (we do not host Keycloak), but confirms a responsive disclosure program — closes the "CVE absence misread as safe" risk from `docs/research/README.md:111`.
  - **CVE-2023-6927** (CVSS 6.1) — open redirect via wildcard `redirect_uri`. **Directly relevant:** Candidate 1's `/callback` redirect must be **fixed** to the dashboard origin, never user-supplied.
  - **CVE-2023-0264** (CVSS 6.5) — incorrect authz in the Node.js adapter (JWT validation). Confirms `id_token`-validation is a CVE class; informs the §7 Candidate 1 negative-auth test list.
- **Keycloak source repository:** `https://github.com/keycloak/keycloak` (reachable via API) — used to cross-check that CVE fixes landed in release tags.

## 9. Threat-model implications

Maps to `docs/THREAT_MODEL.md` §2 "Authentication", row **S (Spoofing): Forged JWT**, line 82:

> *"Forged JWT — HS256 with secret only on server; signature verified — `auth/auth.go:39`, `api/middleware.go:59-100` — Residual: Low"*

The current Low residual assumes a **single issuer** (KeepSave itself with a server-held HS256 secret). Adopting Candidates 1–4 **widens** the boundary: a second JWT class (the upstream `id_token`) enters the system, signed by a key KeepSave does not control. The new entity is **the customer-configured upstream OP** identified by `SSOConfig.issuer_url`.

The widening is bounded: the `id_token` is consumed *once* at `/callback` and never persisted. KeepSave still mints its own HS256 JWT (`auth.go:29-45`); middleware is unchanged. Only `/callback` crosses the new boundary. Residual after adoption: **Medium** at `/callback`, Low post-issuance — Candidate 3's negative-auth tests bound the Medium.

Also touched: §2 row **R (Repudiation): Login attempts not audited**, line 85 (Medium today). Candidates 1 and 4 must emit audit events (`auth.sso.start`, `auth.sso.callback.success/failure`, `auth.sso.user_provisioned`) per `docs/AUDIT_LOG_COVERAGE.md` — adopting SSO without auditing would *worsen* this row.

## 10. Verdict

**`adopt-when-trigger-fires`** for all four candidates. Single trigger source.

Quoted verbatim from `docs/ROADMAP_NOT.md:19-22`:

> *"### 2. SSO / SAML / OIDC for end-user auth*
> *- **What we are not building:** federated login for dashboard users.*
> *- **Why:** no customer has asked. Building it before the demand signal arrives means we'll build the wrong shape.*
> *- **Trigger to revisit:** when two customers ask in the same quarter, or when one enterprise prospect makes it a deal-blocker."*

**Finding on trigger precision:** the trigger is well-bounded on customer count ("two in the same quarter, or one deal-blocker") but **silent on protocol** — "SSO / SAML / OIDC" is one bucket. A SAML-asking customer and an OIDC-asking customer count together under the literal text, yet §6/§7 argue OIDC-first / SAML-deferred. **Recommendation:** when the trigger fires, the ADR clarifies "two customers asking for the **same** protocol" and lands OIDC-only unless both ask for SAML. Promote to `docs/FOLLOWUPS.md` in the same PR as the ADR, not now.

Nothing in this dossier is `adopt-now`: there is no path that lands code without contradicting `ROADMAP_NOT`. The existing `SSOConfig` CRUD scaffolding is already merged; we neither remove nor extend it until trigger.

Security Reviewer veto applies per `docs/ROLES.md` (touches `internal/auth`).

## 11. Rollback if adopted

The trigger-fire implementation is additive: two new routes (`/start`, `/callback`), one runtime discovery cache, the JIT user-upsert path. No schema change (migration already exists).

1. **Feature-flag the routes** from day one (env `KEEPSAVE_SSO_ENABLED`, default `false`). Rollback: set flag false, redeploy; the placeholder SSO buttons in `LoginPage.tsx` revert to disabled.
2. **JIT-provisioned users persist** but cannot log in (no `password_hash`, SSO disabled). This is the data that does NOT survive a clean rollback — runbook step: either delete the orphaned users or run a recovery flow.
3. **`SSOConfig` rows persist** (additive migration). Inert with the flag off.
4. **Audit events already written** are not removed.

True rollback to "no JIT users ever existed" is impossible without deleting user rows (which destroys audit attribution). The forward-fix (re-enable, or migrate JIT users to password-login via reset email) is cheaper.

## 12. Won't-break-our-system claim

Integrator and dashboard-user contracts are unchanged until the trigger fires. Verified by code-reading at the file:lines cited.

- **Invariant 1 — password login unaffected.** `apiLogin(email, password)` at `frontend/src/pages/LoginPage.tsx:32-33` hits the existing route; SSO candidates add new routes, never modify this one. The `mode === 'password'` form (`LoginPage.tsx:121-167`) submits unchanged. Verified by code-reading.
- **Invariant 2 — API-key auth unaffected.** Machine identity uses `auth/apikey.go` (SHA-256, scope-bound per `models/models.go:51-60`). SSO candidates do not touch this path. Verified by reading `apikey.go` and middleware.
- **Invariant 3 — existing `SSOConfig` CRUD still works (and remains inert).** Handlers at `handlers_enterprise.go:30-91` and routes at `router.go:175-177` accept writes today; the encrypted client secret is stored but never read by a protocol flow. Candidates 1–4 make those rows *load-bearing* but do not change their schema. Verified by reading `sso_service.go:25-60` and migration `005_phase7_12.sql:4-17`.
- **Invariant 4 — JWT middleware contract unchanged.** Middleware (per threat-model §2) validates KeepSave-issued HS256 JWTs only. The upstream `id_token` is exchanged at `/callback` for a KeepSave JWT; middleware never sees the upstream token. Verified by tracing against `auth.go:47-62`.
- **Invariant 5 — flag default-off means trigger readiness is zero-risk.** Until `KEEPSAVE_SSO_ENABLED=true` and a `SSOConfig` is present, `/start` and `/callback` return `404`. No code path exercised. Verified by §11 design.

Security Reviewer veto applies. Suggested checks: (a) `iss` and `aud` compared with strict equality against `SSOConfig.issuer_url` / `SSOConfig.client_id`, not `Contains`; (b) `state` and `nonce` bound together and TTL'd; (c) dashboard `redirect_uri` is **fixed**, never user-supplied (CVE-2023-6927 mitigation); (d) every `/callback` path emits an audit event per `AUDIT_LOG_COVERAGE.md`, including failures.

## 13. References

All retrieved 2026-05-15.

- RFC 6749 (OAuth 2.0): `https://www.rfc-editor.org/rfc/rfc6749`
- RFC 7519 (JWT): `https://www.rfc-editor.org/rfc/rfc7519`
- RFC 7636 (PKCE): `https://www.rfc-editor.org/rfc/rfc7636`
- RFC 8252 (OAuth 2.0 for Native Apps): `https://www.rfc-editor.org/rfc/rfc8252`
- RFC 8414 (Authorization Server Metadata): `https://www.rfc-editor.org/rfc/rfc8414`
- OpenID Connect Core 1.0: `https://openid.net/specs/openid-connect-core-1_0.html`
- OWASP OIDC Cheat Sheet: `https://cheatsheetseries.owasp.org/cheatsheets/OAuth2_OIDC_Cheat_Sheet.html`
- Keycloak docs (server admin): `https://www.keycloak.org/docs/latest/server_admin/` *(unreachable via tooling — vendor 403; cited for traceability)*
- Keycloak source: `https://github.com/keycloak/keycloak`
- CVE-2024-1132 (path traversal): `https://nvd.nist.gov/vuln/detail/CVE-2024-1132`
- CVE-2023-6927 (open redirect / wildcard `redirect_uri`): `https://nvd.nist.gov/vuln/detail/CVE-2023-6927`
- CVE-2023-0264 (Node.js adapter JWT validation): `https://nvd.nist.gov/vuln/detail/CVE-2023-0264`
