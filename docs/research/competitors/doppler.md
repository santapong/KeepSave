# Competitor Dossier — Doppler

## 1. Header

- **Vendor:** Doppler
- **Category:** secrets management (commercial SaaS peer)
- **License:** proprietary closed-source SaaS — patterns only, no code inspection possible
- **Last updated:** 2026-05-15
- **Analyst:** Secrets Management Analyst
- **Reviewer:** Security Reviewer (accepted 2026-05-15)
- **Status:** accepted
- **Priority:** P0 (per `docs/research/README.md:55`)

## 2. One-paragraph overview

Doppler is a closed-source commercial secrets manager (SOC 2 Type II) competing on shape: project hierarchy, environment branching, encrypted storage, audit logging, machine-identity tokens. Claims about internal mechanism rest on public docs, the Security Fact Sheet, and (where load-bearing) non-vendor corroborators (CircleCI 2023 post-mortem etc.). Doppler ships visible answers to three KeepSave-open problems: service-token vs service-account distinction, documented audit-log taxonomy, and Change Requests (Oct 2024) mirroring FU 0d (approver≠requester).

## 3. Architecture summary

Hierarchy: **Workplace → Project → Environment → Config** (configs are *root* per env or *branch* children that inherit+override). KeepSave's `Project → Environment` maps to Doppler's `Project → Config`; branching has no KeepSave analog.

Identity: **Service Tokens** (config-scoped, default read-only, optional expiry); **Service Accounts** (Team/Enterprise — workplace-scoped, multi-project, role-assignable); plus user/CLI tokens.

Trust boundary (vendor-claimed): a *tokenization service* holds the per-workplace AES-GCM key only during a request; the internet-exposed API never sees plaintext keys or ciphertext. Implementation not public; SOC 2 Type II is the third-party signal.

## 4. Security model

Per Security Fact Sheet (`docs.doppler.com/docs/security-fact-sheet` — unreachable 2026-05-15; AES-GCM primitive load-bearing on NIST SP 800-38D):

- **At rest:** AES-GCM, random IV per secret, per-workplace 256-bit key. Matches `docs/adr/0001`.
- **Key custody:** workplace keys wrapped by HSM-backed GCP KMS key; EKM (Enterprise) adds customer-KMS tier above. Same shape as KeepSave MEK→DEK (`docs/adr/0004-key-hierarchy.md`) + extra tier.
- **Identity:** Service Tokens are opaque random strings, hashed at rest (implied by SaaS norms). KeepSave `ks_<hex>` (`backend/internal/auth/apikey.go:11-26`) — same shape, SHA-256, raw shown once.
- **RBAC:** Workplace roles; per-project per-env Project roles; Custom Roles (Team/Enterprise). Service Accounts assignable like users.
- **Audit:** Workplace Activity Log covers member/secret/project/env/role/settings/integration events; Datadog export (non-vendor corroborator).
- **Approval:** **Change Requests** (Enterprise, GA Oct 2024), peer-reviewed config-change flow (SecBoulevard / Manila Times 2024-10-03, non-vendor).
- **Compliance:** SOC 2 Type II claimed; report gated.

**CVE history (last 24 months):** zero published Doppler CVEs (NVD, CISA bulletins Dec 2025, cvedetails 2024 — none list Doppler). For closed-source SaaS this is expected: disclosure goes through HackerOne (`hackerone.com/doppler` — exists, payouts not publicly indexed) and is remediated server-side. **CVE absence is not a safety signal** — disclosure surface is structurally narrower than OSS, flagged per `docs/research/README.md:110`.

**Non-vendor load-bearing corroborator for service-token threat model:** the CircleCI January 2023 incident (`circleci.com/blog/jan-4-2023-incident-report/`, `threat.wiki/ops/circleci-2023-customer-secret-exposure-incident/`). When a SaaS holding customers' service tokens is compromised, every token rotates. Doppler's exposure model is analogous; this informs §7 Candidate 1 cons.

## 5. KeepSave-comparable surface

