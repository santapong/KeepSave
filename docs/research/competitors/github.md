# Competitor Dossier — GitHub (Fine-Grained PATs + Environments)

## 1. Header

- **Vendor:** GitHub (fine-grained Personal Access Tokens + Deployment Environments)
- **Category:** machine identity (PATs) + approval workflow (Environments)
- **License:** proprietary / N/A — patterns only, not deployed product
- **Last updated:** 2026-05-15
- **Analyst:** Machine Identity Analyst + Approval Workflow Analyst
- **Reviewer:** Security Reviewer (accepted-with-changes 2026-05-15)
- **Status:** accepted
- **Priority:** P0 (tied to FU 0d approver≠requester + Phase-B fine-grained API key scopes)

## 2. One-paragraph overview

GitHub ships two platform pieces mapping onto two open KeepSave gaps. Fine-grained PATs (GA October 2022) replaced OAuth scopes with a `repository × permission × expiration` tuple — the shape `ks_` keys need for least-privilege. Deployment Environments add protection rules (required reviewers, wait timer, branch allow-list); the required-reviewers rule enforces `approver ∉ requester` at the platform layer — the invariant FU 0d says KeepSave doesn't. We adopt **patterns**, not GitHub.

## 3. Architecture summary

**Fine-grained PATs:** mint with (a) resource owner, (b) repo selector, (c) per-permission level (`read`/`write`/`admin`) across ~50 named permissions, (d) **mandatory** expiration (≤366d; default 30; no "never"). `github_pat_` prefix enables secret-scanning. Every API call logs the token ID.

**Environments:** named Environments carry protection rules evaluated before deployment: required reviewers (1–6 users/teams, requester excluded), wait timer, deployment-branch allow-list, environment-scoped secrets.

KeepSave borrows the grammar (PATs → `ks_`) and protection-rule shape (Environments → promotion engine), not the products.

## 4. Security model

PAT primitives (vendor docs retrieved 2026-05-15 via search summary; direct `docs.github.com` returned 403):

- **Token shape:** opaque random + `github_pat_` prefix; SHA hash at rest; raw shown once. Same posture as KeepSave (`auth/apikey.go:21-23`).
- **Scope enforcement:** gateway evaluates `(token, target_repo, required_permission)` per request → 403 on miss. Denylist-by-default.
- **Expiration:** mandatory; default 30d; ceiling 366d.
- **Audit:** every call writes `{actor, token_id, action, repo, timestamp}` to org audit log.

Environments: approver pool (membership resolved at decision time); approver ≠ requester (API rejects self-approval `422 cannot approve own deployment`); every approval/rejection audited.

CVE history (24 months): **CVE-2024-4985** (GHES SAML signature bypass) and **CVE-2024-8770/8810** (GHES sensitive-data exposure) evidence a responsive program. No CVE against fine-grained-PAT permission logic in 24 months — suggestive, not load-bearing; §10 security claim rests on NIST SP 800-63B and OWASP ASVS V8/V14.

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

- **Pros:** Closes FU 0d. Single migration + one service guard. DB constraint = defence in depth. Closes gap at `docs/THREAT_MODEL.md:100`.
- **Cons (operational):** Existing `requested_by == approved_by` rows (if any) migrate to `rejected` with `migration_reason`. Self-request + self-approve dev habits break — correct behaviour.
- **Cons (security):** `none material — invariant narrows boundary; sock-puppet is out of scope per docs/THREAT_MODEL.md:134.`

### Candidate 2: Fine-grained scope grammar

- **Pros:** Addresses Phase-B deferred item (`docs/FOLLOWUPS.md:168`). Backward-compatible: bare `"read"`/`"write"` parses as `*:*:read`/`*:*:write`. No schema change; scope decisions become structured/auditable.
- **Cons (operational):** Doubles authorization test surface (FU 0c matrix incomplete). No GUI policy editor per `ROADMAP_NOT.md` §4 — tuples managed as code/CLI strings.
- **Cons (security):** Parser bug silently coercing `prod:secrets:read` → `*:*:*` is a privilege-escalation primitive (CVE-2022-23529 class). Mitigation: deny-on-unparseable, fuzz, lint grammar closed. CVE-2024-8770/8810 confirms permission-eval code is a CVE class.

### Candidate 3: Default expiration on `ks_` keys

- **Pros:** Closes a silent gap: `Create` at `apikey_service.go:33-60` never sets `ExpiresAt` — every `ks_` key is **immortal** until manual rotation. Materially worse than GitHub's mandatory ≤366d.
- **Cons (operational):** Hard-coded customer integrations fail at 90 days. Mitigation: monitor-then-enforce (warn at 60d, enforce at 90d, runbook + release note).
- **Cons (security):** `none material — bounded < indefinite. Indefinite-required edge case solved by audited sentinel ExpiresAt='9999-12-31' with indefinite_key_granted event.`

### Candidate 4: Per-use API-key audit emission

- **Pros:** Closes §5 row 4 — today zero middleware-layer trail of API-key actions. Sibling to FU 0 (service-layer audit gaps).
- **Cons (operational):** Audit-log volume. Agent polling 1/s → 86,400 rows/day per key. Mitigation: per-key/per-day dedupe. Without dedupe, pruner becomes load-bearing and any tuning bug is a DB-fill incident.
- **Cons (security):** Under dedupe, none material. Without dedupe, audit volume is a minor covert channel for caller load profile — only relevant if audit log is exposed to lower-trust principals.

### Candidate 5: Declarative reviewer pool with `min_count`

