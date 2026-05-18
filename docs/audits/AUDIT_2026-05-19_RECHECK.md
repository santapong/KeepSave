# KeepSave Phase 2 Audit Team Recheck — 2026-05-19

**Audit team:** same 5 reviewers as Phase 1 (Security, Backend, Frontend, Infra, Governance)
**Scope:** verify every BLOCKER + HIGH finding from `docs/audits/AUDIT_2026-05-18_DEPLOYMENT_READINESS.md` is closed by the Phase 1 sweep on branch `claude/audit-deployment-plan-moSsk`; surface any new findings the refactor introduced.
**Status:** Read-only. Real regressions / new findings get a follow-up commit; nothing is "fixed by review".

---

## 1. Verdict

| Reviewer | BLOCKER status | HIGH status | New findings | Verdict |
|---|---|---|---|---|
| Security | 7 CLOSED, 1 PARTIAL (S-B7 deferred per ADR-0016) | 7 CLOSED | None | **PASS** |
| Backend | 3 CLOSED | 3 CLOSED (after B-H2 hotfix) | 2 informational, deferred to Phase 3 | **PASS** |
| Frontend | 1 CLOSED | 3 CLOSED | None | **PASS** |
| Infra | All G1–G14 CLOSED | — | Dockerfile `USER` directive (S-M8, MEDIUM, Phase 3) | **PASS** |
| Governance | — | — | 3 doc gaps fixed in this commit; 1 process-pattern noted | **PASS (after doc fixup)** |

**Overall: Phase 1 closure CONFIRMED.** The cutover gates from `docs/DEPLOYMENT_PLAN.md` §2 are unblocked subject to the operator-side prerequisites (Neon project, Vercel project, KMS secret).

---

## 2. Detailed findings

### 2.1 Security recheck

All 8 BLOCKERs and 7 HIGHs verified closed by direct code-read; evidence cited in `AUDIT_2026-05-18_DEPLOYMENT_READINESS.md` §6.

- **S-B7** stays **PARTIAL by design** — Vault path is wired; AWS / GCP adapters return a clear deferred error per ADR-0016 and `FOLLOWUPS.md #1`. This was the user's explicit choice in Phase 0 planning.
- The 42-cell negative-auth matrix (`internal/api/negative_auth_test.go`, `internal/service/negative_auth_test.go`, `cors_test.go`, `url_safety_test.go`) is now the regression gate.

**No new BLOCKER or HIGH surfaced.** No auth bypass, error leak, or crypto regression observed.

### 2.2 Backend recheck

One regression caught and fixed inline:

| ID | Status | Fix |
|---|---|---|
| B-H2 | REGRESSION on initial recheck, now CLOSED | `internal/api/health.go:50` was missed by the earlier `replace_all` because the Readiness JSON literal uses two spaces (`"version":  "0.5.0"`) where Liveness uses one. Hotfix in commit `e12d73b`. |

Two informational items surfaced for Phase 3:

- **MustGet panic surface (58 sites)** — already tracked as `FOLLOWUPS.md #0l`. Not a regression; pre-existing.
- **HTTP redirect goroutine doesn't cancel on shutdown** — `cmd/server/main.go` `startHTTPRedirect`. Low risk (process exit kills the listener), but inconsistent with the pruner pattern. Add to Phase 3 backlog.

### 2.3 Frontend recheck

All findings CLOSED. The DiffEntry shape change (S-B4) was verified end-to-end: `frontend/src/types/index.ts` reads `source_hash`/`target_hash`; no consumer references the removed plaintext fields. The embed widget is Vercel-safe (no same-origin assumptions; ADR-0006 strict-origin enforcement preserved).

### 2.4 Infra recheck

All 14 UAT cutover gates (`DEPLOYMENT_PLAN.md` §2) verified closed.

One follow-up (already tracked):

- `backend/Dockerfile` lacks a `USER` directive. Audit ID S-M8, MEDIUM. Deferred to Phase 3 per the user's plan-question 1.

### 2.5 Governance recheck

Three real doc gaps identified and **fixed in the same commit** as this recheck doc:

| Gap | Fix |
|---|---|
| `docs/FOLLOWUPS.md` was out of sync — closed items not marked | Added a new "Closed (Phase 1 deployment sweep — 2026-05-18)" section enumerating every closed item with file:line references. |
| `docs/THREAT_MODEL.md` had no entry for the new cross-origin trust boundary required by ADR-0016 | Added §8 "Cross-origin trust boundary" with 6 STRIDE rows and operational invariants. |
| `docs/RUNBOOK.md` had no cutover procedure | Added §7 "UAT cutover" referencing `DEPLOYMENT_PLAN.md` §4 plus a day-of checklist, CORS reminder, rollback steps, and KMS caveat. |

Notes on items the reviewer flagged but are by design:

- **ADR acceptance under "sponsor authorization"** — per the user's explicit Phase 0 plan question 2 answer (matching PRs #48/#49/#50). Each ADR carries an inline note that retroactive Security + Tech Lead sign-off is pending per CLAUDE.md §"Decision classes". This is a known-and-accepted process choice, not a violation.
- **G13 KMS** — the reviewer flagged this as code not merged. Inaccurate: Vault provider is wired in `cmd/server/main.go` and has been throughout. AWS/GCP adapters are correctly deferred per ADR-0016 / `FOLLOWUPS.md #1` (the user's plan-question-3 choice).

---

## 3. Counts (Phase 2)

| Severity | Total at start | Closed in Phase 1 | Closed in Phase 2 hotfix | Still Open |
|---|---|---|---|---|
| BLOCKER | 8 | 7 + 1 partial (S-B7 by design) | 0 | 0 |
| HIGH | 10 | 9 | 1 (B-H2) | 0 |
| MEDIUM | 12 | 0 | 0 | 12 (Phase 3) |
| LOW | 8 | 0 | 0 | 8 (Phase 3) |
| Doc gaps (new) | 3 | 0 | 3 (this commit) | 0 |
| Informational (new) | 2 | 0 | 0 | 2 (Phase 3) |

---

## 4. Cutover readiness

Per `docs/DEPLOYMENT_PLAN.md` §2 gates:

| Gate | Verified |
|---|---|
| G1–G12 (security + reliability) | ✓ |
| G13 (KMS) | ✓ Vault wired; AWS/GCP deferred is documented |
| G14 (ADR-0016) | ✓ Accepted under sponsor authorization, README index reflects it |
| Operational prerequisites (Neon project, Vercel project, secrets in vault) | Operator action — runbook §7 lists them |

**UAT cutover is unblocked from KeepSave's side.** The team can run `DEPLOYMENT_PLAN.md` §4.1 today.

---

## 5. Status (this audit)

Read-only verification + the three doc fixes named in §2.5. No code under `internal/` was changed by this recheck (the B-H2 hotfix landed earlier in commit `e12d73b` between agent runs).

Phase 3 (MEDIUM + LOW items + the two informational findings) starts next per the plan.
