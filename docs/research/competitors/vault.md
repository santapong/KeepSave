# Competitor Dossier — HashiCorp Vault

## 1. Header

- **Vendor:** HashiCorp Vault (open-source + Enterprise; Sentinel + Transit covered here)
- **Category:** secrets management + machine identity + approval workflow
- **License:** MPL 2.0 (OSS); BSL for Enterprise features (Sentinel)
- **Last updated:** —
- **Analyst:** Secrets Management Analyst (lead) + Approval Workflow Analyst (Sentinel section)
- **Reviewer:** Security Reviewer
- **Status:** not-started
- **Priority:** P0

## 2. One-paragraph overview

_TBD by analyst._

## 3. Architecture summary

_TBD._ Cover: storage backends, seal/unseal, auth methods plug-in model, secret engines, lease/renewal lifecycle.

## 4. Security model

_TBD._ Cover: barrier encryption, key shamir sharing, auto-unseal via cloud KMS, response-wrapping tokens, identity entities + aliases.

## 5. KeepSave-comparable surface

| Vault concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| KV v2 secret engine | Secret + version model | `backend/internal/models/models.go` (Secret) |
| Envelope / barrier encryption | Envelope encryption ADR | `docs/adr/0001-envelope-encryption.md`, `backend/internal/crypto/` |
| Auth methods (token, AppRole, JWT, OIDC) | JWT + API key | `backend/internal/auth/auth.go:11-62`, `apikey.go:11-32` |
| Policies (HCL) | OrgMember role string + API key scopes | `backend/internal/models/models.go:51-61, 220-227` |
| Sentinel (Enterprise) | Promotion engine rules | `docs/adr/0003-promotion-engine.md`, `backend/internal/promotion/` |
| Transit secret engine | Crypto layer | `backend/internal/crypto/` |
| Lease + renewal | Secret leases | `backend/internal/api/router.go:134` (`/projects/:id/leases`) |

## 6. Adapt candidates

_TBD._ Seed list for analyst: response-wrapping tokens for one-time secret transfer; Sentinel policy-as-code for promotion gates; AppRole pattern for AI-agent identity; lease renewal grammar; barrier encryption seal types.

## 7. Pros / cons of adapting

_TBD per candidate._

## 8. Validation evidence

_Required:_ Vault security model whitepaper; Trail of Bits 2018 audit; CVE-2020-16250 (auth-method bypass); CVE-2023-25000; Sentinel docs.

## 9. Threat-model implications

_TBD._ Map each candidate to `docs/THREAT_MODEL.md` STRIDE row. Note especially Repudiation (audit) and Elevation-of-Privilege (policy ACL).

## 10. Verdict

_TBD._

## 11. Rollback if adopted

_TBD per candidate._

## 12. Won't-break-our-system claim

_TBD._ Specifically verify: existing `secret_service.go` behavior, `internal/crypto` envelope contract, `internal/promotion` gate hooks.

## 13. References

- (to be populated; minimum one of: vault security whitepaper, trail-of-bits audit, CVE entry, maintainer post-mortem)