- **Pros:** Replaces implicit-single-approver with a declarative table the Security Engineer can review. `min_count=2` is the per-project knob ADR-0003's "two-of-N if regulatory pressure" defers.
- **Cons (operational):** New table + admin UI (forbidden in Phase A per `ROADMAP_NOT.md` §4) or CLI surface. Reviewer-pool changes themselves require audit + approval (recursive). Defer to Phase B.
- **Cons (security):** A pool including the requester's role-peers (e.g. all `promoter` members of a 2-person org) collapses to single-approver. Mitigation is process; needs org-size guidance.

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

**Candidate 1:** `DROP CONSTRAINT promotion_requests_no_self_approve` + revert service guard. Runbook: feature-flag `PROMOTION_REQUIRE_DISTINCT_APPROVER` defaults true; set false in incident.

**Candidate 3:** Forward-fix only. Once keys are minted with `ExpiresAt = NOW() + 90d` they hit expiry regardless of rollback. Rollback changes default to `NULL` for new keys; expired keys cannot be un-expired (raw value not recoverable per `auth/apikey.go:21-23`). Customer mints new key.

**Candidate 4:** Trivial — remove `auditRepo.Create` from `APIKeyAuthMiddleware`. Existing rows stay; pruner clears on schedule.

**Candidates 2 and 5:** Deferred — rollback drafted at ADR time when triggers fire.

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
- KeepSave: `docs/THREAT_MODEL.md` §2 line 88, §3 line 100, Findings §1 lines 38-41; `docs/FOLLOWUPS.md` 0d lines 48-52, 0k lines 83-88, Phase-B lines 161-162; `docs/adr/0003-promotion-engine.md` lines 74, 90; `docs/ROADMAP_NOT.md` §4; `docs/audits/SECURITY_AUDIT_2026-05-15.md` A04-F1, A07-F1, §6 row 6; `docs/audits/BACKEND_CRASH_RISKS.md` §3.4 (#10-#14 MustGet sites)

## Security Reviewer notes

**Verdict:** accepted-with-changes. Veto **not** exercised on §9/§10/§12.

**Refs spot-checked (Read, 2026-05-15) — all resolve:**

- `apikey_service.go:33-60` — `Create` has no `expiresAt` param; line 51 omits expiry → row stored `NULL`. §5 row 3 / Candidate 3 correct.
- `promotion_service.go:215-240` — `ApprovePromotion` never reads `RequestedBy`; line 225 `UpdateStatus(approverID)` direct. Self-approval silent. §5 row 6 / Candidate 1 exact.
- `models.go:51-61` — `APIKey.ExpiresAt *time.Time` nullable; `Scopes StringList` flat. Candidate 2 schema-free interpretation feasible.

**§9 STRIDE rows quoted from `docs/THREAT_MODEL.md`:**

- §3 row E (line 100): *"Requester self-approves … Invariant not currently enforced at DB layer (gap) … Residual: Medium"* — matches; Candidate 1 narrows.
- §2 row E (line 88): *"API key scope escalation … Residual: Low"* — matches as stated. Audit A01-F2 shows actual residual is **High** because scope context is set but never consumed by handlers. Candidate 2 still tightens grammar; consumer-side enforcement is a prerequisite — note for Type-1 ADR.
- §2 row T (line 84): *"… no denylist … Residual: Medium"* — Candidate 3 closes the API-key half.
- v1.2.0 Findings §1 (lines 38-41): *"Secret/Project/APIKey mutations are not audited … Repudiation is currently not mitigated"* — Candidate 4 narrows.

**§10 trigger verification:** FU 0d quote at `docs/FOLLOWUPS.md:48-52` matches verbatim. Phase-B fine-grained-scope bullet at line 168 (dossier cites 161, section header — minor drift, accepted). FU 0k added by matrix synthesis at lines 83-88 citing this dossier — loop closed.

**Audit triangulation (load-bearing):** `docs/audits/SECURITY_AUDIT_2026-05-15.md` independently re-discovered all four adopt-now findings on the same day — rare predictive-value signal:

- A04-F1 (P2 High) confirms Candidate 1; audit §6 row 6 prescribes "service-layer check + DB CHECK constraint" — exactly §12's canonical-fix shape.
- A07-F1 (P3 standalone, **P2 chained with A01-F2**) confirms Candidate 3; cross-refs FU 0k.
- A09-F2 (P2) confirms Candidate 4 mutation scope; dossier additively extends to API-key reads.
- §5.2 new STRIDE row on JWT-scope absence — sibling of Candidate 2.
- `BACKEND_CRASH_RISKS.md` §3.4 — `c.MustGet("user_id").(uuid.UUID)` × 58 sites; the proposed `getUserID(c)` helper is a **prerequisite** for Candidate 4's per-call middleware.

**Inline changes:** §1 status `draft` → `accepted`; §13 cites both audit docs and FU 0k 83-88. §5-§12 unchanged.

**Non-blocking observations (for ADR Drafter, day 45):**

- Candidate 1: verify named CHECK constraint syntax across PG/SQLite migration framework.
- Candidate 3: dossier's `default_legacy` flag + rotation grace window is right; ADR defines window and runbook step.
- Candidate 4: dedupe key MUST be `(api_key_id, date_trunc('day', now()))`, not `(user_id, ...)` (§12 already prescribes); `BACKEND_CRASH_RISKS.md` MustGet refactor lands first.
- Candidate 2: deferred correctly; A01-F2 consumer-side fix is the prerequisite — grammar is moot until handlers read scope.

No reference failed. Veto remains available for Candidates 1/4 Type-1 ADRs.
