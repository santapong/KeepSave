# Competitor Dossier — GitHub (Fine-Grained PATs + Environments)

## 1. Header

- **Vendor:** GitHub (fine-grained Personal Access Tokens + Deployment Environments)
- **Category:** machine identity (PATs) + approval workflow (Environments)
- **License:** proprietary / N/A — patterns only, not deployed product
- **Last updated:** 2026-05-15
- **Analyst:** Machine Identity Analyst + Approval Workflow Analyst
- **Reviewer:** Security Reviewer (pending)
- **Status:** draft
- **Priority:** P0 (tied to FU 0d approver≠requester + Phase-B fine-grained API key scopes)

## 2. One-paragraph overview

GitHub ships two pieces of platform infrastructure that map onto two open KeepSave gaps. Fine-grained Personal Access Tokens (GA October 2022) replaced legacy OAuth-scope strings with a `repository × permission × expiration` tuple — the shape KeepSave needs to make `ks_` keys honour least privilege. Deployment Environments add a protection-rule set (required reviewers, wait timer, deployment-branch allow-list) whose required-reviewers rule enforces `approver ∉ requester` at the platform layer — the invariant FU 0d says KeepSave does not enforce. We adopt **patterns**, not GitHub itself.

## 3. Architecture summary

**Fine-grained PATs:** mint via UI selecting (a) resource owner, (b) repository selector (`all`/`public`/explicit list), (c) per-permission level (`read`/`write`/`admin`) across ~50 named permissions (`contents`, `actions`, `secrets`, `deployments`, …), (d) **mandatory** expiration (≤366d; default 30; no "never"). `github_pat_` prefix enables secret-scanning. Org admins can require fine-grained-only and pre-approve the permission set. Every API call is logged with the token ID.

**Environments:** named Environments (`production`, `staging`, …) carry protection rules evaluated before a deployment job: required reviewers (1–6 users/teams, requester excluded), wait timer (0–43200 min), deployment-branch allow-list, environment-scoped secrets.

KeepSave interacts with neither directly. We borrow the grammar (PATs → `ks_` keys) and the protection-rule shape (Environments → promotion engine).

## 4. Security model

PAT primitives (vendor docs retrieved 2026-05-15 via search summary; direct `docs.github.com` returned 403):

- **Token shape:** opaque random + `github_pat_` prefix; SHA hash at rest; raw shown once. Same posture KeepSave has (`auth/apikey.go:21-23`).
- **Scope enforcement:** API gateway evaluates `(token, target_repo, required_permission)` per request → 403 on missing permission. Denylist-by-default — fresh PAT has zero permissions until granted.
- **Expiration:** mandatory; default 30d; ceiling 366d. Org admins can lower the ceiling.
- **Audit:** every authenticated call writes `{actor, token_id, action, repo, timestamp}` to org audit log.

Environments: approver pool (users/teams, membership resolved at decision time); approver ≠ requester (triggering user removed from eligible set; API rejects self-approval `422 cannot approve own deployment`); every approval/rejection audited with `actor`, `environment`, `run_id`, requester.

CVE / disclosure history (NVD + GHSA, last 24 months): **CVE-2024-4985** (GHES SAML signature bypass, 2024-05) and **CVE-2024-8770/8810** (GHES sensitive-data exposure, 2024-09) evidence a responsive program (closes the "CVE absence misread as safe" risk from `docs/research/README.md:111`). No CVE filed against fine-grained-PAT permission-evaluation logic itself in 24 months — **suggestive, not load-bearing**; §10 security claim rests on NIST SP 800-63B and OWASP ASVS V8/V14.

## 5. KeepSave-comparable surface

