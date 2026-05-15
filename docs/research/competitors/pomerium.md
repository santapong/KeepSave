# Competitor Dossier — Pomerium

## 1. Header

- **Vendor:** Pomerium
- **Category:** edge auth / identity-aware proxy
- **License:** Apache 2.0 (OSS core); commercial Pomerium Enterprise / Pomerium Zero
- **Last updated:** 2026-05-15
- **Analyst:** Edge Auth Analyst
- **Reviewer:** Security Reviewer (pending)
- **Status:** draft
- **Priority:** P0 (tied directly to FU 0b embed widget origin)

## 2. One-paragraph overview

Pomerium is an open-source identity-aware reverse proxy that authenticates every request to internal applications and forwards a signed JWT assertion (`X-Pomerium-Jwt-Assertion`) so upstreams can cryptographically verify the request came from Pomerium, not a forged client. It is used by teams replacing VPNs with per-request, context-aware authz in front of HTTP services (Kubernetes dashboards, Grafana, internal admin tools). We care about Pomerium not as a deployment target — KeepSave is not putting a proxy in front of its API — but as a **reference implementation of two specific patterns** that map directly onto FU 0b: signed-message trusted-header injection, and per-route origin allow-listing. KeepSave's `frontend/src/embed/auth.ts` postMessage handshake has neither today.

## 3. Architecture summary

Pomerium has four logical services (`proxy`, `authenticate`, `authorize`, `databroker`) which can run in a single binary or separately. For the patterns we care about, only two flows matter:

1. **Sign-on flow (one-time per session):** Browser hits `proxy` → redirected to `authenticate` → OIDC dance with upstream IdP → Pomerium-signed session cookie issued, scoped to the Pomerium domain.
2. **Per-request flow (the part KeepSave borrows):** Each request to a protected route is evaluated by `authorize` (per-route policy: who, from where, with what claims). If allowed, `proxy` strips the session cookie, mints a short-lived JWT signed with Pomerium's private key, and injects it as `X-Pomerium-Jwt-Assertion`. The upstream fetches the JWKS at `/.well-known/pomerium/jwks.json`, verifies the signature using the `kid` header, and trusts the claims.

KeepSave would interact with **only the per-request signed-assertion idea**, transposed onto a postMessage channel. We would ignore Pomerium's IdP integration, its proxy data plane (Envoy), and the `databroker` (the component implicated in CVE-2024-47616, see §8).

## 4. Security model

Auth primitives Pomerium publishes (docs retrieved 2026-05-15):

- **Signing algorithm:** ES256 (ECDSA P-256). Pomerium's private key never leaves the `authenticate`/`authorize` service. The public key is published at a per-route JWKS endpoint. Source: `pomerium.com/docs/guides/verify-jwt` and `pomerium.com/docs/capabilities/getting-users-identity`.
- **Header name:** `X-Pomerium-Jwt-Assertion` (RFC 7519 JWT).
- **Claims forwarded to upstream:** `iss` (Pomerium issuer URL), `aud` (the route domain — prevents cross-route token replay), `exp`, `iat`, `sub`, plus identity claims (`email`, `groups`).
- **Per-route policy:** `from`, `to`, `allowed_users`, `allowed_domains`, `allowed_idp_claims`, CORS allow-list, plus a Rego-compatible policy language. The CORS / origin enforcement is declarative per route, not a single global setting.
- **Trust-boundary stance:** Pomerium publishes its threat model in passing in its mutual-auth docs and its Cure53 audit report (March 2021). The model assumes: (a) the JWKS endpoint is fetched over TLS; (b) upstream services treat any request *not* carrying a verified assertion as anonymous; (c) compromise of Pomerium's private key is total compromise — there is no defence-in-depth below the JWT.

CVE history (last 24 months, NVD + GitHub Security Advisories):

