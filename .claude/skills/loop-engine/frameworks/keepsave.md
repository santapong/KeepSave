---
name: keepsave
summary: KeepSave's AI-Assisted Development Life Cycle (docs/ADLC.md) — the 10-stage gated pipeline with decision-class classification, threat-model delta, audit-row assertion, never-surface-plaintext, and Security-Engineer veto baked in.
when-to-use: Default for any non-trivial KeepSave change, and MANDATORY for anything touching internal/crypto, internal/auth, or the promotion engine. Prefer over AIDLC in this repo — AIDLC has none of KeepSave's security gates.
---

# keepsave — KeepSave AIDLC

Canonical reference: `docs/ADLC.md` (10 stages) and `docs/HARNESS_ENGINEERING.md` (harness
patterns). This framework is the runnable summary; those docs are the source of truth. Start
Build→Verify Workflow scripts from `../templates/keepsave-task.workflow.js` — its gate phases
(Understand → Classify → Build → Verify → Review) are the point; keep them intact.

Right-size before authoring a workflow — pick the smallest harness that fits
(`docs/HARNESS_ENGINEERING.md` §1):

| Use a workflow | Do it inline (no workflow) |
|-----------------|----------------------------|
| Touches `internal/crypto` / `internal/auth` / promotion (needs adversarial verify) | A typo, a doc edit, a one-file refactor |
| Multi-file or multi-package change (migrate N sites) | A change you can hold in one context |
| Subsystem audit / pre-pentest sweep | A single lookup ("where is X defined?") — one Explore agent |
| Unknown-size discovery ("find every place a secret could be logged") | A change with no security-relevant claim to refute |

## Phase: Intake & Classify

- **Purpose**: Restate the task; locate it on `docs/ARCHITECTURE.md` (subsystem, trust boundary); prefer tracked work in `docs/FOLLOWUPS.md`; assign **Type-1 / Type-2 / Type-3** (`docs/ROLES.md` §3.1). When in doubt, treat as the next class up — **misclassification is itself a bug**. (ADLC stages 0–1.)
- **Entry criteria**: A task statement from the user.
- **Agent activities**: Driver inline — too cheap to spawn agents for.
- **Orchestration hint**: None/inline.
- **Exit gate**: (human — Tech Lead; + Security on sensitive surfaces) Class assigned; subsystem and trust boundary named; escalation trigger named. **Type-1** (crypto scheme, key hierarchy, breaking schema, audit-field change, PROD key rotation) ⇒ the workflow may draft the ADR and a plan, but **must NOT land implementation** — the human owns the decision.

## Phase: Threat-Model Delta

- **Purpose**: STRIDE rows with file:line refs for any widened trust boundary. (ADLC stage 2.)
- **Entry criteria**: Classified task.
- **Agent activities**: One Explore agent maps the affected boundary and drafts the delta.
- **Orchestration hint**: Single agent — no fan-out needed.
- **Exit gate**: (human — **Security Engineer veto**) `docs/THREAT_MODEL.md` updated, or an explicit "no boundary change".

## Phase: Design (ADR/RFC)

- **Purpose**: ADR for Type-1, RFC for non-trivial Type-2; ≥2 options + rollback. (ADLC stage 3.)
- **Entry criteria**: Threat delta accepted.
- **Agent activities**: One Plan agent drafts the ADR/RFC.
- **Orchestration hint**: Single agent.
- **Exit gate**: (human — Tech Lead + **Security veto** on crypto/auth/promotion) **Type-1 ADR is merged as Accepted BEFORE any implementation.**

## Phase: Harness Plan

- **Purpose**: Choose solo edit vs. subagent vs. workflow; define a verification step per security-relevant claim; model/effort/budget per `docs/HARNESS_ENGINEERING.md` §4. (ADLC stage 4.)
- **Entry criteria**: Approved design (or a Type-2/3 task that needs none).
- **Agent activities**: Driver writes the phase list.
- **Orchestration hint**: Inline. If the answer is "solo edit," stop and do it inline — don't author a workflow for a one-liner.
- **Exit gate**: Phases + per-claim verification listed. No plan → no build.

## Phase: Build

