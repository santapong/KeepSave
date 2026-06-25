# KeepSave — Harness Engineering

The **harness** is everything around the model: the tools it can call, the context it's given, the subagents it spawns, the workflows that sequence them, and the verification that decides whether work is trustworthy. Writing a good prompt is necessary but not sufficient on a secrets product — the *harness* is what turns "the model tried" into "the change is verified, gated, and reversible."

This doc is the **"how"** behind Stage 4 of the [ADLC](ADLC.md). The ADLC says *every non-trivial task states how it will be executed and verified before code is written*; this doc is the menu that plan is chosen from.

> Design rule for this repo: **escalate harness complexity only when the task earns it.** A doc fix is a single edit. A cross-package security change is a workflow with adversarial verification. Over-engineering the harness wastes tokens and hides the real change; under-engineering it ships unverified crypto. Both are bugs.

---

## 1. Building blocks

The KeepSave harness is assembled from five primitives, smallest to largest:

| Primitive | What it is | Reach for it when |
|-----------|------------|-------------------|
| **Tools** | `Read`, `Edit`, `Grep`, `Bash`, MCP tools (GitHub, etc.) | Always — the base layer. |
| **Skills** | Packaged procedures in `.claude/skills/` (`/code-review`, `/security-review`, `/verify`, `/simplify`, `/workflow`) | A recurring procedure with a known shape. |
| **Subagents** (`Agent` tool) | A fresh context window doing one scoped job, returning a conclusion | A search/read/verify that would flood the main context, or independent work to parallelize. |
| **Workflows** (`Workflow` tool) | A deterministic JS script orchestrating many subagents — `pipeline`, `parallel`, loops | Fan-out, multi-stage pipelines, or scale one context can't hold. |
| **Agent team** | The lean core in `.claude/skills/` (`project-manager`, `agent-designer`, `general-worker`) + recruited specialists | A long-lived, multi-phase effort needing persistent roles and state. |

**Subagent types available** (pass as `agentType` / `subagent_type`): `Explore` (read-only fan-out search — returns conclusions, not file dumps), `Plan` (architecture/implementation planning), `general-purpose` (multi-step), plus this repo's `code-review` style reviewers. Use `Explore` for "where does X live across the codebase" so the main context stays clean.

### Choosing the smallest harness that fits

```
Is it one obvious edit in one file?                    → solo tools, no subagent
Is it "find/understand across many files"?             → one Explore subagent, keep the conclusion
Does it touch crypto / auth / promotion?               → workflow w/ adversarial verify (§5), never solo
Is it multi-file or multi-package?                     → pipeline() over the work-list
Do parallel edits touch the same files?                → worktree isolation (§6)
Is it a long, multi-phase feature?                     → recruit a team (multi-agent-team-planner skill)
```

When unsure, scout inline first (list the files, scope the diff), *then* pick the orchestration. You don't need to know the shape before the task — only before the orchestration step.

---

## 2. Mapping ADLC stages to harness phases

A `/workflow` run for a non-trivial KeepSave task mirrors the [ADLC](ADLC.md). Not every stage is a separate agent — cheap stages collapse into the driver's own context; expensive or parallelizable ones become subagents.

| ADLC stage | Harness realization |
|------------|---------------------|
| 0 Intake / 1 Classify | driver inline (1-2 `Read`s + `docs/FOLLOWUPS.md` check) — too cheap to spawn |
| 2 Threat delta | one `Explore` over the touched code → threat-surface summary the driver turns into the THREAT_MODEL delta |
| 3 Design | a `Plan` subagent drafts ADR/RFC options; the **human** decides |
| 4 Harness plan | the driver writes the phase list (this doc is the menu) |
| 5 Build | one implementer subagent per independent unit (`pipeline` if multi-file) |
| 6 Verify | the **adversarial-verify** pattern (§5) — independent refuters, one per security claim |
| 7 Review | `/code-review` + (sensitive diffs) `/security-review` as subagent phases |
| 8 Land | driver inline — draft PR, CI, status |
| 9 Post-merge | driver inline — doc + FOLLOWUPS updates |

The default multi-stage shape is **pipeline**, not barrier: an item that's done building should start verifying while the next item is still building. Use a barrier (`parallel` between stages) only when a stage genuinely needs *all* prior results at once — e.g. dedupe every finding before the expensive verification pass.