| Doppler concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Project → Environment → Config (root) | Project → Environment | `backend/internal/models/models.go:21-37` |
| Branch config (child of a root) | None — `Environment` has no parent/child link | `backend/internal/models/models.go:32-37` |
| Service Token (config-scoped, default read-only) | API key (project-scoped, optional `Environment *string`, `Scopes StringList`) | `backend/internal/auth/apikey.go:11-32`; `backend/internal/models/models.go:51-61` |
| Service Account (workplace-scoped programmatic identity, multi-project) | None — `APIKey.ProjectID uuid.UUID` is single-project, non-nullable | `backend/internal/models/models.go:56` |
| Custom Roles (per-permission tuples) | Org member `Role` is enum string (viewer/editor/admin/promoter) | `backend/internal/models/models.go:220-227` |
| Activity Log event taxonomy (member/secret/integration/settings) | `audit_log` rows via `AuditRepository.Create(userID, projectID, action, environment, details, ip)` | `backend/internal/repository/audit_repo.go:21-35`; `backend/internal/models/models.go:63-72` |
| Activity Log export (Datadog integration) | Not implemented; soft-no in `docs/ROADMAP_NOT.md:53` | `docs/ROADMAP_NOT.md:53` |
| Change Requests (peer-approval flow for secret changes) | `PromotionRequest` with `RequestedBy` / `ApprovedBy` (promotion-only; not per-secret-edit) | `backend/internal/models/models.go:101-115`; routes at `backend/internal/api/router.go:90-96` |
| Token expiration on service tokens | `APIKey.ExpiresAt *time.Time` (nullable) | `backend/internal/models/models.go:59` |
| Workplace key wrapped by HSM/KMS; optional customer EKM | Master key via `MasterKeyProvider`; AWS/GCP providers exist but not wired (per threat-model §1) | `docs/adr/0004-key-hierarchy.md`; `backend/internal/crypto/keyprovider/` (per `THREAT_MODEL.md:69`) |
| Config inheritance + override (parent → child secrets) | None — promotion *copies* between envs, no inherit-and-override link | `docs/adr/0003-promotion-engine.md` |

**Concepts deliberately not adopted** (pattern-only adoption; we do not deploy their product):

| Their concept | Reason we don't adopt |
|---|---|
| Integrations layer (push to AWS SM, Vercel, etc.) | `docs/ROADMAP_NOT.md:39-42` — hard no for Phase A. |
| `doppler run` CLI agent | Out of scope — KeepSave's machine-identity story is `ks_` API key + REST. |
| Doppler Share (one-off encrypted links) | Adjacent product space; non-goal. |
| Tokenization-service architecture | Type-1 shift; defer until `docs/ROADMAP_NOT.md:13-17` tenancy work. |
| Hosted SaaS multi-tenant runtime | `docs/ROADMAP_NOT.md:13-17` — hard no for Phase A. |

## 6. Adapt candidates

Pattern adoptions only — KeepSave does not deploy Doppler.

1. **Service Token vs Service Account distinction.** Doppler separates narrow-scoped config tokens (default read-only, single config) from broad-scoped workplace identities (multi-project, role-bound). KeepSave's `APIKey` is the first; the second has no analog. Adopting means a new `service_account` table whose tokens can hold *N* per-project role rows. Directly relevant to AI-agent identity.
2. **Audit-log taxonomy gaps.** Doppler's documented categories (member/secret/integration/settings/project) align with KeepSave's `docs/AUDIT_LOG_COVERAGE.md`. The value is confirming the gap: **role-change** and **settings-change** events are in Doppler's taxonomy but missing from KeepSave's canonical list at `docs/AUDIT_LOG_COVERAGE.md:21-43`.
3. **Change Requests for secret edits.** Doppler's Change Request applies the approver≠requester pattern to *config edits*, not just environment promotion. KeepSave's `PromotionRequest` (`models.go:101-115`) gives the pattern at promotion time only; this extends it one lifecycle step earlier.
4. **Branch config / inherit-and-override** (likely reject). Solves per-region overrides without copy-paste, but conflicts with ADR-0003 promotion-as-copy. Listed for completeness.

