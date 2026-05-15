# Competitive Research Initiative — Auth, Secrets, Approval

- **Branch of record:** `claude/research-auth-competitors-H8VmQ`
- **Initiative kicked off:** 2026-05-15
- **Sponsor:** acting PM (Tech Lead is interim per `docs/ROADMAP_NOT.md`)
- **Scope:** Auth/identity + secrets-management + approval-workflow competitors
- **Status:** Scaffolding stood up; analysts not yet active

## Why this exists

KeepSave is wrapping Phase A hardening. The auth surface is substantial but partial: the OAuth 2.0 server in `backend/internal/api/handlers_oauth.go` exposes an empty JWKS and no RS256; `SSOConfig` exists in `backend/internal/models/models.go:278` with no OIDC/SAML library wired; FU #5 (JWT denylist) and "fine-grained API key scopes" are Phase-B-deferred. Before Phase B architectural review at day 90, we want a documented external-pattern corpus that tells us which patterns to adapt, which to reject, and where KeepSave can leapfrog the field.

Hard constraints — all recommendations must respect these:

- **`docs/ROADMAP_NOT.md`** — SSO/SAML/OIDC for dashboard end-user auth is a hard "no" until two customers ask. Research may study; adoption requires the trigger.
- Auth/crypto/promotion changes are **Type-1** per `CLAUDE.md` and `docs/ROLES.md` → ADR + Security Engineer veto + Tech Lead sign-off before code.
- Phase A P0 follow-ups (0b/0c/0d/0e) are not blocked by this research; the research feeds them, it does not gate them.

## What this initiative produces

| Deliverable | Path | Owner |
|---|---|---|
| Per-competitor dossiers | `docs/research/competitors/<vendor>.md` | Analyst agents |
| Pattern adoption matrix | `docs/research/PATTERN_MATRIX.md` | Research Lead |
| "Beyond" ideation memo | `docs/research/BEYOND.md` | Research Lead |
| Draft ADRs for top recommendations | `docs/adr/0005-*.md` … `docs/adr/0009-*.md` | ADR Drafter |

## Research team (all agents = Opus)

| Agent | Owns | Output |
|---|---|---|
| Research Lead | Scope, sequencing, weekly sync, matrix + BEYOND memo synthesis | `PATTERN_MATRIX.md`, `BEYOND.md` |
| Identity Provider Analyst | Keycloak (P0), Zitadel (P1), Okta/Auth0/Entra (P2 paragraphs) | OIDC/SAML wire patterns for dormant `SSOConfig` |
| Self-Hosted Auth Analyst | Ory Hydra/Kratos/Oathkeeper (P0), Authentik (P1), Authelia (P2) | JWKS/RS256, refresh-token rotation, denylist patterns |
| Machine Identity Analyst | GitHub fine-grained PATs (P0), SPIFFE/SPIRE (P0), Teleport (P1) | Scope grammar for `ks_` keys, ephemeral creds |
| Secrets Management Analyst | Vault (P0), Infisical (P0), Doppler (P0), 1Password Connect (P1) | Product-shape parity, API design |
| Approval Workflow Analyst | Vault Sentinel (folded into Vault dossier), GitHub Environments (P0) | Required reviewers, approver≠requester (FU 0d), kill switch (FU 0e) |
| Edge Auth Analyst | Pomerium (P0), Cloudflare Access (P1), OAuth2-proxy (P2) | Origin validation, signed headers — feeds FU 0b |
| Security Reviewer | Validates every dossier against `docs/THREAT_MODEL.md`. **Veto** on Verdict / Threat-Implications / Won't-Break sections for any dossier touching `internal/auth`, `internal/crypto`, promotion (mirrors `docs/ROLES.md:33`). | Accept/reject signal on each dossier |
| ADR Drafter | Converts `adopt-now` / `adopt-when-trigger` verdicts into ADRs 0005+ using `docs/adr/0000-template.md`. Activates day 45. | `docs/adr/0005-*.md` … (cap 5 drafts) |

**Coordination model:** Async dossier handoff (analyst → Security Reviewer → Lead accepts). Weekly Mon sync run by Lead. ADR Drafter is downstream-only; activates day 45.

## Competitor target list

**P0 (must research — each tied to a concrete KeepSave gap):**

| Competitor | Tied to KeepSave gap |
|---|---|
| HashiCorp Vault (auth methods + Sentinel + transit) | `docs/adr/0001` envelope encryption; `docs/adr/0003` promotion; `docs/adr/0004` key hierarchy |
| Ory (Hydra + Kratos + Oathkeeper) | `handlers_oauth.go` empty JWKS / no RS256; FU #5 JWT denylist |
| Keycloak | Dormant `SSOConfig` model; ROADMAP_NOT #2 trigger readiness |
| GitHub fine-grained PATs + Environments | FU 0d (approver≠requester); Phase-B fine-grained API key scopes |
| Infisical | Closest peer product; shape parity |
| Doppler | Commercial peer; API-key scoping, audit-log shape |
| SPIFFE/SPIRE | BEYOND memo — ephemeral workload identity for AI agents |
| Pomerium | FU 0b embed widget origin validation |