- **CVE-2024-47616** (GHSA-r7rh-jww5-5fjr, 2024-10-02, CVSS 6.8) — incomplete JWT validation in the `databroker` API treated some service-account tokens as valid for authorization. Fixed in v0.27.1. **Directly relevant:** validation logic for the exact pattern we'd adopt is itself a CVE class. Tells us audience/expiry checks need test coverage.
- **GHSA-rrqr-7w59-637v** (2024-07-02, moderate) — OAuth2 access/ID tokens exposed in user-info endpoint response. Not directly relevant to the signed-header pattern.
- **CVE-2021-39204** (GHSA-5wjf-62hw-q78r) — Envoy HTTP/2 reset DoS. Not applicable to KeepSave (we are not deploying Envoy).

## 5. KeepSave-comparable surface

| Pomerium concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| `X-Pomerium-Jwt-Assertion` signed-JWT header | None — embed widget passes raw token/apiKey via postMessage with no signature | `frontend/src/embed/auth.ts:21-26` |
| Per-route CORS / origin allow-list (declarative) | Missing — listener has no `ev.origin` check | `frontend/src/embed/auth.ts:21-26`; policy doc `docs/EMBED_ORIGIN_POLICY.md:42-60` (target state) |
| Outbound message bound to a specific recipient origin | Outbound `postMessage` uses wildcard `'*'` | `frontend/src/embed/auth.ts:33` |
| JWKS endpoint (`/.well-known/...`) for upstream verification | Backend exposes JWKS handler but empty (Phase B blocker, separate dossier) | `backend/internal/api/handlers_oauth.go` (per `docs/research/README.md:11`) |
| Per-route policy (Rego) | Promotion / API-key scoping is code-resident; no declarative origin policy | `backend/internal/auth/apikey.go`; no embed-policy file yet |
| `databroker` gRPC store | Not applicable (KeepSave uses Postgres directly) | — |
| Session-cookie sign-on | Dashboard uses JWT in `localStorage`; embed widget uses postMessage handshake | `frontend/src/pages/LoginPage.tsx:32-33`; `frontend/src/embed/keepsave-widget.ts:101-117` |
| Envoy data plane | Not applicable (KeepSave is Gin-only) | — |
| Identity-aware authz at every request | KeepSave middleware re-validates JWT/API key per request | `backend/internal/api/middleware.go:59-120` (per threat-model §2) |

## 6. Adapt candidates

These are **pattern adoptions only**. KeepSave does not deploy Pomerium; we transcribe two of its ideas onto the existing embed widget and one server-side endpoint.

1. **Origin allow-list as per-project server-side configuration.** Mirror Pomerium's per-route `allowed_origins` as a per-project column (`projects.allowed_embed_origins TEXT[]`). The widget bootstrap fetches this list before installing the message listener. Already drafted in `docs/EMBED_ORIGIN_POLICY.md:42-60`; Pomerium's pattern validates the shape (declarative, per-resource, fetched by the client before policy enforcement begins).
2. **Strict `ev.origin` check + non-wildcard `targetOrigin`.** Both directions of the postMessage flow get an exact-equality check against the allow-list. Pomerium's contribution here is the *invariant*: a wildcard is forbidden by code review, not just discouraged. Implementation is local to `frontend/src/embed/auth.ts:21-33`.
3. **Signed message envelope for embed→host and host→embed traffic (Phase B).** Pomerium's `X-Pomerium-Jwt-Assertion` pattern transposed: the host page mints a short-lived ES256-signed assertion (kid + `aud` = widget instance ID + `exp` ≤ 60s) and the widget verifies it via a KeepSave-served JWKS before accepting any `keepsave-auth` payload. This is **defence-in-depth** beyond the origin allow-list — it survives an attacker who somehow gets a script onto an allow-listed origin but cannot reach the host's signing key. `docs/EMBED_ORIGIN_POLICY.md:103-105` defers per-message MAC to Phase B; Pomerium's pattern is the concrete shape we'd adopt when that trigger fires.

Candidates 1 and 2 are sized for a single ADR (frontend-only + one additive schema migration). Candidate 3 is a separate, later ADR.

## 7. Pros / cons of adapting

### Candidate 1: Per-project origin allow-list