## 7. Pros / cons of adapting

### Candidate 1: Service Token vs Service Account distinction

- **Pros:** Solves a real AI-agent identity gap — one runtime needing read access across several KeepSave projects today requires N distinct `ks_` keys (because `APIKey.ProjectID` at `models.go:56` is non-nullable). Service-account shape would let one identity hold N project-role grants. Aligns with the Machine-Identity dossier (GitHub fine-grained PATs).
- **Cons (operational):** Net-new entity + tables (`service_accounts`, `service_account_project_roles`); revocation semantics expand (revoke account vs revoke single grant); doubles the support-surface "what does this token allow" question.
- **Cons (security):** Broader-scoped token = larger blast radius if leaked. The CircleCI 2023 incident is the load-bearing reminder — when a SaaS holding tokens is compromised, every token rotates. Mitigation: mandatory `ExpiresAt`, default ≤90 days. Type-1 auth change; Security Engineer veto applies per `CLAUDE.md` decision-class table.

### Candidate 2: Audit-log taxonomy gaps (role-change, settings-change events)

- **Pros:** Two-line addition to `docs/AUDIT_LOG_COVERAGE.md`. Doppler's taxonomy reveals KeepSave is missing `role.changed` (`OrgMember.Role` at `models.go:224` is mutable via `router.go:171` with no audit-coverage entry) and `settings.changed` (SSO config updates via `router.go:175`). Pure spec work.
- **Cons (operational):** Adds two events to FU #0 scope — marginal; both endpoints are already in scope.
- **Cons (security):** `none material — adding audit coverage is monotonically safer`.

### Candidate 3: Change Requests for secret edits

- **Pros:** Extends FU 0d's approver≠requester invariant past the promotion boundary into per-secret-edit. Higher ceiling for high-blast-radius PROD secrets. Doppler shipped it Oct 2024 — proven at commercial scale.
- **Cons (operational):** Per-edit approval is a real friction tax; Doppler ships only on Enterprise tier, hinting it's heavy. KeepSave's current customer set is too small for the value to be clear.
- **Cons (security):** Performative approval is the failure mode — if approver auto-clicks, the audit row is misleading. Same class as `THREAT_MODEL.md:100` "Requester self-approves" (Medium residual); fix is a DB-layer invariant (`requester_id ≠ approver_id`), not just app-layer.

### Candidate 4: Branch config / inherit-and-override

- **Pros:** Solves per-region / per-feature override without copy-paste.
- **Cons (operational):** Conflicts directly with ADR-0003 promotion-as-copy semantics. Either model is internally consistent; running both is incoherent.
- **Cons (security):** Inheritance widens the blame surface — "which config does this value come from?" is a real operator question Doppler users hit.

## 8. Validation evidence

Doppler is closed-source commercial SaaS — the §8 vendor-unreachable triangulation procedure applies. All Doppler docs URLs returned HTTP 403 on 2026-05-15; content recovered via search-result summaries; **load-bearing security claims corroborated by non-vendor sources**.

