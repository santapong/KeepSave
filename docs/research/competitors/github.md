# Competitor Dossier — GitHub (Fine-Grained PATs + Environments)

## 1. Header

- **Vendor:** GitHub (fine-grained Personal Access Tokens + Deployment Environments)
- **Category:** machine identity (PATs) + approval workflow (Environments)
- **License:** proprietary / N/A — patterns only
- **Last updated:** —
- **Analyst:** Machine Identity Analyst (PATs) + Approval Workflow Analyst (Environments)
- **Reviewer:** Security Reviewer
- **Status:** not-started
- **Priority:** P0

## 2. One-paragraph overview

_TBD._

## 3. Architecture summary

_TBD._ Cover: fine-grained PAT scope grammar (repo + permission tuples); expiration policy; org-level admin enablement; Environments protection rules (required reviewers, wait timer, deployment branches); secrets scoped to env.

## 4. Security model

_TBD._ Cover: PAT prefix (`github_pat_`), token storage, scope enumeration, audit-log emission per use; Environments approver gating.

## 5. KeepSave-comparable surface

| GitHub concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Fine-grained PAT (`github_pat_*`) | API key (`ks_*`) | `backend/internal/auth/apikey.go:11-32` |
| PAT scope grammar (resource × permission) | `APIKey.Scopes` StringList | `backend/internal/models/models.go:51-61` |
| PAT expiration policy | `APIKey.ExpiresAt` optional | `backend/internal/models/models.go:51-61` |
| Environment with required reviewers | Promotion engine approvers | `docs/adr/0003-promotion-engine.md` |
| Approver ≠ requester | **Unverified — FU 0d** | `docs/FOLLOWUPS.md` |
| Deployment kill switch | **Missing — FU 0e** | `docs/FOLLOWUPS.md` |

## 6. Adapt candidates

_TBD._ Seed: fine-grained scope grammar for `ks_` keys (directly addresses Phase-B deferred item); required-reviewer enforcement schema; Environments-style kill-switch UX.

## 7. Pros / cons of adapting

_TBD._

## 8. Validation evidence

_Required:_ GitHub's fine-grained PAT GA blog post + threat-model write-up; GitHub's 2022 token-prefix CVE response post-mortem; SLSA/SSDF references for deployment gates.

## 9. Threat-model implications

_TBD._ Map to Information Disclosure (scope narrowing reduces blast radius) and Elevation of Privilege (approver≠requester).

## 10. Verdict

_TBD — strong candidate `adopt-now` for FU 0d + Phase-B scope grammar._

## 11. Rollback if adopted

_TBD._

## 12. Won't-break-our-system claim

_TBD._ Verify: existing `APIKey.Scopes` accepts new grammar without schema change; backward-compat for existing keys (coarse scope = "all" under new grammar).

## 13. References

- (to be populated)
