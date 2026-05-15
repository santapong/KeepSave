# Competitor Dossier — Infisical

## 1. Header

- **Vendor:** Infisical
- **Category:** secrets management (closest peer product)
- **License:** MIT (core); commercial for Cloud / Enterprise
- **Last updated:** —
- **Analyst:** Secrets Management Analyst
- **Reviewer:** Security Reviewer
- **Status:** not-started
- **Priority:** P0

## 2. One-paragraph overview

_TBD._ Note: Closest direct product analog to KeepSave (env-var-centric, env-promotion workflow, dashboard + SDK).

## 3. Architecture summary

_TBD._ Cover: workspace/project/environment model; CLI + SDK; integrations marketplace; PKI/auth/secrets engines split.

## 4. Security model

_TBD._ Cover: E2E encryption claim (analyst MUST verify against actual implementation, not marketing copy); machine identity model; KMS integration.

## 5. KeepSave-comparable surface

| Infisical concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Project + environment | Project + environment | `backend/internal/models/models.go` (Project) |
| Secret + version | Secret + version | `backend/internal/models/models.go` |
| Machine identity (service token / Universal Auth) | API key | `backend/internal/auth/apikey.go:11-32` |
| Secret approval policies | Promotion engine | `docs/adr/0003-promotion-engine.md` |
| Audit log | Audit log | `backend/internal/models/models.go` (audit table) |

## 6. Adapt candidates

_TBD._ Seed: secret approval policy DSL; machine identity Universal Auth pattern; integration sync model (push secrets to AWS SM / Vercel / GCP).

## 7. Pros / cons of adapting

_TBD._ Be specific about E2E encryption claims — does the design actually prevent server-side decryption or is it marketing-shape?

## 8. Validation evidence

_Required:_ Infisical source-code references; their security whitepaper; any third-party audit; CVE history if any.

## 9. Threat-model implications

_TBD._

## 10. Verdict

_TBD._

## 11. Rollback if adopted

_TBD._

## 12. Won't-break-our-system claim

_TBD._

## 13. References

- (to be populated)
