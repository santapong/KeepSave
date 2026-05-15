# Competitor Dossier — Pomerium

## 1. Header

- **Vendor:** Pomerium
- **Category:** edge auth / identity-aware proxy
- **License:** Apache 2.0 (OSS); commercial Enterprise
- **Last updated:** —
- **Analyst:** Edge Auth Analyst
- **Reviewer:** Security Reviewer
- **Status:** not-started
- **Priority:** P0 (tied directly to FU 0b embed widget origin)

## 2. One-paragraph overview

_TBD._

## 3. Architecture summary

_TBD._ Cover: identity-aware proxy in front of upstream services; per-request authz; signed-JWT header injection; integration with upstream IdP for session.

## 4. Security model

_TBD._ Cover: trusted-header signing; origin enforcement; per-route policy.

## 5. KeepSave-comparable surface

| Pomerium concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Signed trusted-header injection | None — embed widget uses unverified postMessage | `frontend/src/embed/auth.ts:21-26` |
| Per-route allowed origins | Missing | `frontend/src/embed/auth.ts:33` (target `'*'`) |
| Signed-JWT egress to upstream | Not yet applicable | — |
| CORS / postMessage origin validation policy | Missing — FU 0b | `docs/FOLLOWUPS.md` 0b |

## 6. Adapt candidates

_TBD._ Seed: signed-header / signed-postMessage pattern for embed widget → backend communication; origin-allowlist policy schema (informs `docs/EMBED_ORIGIN_POLICY.md`).

## 7. Pros / cons of adapting

_TBD._

## 8. Validation evidence

_Required:_ Pomerium architecture docs; their threat-model write-up; any third-party audit; comparison with Cloudflare Access / oauth2-proxy.

## 9. Threat-model implications

_TBD._ Map directly to `docs/THREAT_MODEL.md` §4 (embed widget origin bypass row).

## 10. Verdict

_TBD — likely `adopt-now` (pattern only, not deploying Pomerium) to close FU 0b._

## 11. Rollback if adopted

_TBD._ Frontend-only change; rollback = revert client code.

## 12. Won't-break-our-system claim

_TBD._ Verify: embed widget continues to function on allowed origins; integrator docs updated; no regression in `frontend/src/embed/auth.ts` test fixtures.

## 13. References

- (to be populated)