- **Pros:** Closes FU 0b's root cause: there is no source-of-truth for "who is allowed to embed". Declarative, audit-able (writes go through the projects API → audit log per `docs/AUDIT_LOG_COVERAGE.md`). Mirrors a pattern with public audit precedent (Cure53 reviewed Pomerium's per-route policy in 2021).
- **Cons (operational):** Adds one Postgres column + one unauthenticated `GET /api/v1/projects/:id/embed-config` endpoint. That endpoint becomes a project-ID enumeration oracle if not rate-limited (already flagged in `docs/EMBED_ORIGIN_POLICY.md:48`). Operators must teach customers to manage the list — a support surface that did not exist before.
- **Cons (security):** The unauthenticated config endpoint widens the public attack surface by one route. Mitigation: IP rate-limit + return identical 404 for "unknown project" and "empty allow-list" so the endpoint is not a project-existence oracle. CVE-2024-47616 is a reminder that the *validation* code path on the new column needs test coverage equivalent to the auth middleware (FU 0c).

### Candidate 2: Strict `ev.origin` check + non-wildcard `targetOrigin`

- **Pros:** Three-line code change in `frontend/src/embed/auth.ts`. Closes the STRIDE-S row in threat-model §4 outright. No backend coupling. Existing unit tests in `frontend/src/embed/auth.test.ts` continue to pass because they use `window.dispatchEvent` with default origin (the test environment's own origin), which will be in the allow-list for tests.
- **Cons (operational):** Existing integrators who were silently relying on no-check behaviour will break the day they're added without listing their origin. Mitigation: ship behind a per-project flag and roll out in monitor-then-enforce phases.
- **Cons (security):** None we can identify. The pattern *narrows* the trust boundary — see §9. A weak implementation (e.g., `startsWith` instead of strict equality) would re-open it; we mitigate with the lint rule already prescribed in `docs/EMBED_ORIGIN_POLICY.md:62`.

### Candidate 3: Signed message envelope (Phase B)

- **Pros:** Defence-in-depth: survives an XSS on an allow-listed origin that the origin check alone would not. Forces explicit audience binding per widget instance.
- **Cons (operational):** Requires a key-management story on the host page (where does the integrator's signing key live?). For most KeepSave customers, this is heavier than the threat warrants.
- **Cons (security):** Adds a new signing oracle on the customer side. If the customer leaks the signing key, the attacker can mint valid assertions and bypass the allow-list silently — a worse failure mode than today's "obviously wrong" wildcard. The trigger threshold needs to be explicit: only adopt when a customer with a justified threat model asks.

## 8. Validation evidence

Required mix: at least one non-blog source. We have multiple.

- **RFC 7519 (JWT) and HTML Living Standard §`window.postMessage`** — `https://html.spec.whatwg.org/multipage/web-messaging.html` (retrieved 2026-05-15 via WebSearch summary; direct fetch blocked by 403). Specifies the `event.origin` field and that wildcard `targetOrigin` "should be avoided" for sensitive data.
- **Public security audit:** Cure53 pentest of Pomerium, published March 2021. Linked from `pomerium.com/blog/pomerium-completes-independent-security-audit-by-cure53`. Maintainer states all impactful issues were resolved in subsequent releases.
- **CVE-2024-47616 (GHSA-r7rh-jww5-5fjr)** — Pomerium databroker JWT validation. Direct evidence that JWT-validation code on the *exact pattern* we propose is a known CVE class; informs §7 cons.
- **GHSA-rrqr-7w59-637v** (2024-07-02) — OAuth2 token exposure in user-info endpoint; tangential, but confirms Pomerium runs a responsive disclosure program (closing the "CVE absence misread as safe" risk from `docs/research/README.md:110`).
- **OWASP HTML5 Security Cheat Sheet** — `cheatsheetseries.owasp.org/cheatsheets/HTML5_Security_Cheat_Sheet.html` — codifies the strict-equality origin-check requirement we propose.
- Official Pomerium docs (vendor — supplementary, not load-bearing alone): `pomerium.com/docs/guides/verify-jwt`, `pomerium.com/docs/capabilities/getting-users-identity` (both retrieved 2026-05-15 via search summary; direct fetch returned 403 — vendor blocks the WebFetch user agent).

## 9. Threat-model implications

Maps directly to `docs/THREAT_MODEL.md` §4 "Embed widget (browser trust domain)", row S:

> *"Malicious host page injects fake `keepsave-auth` postMessage … Mitigation: **None today** — listener has no origin check … Residual: **High**"*

STRIDE category quoted verbatim: **Spoofing** (the "S" row of §4).

Adopting Candidates 1 + 2 **narrows** the trust boundary: today, any window in the same browser that can reach the widget can spoof an auth message; after adoption, only windows whose origin matches a per-project, server-attested allow-list can. No new entity enters the trust boundary — the allow-list endpoint is unauthenticated by design (the widget hasn't authenticated yet) but does not extend trust, it constrains it. The per-project `allowed_embed_origins` row becomes a new sensitive value, but it lives behind the existing authenticated projects API for writes, so it inherits whatever audit coverage FU #0 lands.

Candidate 3 would also narrow further, at the cost of adding the integrator's signing key as a new asset in the boundary. Hold until trigger.

## 10. Verdict

**`adopt-now`** for Candidates 1 and 2.

Against the template's four options:
- Not `adopt-when-trigger-fires`: FU 0b is an *open P0 with a 30-day SLA* per `docs/FOLLOWUPS.md:34-39`. The trigger has already fired.
- Not `reject`: the gap is real, the pattern is proven, the cost is bounded.
- Not `note-only`: this is a P0 dossier.

`adopt-now` carries the requirement of a Type-1 ADR (auth-adjacent) per `CLAUDE.md` Decision-classes table. Security Engineer veto applies per `docs/ROLES.md`. ADR Drafter picks this up on day 45.

**Candidate 3** (signed envelope): **`adopt-when-trigger-fires`**. Trigger quoted verbatim from `docs/EMBED_ORIGIN_POLICY.md:103-105`:

> *"Per-message MAC / replay protection. Considered but not needed at the current scope; `postMessage` semantics + origin checks are enough. Revisit if a customer with a high-threat model asks."*

## 11. Rollback if adopted

Frontend-only change for Candidate 2; additive schema for Candidate 1.

**Candidate 2 (origin checks in widget):**
- Rollback = `git revert` the auth.ts change and ship a new widget bundle. No data migration; no server change.
- During the rollback window (CDN propagation, integrator cache TTL — typically minutes to hours), the widget falls back to today's behaviour (open postMessage). Existing FU 0b vulnerability re-opens, so rollback is itself a P0-severity decision.

**Candidate 1 (per-project allow-list column + endpoint):**
- Rollback = drop the endpoint route and stop reading the column; leave the column in place. The migration is additive (`ALTER TABLE projects ADD COLUMN allowed_embed_origins TEXT[] DEFAULT '{}'`) and a `DROP COLUMN` is unsafe under our zero-downtime policy.
- Data does NOT survive rollback in the sense that allow-lists configured by customers will be ignored. They are not destroyed; they are dormant in the table until forward-fix.
- Runbook step: flip a feature flag on `/api/v1/projects/:id/embed-config` to return `{ allowed_origins: ["*"] }` (sentinel for "policy disabled"). The widget must treat `["*"]` as "disabled" (logged warning), not as "permit all" — code reviewer enforces.

True rollback to "no policy ever existed" is not possible without breaking integrators who started relying on the allow-list. State that openly.

## 12. Won't-break-our-system claim

The integrator-facing contract is one `<script>` tag plus HTML attributes (`project-id`, `api-url`, `theme`, `mode`, `token`, `api-key`). Verified by reading `frontend/src/embed/INTEGRATION.md:50-58` and `frontend/src/embed/keepsave-widget.ts:7-9` (the `observedAttributes` list). Neither candidate adds or removes an attribute.

- **Invariant 1 — direct-credential integrators are unaffected.** Integrators using `token=` or `api-key=` HTML attributes never enter the postMessage path at all: see `frontend/src/embed/keepsave-widget.ts:78-87` (the `if (this.directToken) ... else if (this.directApiKey) ...` branch returns before `setupPostMessageAuth` is called). Origin checks only apply to the handshake branch, so direct-credential mode is untouched. Verified by code-reading.
- **Invariant 2 — existing unit tests stay green.** `frontend/src/embed/auth.test.ts` covers six cases: register listener, send request, accept token, accept apiKey, ignore unrelated messages, remove on destroy. All six dispatch `MessageEvent`s with default origin (the test environment's origin, which we ensure is in the test fixture's allow-list). The "ignore unrelated messages" test (lines 70-92) is *strengthened*, not broken, by an origin check. Verified by reading the test file.
- **Invariant 3 — integrators on allow-listed origins keep working.** The widget bootstrap fetches the allow-list before installing the listener; if the integrator's origin is on it, behaviour is identical to today. Verified by tracing through `frontend/src/embed/keepsave-widget.ts:68-117` and the target policy at `docs/EMBED_ORIGIN_POLICY.md:42-60`.
- **Invariant 4 — Shadow DOM isolation and CSS variable contract unchanged.** No change to `keepsave-widget.ts` style handling (`applyTheme`, lines 90-99). Verified by code-reading; no edit proposed in this area.
- **Invariant 5 — direct-attribute LoginPage flow unaffected.** Dashboard login (`frontend/src/pages/LoginPage.tsx:32-33`) does not use the embed widget at all; it uses the `apiLogin` REST client directly. Verified by code-reading. The widget origin policy has no surface there.

Security Reviewer veto applies. Suggested reviewer checks: (a) the `["*"]` sentinel in §11 is actually treated as "disabled" not "permit"; (b) the unauthenticated config endpoint has the rate-limit promised in `docs/EMBED_ORIGIN_POLICY.md:48`; (c) origin comparison uses strict equality, not `startsWith` / `includes` / regex.

## 13. References

- HTML Living Standard, Web Messaging (`postMessage`): `https://html.spec.whatwg.org/multipage/web-messaging.html` — retrieved 2026-05-15 (via search summary; direct fetch returned 403).
- OWASP HTML5 Security Cheat Sheet: `https://cheatsheetseries.owasp.org/cheatsheets/HTML5_Security_Cheat_Sheet.html` — retrieved 2026-05-15.
- OWASP WSTG, Testing Web Messaging: `https://owasp.org/www-project-web-security-testing-guide/latest/4-Web_Application_Security_Testing/11-Client-side_Testing/11-Testing_Web_Messaging` — retrieved 2026-05-15.
- Pomerium docs — JWT verification: `https://www.pomerium.com/docs/guides/verify-jwt` — retrieved 2026-05-15 (via search summary).
- Pomerium docs — Getting Users Identity: `https://www.pomerium.com/docs/capabilities/getting-users-identity` — retrieved 2026-05-15.
- Pomerium docs — Mutual Authentication: `https://www.pomerium.com/docs/internals/mutual-auth` — retrieved 2026-05-15.
- CVE-2024-47616 / GHSA-r7rh-jww5-5fjr (Databroker JWT validation): `https://github.com/pomerium/pomerium/security/advisories/GHSA-r7rh-jww5-5fjr` — retrieved 2026-05-15.
- GHSA-rrqr-7w59-637v (OAuth2 token exposure): `https://github.com/pomerium/pomerium/security/advisories` — retrieved 2026-05-15.
- CVE-2021-39204 / GHSA-5wjf-62hw-q78r (Envoy HTTP/2 reset DoS): `https://github.com/pomerium/pomerium/security/advisories/GHSA-5wjf-62hw-q78r` — retrieved 2026-05-15.
- Cure53 audit of Pomerium (March 2021): `https://www.pomerium.com/blog/pomerium-completes-independent-security-audit-by-cure53` — retrieved 2026-05-15 (vendor blog; cited as a pointer to the public audit, which is the load-bearing source).
- Pomerium source repository: `https://github.com/pomerium/pomerium` — retrieved 2026-05-15.
- RFC 7519 (JSON Web Token): `https://www.rfc-editor.org/rfc/rfc7519` — retrieved 2026-05-15.
