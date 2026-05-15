# Competitor Dossier — Doppler

## 1. Header

- **Vendor:** Doppler
- **Category:** secrets management (commercial peer)
- **License:** proprietary SaaS — patterns only
- **Last updated:** —
- **Analyst:** Secrets Management Analyst
- **Reviewer:** Security Reviewer
- **Status:** not-started
- **Priority:** P0

## 2. One-paragraph overview

_TBD._

## 3. Architecture summary

_TBD._ Cover: project/config/environment hierarchy; integrations layer; service tokens; CLI agent.

## 4. Security model

_TBD._ Cover: AES-256-GCM at rest (matches KeepSave's `docs/adr/0001`); service-token model; audit-log shape; SOC 2 scope.

## 5. KeepSave-comparable surface

| Doppler concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Project / config branch | Project / environment | `backend/internal/models/models.go` |
| Service token | API key | `backend/internal/auth/apikey.go:11-32` |
| Config inheritance + override | Promotion engine | `docs/adr/0003-promotion-engine.md` |
| Activity log | Audit log | `backend/internal/models/models.go` |
| Integrations (AWS SM, Vercel, Netlify, etc.) | Not yet implemented | — |

## 6. Adapt candidates

_TBD._ Seed: config inheritance + override semantics (relevant to promotion engine); service-token shape; CLI agent architecture for AI-agent use case.

## 7. Pros / cons of adapting

_TBD._

## 8. Validation evidence

_Required:_ Doppler trust portal / SOC 2 attestation summary; public security incident disclosures; any post-mortems.

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