- **AES-GCM at rest:** Security Fact Sheet (unreachable). Load-bearing on **NIST SP 800-38D** — non-vendor, authoritative.
- **Service-token scope:** `docs.doppler.com/docs/service-tokens` (unreachable). Corroborator: **Security Boulevard "How to set up Doppler for secrets management" 2025-09** — non-vendor.
- **Service Accounts:** `docs.doppler.com/docs/service-accounts` (unreachable); shape inferred from CLI behaviour + vendor RBAC glossary. **Non-vendor corroborator:** Security Boulevard 2025-09 step-by-step guide enumerates Service-Account scope shape (workplace-scoped, multi-project, role-assignable) — the load-bearing claim that drives §9 "widens trust boundary" rests on this non-vendor source, not the vendor glossary.
- **Activity Log shape:** `docs.doppler.com/docs/workplace-logs` (unreachable). Corroborator: **Datadog Doppler integration docs** (`docs.datadoghq.com/integrations/doppler/`) — non-vendor, enumerates event categories.
- **Change Requests (Oct 2024):** vendor press release; non-vendor corroborators **Security Boulevard 2024-10-03, Manila Times / GlobeNewswire 2024-10-03**.
- **Service-token blast-radius:** **CircleCI Jan 2023 incident report** + **threat.wiki/ops/circleci-2023-customer-secret-exposure-incident** — non-vendor, load-bearing for §7 Candidate 1 cons.
- **Branch configs:** `docs.doppler.com/docs/branch-configs` (unreachable). No load-bearing security claim; informs shape only.
- **CVE history:** NVD + CISA searched 2026-05-15 — zero Doppler entries. HackerOne `hackerone.com/doppler` exists; payouts not indexed. CVE absence is structural (closed-source SaaS), **not** a safety signal — per `docs/research/README.md:110`.

## 9. Threat-model implications

Maps to `docs/THREAT_MODEL.md` §2 "Authentication", row **E (Elevation): API key scope escalation** at line 88. The current mitigation reads:

> *"Scope is row-bound; per-project / per-env enforced in handlers — `auth/apikey.go`, `models/models.go:56-59` — Residual: Low"*

Adopting **Candidate 1** (service accounts) **widens** the trust boundary: a new entity, the workplace-scoped service account, sits above any single project. Today an API key is bounded by `ProjectID` (one row, one project); a service account is bounded by *N* role grants, each revocable independently. The new entity's compromise has wider blast radius than today's `APIKey`. Residual for row E reclassifies from Low to **Medium** until expiry enforcement + rate-limited rotation are demonstrated.

Adopting **Candidate 2** (audit-log taxonomy) **narrows** the boundary on row R (Repudiation) at `THREAT_MODEL.md:85-86` — currently `Medium` residual because not all mutating actions emit audit events.

Adopting **Candidate 3** (Change Requests) **narrows** the boundary on §3 promotion row "Requester self-approves" at `THREAT_MODEL.md:100` — same DB-invariant fix applies one layer deeper (per-secret-edit, not just promotion).

Candidate 4 (branch configs) — not adopting; no implication.

## 10. Verdict

- **Candidate 1 (Service Account):** `adopt-when-trigger-fires`. Trigger quoted verbatim from `docs/FOLLOWUPS.md:168`:

  > *"Fine-grained API key scopes (per-secret or per-action; ADR-0002). Build when a use case appears, not before."*

  Service accounts are the natural shape once the per-action axis is granted. Type-1 ADR + Security Engineer veto apply.

- **Candidate 2 (audit taxonomy):** `adopt-now`. Two events added to `docs/AUDIT_LOG_COVERAGE.md`; handlers (`router.go:171, 175`) already in FU #0 scope. Type-3 doc + Type-2 implementation; no Type-1 ADR required.

- **Candidate 3 (Change Requests):** `adopt-when-trigger-fires`. Trigger quoted verbatim from `docs/ROADMAP_NOT.md:32`:

  > *"Trigger to revisit: non-technical customer admin role becomes a buyer."*

  Per-edit approval is the same shape as code-driven policy; same buyer signal. Type-1 ADR applies.

- **Candidate 4 (branch configs):** `reject`. Conflicts with ADR-0003 promotion semantics.

Security Reviewer veto applies on §9, §10, §12 (Candidate 1 touches `internal/auth`).

## 11. Rollback if adopted

- **Candidate 2 (audit taxonomy):** trivial — `git revert` doc + audit-emit calls. No data destroyed; existing rows remain queryable.
- **Candidate 1 (service accounts) if fired:** drop new `service_accounts` / `service_account_project_roles` tables only after a deprecation window. Existing `api_keys` rows unaffected — old-shape keys continue to work. True rollback possible; forward-fix preferred (feature-flag issuance endpoint to 503, keep rows readable for revocation).
- **Candidate 3 (Change Requests) if fired:** approval rows reference approver IDs in audit history. Hard-delete destroys audit; orphan-and-deprecate is the only acceptable path. Forward-fix only.