**P1 (research if bandwidth):** Zitadel, Authentik, Cloudflare Access, 1Password Connect, Teleport.

**P2 (one paragraph only):** Okta, Auth0, AWS Cognito, Azure Entra, AWS Secrets Manager, GCP Secret Manager, Bitwarden Secrets Manager, FusionAuth, Authelia, OAuth2-proxy, ArgoCD/Spinnaker/Octopus.

**Rejected:** Boundary, Tailscale ACLs, AWS IAM grants (different problem domain).

## Dossier template

See [`_DOSSIER_TEMPLATE.md`](./_DOSSIER_TEMPLATE.md). All 13 sections are mandatory for P0 dossiers. P1 may collapse sections 3–5. P2 is a single paragraph.

Word caps: P0 ≤ 2500 words; P1 ≤ 400; P2 ≤ 100.

## Sequencing & exit criteria

Aligned to `docs/ROLES_30_60_90.md`.

### Days 0–30 (parallel to Phase A hardening)

- Deliverables: 8 P0 dossiers in `draft`; matrix v1; 4 weekly syncs.
- **Exit criteria:**
  1. All 8 P0 dossiers have all 13 sections non-empty.
  2. Security Reviewer accepted ≥5 of 8.
  3. Matrix v1 has no `TBD` cells in P0 columns.
  4. Zero changes to `backend/` or `frontend/` from this effort (`git diff main -- backend/ frontend/` empty).

### Days 31–60

- Deliverables: remaining P0 dossiers `accepted`; 3–5 P1 dossiers drafted; draft ADRs 0005+ for top 3 `adopt-now` recommendations; BEYOND memo v1; matrix v2.
- **Exit criteria:**
  1. All P0 dossiers `accepted`.
  2. Every `adopt-now` / `adopt-when-trigger` verdict has a draft ADR with Context, Options, Decision, Rollback filled.
  3. BEYOND memo lists ≥5 candidates with feasibility notes.
  4. Matrix v2 has `Tied FU#` populated for every `adapt` cell.

### Days 61–90

- Deliverables: ADRs reviewed by Tech Lead + Security Engineer; accepted ADRs merged; external-review prep doc references this corpus.
- **Exit criteria:**
  1. ≥2 ADRs in `Accepted` status with both sign-offs.
  2. Rejected ADRs have written rejection rationale per template (`docs/adr/0000-template.md` §Rejection rationale).
  3. `docs/research/` referenced from external-pentest scope doc.

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Scope creep — dossiers bloat | Word caps; >2500w P0 sent back to analyst |
| ADR backlog | ADR Drafter starts day 45; cap 5 draft ADRs total |
| Research disconnected from Phase A | Matrix `Tied FU#` column mandatory for every `adapt` cell |
| Vendor docs misleading / marketing-shaped | Section 8 requires RFC / public audit / CVE / post-mortem — blog-only fails review |
| ROADMAP_NOT violations (Trojan-horse SSO) | `adopt-when-trigger-fires` verdicts must quote the trigger from `docs/ROADMAP_NOT.md` verbatim |
| CVE absence misread as safe | Security Reviewer adds disclosure-program-maturity line for every `adopt-now` |
| Duplicate work | Each pattern row in matrix has exactly one owning analyst |
| Security Reviewer bottleneck | Cap 2 dossiers/week/reviewer; Lead pauses new starts rather than skipping review |
| ADRs slip past day 90 | Day-60 criterion forces rejection rationale even for rejected ADRs |

## What this initiative is NOT

- **Not code changes.** Read-only relative to `backend/` and `frontend/`. The branch `claude/research-auth-competitors-H8VmQ` produces docs only. Implementation lives on separate branches that *cite* the research.
- **Not adoption commitments.** Every `adopt-now` recommendation goes through the standard Type-1 ADR gate. The PM does not pre-approve adoptions.
- **Not SSO/SAML/OIDC dashboard work.** Blocked by `docs/ROADMAP_NOT.md` until the two-customer trigger fires.

## How to engage an analyst

The team is a roster of Opus subagents. To activate one for a specific dossier, the PM (or any reviewer) spawns the agent with:

1. The vendor name and category.
2. The corresponding stub file in `docs/research/competitors/<vendor>.md`.
3. A pointer to `_DOSSIER_TEMPLATE.md` and the KeepSave files the analyst must cite.
4. Word cap and section requirements.

Analyst writes the dossier, then hands off to Security Reviewer for sign-off, then to the Research Lead for acceptance. No analyst may self-accept.