| GitHub concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Fine-grained PAT (`github_pat_*`), SHA-hashed at rest | API key (`ks_*`), SHA-256 hashed, constant-time compare | `backend/internal/auth/apikey.go:11-26` |
| PAT scope tuple (resource × permission × level) | `APIKey.Scopes` flat `StringList` (e.g. `["read"]`) | `backend/internal/models/models.go:51-61` |
| PAT mandatory expiration (≤366d, default 30d) | `APIKey.ExpiresAt *time.Time` — **nullable; `Create` accepts no `expiresAt` arg → keys default indefinite** | `backend/internal/models/models.go:59`; `backend/internal/service/apikey_service.go:33-60` |
| Per-PAT-call audit-log entry | API-key middleware does **not** emit audit on use | `backend/internal/api/middleware.go:88-120` |
| Environment with required reviewers (1–6) | PROD promotion has one `ApprovedBy` slot; `OrgMember.Role` knows `promoter`; reviewer pool not declarative | `backend/internal/service/promotion_service.go:214-240`; `backend/internal/models/models.go:220-227` |
| `approver ∉ requester` enforced at platform | **Not enforced** — `ApprovePromotion` never compares `approverID` to `promotion.RequestedBy` | `backend/internal/service/promotion_service.go:215-240` (FU 0d) |
| Environment wait-timer | No analog — promotion is synchronous | none |
| Deployment-branch allow-list | No analog — promotion source is a string-named env | `backend/internal/service/promotion_service.go:180` |

**Concepts deliberately not adopted:**

| Their concept | Reason |
|---|---|
| GitHub Apps (installation tokens, signed JWTs) | Different problem domain — KeepSave clients are agents and CI, not third-party services installed in a tenant. |
| Secret-scanning partner program | Separate product surface; revisit if a `ks_*` token leak appears in a public repo. |
| OAuth-app scopes (legacy `repo`, `admin:org`) | The thing GitHub itself replaced — adopting the predecessor. |
| GraphQL audit-log endpoint | Query-layer concern; KeepSave audit log is row-based JSONB (`models/models.go:63-72`). |

## 6. Adapt candidates

1. **Approver ≠ requester DB-level invariant** — `CHECK (approved_by IS NULL OR approved_by <> requested_by)` on `promotion_requests` + service-layer 422 guard in `ApprovePromotion` before touching DB. Closes FU 0d. Pattern source: Environments' `cannot_approve_own_deployment`.
2. **Fine-grained scope grammar for `ks_` keys** — Replace flat `StringList` interpretation with parsed tuple `{environment, resource, verb}` (e.g. `prod:secrets:read`). Verbs map to handler tags; resources to entity types; environment defaults to the key's existing `Environment *string`. On-disk column stays `TEXT[]` — only interpretation changes. Pattern source: PAT permission tuples.
3. **Default expiration on `ks_` key creation** — Change `APIKeyService.Create` signature to require `expiresAt`, with 90-day default and 365-day ceiling. Legacy `ExpiresAt = NULL` rows get a `default_legacy` flag valid until first rotation. Pattern source: PAT mandatory expiration.
4. **Audit emit on every API-key authenticated request** (or per-key first-use-per-day) — `APIKeyAuthMiddleware` writes an `apikey_used` audit row so an in-product trail exists for every agent action. Pattern source: GitHub's `(actor, token_id, action, repo)` audit shape. Volume trade-off in §7.
5. **Declarative reviewer pool with `min_count`** — `promotion_reviewers (project_id, environment, reviewer_user_id, min_count)`. Promotion service queries this table at approval time, excludes `requested_by`, requires `min_count` distinct approvals. `min_count=1` is today; `min_count=2` is the Phase-B "two-of-N" knob ADR-0003 §74 names. Pattern source: Environments required-reviewers rule.

## 7. Pros / cons of adapting

### Candidate 1: Approver ≠ requester DB invariant

- **Pros:** Closes FU 0d. Single migration + one service guard. Defence in depth (DB constraint catches future regression). Closes the named gap at `docs/THREAT_MODEL.md:100`.
- **Cons (operational):** Existing rows where `requested_by == approved_by` (if any) must be migrated to `rejected` with `migration_reason`. Single-developer dev workflows (self-request + self-approve for convenience) break — correct behaviour, but breaks habits.
- **Cons (security):** `none material — invariant narrows the trust boundary; sock-puppet accounts are an org-membership problem out of scope per docs/THREAT_MODEL.md:134.`

### Candidate 2: Fine-grained scope grammar