## 12. Won't-break-our-system claim

Only Candidate 2 is proposed for adopt-now; invariants for Candidates 1 and 3 are stated for the trigger-fires moment.

- **Invariant 1 — `audit_log` schema unchanged.** Adding new `action` strings (`role.changed`, `settings.changed`) requires no migration. Verified at `audit_repo.go:21-35` — `INSERT` accepts arbitrary `action`; no enum constraint. `AuditEntry.Action` is `string` (`models.go:67`).
- **Invariant 2 — `ListByProjectID` continues to work.** New events appear in existing query results; handlers retrieve the full list. Verified at `audit_repo.go:37-60`.
- **Invariant 3 — `audit_repo_test.go` stays green.** Existing four tests insert/retrieve arbitrary action strings; new strings don't break them. Test-file existence verified per `THREAT_MODEL.md:59`.
- **Invariant 4 — nightly pruner unaffected.** `DeleteOlderThan` is action-agnostic (deletes by `created_at < cutoff`). Verified at `audit_repo.go:66-83`.
- **Invariant 5 — Candidates 1/3 (if fired) preserve `APIKey.HashedKey = sha256(rawKey)`.** Verified at `auth/apikey.go:21-24`. Service-account tokens reuse the same primitive.

Security Reviewer veto applies. Suggested checks: (a) `role.changed` emit includes both `old_role` and `new_role` in `details` — otherwise audit is half-blind; (b) `settings.changed` names the field changed without logging secret values (never raw `ClientSecretEncrypted` at `models.go:284`); (c) Candidate 1's trigger is met by a real customer ask, not analyst opinion, before the ADR opens.

## 13. References

All `docs.doppler.com` URLs unreachable via tooling 2026-05-15 (HTTP 403); content via search summary.

- Doppler vendor docs (Service Tokens, Service Accounts, Advanced Permissions, Custom Roles, Branch Configs, Config Inheritance, Workplace Structure, Activity Logs, Security Fact Sheet, Enterprise Key Management): under `https://docs.doppler.com/docs/` (retrieved 2026-05-15)
- Datadog Doppler integration (non-vendor corroborator): `https://docs.datadoghq.com/integrations/doppler/` (2026-05-15)
- Security Boulevard — "Doppler Launches 'Change Requests'" 2024-10: `https://securityboulevard.com/2024/10/doppler-launches-change-requests-...` (2026-05-15)
- Security Boulevard — "How to set up Doppler" 2025-09: `https://securityboulevard.com/2025/09/how-to-set-up-doppler-...` (2026-05-15)
- Manila Times / GlobeNewswire — Change Requests release 2024-10-03 (2026-05-15)
- CircleCI Jan 4 2023 incident report (non-vendor, load-bearing): `https://circleci.com/blog/jan-4-2023-incident-report/` (2026-05-15)
- threat.wiki — CircleCI 2023 customer secret exposure (2026-05-15)
- NIST SP 800-38D (AES-GCM): `https://csrc.nist.gov/publications/detail/sp/800-38d/final` (2026-05-15)
- RFC 7519 (JWT) (2026-05-15)
- Doppler HackerOne program: `https://hackerone.com/doppler` (2026-05-15)
- NVD CVE search for Doppler — no matches last 24 months (2026-05-15)

## Security Reviewer notes

**Verdict:** accepted-with-changes (retrofitted inline). Veto **not** exercised. Candidate 2 `adopt-now`; 1+3 stay `adopt-when-trigger-fires`; 4 reject stands.

