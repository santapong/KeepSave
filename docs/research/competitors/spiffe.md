# Competitor Dossier — SPIFFE / SPIRE

## 1. Header

- **Vendor:** SPIFFE / SPIRE (CNCF graduated)
- **Category:** machine identity (workload attestation)
- **License:** Apache 2.0
- **Last updated:** —
- **Analyst:** Machine Identity Analyst
- **Reviewer:** Security Reviewer
- **Status:** not-started
- **Priority:** P0 (primarily feeds `BEYOND.md` §2.5)

## 2. One-paragraph overview

_TBD._

## 3. Architecture summary

_TBD._ Cover: SPIFFE ID format (URI scheme); SVID (X.509 or JWT); SPIRE server + agents; attestor plugins (Kubernetes, AWS, OS-level).

## 4. Security model

_TBD._ Cover: attestation chain; trust domain federation; SVID rotation cadence; key custody (server CA).

## 5. KeepSave-comparable surface

| SPIFFE concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Workload identity (SVID) | API key for AI agent | `backend/internal/auth/apikey.go:11-32` |
| Trust domain | Organization | `backend/internal/models/models.go:220-227` |
| Attestor (proves "this workload is X") | None — agent self-asserts via key | — |
| Federation across trust domains | Not yet | — |

## 6. Adapt candidates

_TBD._ Seed: SPIFFE-shaped workload identity for AI agents (replacing long-lived `ks_` keys with short-lived attested SVIDs — primary input to `BEYOND.md` §2.5).

## 7. Pros / cons of adapting

_TBD._ Be honest about operational cost: customer must run SPIRE agent or accept platform-attestation dependency.

## 8. Validation evidence

_Required:_ SPIFFE spec documents (workload-api, trust-domain, SVID-X509, SVID-JWT); SPIRE production deployment write-ups (HPE, Bloomberg, Square).

## 9. Threat-model implications

_TBD._ Map to Spoofing (workload identity replaces shared secret).

## 10. Verdict

_TBD — likely `adopt-when-trigger-fires` where trigger = MCP/AI-agent integration demand._

## 11. Rollback if adopted

_TBD._ Note: this is additive (new auth method alongside JWT/API-key), so rollback = remove the auth method registration.

## 12. Won't-break-our-system claim

_TBD._ Verify: existing API-key auth path unchanged; new auth method behind feature flag.

## 13. References

- (to be populated)