- **Purpose**: Implement to CLAUDE.md conventions on a feature branch. (ADLC stage 5.)
- **Entry criteria**: Harness plan; Type-1 only after its Accepted ADR.
- **Agent activities**: One implementer per unit of work, each carrying the guardrail block below verbatim in its prompt.
- **Orchestration hint**: `pipeline(units, ...)` if multi-file; `isolation: 'worktree'` only when parallel units mutate overlapping files.
- **Exit gate**: Lint clean; **every state-mutating handler emits an audit event** from `docs/AUDIT_LOG_COVERAGE.md`; **no plaintext** logged/returned/persisted; client errors via `httperror`, never `err.Error()`.

## Phase: Verify

- **Purpose**: Tests at all layers + adversarial refutation of every security claim. (ADLC stage 6.)
- **Entry criteria**: Build complete.
- **Agent activities**: ≥3 independent, lens-diverse verifiers per claim (plaintext-leak, cross-tenant, downgraded-token, audit-row-present), each prompted to REFUTE; **default-deny on uncertainty** (`docs/HARNESS_ENGINEERING.md` §5).
- **Orchestration hint**: `parallel()` adversarial fan-out with a barrier — the verdict genuinely needs all lenses at once.
- **Exit gate**: (**CLAUDE.md hard gate**) Every mutating handler's test **asserts the audit row was written**; a negative-auth cell exists for each new surface (`tests/NEGATIVE_AUTH_PLAN.md`); no claim survives with a majority refuting it.

## Phase: Review

- **Purpose**: `/code-review` always; `/security-review` on sensitive diffs. (ADLC stage 7.)
- **Entry criteria**: Verify passed.
- **Agent activities**: Review skills as phases; triage findings back into Build if confirmed.
- **Orchestration hint**: Sequential — review needs the whole diff.
- **Exit gate**: (human — **Security veto**) Findings triaged; **Security-Engineer sign-off on crypto/auth/promotion**.

## Phase: Land

- **Purpose**: Draft PR on the feature branch; drive CI green; squash-merge. (ADLC stage 8.)
- **Entry criteria**: Review passed.
- **Agent activities**: Driver + GitHub MCP tools.
- **Orchestration hint**: Inline.
- **Exit gate**: (human — Tech Lead merge) All CI + required reviewers approve.

## Phase: Post-Merge

- **Purpose**: Docs match reality. (ADLC stage 9.)
- **Entry criteria**: Merged.
- **Agent activities**: Driver updates docs inline.
- **Orchestration hint**: Inline.
- **Exit gate**: `docs/THREAT_MODEL.md` / `docs/ARCHITECTURE.md` updated if a boundary moved; `docs/FOLLOWUPS.md` ticked.

## Hard gates (block the run — never route around)

- **Never surface plaintext** — not in logs, error bodies, browser storage, or fixtures; use `httperror`.
- **Audit-or-it-didn't-happen** — every state-mutating handler emits a canonical audit event AND its test asserts the row.
- **Type-1 blocks Build** — no crypto/schema/key-hierarchy code before its ADR is Accepted (the Threat-Delta and Design phases gate Build).
- **Security-Engineer veto** on `internal/crypto`, `internal/auth`, and the promotion engine.
- **requester ≠ approver** on PROD promotion stays enforced (app layer + DB CHECK).
- Feature branch only; never edit `main`. If a gate cannot be satisfied, stop and report — do not route around it.
- Model policy: judgment / verification / governance agents never run on Haiku.

## Guardrails compiled into every agent prompt

Inject verbatim into every build/verify agent prompt (`../templates/keepsave-task.workflow.js`
already embeds this as its `GUARDRAILS` constant):

> Never log, return in an error body, persist to browser storage, or commit to a fixture any plaintext secret value. Use the `httperror` package — never `err.Error()` — for client responses. Every state-mutating handler must emit an audit event from `docs/AUDIT_LOG_COVERAGE.md`, and its test must assert the row was written. Work on the feature branch only; never edit `main`. If a gate cannot be satisfied, stop and report it — do not route around it.

## Tracker sync (optional)

Off by default; `docs/FOLLOWUPS.md` is the source of truth. To opt in: create the task at
**Intake** and advance its status at **Build / Review / Land** (ClickUp `clickup_update_task`, or a
GitHub issue linked to the PR).
