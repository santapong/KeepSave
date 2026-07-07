# KeepSave — AI-Assisted Development Life Cycle (ADLC)

This document defines the lifecycle KeepSave uses **when an AI agent participates in development** — Claude Code on the web, a local coding agent, or any LLM-driven contributor. It does not replace [`docs/ROLES.md`](ROLES.md), the [ADR process](adr/README.md), or the [threat model](THREAT_MODEL.md). It **sequences** them into a pipeline an agent can execute one stage at a time, with a human gate at every point where a wrong move would leak a secret, lose the audit trail, or take an irreversible step.

> KeepSave handles other people's secrets. An agent that can edit `internal/crypto`, `internal/auth`, or the promotion engine is a new trust grant — the same way a new hire is (`docs/ROLES.md` §4). The ADLC exists to **bound that grant**: every agent action is classified, gated, verified, and traceable, and **no agent lands a Type-1 change or a `main`/PROD-affecting change alone.**

Companion docs:
- [`docs/HARNESS_ENGINEERING.md`](HARNESS_ENGINEERING.md) — *how* the agent harness executes each stage (agents, skills, workflows, verification patterns).
- [`.claude/skills/loop-engine/SKILL.md`](../.claude/skills/loop-engine/SKILL.md) — the `/loop-engine` skill that turns a single task into a runnable, gated harness; run it with `--framework keepsave` ([`.claude/skills/loop-engine/frameworks/keepsave.md`](../.claude/skills/loop-engine/frameworks/keepsave.md)) so this lifecycle's gates govern the run.

---

## 1. Principles (inherited + AI-specific)

The ADLC inherits all six **Operating Principles** from `docs/ROLES.md` §1 — threat-model first, ADRs for irreversible decisions, blast radius before convenience, no silent fallbacks, audit-log first, reversibility budget. On top of those, agent work obeys five rules of its own:

1. **The human keeps the irreversible decisions.** An agent may *draft* an ADR, propose a key-hierarchy change, or write a migration. It may not *decide* a Type-1 (`docs/ROLES.md` §3.1). Type-1 acceptance is a human signature — Security Engineer + Tech Lead — every time.
2. **Every agent action is reviewable and reversible by default.** Work happens on a feature branch, never directly on `main`. Large or parallel edits run in an isolated git worktree (`docs/HARNESS_ENGINEERING.md` §6) so a bad run is `git worktree remove`, not a cleanup project.
3. **The harness is engineered, not improvised.** Before an agent writes code for a non-trivial task, it states *how* it will execute and verify the work (Stage 4). "I'll just start editing" is the agent equivalent of "let's just rotate the key in PROD quickly" (`docs/ROLES.md` §7) — banned.
4. **Verification is adversarial, not confirmatory.** A secrets product cannot accept "the test passes" as proof. Security-relevant claims (no plaintext leak, auth cannot be downgraded, audit row is written) are checked by an independent agent whose job is to *refute* them (`docs/HARNESS_ENGINEERING.md` §5).
5. **No action without a trace.** This is `docs/ROLES.md` §1.5 applied to the agent itself: the work leaves a branch, a PR, ADR/threat-model deltas, and — for any state-mutating handler — an audit-log assertion. If the trace isn't there, the stage isn't done.

**Fail-closed.** If any gate below cannot be satisfied — the threat delta can't be written, the audit assertion can't be added, the Security reviewer is unavailable for a crypto change — the agent **stops and surfaces the blocker**. It does not route around the gate, lower the decision class, or ship "and we'll fix it later." Security defaults are deny (`docs/ROLES.md` §1.4).

---

## 2. The lifecycle