- **Pros:** Addresses Phase-B deferred item (`docs/FOLLOWUPS.md:161`). Backward-compatible if parser treats bare `"read"`/`"write"` as `*:*:read`/`*:*:write`. No schema change; each scope decision becomes auditable structured data.
- **Cons (operational):** Doubles authorization test surface (FU 0c matrix already incomplete). UI for picking permissions becomes non-trivial; per `ROADMAP_NOT.md` §4 no GUI policy editor — customers manage tuples as code/CLI strings.
- **Cons (security):** A parser bug that silently coerces `prod:secrets:read` to `*:*:*` is a privilege-escalation primitive (CVE-2022-23529 class). Mitigation: deny-on-unparseable, fuzz, lint that the grammar set is closed. CVE-2024-8770/8810 confirms permission-evaluation code is a CVE class.

### Candidate 3: Default expiration on `ks_` keys

- **Pros:** Closes a silent gap: `Create` at `apikey_service.go:33-60` never sets `ExpiresAt`, so every `ks_` key in the system is **immortal** until manual rotation — materially worse than GitHub's mandatory ≤366d.
- **Cons (operational):** Hard-coded customer integrations fail at 90 days. Mitigation: monitor-then-enforce (warn at 60d, enforce at 90d, runbook + release note).
- **Cons (security):** `none material — moving from indefinite to bounded only narrows blast radius. Edge case (customer truly needs indefinite) is solved by an auditable sentinel ExpiresAt='9999-12-31' with indefinite_key_granted event, not by removing the bound.`

### Candidate 4: Per-use API-key audit emission

- **Pros:** Closes §5 row 4 — today **zero** in-product trail of which API key did what at the middleware layer. Sibling to FU 0 (which covers service-layer audit gaps).
- **Cons (operational):** Audit-log volume. An agent polling every second produces 86,400 rows/day per key. Mitigation: per-key/per-day or per-key/per-distinct-action dedupe. Without dedupe, the audit-log pruner becomes load-bearing and any tuning bug is a DB-fill incident.
- **Cons (security):** Under dedupe, none material. Without dedupe, audit volume is a minor covert channel for caller load profile — only relevant if audit log is exposed to lower-trust principals.

### Candidate 5: Declarative reviewer pool with `min_count`

- **Pros:** Replaces the implicit-single-approver model with a declarative table the Security Engineer can review. `min_count=2` becomes a per-project knob, satisfying ADR-0003's "two-of-N if regulatory pressure" deferral without rewriting the engine.
- **Cons (operational):** New table + admin UI (forbidden in Phase A per `ROADMAP_NOT.md` §4) or CLI surface. Reviewer-pool changes themselves must be audited and approved (recursive approval). Defer to Phase B.
- **Cons (security):** A pool that includes the requester's role-peers (e.g. all `promoter` members of a 2-person org) collapses to single-approver in practice. Mitigation is process, not code; adoption requires org-size guidance.

## 8. Validation evidence

- **NIST SP 800-63B §4.2-4.3** — `pages.nist.gov/800-63-3/sp800-63b.html`. Least-privilege + credential-expiration principles that load-bear Candidates 2 and 3. **Non-vendor source carrying the §10 security claim.**
- **OWASP ASVS v4.0.3** — V8 (Data Protection) + V14.1 (separation of duties) — direct analog to Candidates 1 and 3.
- **RFC 6749 §1.4 / §1.5** — PATs are bearer tokens; refresh discussion is the formal analogue of "rotate before expiry".
- **CVE-2022-23529** (JWT alg-confusion) — grammar parsers / signature verifiers are a recurring CVE class (informs Candidate 2 cons).
- **CircleCI January 2023 incident report** — concrete real-world cost of long-lived machine credentials (the gap Candidate 3 closes).
- **GitHub Engineering blog — fine-grained PAT GA (2022-10-18)** — vendor, supplemental only. **Not load-bearing.**
- **GitHub docs (vendor, unreachable via tooling 2026-05-15)** — PAT management + environment management pages returned 403; cited via search summary + NIST/OWASP corroborators above per template §8 triangulation.

## 9. Threat-model implications

Two STRIDE rows in `docs/THREAT_MODEL.md`:

**§3 row E (line 100):** *"Requester self-approves … Invariant not currently enforced at DB layer (gap) … Residual: Medium"*. **Candidate 1** moves Medium → Low and adds the DB-layer enforcement the row says is missing. Trust boundary **narrows**: today a single compromised `promoter` account can both request and approve; after adoption, it cannot. No new entity enters the boundary.

**§2 row E (line 88):** *"API key scope escalation … Residual: Low"*. **Candidate 2** keeps Low and tightens the grammar: scope check moves from "stringly typed" to "parsed tuple". Parser becomes a new sub-component inside boundary 2 (caller identity) that must be tested at middleware rigor.

**§2 row T (line 84)** (no denylist; Medium): **Candidate 3** removes the API-key half of the symmetric problem — exfiltrated `ks_` keys today have no time bound; after adoption, ≤90d.

**Candidate 4** narrows the Repudiation gap in "Findings new v1.2.0 §1" (`docs/THREAT_MODEL.md:38-41`): API-key-authenticated actions today leave no middleware-layer trail. Boundary doesn't move; gap inside shrinks.

## 10. Verdict

**Candidate 1 (approver ≠ requester DB invariant): `adopt-now`.** Trigger quoted verbatim from `docs/FOLLOWUPS.md:48-52`:

> *"### 0d. Approver-cannot-be-requester invariant unverified · **Status:** ADR-0003 §Open Questions calls it out. Not yet verified whether enforced at DB or only service code. · **Why it matters:** The multi-party-control linchpin for PROD promotions. · **Owner:** Backend Engineer + Security Engineer. · **Due:** 30 days."*

Type-1 ADR required (touches promotion engine) per `CLAUDE.md` Decision-classes; Security Engineer veto applies per `docs/ROLES.md`.

**Candidate 2 (fine-grained scope grammar): `adopt-when-trigger-fires`.** Trigger quoted verbatim from `docs/FOLLOWUPS.md:161`:

> *"**Fine-grained API key scopes** (per-secret or per-action; ADR-0002). Build when a use case appears, not before."*

Hold the design in this dossier as the canonical shape; do not implement until a customer or threat surfaces it.

**Candidate 3 (default expiration on `ks_` keys): `adopt-now`.** No deferral trigger exists; the current indefinite-key state is an implementation gap, not a deliberate choice. The dossier promotes this to a new FU 0k (*Default-expiration on ks_ API keys*) in the same PR; the verdict text serves as the trigger for an ADR-0002 revision.

**Candidate 4 (per-use API-key audit emission): `adopt-now`** as a sub-task of FU 0. The audit-log coverage gap explicitly scopes Create/Update/Delete; this dossier extends the same logic to authenticated *reads* via API key, which `docs/AUDIT_LOG_COVERAGE.md` should add. Dedupe scheme is non-negotiable (per §7) — ADR must specify.

**Candidate 5 (declarative reviewer pool): `adopt-when-trigger-fires`.** Trigger quoted verbatim from `docs/FOLLOWUPS.md:162`:

> *"**Three-of-N approval for PROD** (ADR-0003). Build if regulatory pressure or a customer commitment forces it."*

ADR-0003 line 74 corroborates: *"PROD approval is one approver other than the requester. Two-of-N approval is not implemented; revisit if regulatory pressure appears."*

Security Reviewer veto applies (touches `internal/auth` + promotion).

## 11. Rollback if adopted

