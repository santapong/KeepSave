# Competitor Dossier — Authentik

## 1. Header

- **Vendor:** Authentik (goauthentik.io)
- **Category:** identity provider / OIDC + SAML
- **License:** MIT + commercial add-ons
- **Last updated:** 2026-05-15
- **Analyst:** Identity Provider Analyst
- **Reviewer:** Security Reviewer (accepted 2026-05-15)
- **Status:** accepted
- **Priority:** P1

## 2-5. Overview + KeepSave-comparable surface (collapsed)

Authentik is a Python/Django + Go OIDC + SAML IdP — lighter than Keycloak (single image, no JVM/WildFly). Differs from Keycloak via a **flow-based** declarative pipeline (stages: identification → password → MFA → captcha, composed per binding) and an **Outpost** sidecar-proxy for non-OIDC apps. Differs from Ory by being a full UI/IdP, not a headless primitive library. Integration identical to Keycloak — OIDC RP against an Authentik tenant via dormant `SSOConfig`.

| Authentik concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| OIDC provider + JWKS | OAuth handler / empty JWKS | `backend/internal/api/handlers_oauth.go:216-218` |
| Provider config | `SSOConfig` (scaffolded, no flow) | `backend/internal/models/models.go:277-290` |
| Flow / stage pipeline | Hardcoded mode-switch tabs | `frontend/src/pages/LoginPage.tsx:104-117` |

**Not adopted:** Outpost binary (Pomerium owns edge-auth); LDAP/SCIM (no demand); admin UI; Django runtime.

## 6. Adapt candidates

1. **Declarative flow-based login pipeline (frontend).** Replace `LoginPage.tsx:104-117`'s hardcoded mode-switch with a config-driven stage list (`identification → password|sso → mfa`). Borrows Authentik's binding-ordering, not schema. Overlaps Keycloak Cand. 1 but adds declarative ordering.
2. **Outpost auth proxy** — `reject`. Pomerium owns this.

## 8. Validation evidence

- **RFC 6749 / 7636 / 8252** — load-bearing OIDC + PKCE foundation.
- **Authentik docs:** `https://docs.goauthentik.io/` (retrieved 2026-05-15).
- **CVEs (24 mo):** CVE-2024-47077 (CVSS 9.1, IP spoof in proxy outpost), CVE-2024-37905 (recovery-flow takeover), CVE-2023-39522 (impersonation flow ACL bypass). Three flow/outpost CVEs — flow-evaluator complexity is a real cost class.
- **Cure53:** none located (Ory Hydra v1.4 remains canonical).

## 9. Threat-model implications

Identical widening shape to Keycloak §9. Cand. 1 alone (frontend) does **not** widen the boundary — no upstream OP enters until Keycloak's `/callback` lands. STRIDE `docs/THREAT_MODEL.md:82` (Forged JWT) unchanged; `auth.go:39` still mints.

## 10. Verdict

- **Cand. 1 (flow pipeline):** `adopt-when-trigger-fires`, same trigger as Keycloak — verbatim from `docs/ROADMAP_NOT.md:19-22`: *"Trigger to revisit: when two customers ask in the same quarter, or when one enterprise prospect makes it a deal-blocker."* Lands inside the Keycloak SSO ADR as a frontend refactor.
- **Cand. 2 (Outpost):** `reject` — Pomerium is canonical.
- **LDAP/SCIM bridge:** `reject` — no `ROADMAP_NOT` trigger covers directory federation.

Security Reviewer veto applies (SSO ADR touches `internal/auth`).

## Security Reviewer notes

**Verdict:** accepted. Veto **not** exercised on §9 or §10.

**Refs spot-checked (Read tool, 2026-05-15) — all resolve:**

- `handlers_oauth.go:216-218` — empty JWKS + RS256-planned note as cited.
- `models/models.go:277-290` — `SSOConfig` struct exactly as framed.
- `LoginPage.tsx:104-117` — hardcoded `['password','key','sso']` mode-switch confirmed; matches §5 row 3 "hardcoded mode-switch tabs".

**§10 trigger:** `ROADMAP_NOT.md:22` matches the §10 quote verbatim. Citation range `:19-22` is the full bullet block (consistent with Keycloak P0 citation style).

**§9 STRIDE:** `THREAT_MODEL.md:82` is the §2 section header; the Forged JWT row is line 86 (carried from Keycloak P0; both unambiguous within §2). Boundary direction (unchanged for Cand. 1, widening deferred to Keycloak ADR) is correct.

**§6 rejections sound:**

- Cand. 2 (Outpost) — Pomerium (accepted P0) is the canonical edge-auth borrow; double-borrow is correctly avoided.
- LDAP/SCIM — no `ROADMAP_NOT` trigger covers directory federation; Keycloak dossier §5 also explicitly drops "User federation (LDAP/AD)"; no customer-demand signal in `FOLLOWUPS.md`. **No override.**

**§8:** RFCs + three Authentik CVEs cover the non-vendor-corroborator + 24-month CVE-history bars. Cure53-absence note honest.

No reference failed to check out.
