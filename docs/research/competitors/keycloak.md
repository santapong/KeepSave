# Competitor Dossier — Keycloak

## 1. Header

- **Vendor:** Keycloak (CNCF incubation, originally Red Hat)
- **Category:** identity provider / OIDC + SAML
- **License:** Apache 2.0
- **Last updated:** 2026-05-15
- **Analyst:** Identity Provider Analyst
- **Reviewer:** Security Reviewer (accepted 2026-05-15)
- **Status:** accepted
- **Priority:** P0 (tied to dormant `SSOConfig` model and `ROADMAP_NOT.md` #2 trigger readiness)

## 2. One-paragraph overview

Keycloak is the canonical open-source OIDC + SAML identity provider: realms, clients, identity brokering, LDAP/AD federation, REST admin API. We do **not** intend to deploy Keycloak — KeepSave already has its own OAuth server (`handlers_oauth.go`) and HS256 dashboard JWT auth (`auth/auth.go:39`). We care about Keycloak as a **reference shape** for when `docs/ROADMAP_NOT.md:19-22` triggers: KeepSave has a `SSOConfig` model with CRUD scaffolding (`models/models.go:277-290`, `service/sso_service.go`, `migrations/005_phase7_12.sql:4-17`) but **no protocol flow** — no auth-code redirect, no upstream JWKS fetch, no JIT provisioning, no SCIM. This dossier captures what we would adopt the day the trigger fires, so the next engineer does not re-research from cold.

## 3. Architecture summary

Keycloak is a JVM monolith + relational DB exposing **realms** (isolation boundary; per-realm users / clients / signing keys), **clients** (OIDC RP or SAML SP), **identity brokering**, **user federation** (LDAP/AD), **token mappers**, and an **admin REST API**.

Two flows matter: (1) **Auth-code + PKCE** (RFC 7636) — browser → Keycloak → redirect to `LoginPage.tsx` with `code` → backend exchanges for `id_token` → verifies via JWKS at `/.well-known/openid-configuration` (RFC 8414); (2) **JIT user provisioning** — `sub`/`email` claims materialised into a `users` row + org membership.

We touch realms (as `Organization`), clients (we are the RP), and the auth-code flow. We ignore identity brokering, user federation, token mappers, and the admin UI — we integrate *to* an external OP, never deploy one.

## 4. Security model

Keycloak publishes (docs retrieved 2026-05-15 via search; direct WebFetch 403 — §8 triangulation applied):

- **Token signing:** RS256 default; ES256/PS256 optional. Per-realm key rotation; JWKS advertises active + recently-retired `kid`s.
- **Session management:** SSO cookie + refresh-token rotation; introspection (RFC 7662) available but we want stateless JWT verify.
- **SAML signing/encryption:** XML-DSig per-realm. **XML signature wrapping** is a recurring CVE class — load-bearing reason to land OIDC-only in the first wave.
- **Brute-force + password policy:** built-in account lockout, per-IP throttle, declarative password policy.
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

**Concepts deliberately not adopted** (Keycloak has substantial surface area irrelevant to KeepSave; we are adopting patterns, not deploying it):

| Their concept | Reason we don't adopt |
|---|---|
| Keycloak server deployment (JVM monolith + DB) | Out of scope; we integrate *to* a customer-hosted OP as an OIDC client. |
| User federation (LDAP/AD connectors) | OIDC `sub`/`email` claims are sufficient. |
| Token mappers / claim-transformation engine | Premature; JIT default `viewer` + admin-set role suffices in v1. |
| SAML 2.0 | XML signature wrapping is a recurring CVE class; OIDC-only first wave per ADR. |
| Admin REST API (managing realms / clients) | We are the RP, not the OP admin. |
| Keycloak session-cookie issuance | KeepSave issues its own session JWT; the `id_token` is consumed once. |

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

Vendor docs: `https://www.keycloak.org/docs/latest/server_admin/` — direct WebFetch **403 (unreachable 2026-05-15)**; cited for traceability. Load-bearing claims rest on non-vendor sources.

- **RFC 6749 + OpenID Connect Core 1.0** — normative for Candidate 1.
- **RFC 7636 + RFC 8252 §8.1** — mandate PKCE for public clients.
- **RFC 8414** — discovery doc shape for Candidate 2.
- **OWASP OIDC Cheat Sheet** — codifies `state`/`nonce`/`iss`/`aud` validation for Candidate 3.
- **Keycloak CVEs (last 24 months):**
  - **CVE-2024-1132** (CVSS 8.1) — admin path traversal, fixed 24.0.3. Tangential, but confirms a responsive disclosure program.
  - **CVE-2023-6927** (CVSS 6.1) — open redirect via wildcard `redirect_uri`. **Directly relevant:** Candidate 1's `/callback` redirect must be fixed to dashboard origin, never user-supplied.
  - **CVE-2023-0264** (CVSS 6.5) — Node.js adapter authz/JWT bug. Confirms `id_token`-validation is a CVE class; informs §7 Candidate 1's negative-auth test list.
- **Keycloak source:** `https://github.com/keycloak/keycloak` — used to cross-check CVE fixes landed in release tags.

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

**Finding on trigger precision:** trigger is bounded on customer count but silent on protocol ("SSO / SAML / OIDC" is one bucket). §6/§7 argue OIDC-first / SAML-deferred. **Recommendation:** the ADR clarifies "two customers asking for the same protocol" and lands OIDC-only unless both ask for SAML. Promote to `FOLLOWUPS.md` in the ADR's PR, not now.

Nothing in this dossier is `adopt-now`: there is no path that lands code without contradicting `ROADMAP_NOT`. The existing `SSOConfig` CRUD scaffolding is already merged; we neither remove nor extend it until trigger.

Security Reviewer veto applies per `docs/ROLES.md` (touches `internal/auth`).

## 11. Rollback if adopted

Trigger-fire implementation is additive: two new routes (`/start`, `/callback`), one runtime discovery cache, the JIT user-upsert path. No schema change (migration already exists).

1. **Feature-flag** the routes (env `KEEPSAVE_SSO_ENABLED`, default `false`). Rollback: flip flag false; `LoginPage.tsx` SSO buttons revert to disabled.
2. **JIT-provisioned users persist** but cannot log in (no `password_hash`, SSO off). The data that does NOT survive clean rollback — runbook step: delete orphans or run a password-reset recovery flow.
3. **`SSOConfig` rows persist** (additive migration). Inert with flag off.
4. **Audit events** already written are not removed.

True rollback to "no JIT users ever existed" is impossible without destroying audit attribution. Forward-fix (re-enable, or password-reset migration) is cheaper.

## 12. Won't-break-our-system claim

Integrator and dashboard-user contracts unchanged until trigger fires. Verified by code-reading.

- **Invariant 1 — password login unaffected.** `apiLogin(email, password)` at `LoginPage.tsx:32-33` hits the existing route; SSO candidates add new routes, never modify this one. `mode === 'password'` form (`LoginPage.tsx:121-167`) unchanged.
- **Invariant 2 — API-key auth unaffected.** `auth/apikey.go` (SHA-256, scope-bound per `models/models.go:51-60`) untouched.
- **Invariant 3 — existing `SSOConfig` CRUD still works and remains inert.** Handlers at `handlers_enterprise.go:30-91`, routes at `router.go:175-177`, encrypted client secret stored but never read by a protocol flow. Candidates 1–4 make those rows load-bearing without schema change. Verified against `sso_service.go:25-60` + `005_phase7_12.sql:4-17`.
- **Invariant 4 — JWT middleware contract unchanged.** Middleware validates KeepSave-issued HS256 only. The upstream `id_token` is exchanged at `/callback` for a KeepSave JWT; middleware never sees it. Verified against `auth.go:47-62`.
- **Invariant 5 — flag default-off = zero-risk readiness.** Until `KEEPSAVE_SSO_ENABLED=true` + `SSOConfig` present, `/start` and `/callback` return 404.

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

## Security Reviewer notes

**Verdict:** accepted. Veto **not** exercised on §9, §10, or §12. Status flipped `draft` → `accepted`.

**Refs spot-checked (Read tool, 2026-05-15) — all resolve:**

- `models/models.go:277-290` — `SSOConfig` struct exactly as cited (ID, OrgID, Provider, IssuerURL, ClientID, ClientSecretEncrypted/Nonce, Metadata, Enabled, timestamps). §5 row 3 / §12 Invariant 3 confirmed.
- `models/oauth.go:10-25` — `OAuthClient` struct as cited; clients are *downstream* of KeepSave-as-OP, distinct from the RP framing of §5 row 2.
- `models/models.go:219-227` — `OrgMember` struct as cited; `Role string` at line 224 is a free-string with no claim-mapping layer, matching §6 Candidate 4's default-`viewer` plan.
- `service/sso_service.go:25-60`, `migrations/005_phase7_12.sql:4-17` — full CRUD + migration confirmed; §2 + §6 Candidate 2 "no new schema" claim accurate.
- `handlers_oauth.go:216-218` — JWKS returns empty `keys: []` with HS256-now/RS256-planned note; §5 row 5 accurate.
- `auth/auth.go:29-45, 47-62` — HS256 issuance + validation; §12 Invariant 4 contract holds.
- `LoginPage.tsx:32-33, 104-117, 192-207` — `apiLogin` path + three-mode tab + `disabled` SSO placeholder buttons; §5 row 7, §6 Candidate 1, §12 Invariant 1 confirmed.

**ROADMAP_NOT trigger verbatim verification:** `docs/ROADMAP_NOT.md:19-22` matches §10 quote character-for-character. Dossier's flagged ambiguity (protocol-agnostic "SSO / SAML / OIDC" bucket) is correctly deferred to ADR-time per `CLAUDE.md` Type-1 process.

**§9 STRIDE rows quoted (`docs/THREAT_MODEL.md`):**

- Line 82 (S, Authentication): *"S | Forged JWT | HS256 with secret only on server; signature verified | `auth/auth.go:39`, `api/middleware.go:59-100` | Low"* — quoted accurately.
- Line 85 (R, Authentication): *"R | Login attempts not audited | Auth events not in audit log (verify in 30d) | `auth_service.go` | Medium"* — dossier correctly notes adopting SSO without auditing would worsen this row.

Trust-boundary direction: dossier correctly says **widens** (opposite of Pomerium); new entity correctly named ("customer-configured upstream OP identified by `SSOConfig.issuer_url`"). Bounding statement (`id_token` consumed once at `/callback`, never persisted) is sound.

**Cross-reference with `docs/audits/SECURITY_AUDIT_2026-05-15.md`:** Audit **A10-F2 (P4 Low)** at lines 333-335 flags `sso_service.go:36`'s `IssuerURL` as a latent SSRF surface. Candidate 2's runtime discovery fetch is the same gap; the ADR must fold A10-F2's allowlist (reject RFC1918 / loopback / link-local / cloud-metadata) into the discovery client. **Non-blocking** for this review.

**Non-blocking observations (for ADR Drafter, day 45):**

1. **SSRF guard for upstream discovery** — see audit A10-F2 cross-reference above; org-admin-supplied `issuer_url: "http://169.254.169.254/..."` is an IMDS read from the API container without it.
2. **Trigger precision** — ADR must explicitly state "OIDC-only first wave; SAML deferred even if asked" so the same-protocol-two-customers gate is binding, not advisory.
3. **`(iss, sub)` keying warrant** (Candidate 4) — ADR must capture the warrant and the orphan-rows failure mode if `issuer_url` is later migrated on an in-use `SSOConfig`.
4. **CVE-2023-6927 fixed-redirect-URI** (§12 check c) — dashboard `redirect_uri` at `/callback` must be server-pinned, never user-supplied, even via `state`-stashed value. Land in the negative-auth test list.
5. **Audit-event taxonomy** — `auth.sso.start`, `auth.sso.callback.success/failure`, `auth.sso.user_provisioned` must be added to `docs/AUDIT_LOG_COVERAGE.md` in the same PR as the ADR per `CLAUDE.md`.

No reference failed to check out.
