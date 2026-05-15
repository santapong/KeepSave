# Competitor Dossier — Keycloak

## 1. Header

- **Vendor:** Keycloak (CNCF, formerly Red Hat)
- **Category:** identity provider / OIDC + SAML
- **License:** Apache 2.0
- **Last updated:** —
- **Analyst:** Identity Provider Analyst
- **Reviewer:** Security Reviewer
- **Status:** not-started
- **Priority:** P0

## 2. One-paragraph overview

_TBD._

## 3. Architecture summary

_TBD._ Cover: realms, clients, identity brokering (federate to upstream IdPs), user federation (LDAP/AD), token mappers.

## 4. Security model

_TBD._ Cover: token signing (RS256/ES256), JWKS rotation policy, session management, SAML assertion signing/encryption, brute-force detection, password policies.

## 5. KeepSave-comparable surface

| Keycloak concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Realm | Organization | `backend/internal/models/models.go:220-227` (`OrgMember`) |
| Client | `OAuthClient` | `backend/internal/models/models.go:10-25` |
| Identity provider broker (OIDC/SAML) | `SSOConfig` (stub, unwired) | `backend/internal/models/models.go:278-290` |
| Role mappers | Org-level role string | `OrgMember.Role` |
| Admin REST API | `/api/v1/organizations/*` | `backend/internal/api/router.go:164-177` |

## 6. Adapt candidates

_TBD._ **Note:** Dashboard SSO adoption is blocked by `docs/ROADMAP_NOT.md:19-22` until two-customer trigger. Patterns may still inform stub completion. Seed: OIDC broker config schema; SAML assertion validation library choice; user-federation abstraction.

## 7. Pros / cons of adapting

_TBD._

## 8. Validation evidence

_Required:_ Keycloak OIDC certification; CVE history (Keycloak has substantial CVE volume — analyze pattern, not absence); OAuth/OIDC RFCs; SAML 2.0 Core spec.

## 9. Threat-model implications

_TBD._ Map to Spoofing + Tampering (SAML XML signature wrapping is a recurring class).

## 10. Verdict

_TBD — likely `adopt-when-trigger-fires` per `ROADMAP_NOT.md:19-22`._ Trigger must be quoted verbatim.

## 11. Rollback if adopted

_TBD._

## 12. Won't-break-our-system claim

_TBD._ Verify: existing `SSOConfig` model schema; `handlers_oauth.go` independence (Keycloak as inbound broker, not replacement for our own OAuth server).

## 13. References

- (to be populated)