**Refs spot-checked (Read, 2026-05-15):** `audit_repo.go:21-35` — `Create` signature as quoted, no enum on `action` (§12 Inv 1 confirmed). `models.go:51-61` — `APIKey.ProjectID uuid.UUID` non-nullable; multi-project gap real. `auth/apikey.go:11-26` — `ks_` + `sha256.Sum256` (§12 Inv 5; dossier `:11-32` acceptable, file ends 33). `models.go:67` — `Action string`, no enum, confirms zero-migration. `models.go:220-227` — `OrgMember.Role string` mutable; `role.changed` gap real.

**§8 triangulation — 7 sources, 6 pass / 1 fixed inline:**

| # | Claim | Non-vendor corroborator | Verdict |
|---|---|---|---|
| 1 | AES-GCM at rest | NIST SP 800-38D | PASS |
| 2 | Service-token scope | Security Boulevard 2025-09 | PASS |
| 3 | Service Accounts (load-bearing §9) | **Was vendor-only — FIX**: SecBoulevard 2025-09 added | PASS after fix |
| 4 | Activity Log shape | Datadog Doppler integration | PASS |
| 5 | Change Requests Oct 2024 | SecBoulevard + Manila Times/GlobeNewswire | PASS |
| 6 | Service-token blast-radius | CircleCI 2023 + threat.wiki | PASS |
| 7 | Branch configs (not load-bearing) | Template permits vendor-only | PASS |

CVE history (zero) via NVD/CISA — authoritative. First heavy §8 field test: 6/7 correct, 1 fixed — exactly what the procedure exists to catch.

**§9 STRIDE quotes (verbatim against `docs/THREAT_MODEL.md`):** §2 row E line 88: *"Scope is row-bound; per-project / per-env enforced in handlers — `auth/apikey.go`, `models/models.go:56-59` — Residual: Low"* — match. **Caveat:** `SECURITY_AUDIT_2026-05-15.md` §5.1 already downgrades this to High under A01-F2; Candidate 1 lands on broken baseline. §2 row R lines 85-86: *"Login attempts not audited"* / *"Auth events not in audit log (verify in 30d)"* Medium; Candidate 2 narrows via `role.changed`/`settings.changed` but does **not** close `auth.login_failed` (A09-F3). §3 line 100: *"Invariant not currently enforced at DB layer (gap)"* Medium; Candidate 3 matches A04-F1 remediation #6.

**§10 verbatim quotes:** Candidate 1 text correct, **line wrong** — `FOLLOWUPS.md:161` (heading) → corrected to `:168`. Candidate 3 `ROADMAP_NOT.md:32` correct.

**Audit-log taxonomy cross-ref:** `AUDIT_LOG_COVERAGE.md:21-43` gaps confirmed: `role.changed` (no row for mutable `OrgMember.Role`), `settings.changed` (no row for SSO/org). `SECURITY_AUDIT_2026-05-15.md` A09-F2 covers a **different axis** (existing taxonomy entries not emitted); dossier surfaces **additional holes** A09 didn't enumerate. Complementary. Fold both rows into `AUDIT_LOG_COVERAGE.md` in the same PR as A09-F2 emission fix (shared path `router.go:171, 175`).

**§11/§12 rollback:** Checklist asked if audit-log shape change needs migration+backfill. **No** — `audit_repo.go:21-35` has no enum; `Action string` accepts arbitrary values. Pure code+doc. Candidate 3 forward-fix-only stance correct (approval rows in audit history; hard-delete intersects A08-F1 tamper-evidence).

**Inline changes:** (1) §1 status `draft`→`accepted`. (2) §8 Service-Accounts row — SecBoulevard 2025-09 added. (3) §10 Candidate 1 trigger line `:161`→`:168`.

**Non-blocking observations (ADR Drafter on trigger):** (a) Candidate 1 ADR must re-baseline against 2026-05-15 audit (row E already High); mandatory `ExpiresAt` ≤90d co-ships with FU #0k. (b) Candidate 2's `role.changed` captures `old_role`+`new_role`; `settings.changed` names field, never logs `ClientSecretEncrypted`. (c) Candidate 3 DB-layer `requester_id ≠ approver_id` matches A04-F1; ship constraint + app check together.

No reference failed.