---

## 3. Orchestration patterns

These are the reusable shapes. Each maps to a KeepSave use case.

- **Pipeline (default).** `pipeline(items, build, verify)` — each file/endpoint flows through build→verify independently, no barrier. Wall-clock is the slowest single chain, not the sum. *Use for:* migrating N error sites to `httperror`; adding audit emission to N handlers.
- **Parallel barrier.** `parallel(thunks)` — await all, then proceed. *Use for:* collecting findings from several reviewers before deduping, or an early-exit ("0 findings → skip verification").
- **Adversarial verify (§5).** N independent skeptics per claim, each prompted to *refute*. *Use for:* every crypto/auth/promotion claim. This is the highest-value pattern in the repo.
- **Perspective-diverse verify.** When a finding can fail multiple ways, give each verifier a distinct lens — *correctness*, *plaintext-leak*, *cross-tenant*, *downgraded-token*, *audit-row-present* — instead of N identical checks.
- **Loop-until-dry.** Keep spawning finders until K consecutive rounds surface nothing new. *Use for:* "find every place a secret could be logged" — a fixed count misses the tail.
- **Judge panel.** N independent design attempts, scored, synthesize from the winner. *Use for:* a Stage-3 design with a wide solution space (e.g., a new key-rotation scheme) — but the human still owns the Type-1 decision.
- **Multi-modal sweep.** Parallel finders each searching a *different way* (by-package, by-string, by-data-flow). *Use for:* a pre-pentest audit where one search angle won't find everything.
- **Completeness critic.** A final agent asks "what's missing — a handler not audited, a claim unverified, a negative-auth cell not written?" Its output becomes the next round.

---

## 4. Model, effort, and budget

The harness costs money and latency; spend both where verification value is highest.

- **Model.** Default: inherit the session model — almost always correct. Override only when confident a tier fits: a cheap mechanical stage (mass `httperror` migration) can run a smaller model; the hardest adversarial-verify / judge stage on a crypto change can run the strongest. The lean-core convention (`.claude/skills/SKILL.md`) is *workers default cheap, PM/architect/reviewer go strong.*
- **Effort.** `low` for mechanical edits; reserve `high`/`xhigh` for the refute-the-security-claim stages. Diversity of *lens* beats raising effort on identical checks.
- **Budget.** A `+Nk` directive is a **hard ceiling** shared across the main loop and all workflows. Scale the fleet to it: `const FLEET = budget.total ? Math.floor(budget.total / 100_000) : 5`. For unknown-size discovery, loop while `budget.total && budget.remaining() > 50_000`. **Never** loop on `remaining()` without the `budget.total` guard — with no target it's `Infinity` and runs to the agent cap.
- **No silent caps.** If a sweep bounds coverage (top-N findings, sampled files, no-retry), `log()` what was dropped. On a secrets product, silent truncation reads as "we checked everything" when we didn't — a `docs/ROLES.md` §1.4 "no silent fallback" violation at the harness level.

---

## 5. Adversarial verification (the load-bearing pattern)

KeepSave cannot accept "the test is green" as proof (ADLC §1.4, §6). The harness closes that gap by spawning verifiers whose **only job is to refute** a claim, defaulting to *refuted* when uncertain.

For each security-relevant claim a change makes — *no plaintext is logged or returned*, *auth cannot be downgraded/forged onto this path*, *the audit row is written*, *the query is tenant-scoped* — spawn ≥3 independent verifiers, each with a distinct lens, and kill the claim unless a majority clears it:

```js
// inside a workflow stage — one claim, three lenses, majority must clear
const lenses = ['plaintext-leak', 'cross-tenant', 'downgraded-token']
const votes = await parallel(lenses.map(lens => () =>
  agent(
    `Adversarially verify, via the ${lens} lens: "${claim}". ` +
    `Try to REFUTE it against ${files}. Default to refuted=true if uncertain.`,
    { phase: 'Verify', schema: VERDICT_SCHEMA }
  )))
const survives = votes.filter(Boolean).filter(v => !v.refuted).length >= 2
```

