---
name: aidlc
description: KeepSave's default AI-Assisted Development Life Cycle — the 10-stage gated pipeline from docs/ADLC.md, with decision-class, threat-delta, audit-row assertion, never-surface-plaintext, and Security-Engineer veto baked in.
default: true
---

# AIDLC (KeepSave)

## When to use
The default for any non-trivial KeepSave change, and **mandatory** for anything touching
`internal/crypto`, `internal/auth`, or the promotion engine. Trivial reversible edits can use
`lightweight`; a plain phase-gated pass without KeepSave's security governance can use `sdlc` (but
NOT for crypto/auth/promotion — those require this framework's gates).

Canonical reference: `docs/ADLC.md` (10 stages) and `docs/HARNESS_ENGINEERING.md` (harness
patterns). This framework is the runnable summary; those docs are the source of truth. The
existing `templates/keepsave-task.workflow.js` is the starter Workflow script for the Build→Verify
stages.

## Stages

**0. Intake & triage** — restate the task; locate it on `docs/ARCHITECTURE.md`; prefer tracked work in `docs/FOLLOWUPS.md`.
- Exit gate: subsystem and trust boundary named.
- Human-gated: no
- Harness: inline.

**1. Classify the decision** — assign Type-1 / Type-2 / Type-3 (`docs/ROLES.md` §3.1).
- Exit gate: class assigned; escalation trigger named. **Misclassification is itself a bug.**
- Human-gated: yes (Tech Lead; + Security on sensitive surfaces).
- Harness: inline.

**2. Threat-model delta** — STRIDE rows with file:line refs for any widened trust boundary.
- Exit gate: `docs/THREAT_MODEL.md` updated, or an explicit "no boundary change".
- Human-gated: yes (**Security Engineer veto**).
- Harness: one Explore agent.

**3. Design (ADR/RFC)** — ADR for Type-1, RFC for non-trivial Type-2; ≥2 options + rollback.
- Exit gate: **Type-1 ADR is merged as Accepted BEFORE any implementation.**
- Human-gated: yes (Tech Lead + **Security veto** on crypto/auth/promotion).
- Harness: one Plan agent.

**4. Harness plan** — choose solo vs subagent vs `/workflow`; a verification step per security claim.
- Exit gate: phases + per-claim verification listed.
- Human-gated: no
- Harness: driver writes the phase list.

**5. Build** — implement to CLAUDE.md conventions on a feature branch.
- Exit gate: lint clean; **every state-mutating handler emits an audit event** from `docs/AUDIT_LOG_COVERAGE.md`; **no plaintext** logged/returned/persisted; client errors via `httperror`, never `err.Error()`.
- Human-gated: no
- Harness: one implementer per unit (`pipeline` if multi-file; worktree isolation on overlapping files).

**6. Verify** — tests at all layers + adversarial refutation.
- Exit gate (**CLAUDE.md hard gate**): every mutating handler's test **asserts the audit row was written**; a negative-auth cell exists for each new surface; ≥3 independent, lens-diverse verifiers default-deny on uncertainty.
- Human-gated: no
- Harness: adversarial parallel verify.

**7. Review** — `/code-review` + `/security-review` on sensitive diffs.
- Exit gate: findings triaged; **Security-Engineer sign-off on crypto/auth/promotion**.
- Human-gated: yes (**Security veto**).
- Harness: `/code-review` (+ `/security-review`) phase.

**8. Land** — draft PR on the feature branch; drive CI green; squash-merge.
- Exit gate: all CI + required reviewers approve.
- Human-gated: yes (Tech Lead merge).
- Harness: driver + GitHub MCP.

**9. Post-merge** — docs match reality; tick `docs/FOLLOWUPS.md`.
- Exit gate: `THREAT_MODEL.md`/`ARCHITECTURE.md` updated; follow-ups recorded.
- Human-gated: no
- Harness: inline.

## Hard gates
- **Never surface plaintext** — not in logs, error bodies, browser storage, or fixtures; use `httperror`.
- **Audit-or-it-didn't-happen** — every state-mutating handler emits a canonical audit event AND its test asserts the row.
- **Type-1 blocks Build** — no crypto/schema/key-hierarchy code before its ADR is Accepted (stages 2–3 gate stage 5).
- **Security-Engineer veto** on `internal/crypto`, `internal/auth`, and the promotion engine.
- **requester ≠ approver** on PROD promotion stays enforced (app layer + DB CHECK).
- Feature branch only; never edit `main`. If a gate cannot be satisfied, stop and report — do not route around it.

## Tracker sync (optional)
Off by default; `docs/FOLLOWUPS.md` is the source of truth. To opt in: create the task at
**Intake** and advance its status at **Build / Review / Land** (ClickUp `clickup_update_task`, or a
GitHub issue linked to the PR).