```
 0 INTAKE ─▶ 1 CLASSIFY ─▶ 2 THREAT ─▶ 3 DESIGN ─▶ 4 HARNESS ─▶ 5 BUILD ─▶ 6 VERIFY ─▶ 7 REVIEW ─▶ 8 LAND ─▶ 9 POST-MERGE
    │            │ ▲          │ ▲          │ ▲          │            │           │ ▲         │ ▲        │          │
 FOLLOWUPS   Type-1/2/3   THREAT_MODEL  ADR / RFC   HARNESS_ENG   feature     tests +    human +    draft     docs +
  + scope     ── GATE ──   delta        ── GATE ──  plan          branch      audit      agent       PR        audit
              (next class  ── GATE ──   (Security                (conventions) ── GATE ── review                trail +
               up if       (if trust    sign-off                              (assert    ── GATE ──           FOLLOWUPS
               unsure)      boundary     on crypto/                            audit      (Security
                            moves)       auth/promo)                           row)       veto)
                                │            │                                              │
                                └── on Type-1, Stages 2-3 BLOCK Stage 5: no code before the ADR is Accepted ──┘
```

Each stage has an **entry condition**, an **exit/gate**, a **human owner** (role from `docs/ROLES.md`), and the **agent's job**. Stages are not skippable for non-trivial work; for a Type-3 doc edit, Stages 1-4 collapse to a single sentence ("Type-3, no trust boundary, no harness needed") and you go straight to Build → Review → Land.

### Stage 0 — Intake & triage
- **Entry:** a request exists (user ask, PR review comment, CI failure, incident).
- **Agent's job:** restate the task in one line; locate it on the dependency map (`docs/ARCHITECTURE.md`) — which subsystem (crypto / auth / promotion / api / repository / frontend / embed / infra / docs)? Check `docs/FOLLOWUPS.md` — **pick from tracked work before inventing a new task** (CLAUDE.md). Link the FOLLOWUPS item or note that this is net-new.
- **Gate / exit:** the subsystem and the affected trust boundary (1, 2, or 3 in `docs/ARCHITECTURE.md`) are named. A task that can't be located is under-specified — ask, don't guess.
- **Owner:** Product Manager (interim: Tech Lead).

### Stage 1 — Classify the decision
- **Agent's job:** assign **Type-1 / Type-2 / Type-3** per `docs/ROLES.md` §3.1 and the CLAUDE.md table. State the trigger that would bump it up a class.
- **Gate:** *misclassification is itself a bug* (CLAUDE.md). When in doubt, treat as the next class up. **Type-1 blocks Stage 5** — no implementation code may be written until the ADR is `Accepted` (Stage 3).
- **Owner:** Tech Lead; Security Engineer co-signs the class for anything touching `internal/crypto`, `internal/auth`, or promotion.

### Stage 2 — Threat-model delta
- **Entry:** the change touches a request path, a stored field, a key, an origin, or a token.
- **Agent's job:** write the delta to `docs/THREAT_MODEL.md` **first** (`docs/ROLES.md` §1.1) — new STRIDE entries, with `file:line` refs, for any widened attack surface. If the change widens a trust boundary, say so explicitly (CLAUDE.md).
- **Gate:** "If you can't describe the new attack surface, you're not ready to design" (`docs/ROLES.md` §1.1). Pure refactors and doc edits that move no boundary skip this stage with a one-line justification.
- **Owner:** Security Engineer (veto).

### Stage 3 — Design (ADR / RFC)
- **Agent's job:** for a **Type-1**, draft `docs/adr/NNNN-*.md` from [`docs/adr/0000-template.md`](adr/0000-template.md) — ≥2 options, explicit **rejection rationale**, **rollback plan**, open questions. For a non-trivial **Type-2**, write an RFC. The agent *authors*; it does not set status to `Accepted`.
- **Gate:** ADR merged as `Accepted` **before** any implementation (`docs/adr/README.md` §Lifecycle, step 2: "Open a PR with only the ADR — no implementation yet"). Security Engineer sign-off is mandatory for crypto / auth / promotion (`docs/ROLES.md` §2.2 veto).
- **Owner:** Tech Lead (ADR final reviewer); Security Engineer (veto on the listed surfaces).