**Candidate 1:** `DROP CONSTRAINT promotion_requests_no_self_approve` + revert service guard. Audit rejection rows do not survive re-approval (we don't rewrite history). Runbook: feature-flag `PROMOTION_REQUIRE_DISTINCT_APPROVER` defaults true; set false in incident.

**Candidate 3:** Forward-fix only. Once keys are minted with `ExpiresAt = NOW() + 90d`, they hit their expiry regardless of rollback. Rollback: change default to `NULL` for new keys; expired keys cannot be un-expired (raw value not recoverable — only SHA hash per `auth/apikey.go:21-23`). Customer must mint a new key.

**Candidate 4:** Trivial — remove the `auditRepo.Create` call from `APIKeyAuthMiddleware`. Rows already written stay; pruner clears on schedule.

**Candidates 2 and 5:** Deferred — rollback plans drafted at ADR time when triggers fire.

## 12. Won't-break-our-system claim

Verified by reading the named files at the named lines, 2026-05-15.

- **Invariant 1 — Self-approval dev workflows fail loudly after Candidate 1 (intended, not regression).** Verified by reading `promotion_service.go:215-240`: today `ApprovePromotion` never reads `promotion.RequestedBy`, so same-user request+approve succeeds silently. After adoption, the service guard returns 422 and the DB constraint backstops. `promotion_service_test.go` will need a second-user fixture — Backend Engineer adds in the same PR.
- **Invariant 2 — Existing `ks_` keys with `ExpiresAt = NULL` remain valid after Candidate 3.** Verified by reading `middleware.go:101`: `if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now())` — NULL short-circuits to "not expired". Candidate 3 changes only the default for new keys; legacy NULL rows continue to authenticate until a separate FU rotates them.
- **Invariant 3 — Scopes column shape unchanged under Candidate 2.** Verified by reading `models.go:51-61` (`Scopes StringList`) and `models.go:158-167` (`StringList.Value()`). Candidate 2 changes interpretation in a new `auth/scope.go`, not storage. `["read"]` parses as `[{environment:"*", resource:"*", verb:"read"}]` — same authorization result.
- **Invariant 4 — Audit-log pruner contract unchanged under Candidate 4 with dedupe.** Pruner takes a retention window (`startAuditLogPruner` + `AuditRepository.DeleteOlderThan`); per-day dedupe keeps volume bounded so retention math doesn't change.
- **Invariant 5 — Embed widget direct-credential mode unaffected.** Verified by tracing: all candidates touch `auth/apikey.go`, the apikey middleware, or the promotion service. None are on the embed widget's outbound path.
- **Invariant 6 — JWT path untouched.** Candidates name `apikey.go`, `apikey_service.go`, `middleware.go` (API-key half), `promotion_service.go`. The JWT issuance/verification path (`auth/auth.go:25-50`) is not edited.

Security Reviewer veto applies. Suggested checks: (a) Candidate 2 parser denies unparseable scopes rather than silently coercing; (b) Candidate 1 constraint name is stable across PG/SQLite (we support both per `docs/SECRET_SOURCES.md`); (c) Candidate 4 dedupe key is `(api_key_id, date_trunc('day', now()))` not `(user_id, ...)` — otherwise a compromised key shared across two agents under one user hides.

## 13. References

All URLs retrieved 2026-05-15.

- NIST SP 800-63B: `https://pages.nist.gov/800-63-3/sp800-63b.html` (via search summary)
- NIST SP 800-204C: `https://csrc.nist.gov/pubs/sp/800/204/c/final`
- OWASP ASVS v4.0.3: `https://owasp.org/www-project-application-security-verification-standard/`
- RFC 6749: `https://www.rfc-editor.org/rfc/rfc6749`
- CVE-2022-23529: `https://nvd.nist.gov/vuln/detail/CVE-2022-23529`
- CVE-2024-4985: `https://nvd.nist.gov/vuln/detail/CVE-2024-4985`
- CVE-2024-8770 / 8810: `https://nvd.nist.gov/vuln/detail/CVE-2024-8770`
- GitHub Engineering blog — PAT GA: `https://github.blog/2022-10-18-introducing-fine-grained-personal-access-tokens-for-github/` (direct fetch 403)
- GitHub docs — PATs and environments (unreachable via tooling): `docs.github.com/en/authentication/...managing-your-personal-access-tokens`, `docs.github.com/en/actions/...managing-environments-for-deployment`
- CircleCI 2023-01 incident: `https://circleci.com/blog/jan-4-2023-incident-report/`
- KeepSave: `docs/THREAT_MODEL.md` §2 line 88, §3 line 100, Findings §1 lines 38-41; `docs/FOLLOWUPS.md` 0d lines 48-52, lines 161-162; `docs/adr/0003-promotion-engine.md` lines 74, 90; `docs/ROADMAP_NOT.md` §4
