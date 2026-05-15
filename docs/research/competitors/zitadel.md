# Competitor Dossier — Zitadel

## 1. Header

- **Vendor:** Zitadel
- **Category:** identity provider / OIDC + SAML
- **License:** Apache 2.0
- **Last updated:** 2026-05-15
- **Analyst:** Identity Provider Analyst
- **Reviewer:** Security Reviewer (accepted 2026-05-15)
- **Status:** accepted
- **Priority:** P1

## 2-4. Architecture + security summary

**Go-native** OIDC/OAuth2/SAML IdP, **event-sourced** core (immutable events, projected state), native per-org tenancy, **Actions** (JS auth hooks) replace Keycloak SPI, PKCE-S256, RS256/ES256 JWKS. Not deployed (see Keycloak P0). Three diffs vs Keycloak: Go runtime; tenancy matches `Organization`; event-sourced audit converges with Doppler Cand. 2.

## 5. KeepSave-comparable surface

| Zitadel | KeepSave | file:line |
|---|---|---|
| Organization | `Organization`+`OrgMember` | `models.go:210-227` |
| Per-org IDP | `SSOConfig` (no flow) | `models.go:278-290` |
| Event store | `AuditEntry`/`AuditRepository.Create` | `models.go:63-72` |
| Actions hooks | None; `ApprovedBy` sole gate | `models.go:101-115` |
| JWKS `kid` rotation | Stub `keys: []`, HS256 | `handlers_oauth.go:216-218` |
| PKCE methods | `S256, plain` | `handlers_oauth.go:238` |

Not adopted: server, Actions runtime, SAML, device-flow.

## 6. Adapt candidates

1. **Per-org `SSOConfig` on `Organization.ID`** — already modelled (`models.go:280`); on trigger-fire, scope JIT via `OrgMember`. `keep`.
2. **Event-sourced audit projection** — `auth.sso.*` + approvals as `AuditEntry`; rebuild counters/approver-trail from log. Supplements ADR-0014 with the projection-based read model primitive (events are source-of-truth; derived state rebuildable) — **not a substitute** for 0014's taxonomy+`actor_type` work. Converges with Doppler Cand. 2; supports FU 0d (`FOLLOWUPS.md:48`).
3. **PKCE-S256-only** — drop `plain` per RFC 7636 §4.2.

## 8. Validation evidence

`https://zitadel.com/docs` **403 (unreachable 2026-05-15)**; load-bearing on non-vendor RFCs + GitHub Advisory DB.

- **RFC 6749 / 7636 §4.2 / 8414** — normative for Cand. 3.
- **CVE-2024-28197** (CVSS 9.0) — session fixation via subdomain cookie; informs §9 strict-equal `aud`/`iss`.
- **GHSA-7wfc-4796-gmg5** — unauth SSRF in V2 Login. Cross-refs **A10-F2** + Keycloak reviewer #1.

## 9. Threat-model implications

`THREAT_MODEL.md` row **S — Forged JWT** (line 86): no wider than Keycloak Cand. 1 — same upstream OP entity. CVE-2024-28197 bounds residual via strict-equal `aud`/`iss`. Cand. 2 **narrows** row **R — Login attempts not audited** (line 89, Medium) by making audit source-of-truth.

## 10. Verdict

**`adopt-when-trigger-fires`** for Cand. 1 & 3, trigger verbatim `docs/ROADMAP_NOT.md:19-22`:

> *"### 2. SSO / SAML / OIDC for end-user auth*
> *- **What we are not building:** federated login for dashboard users.*
> *- **Why:** no customer has asked. Building it before the demand signal arrives means we'll build the wrong shape.*
> *- **Trigger to revisit:** when two customers ask in the same quarter, or when one enterprise prospect makes it a deal-blocker."*

**Cand. 2** is `adopt-now` — payoff SSO-independent; fold into Doppler audit-coverage PR. Security veto applies (`internal/auth`).

## Security Reviewer notes

**Verdict:** accepted-with-changes. Veto not exercised.

**Cand. 2 `adopt-now`: CONFIRMED.** Cross-checked vs ADR-0014. Complementary, not duplicative: 0014 adds named slots + `actor_type`; Cand. 2 adds the **projection** primitive (rebuild counters/approver-trail from the log). §6 amended ("supplements 0014 … not a substitute").

**Refs spot-checked (2026-05-15):** `models.go:278-290` `SSOConfig` exact (analyst `:277`→`:278` tightening correct — 277 is comment); `handlers_oauth.go:216-218` JWKS `keys:[]`+HS256 note ✓; `:238` `["S256","plain"]` confirms Cand. 3; `models.go:101-115` `PromotionRequest` sole-gate ✓.

**§9 line refs corrected inline:** Forged JWT `:82`→`:86`; R-row `:85`→`:89` (82/85 are header/separator). Widens/narrows direction correct.

**§10 trigger:** `ROADMAP_NOT.md:19-22` verbatim match.

**§8:** CVE-2024-28197 + GHSA-7wfc-4796-gmg5 authoritative; SSRF advisory cross-refs THREAT_MODEL §1/I (line 76, High) + Keycloak reviewer #1.

**Non-blocking (ADR Drafter):** (a) A10-F2 SSRF guard folds into discovery client; (b) projections key off `(actor_type, action)` once 0014 lands; (c) `auth.sso.*` names coordinate w/ Keycloak reviewer #5.
