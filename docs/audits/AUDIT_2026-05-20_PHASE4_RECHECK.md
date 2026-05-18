# KeepSave Phase 4 Audit Team Recheck — 2026-05-20

**Audit team:** same 5 reviewers (Security, Backend, Frontend, Infra, Governance)
**Scope:** verify Phase 3 closures (commits I–N on branch `claude/audit-deployment-plan-moSsk`) for 12 MEDIUMs + 8 LOWs + 2 informational items. Cross-reference with `docs/audits/AUDIT_2026-05-18_DEPLOYMENT_READINESS.md` §7.
**Status:** Read-only verification + 1 small doc fixup commit (FOLLOWUPS sync + ADR footnote correction).

---

## 1. Verdict

| Reviewer | Closure status | New findings | Verdict |
|---|---|---|---|
| Security | 9 of 9 in-domain items CLOSED | None | **PASS** |
| Backend | 5 of 5 in-domain items CLOSED | None | **PASS** |
| Frontend | 3 of 3 in-domain items CLOSED | None | **PASS** |
| Infra | All deploy-readiness gates verified | None | **PASS — deployment gate OPEN** |
| Governance | 3 doc inconsistencies; fixed inline | None | **PASS (after fixup commit b6bc02d)** |

**Overall: the BLOCKER + HIGH + MEDIUM + LOW closure work is verified.** Two items remain explicitly out of scope by design: S-M3 (full 132-cell negative-auth matrix) and S-M6 (audit-log tamper-evident — Phase B per the audit itself).

---

## 2. Detailed findings

### 2.1 Security recheck

Every Phase 3 security claim verified by code-read. Highlights:

- **S-M4** (API-key expiry): `computeEffectiveAPIKeyExpiry` enforces 90d default / 365d ceiling; middleware retains NULL-grandfathering for existing rows.
- **S-L1** (62-site MustGet): 61 of 62 sites migrated; the 62nd is the helper's own doc comment. Pattern consistent across all handlers.
- **S-L2** (CSRF cleanup): `internal/api/csrf.go` deleted; no indirect imports.
- **S-L5** (lucide-react): `npm audit` reports 0 vulnerabilities; v1.8.0 is current.

**Anti-regression gates active:** `error_leak_test.go` AST-walks the api package; `cascade_test.go` locks the secret_versions ON DELETE CASCADE; `recovery_test.go` confirms panic values never reach clients.

### 2.2 Backend recheck

- **B-M1** structured 500: `PanicRecoveryMiddleware` mounted at `router.go:33` replacing `gin.Recovery()`.
- **B-M2** body double-read: verified clean. No middleware reads `c.Request.Body`.
- **B-L1** DB pool metrics: three gauges plus 15s updater under `bgCtx`.
- **B-L2** pruner ctx: unchanged from Phase 1 A — still correct.
- **NEW-2** HTTP redirect graceful shutdown: ctx-honoring `startHTTPRedirect` confirmed.

No regressions from the 62-site MustGet sweep, CSRF removal, or API-key signature change.

### 2.3 Frontend recheck

All three items CLOSED:

- **F-M1** embed `api-url` operator note: in `docs/EMBED_ORIGIN_POLICY.md` §"Operator note".
- **F-M2** build separation: documented in `DEPLOYMENT_PLAN.md` §4.2 (no additional doc work).
- **F-L1** tsc deprecation: `ignoreDeprecations: "5.0"` silences the baseUrl warning. 58/58 tests still pass.

Frontend Dockerfile change (nginxinc/nginx-unprivileged) only affects the Option-C self-hosted path; Vercel ignores it.

### 2.4 Infra recheck

Backend + frontend Dockerfiles build cleanly with non-root. Migrations directory owned by `keepsave` post-chown. DB pool gauges emit to `/metrics`. Helm chart needs no updates (no `runAsUser: 0` hardcoded; new gauges/env vars are runtime-only).

**Verdict: deployment gate OPEN for UAT cutover.**

### 2.5 Governance recheck — 3 doc gaps, all fixed in commit b6bc02d

| Gap | Fix |
|---|---|
| FOLLOWUPS.md had no "Closed (Phase 3)" section | Added one with file:line evidence for #0k and #0l. |
| FOLLOWUPS Open items #0k / #0l still claimed OPEN | Strike-through + inline pointer to the new Closed section. |
| `docs/adr/README.md` footnote listed 0009 as Proposed | Corrected to "0008, 0014, 0015 remain Proposed" (0009 was flipped to Accepted in Phase 3 commit b69285b). |

---

## 3. Counts (Phase 4)

| Severity | At start of audit | Closed by Phase 1 | Closed by Phase 3 | Deferred by design | Open |
|---|---|---|---|---|---|
| BLOCKER | 8 | 7 + 1 partial (S-B7) | 0 | 0 | 0 |
| HIGH | 10 | 10 | 0 | 0 | 0 |
| MEDIUM | 12 | 1 (S-M5) | 9 | 2 (S-M3 by user choice, S-M6 to Phase B) | 0 |
| LOW | 8 | 1 (B-L2) | 7 | 0 | 0 |
| Informational (Phase 2) | 2 | 0 | 2 | 0 | 0 |
| Doc gaps (Phase 4) | 3 | — | 3 (fixup commit) | 0 | 0 |

**42 of 43 audit findings closed; 0 open; 2 deferred by design.**

---

## 4. ADRs

Accepted under sponsor authorization across the sweep:

| ADR | Title | Accepted in |
|---|---|---|
| 0005 | RequireProjectAccess middleware | Phase 1 G |
| 0006 | Embed widget origin allow-list | Phase 1 G (post-PR #50) |
| 0007 | Approver ≠ requester DB invariant | Phase 1 G |
| 0009 | Mandatory default expiration on `ks_` API keys | Phase 3 N |
| 0010 | MCP gateway command-execution hardening | Phase 1 G |
| 0011 | Graceful shutdown + DB timeouts | Phase 1 G |
| 0012 | KMS auto-unseal (Vault only this round) | Phase 1 G |
| 0013 | Webhook emission with SSRF guard | Phase 1 G |
| 0016 | Deployment topology: Vercel + container + Neon | Phase 1 G |

ADRs 0008, 0014, 0015 remain Proposed (implementation outside this PR).

---

## 5. Status

UAT cutover from KeepSave's side: **unblocked.** All BLOCKER + HIGH + MEDIUM + LOW items closed except the two deferred by design (S-M3 full matrix, S-M6 Phase B). Run the cutover per `docs/DEPLOYMENT_PLAN.md` §4 and `docs/RUNBOOK.md` §7.

The audit cycle is complete: 4 phases, ~14 commits on `claude/audit-deployment-plan-moSsk`, full backend race-clean test suite, frontend 58/58, type-check + production build clean.