### Stage 4 — Harness plan
- **Entry:** the work is approved to build (Type-3 trivial, accepted Type-2, or Accepted-ADR Type-1).
- **Agent's job:** decide *how* the harness will execute and verify the work — solo edit vs. subagent vs. `/loop-engine --framework keepsave`; which phases; which verification pattern; model/effort/budget; whether parallel edits need worktree isolation. This is the entire subject of [`docs/HARNESS_ENGINEERING.md`](HARNESS_ENGINEERING.md). For trivial work the plan is one sentence; for a multi-file or cross-package change it is a short phase list.
- **Gate:** the plan names a **verification step for every security-relevant claim** the change makes. No verification plan → no build.
- **Owner:** the engineer driving the agent (Tech Lead / Backend / Frontend per subsystem).

### Stage 5 — Build
- **Agent's job:** implement on the designated feature branch, following `CLAUDE.md` §Coding Conventions exactly — Go `internal/` layout and `fmt.Errorf("…: %w", err)` wrapping; **never return `err.Error()` to clients** (use `httperror`, `docs/ERROR_HANDLING_STANDARD.md`); **every state-mutating handler emits an audit event** from the canonical taxonomy (`docs/AUDIT_LOG_COVERAGE.md`); no `any` in TypeScript; conventional-commit messages.
- **Gate:** branch builds; lint clean (the CI `backend-lint`/`go vet` gate in `.github/workflows/ci.yml`); no plaintext secret is logged, returned in an error, or persisted in browser storage (`docs/ROLES.md` §2.4).
- **Owner:** Backend / Frontend / DevOps per subsystem.

### Stage 6 — Verify
- **Agent's job:** add tests at **every relevant pyramid layer** (`tests/PYRAMID.md`) — handler, service, repo, and **audit-log assertion**, plus a **negative-auth test** for any new surface (`tests/NEGATIVE_AUTH_PLAN.md`, `docs/ROLES.md` §2.3). Then run the **adversarial verification** pass: an independent agent tries to *refute* each security claim (`docs/HARNESS_ENGINEERING.md` §5).
- **Gate (hard, CLAUDE.md):** the test **asserts the audit row was written** for every mutating handler — *PRs failing this are rejected.* Negative paths get more attention than happy paths (`docs/ROLES.md` §2.6). A refuted claim sends the work back to Stage 5.
- **Owner:** QA / Test Engineer (interim: the implementing engineer).

### Stage 7 — Review
- **Agent's job:** run `/code-review` (correctness) and, for any security-relevant diff, `/security-review`. Self-review against the threat delta from Stage 2. Surface — not hide — anything the adversarial pass could not fully clear.
- **Gate:** **Security Engineer veto** on `internal/crypto`, `internal/auth`, promotion (`docs/ROLES.md` §2.2). A human reviews every PR; an agent's review supplements, never replaces, the human one for these surfaces.
- **Owner:** Tech Lead + Security Engineer.

### Stage 8 — Land
- **Agent's job:** open a **draft PR** first; keep the branch current; drive CI green (re-diagnose and re-kick on failure, don't go quiet). Mark ready only when every prior gate is green.
- **Gate:** all CI checks pass; required reviewers approved; the PR description links the ADR / threat delta / FOLLOWUPS item. Merge to `main` requires PR review (CLAUDE.md §Coding Conventions). PROD-affecting promotion follows the rollback-drill discipline in `docs/RUNBOOK.md`.
- **Owner:** Tech Lead (merge authority).

### Stage 9 — Post-merge
- **Agent's job:** close the loop — update `docs/THREAT_MODEL.md` and `docs/ARCHITECTURE.md` if a boundary moved (both are *living* docs), tick or add the `docs/FOLLOWUPS.md` entry with owner + date, and confirm the audit trail captured the change. If the change was triggered by an incident, file the blameless post-mortem action items (`docs/ROLES.md` §3.3).
- **Gate:** docs match reality; FOLLOWUPS reflects what shipped and what was deferred (with owner + due date — no-owner items are not tracked, `docs/FOLLOWUPS.md`).
- **Owner:** Tech Lead; Technical Writer for external-facing docs.

---

## 3. Decision-class fast paths