Rules:
1. **Independence.** Verifiers don't see each other's verdicts — no anchoring.
2. **Refute, don't confirm.** A verifier told to "check if it's correct" rationalizes; one told to "break it, assume it's wrong" finds the hole.
3. **Default-deny under uncertainty.** Maps `docs/ROLES.md` §1.4 onto the verifier: ambiguous evidence ⇒ refuted ⇒ back to Stage 5.
4. **Diversity over redundancy.** Three *different* lenses beat three identical refuters — they catch failure modes redundancy can't.

A change to `internal/crypto`, `internal/auth`, or the promotion engine that did **not** go through this pass is not Stage-6 complete, regardless of test color.

---

## 6. KeepSave-specific guardrails

The harness inherits the project's invariants. Bake these into every agent prompt that touches code:

- **Never surface plaintext.** No secret value in a log, an error body, a return value, a test fixture committed to the repo, or browser storage (`docs/ROLES.md` §2.4). Verifiers check for this explicitly.
- **`httperror` only.** No `err.Error()` to a client — `docs/ERROR_HANDLING_STANDARD.md`, lint-enforced.
- **Audit-or-it-didn't-happen.** Every state-mutating handler emits an event from `docs/AUDIT_LOG_COVERAGE.md` *and* a test asserts the row. This is a CLAUDE.md hard gate — verifiers assert it.
- **Human owns Type-1.** The harness may *draft* an ADR or a migration; it may never *accept* a Type-1 (ADLC §1.1). A workflow that would land a crypto/schema change without a merged ADR is malformed.
- **Worktree isolation for parallel writes.** Any `pipeline`/`parallel` stage where agents edit overlapping files runs with `isolation: 'worktree'` — otherwise concurrent edits corrupt each other. It's expensive (~200-500ms + disk per agent); use it *only* for genuine parallel mutation, and the unchanged worktrees auto-clean.
- **Branch, never `main`.** All harness work lands on the designated feature branch; merge to `main` is a human-reviewed PR.
- **Fail closed and loud.** A blocked gate stops the run *and reports the blocker*. Routing around a gate is the worst harness failure mode on this product.
- **Scope discipline.** GitHub reach is limited to the in-scope repo; don't let a sweep or search wander outside it.

---

## 7. State management for multi-phase work

When a task outgrows one context window (a multi-day feature, a large migration), externalize state so nothing is lost to context truncation — the pattern from the `multi-agent-team-planner` skill:

- `.claude/state.md` — current phase, files owned, blocking questions. The driver re-reads it at every phase boundary.
- `.claude/decisions/PHASE-N.md` — decisions made per phase (fixes "decisions live only in the PM's head").
- `.claude/designs/phase-N.md` — architects write designs to *files*, not as tool return values (fixes the output-cap / telephone-game problem).
- `CLAUDE.md` — the invariants every spawned agent inherits.

Workflows themselves are resumable: a killed or edited run relaunches with `resumeFromRunId`, and the unchanged prefix of `agent()` calls returns cached results. Stamp timestamps *after* the run — `Date.now()`/`Math.random()` are unavailable inside scripts by design (they'd break resume).

---

## 8. Failure modes & mitigations

| Risk | Mitigation in this harness |
|------|----------------------------|
| Plausible-but-wrong change ships | Adversarial verify (§5), majority-refute |
| "Green tests" mistaken for verification | Refute-don't-confirm verifiers, default-deny |
| Concurrent edits corrupt each other | `isolation: 'worktree'` on parallel mutation (§6) |
| Context truncation loses decisions | Externalized state to disk (§7) |
| Silent coverage gap | `log()` every dropped item (§4) |
| Runaway token spend | `budget`-guarded loops, fleet scaled to ceiling (§4) |
| Agent routes around a gate | ADLC fail-closed + Security veto at Stage 7 |
| Harness over-engineered for a trivial change | "smallest harness that fits" decision tree (§1) |

---

## 9. See also

- [`docs/ADLC.md`](ADLC.md) — the lifecycle these patterns execute.
- [`.claude/skills/workflow/SKILL.md`](../.claude/skills/workflow/SKILL.md) — the `/workflow` skill that turns a task into one of these harnesses.
- [`.claude/skills/SKILL.md`](../.claude/skills/SKILL.md) — multi-agent team planner (lean core + recruit-on-demand).
- [`docs/ROLES.md`](ROLES.md) — the human gates the harness reports to.
