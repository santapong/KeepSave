# Competitor Dossier — Ory (Hydra + Kratos + Oathkeeper)

## 1. Header

- **Vendor:** Ory (Hydra, Kratos, Oathkeeper)
- **Category:** self-hosted auth / OIDC + OAuth 2.0 server
- **License:** Apache 2.0
- **Last updated:** —
- **Analyst:** Self-Hosted Auth Analyst
- **Reviewer:** Security Reviewer
- **Status:** not-started
- **Priority:** P0

## 2. One-paragraph overview

_TBD._

## 3. Architecture summary

_TBD._ Cover the three-binary split: Hydra (OAuth/OIDC server), Kratos (identity + session), Oathkeeper (zero-trust proxy). State which KeepSave would map to.

## 4. Security model

_TBD._ Cover: JWKS rotation, refresh-token rotation (replay-detection), session revocation, login flow self-service, PKCE enforcement.

## 5. KeepSave-comparable surface

| Ory concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Hydra OAuth/OIDC server | OAuth 2.0 server | `backend/internal/api/handlers_oauth.go` |
| JWKS endpoint + RS256 | **Empty JWKS, HS256 only** | `handlers_oauth.go:216-218`, `auth.go:11-62` |
| Refresh-token rotation w/ replay detection | No refresh today | `auth.go:11-62` |
| Kratos session store + revocation | `SessionToken` model (unwired) | `backend/internal/models/models.go:353-362` |
| Oathkeeper rule-based authz | Middleware role checks | `backend/internal/api/middleware.go:59-100` |

## 6. Adapt candidates

_TBD._ Seed: RS256 + JWKS rotation pattern (closes `handlers_oauth.go:216-218` gap); refresh-token rotation with replay-detection (FU #5); session revocation flow on top of `SessionToken` model.

## 7. Pros / cons of adapting

_TBD._

## 8. Validation evidence

_Required:_ Hydra OIDC certification status; Ory's public security audits; OAuth 2.0 RFC 6749 / RFC 8252 / RFC 7636 (PKCE); CVE history; refresh-token rotation post-mortems.

## 9. Threat-model implications

_TBD._ Map to Spoofing + Repudiation rows in `docs/THREAT_MODEL.md`.

## 10. Verdict

_TBD._

## 11. Rollback if adopted

_TBD per candidate._

## 12. Won't-break-our-system claim

_TBD._ Verify: existing `oauth/token` flow signatures, `OAuthToken` model compatibility, middleware behavior under both HS256 and RS256.

## 13. References

- (to be populated by analyst)
