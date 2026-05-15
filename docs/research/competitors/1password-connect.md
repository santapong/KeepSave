# Competitor Dossier — 1Password Connect

## 1. Header

- **Vendor:** 1Password Connect
- **Category:** secrets management (vault-broker, hybrid)
- **License:** proprietary; self-hosted broker, cloud vault
- **Last updated:** 2026-05-15
- **Analyst:** Secrets Management Analyst
- **Reviewer:** Security Reviewer (accepted-with-changes 2026-05-15)
- **Status:** accepted
- **Priority:** P1

## 5. KeepSave-comparable surface

| Their | KeepSave analog | file:line |
|---|---|---|
| Connect Server (broker → cloud vault) | None — all-on-prem | n/a |
| Service Account vs Personal Token | `ks_` project-scoped; no workplace-scoped analog (gap = `doppler.md` Cand. 1) | `backend/internal/auth/apikey.go:11-26` |
| Connect Access Token | `ks_<hex>`, SHA-256 hashed | `backend/internal/auth/apikey.go:21-23` |
| ~10 language SDKs | Python + Go + Node (3 langs) | `sdks/python/keepsave/client.py`, `sdks/go/keepsave.go`, `sdks/nodejs/src/index.ts` |
| Typed item schema | Flat encrypted string | `backend/internal/models/models.go:39-49` |
| Activity-stream events | `audit_log` rows | `backend/internal/repository/audit_repo.go:21` |

**Not adopted:** vault-broker hybrid; cloud vault; consumer UX (`ROADMAP_NOT.md:44-47`).

## 6. Adapt candidates

1. **Multi-language SDK fleet expansion.** Python+Go+Node ship today; pattern would add Ruby/Java/Rust/.NET. Pro: closes residual adoption gap. Con (op): N pipelines, version skew, per-ecosystem CVE patching — load mirrors `ROADMAP_NOT.md:24-27`. Con (sec): each SDK is a new credential-handling surface.
2. **Typed secret schema.** Categorised items, typed fields. Con (op): `Secret` migration, per-type resolvers, doubled API surface, no demand. Con (sec): `none material`.

## 8. Validation evidence

- Connect docs `developer.1password.com/docs/connect/` (vendor; supplementary).
- Cure53 pentests (2018-2023) + SOC 2 Type II — non-vendor corroborators for crypto posture, not Connect internals.
- CVE history (NVD, 24mo): sparse — closed-source; HackerOne `agilebits`. Absence ≠ safety (`docs/research/README.md:110`).
- SDK-fleet: `github.com/1Password/` binding repos.

**Closed-source verification limitation:** Connect-broker internals not auditable; load-bearing claims rest on non-vendor sources — same constraint as `doppler.md` §8.

## 9. Threat-model implications

- **Cand. 1:** §2 R/E — each SDK is a new client-side handler; widens client count, not server boundary.
- **Cand. 2:** §1 T — narrows malformed-payload risk marginally; no boundary shift.
- **Vault-broker:** not adopted; adds 1Password cloud as new boundary — Type-1 inversion.

## 10. Verdict

- **Cand. 1 — SDK fleet: `reject` (no qualifying trigger).** Go+Node+Python already cover the realistic asks; gap to "~10" is asymptotic. `docs/FOLLOWUPS.md` grep 2026-05-15: zero language-fleet entries. Reviewer: reject stands; SR-note recommends a FU once a 4th-language ask materialises.
- **Cand. 2 — Typed schema: `reject`.** Complexity high, demand absent, conflicts with flat-string model. Reviewer: reject stands.
- **Vault-broker hybrid: `reject`.** Collides with on-prem positioning. No explicit `ROADMAP_NOT.md` rule, but Phase A §1 (multi-tenant) + §6 (marketplace) reinforce all-on-prem; cloud-vault dependency is Type-1 inversion with no signal. Reviewer: reject stands.

## Security Reviewer notes

**Verdict:** accepted-with-changes. Veto **not** exercised. All three rejects upheld — vault-broker reverses on-prem (Type-1 inversion, no signal); typed schema has no demand; SDK-fleet asymptotic with no in-doc trigger.

**Spot-checks (Read, 2026-05-15):** `audit_repo.go:21` Create signature matches §5. `auth/apikey.go:21-23` `ks_`+SHA-256 confirmed. `models/models.go:39-49` flat `Secret` confirmed. `models/models.go:56` `APIKey.ProjectID` non-nullable — workplace-scope gap real.

**Inline fix — §5/§6 SDK row:** dossier said "Python-only" but `sdks/go/keepsave.go` (749 LoC) and `sdks/nodejs/src/index.ts` (577 LoC) ship alongside Python (435 LoC). Corrected to "Python+Go+Node". Cand. 1 reframed "closes residual gap" — **strengthens** reject (gap is 3-vs-10, not 1-vs-10).

**§8:** mirrors `doppler.md` procedure — vendor docs supplementary; load-bearing claims on Cure53 + SOC 2 + NVD/HackerOne. Acceptable.

**§9 STRIDE:** Cand. 1 R/E client-side-only correct. Cand. 2 §1 T marginal-narrow correct. Vault-broker §1 boundary-widen Type-1 framing correct.

**Over-rejection scan:** none. No candidate has a `docs/**` trigger to qualify for `adopt-when-trigger-fires`.

**SDK-distribution-gap → FOLLOWUPS.md recommendation: conditional.** Per task instructions, not promoted by reviewer. Suggested wording for Tech Lead/PM: *"SDK fleet beyond Python/Go/Node. Trigger: customer ask for 4th language (Ruby/Java/Rust/.NET) blocks adoption, or two prospects converge on same missing language in one quarter. Pattern: 1Password Connect (~10 SDKs)."*