The full pipeline is for non-trivial work. Most changes are smaller; classify first, then take the matching path.

| Class | Lifecycle path | Mandatory gates |
|-------|----------------|-----------------|
| **Type-3** (doc edit, in-package refactor) | 0 → 1 → 5 → 6 → 7 → 8 → 9 | PR review. Audit/test gates only if code paths change. |
| **Type-2** (new endpoint, new UI component, dep upgrade) | 0 → 1 → 2 → (RFC if non-trivial) → 4 → 5 → 6 → 7 → 8 → 9 | Threat delta if a boundary moves; audit-log assertion + negative-auth test for new surfaces; PR review. |
| **Type-1** (crypto, key hierarchy, breaking schema, audit-field change, PROD key rotation) | **all stages, in order** | ADR `Accepted` **before** Stage 5; Security + Tech Lead sign-off; threat delta; rollback plan; audit-log assertions; adversarial verify; Security veto at review. |

The Type-1 ordering is the load-bearing rule: **Stages 2-3 block Stage 5.** An agent that has written crypto code before its ADR is Accepted has violated the ADLC regardless of how good the code is.

---

## 4. Responsibility matrix (who owns each gate)

Maps `docs/ROLES.md` roles onto the lifecycle. The **agent** executes the column work; the **human role** owns the gate and the sign-off.

| Stage | Human owner (gate) | Agent's contribution |
|-------|--------------------|----------------------|
| 0 Intake | Product Manager / Tech Lead | restate, locate, link FOLLOWUPS |
| 1 Classify | Tech Lead (+Security on sensitive surfaces) | propose Type, name escalation trigger |
| 2 Threat | **Security Engineer (veto)** | draft THREAT_MODEL delta with file:line |
| 3 Design | Tech Lead (+**Security veto**) | draft ADR/RFC: options, rejection, rollback |
| 4 Harness | driving engineer | propose harness shape + verification plan |
| 5 Build | Backend / Frontend / DevOps | implement to conventions; no plaintext leak |
| 6 Verify | QA / Test Engineer | tests at all layers + audit assert + adversarial refute |
| 7 Review | Tech Lead + **Security (veto)** | run `/code-review`, `/security-review`; self-review |
| 8 Land | Tech Lead (merge) | draft PR, drive CI green, link artifacts |
| 9 Post-merge | Tech Lead / Technical Writer | update living docs, close FOLLOWUPS, confirm audit |

No agent occupies a "human owner" cell. The Security Engineer's veto on crypto/auth/promotion (Stages 2, 3, 7) is the spine: it appears three times on purpose.

---

## 5. Definition of done

Borrowed from `docs/ROLES_30_60_90.md` ("existence alone is not done") and made concrete for agent work. A task is **done** only when all hold:

- [ ] Classified, and if Type-1, the ADR is **Accepted** and was merged before the implementation.
- [ ] `docs/THREAT_MODEL.md` reflects any widened surface (or a one-line "no boundary moved" justification is recorded).
- [ ] Every state-mutating handler emits an audit event **and a test asserts the row was written** (CLAUDE.md — non-negotiable).
- [ ] Negative-auth coverage exists for every new surface (`tests/NEGATIVE_AUTH_PLAN.md`).
- [ ] No `err.Error()` reaches a client; no plaintext secret is logged, returned, or persisted.
- [ ] Security-relevant claims survived an **adversarial** verification pass, not just a green test run.
- [ ] `/code-review` (and `/security-review` for sensitive diffs) ran; Security Engineer approved any crypto/auth/promotion change.
- [ ] CI is green on a real PR; living docs and `docs/FOLLOWUPS.md` updated.

---

## 6. Anti-patterns (agent edition)

Extends `docs/ROLES.md` §7 to AI-assisted work. Any of these is grounds to reject the change:

- **"The test passes, so it's correct."** Confirmatory testing on a secrets product is not verification. Refute it (Stage 6).
- **"I'll add the audit log later."** The audit log *is* the feature (`docs/ROLES.md` §7). It ships in the same PR or the PR doesn't ship.
- **Lowering the decision class to skip a gate.** Reclassifying a Type-1 as a Type-2 to avoid the ADR is the most dangerous failure mode here — it's `docs/ROLES.md` §3.1's "misclassification is a bug" with intent.
- **Writing Type-1 code before the ADR is Accepted.** Stages 2-3 block Stage 5. No exceptions.
- **Editing `main` directly, or a parallel fan-out without worktree isolation.** Both forfeit reversibility (`docs/HARNESS_ENGINEERING.md` §6).
- **"Try harder" fallbacks.** Retry-then-allow, fallback-to-cache, default-to-permit — all banned (`docs/ROLES.md` §1.4). Security defaults are deny.
- **Going quiet on a blocked gate.** Fail closed *and surface it.* A silently abandoned task reads as "done" and is worse than a loud failure.

---

## 7. Worked example

**Task:** "Add an endpoint to list a project's promotion history."

1. **Intake** — net-new read endpoint on the promotion subsystem; trust boundary 2 (authenticated caller). Not in FOLLOWUPS → flag as new.
2. **Classify** — new endpoint ⇒ **Type-2**. Escalation trigger: *if it returns any decrypted value or changes the promotion schema, it becomes Type-1.* It returns metadata only, so it stays Type-2.
3. **Threat** — read path over promotion data: can it leak cross-tenant history, or plaintext? Add a STRIDE *Information Disclosure* row to `docs/THREAT_MODEL.md`; confirm `RequireProjectAccess` (ADR-0005) scopes the query.
4. **Design** — Type-2, non-breaking; a short RFC note suffices, no ADR.
5. **Harness** — single-package change → one implementer agent + one adversarial verifier. Verification plan: refute "this endpoint can return another tenant's history" and "this read is in the audit log."
6. **Build** — handler → service → repo; read is audited (`promotion.history.read`); `httperror` on failure; no raw rows leaked.
7. **Verify** — handler/service/repo tests; **audit-row assertion**; negative-auth cell: caller without project access gets 403. Adversarial agent tries a cross-tenant `project_id` — confirmed blocked.
8. **Review** — `/code-review`; because it touches promotion, `/security-review` + Security Engineer sign-off (veto surface).
9. **Land** — draft PR linking the threat delta; CI green; merge.
10. **Post-merge** — `docs/ARCHITECTURE.md` endpoint list updated; FOLLOWUPS notes the deferred pagination as a new owned item.

The same trace, run for a *Type-1* (say, "change the promotion engine to re-wrap under a rotated DEK"), would stop after Stage 1 until ADR-0003's successor is drafted, reviewed by Security, and **Accepted** — because Stages 2-3 block Stage 5.

---

## 8. Relationship to the rest of the governance

The ADLC is the *sequencer*; these are the *authorities* it calls into. When they conflict, **they win** — the ADLC never overrides a gate, only orders it.

| Concern | Authority |
|---------|-----------|
| Who signs off / who has veto | [`docs/ROLES.md`](ROLES.md) |
| Decision classes | `docs/ROLES.md` §3.1 / CLAUDE.md |
| Irreversible decisions | [`docs/adr/`](adr/) |
| Attack surface | [`docs/THREAT_MODEL.md`](THREAT_MODEL.md) |
| Audit taxonomy | [`docs/AUDIT_LOG_COVERAGE.md`](AUDIT_LOG_COVERAGE.md) |
| Error responses | [`docs/ERROR_HANDLING_STANDARD.md`](ERROR_HANDLING_STANDARD.md) |
| Test layers & gates | [`tests/PYRAMID.md`](../tests/PYRAMID.md), [`tests/NEGATIVE_AUTH_PLAN.md`](../tests/NEGATIVE_AUTH_PLAN.md) |
| Tracked work | [`docs/FOLLOWUPS.md`](FOLLOWUPS.md) |
| *How* the harness runs a stage | [`docs/HARNESS_ENGINEERING.md`](HARNESS_ENGINEERING.md) |
