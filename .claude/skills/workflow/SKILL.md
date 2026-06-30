---
name: workflow
description: Author and run a deterministic multi-agent Workflow for a KeepSave task, sequenced by the AI-Assisted Development Life Cycle (docs/ADLC.md) and built from the patterns in docs/HARNESS_ENGINEERING.md. Use whenever the user wants to orchestrate a non-trivial change with subagents, fan work out across many files, run an adversarial verification pass on a security-sensitive diff, audit a subsystem, or says "use a workflow", "run a workflow", "orchestrate this", "fan out agents", or "/workflow". Bakes in KeepSave's gates — decision-class classification, threat-model delta, audit-log assertion, never-surface-plaintext, and Security-Engineer veto on crypto/auth/promotion — so the harness cannot route around them. Also the reference for how to create or extend a workflow skill of this kind.
---

# KeepSave `/workflow` — task → gated harness

Turns a single KeepSave task into a **deterministic, gated, multi-agent Workflow**. It is the executable form of the [ADLC](../../../docs/ADLC.md): the lifecycle says *what* gates a change must pass; this skill *runs* them as Workflow phases, with KeepSave's invariants compiled into every agent prompt.

It owns three things:
1. **Right-sizing** — pick the smallest harness that fits (solo edit vs. subagent vs. full workflow), per `docs/HARNESS_ENGINEERING.md` §1.
2. **Authoring** — generate a runnable Workflow script from the bundled template, parameterized to the task.
3. **Gate enforcement** — wire the ADLC gates (classify → threat → build → adversarial-verify → review) so the harness fails closed.

> **Opt-in.** The `Workflow` tool spawns many subagents and spends real tokens. Only run a workflow when the user has asked for one (the triggers in this skill's `description`, an explicit "use a workflow", or a task that plainly needs fan-out/audit scale). For a one-off edit, do it inline — say so and skip the workflow.

---

## When to use vs. when not to

| Use `/workflow` | Do it inline (no workflow) |
|-----------------|----------------------------|
| Touches `internal/crypto` / `internal/auth` / promotion (needs adversarial verify) | A typo, a doc edit, a one-file refactor |
| Multi-file or multi-package change (migrate N sites) | A change you can hold in one context |
| Subsystem audit / pre-pentest sweep | A single lookup ("where is X defined?") — use one `Explore` |
| Unknown-size discovery ("find every place a secret could be logged") | A change with no security-relevant claim to refute |

When in doubt, **scout inline first** (list the files, scope the diff), then decide. You don't need the shape before the task — only before the orchestration step.

---

## Workflow (how this skill runs)

### Step 1 — Intake & right-size
Restate the task in one line. Locate it on `docs/ARCHITECTURE.md` (which subsystem, which trust boundary). Check `docs/FOLLOWUPS.md` — prefer tracked work. Then pick the harness from `docs/HARNESS_ENGINEERING.md` §1. If the answer is "inline," **stop and do it inline** — don't author a workflow for a one-liner.

### Step 2 — Classify (ADLC Stage 1)
Assign **Type-1 / Type-2 / Type-3** (`docs/ROLES.md` §3.1). Critically:
- **Type-1** (crypto scheme, key hierarchy, breaking schema, audit-field change, PROD key rotation) → the workflow **may draft the ADR and a plan, but must not land implementation.** Surface that the human owns the decision; the build phases stay gated behind an Accepted ADR. Do not generate a workflow that writes Type-1 code.
- **Type-2 / Type-3** → proceed; the workflow runs end-to-end to a draft PR.

### Step 3 — Author the script
Copy `templates/keepsave-task.workflow.js` and specialize the marked `TODO` blocks:
- the task description and the **work-list** (files/endpoints/claims) the pipeline runs over;
- the **security claims** the adversarial-verify phase must refute (§5 of the harness doc);
- model/effort/budget per `docs/HARNESS_ENGINEERING.md` §4.

Keep the gate phases intact — they're the point. Pass the script inline to the `Workflow` tool (it persists to a file and returns the path; iterate by editing that file and re-invoking with `scriptPath`).

### Step 4 — Run & stay in the loop
Launch it. It runs in the background and notifies on completion. For multi-phase work, run one workflow per phase (understand → design → build → review) and **read each result before deciding the next** — don't fire-and-forget a Type-1.

### Step 5 — Land (ADLC Stages 8-9)
Open a **draft PR**; drive CI green; update `docs/THREAT_MODEL.md` / `docs/ARCHITECTURE.md` if a boundary moved; tick `docs/FOLLOWUPS.md`. Security-Engineer sign-off is required before merging any crypto/auth/promotion diff.

---

## Gate checklist (the workflow must enforce all that apply)

- [ ] **Classified**; Type-1 implementation is *not* auto-landed (human owns the decision).
- [ ] **Threat delta** drafted for any widened surface (`docs/THREAT_MODEL.md`).
- [ ] **Audit-log assertion** — every state-mutating handler emits an event *and* a test asserts the row (CLAUDE.md hard gate).
- [ ] **Negative-auth** cell for every new surface (`tests/NEGATIVE_AUTH_PLAN.md`).
- [ ] **No plaintext** logged, returned, or persisted; **no `err.Error()`** to clients (`httperror`).
- [ ] **Adversarial verify** — security claims refuted by independent, lens-diverse verifiers, default-deny (`docs/HARNESS_ENGINEERING.md` §5).
- [ ] **`/code-review`** ran; **`/security-review`** ran on sensitive diffs.
- [ ] **Worktree isolation** on any parallel stage that mutates overlapping files.
- [ ] **Fail closed** — a blocked gate stops the run and is reported, never routed around.

---

## Guardrails compiled into every agent prompt

Inject these verbatim into build/verify prompts (the template already does):

> Never log, return in an error body, persist to browser storage, or commit to a fixture any plaintext secret value. Use the `httperror` package — never `err.Error()` — for client responses. Every state-mutating handler must emit an audit event from `docs/AUDIT_LOG_COVERAGE.md`, and its test must assert the row was written. Work on the feature branch only; never edit `main`. If a gate cannot be satisfied, stop and report it — do not route around it.

---

## Authoring a new workflow recipe (extending this skill)

"The way to create a `/workflow` skill" is itself documented here so the next one is consistent.

1. **Anatomy of the skill.** A skill is a folder under `.claude/skills/<name>/` with a `SKILL.md` (YAML frontmatter `name` + a trigger-rich `description`, then a Markdown body) and optional `templates/`. Frontmatter `description` is the *only* thing the model sees when deciding whether to trigger — pack it with the user phrasings that should activate it. (Mirror the layout of the sibling `.claude/skills/SKILL.md`.)
2. **Anatomy of a Workflow script** (`templates/*.workflow.js`):
   - Starts with `export const meta = { name, description, phases }` — a **pure literal** (no variables, calls, or interpolation). Phase titles in `meta.phases` must match the `phase()` calls.
   - Body is async JS (not TypeScript): `phase(title)`, `agent(prompt, {schema, phase, label, model, effort, isolation, agentType})`, `pipeline(items, ...stages)`, `parallel(thunks)`, `log(msg)`, and the globals `args` (your input) and `budget` (the token ceiling).
   - `agent(...)` with a JSON-Schema `schema` returns a validated object; without one it returns text. `pipeline` is the default (no barrier); `parallel` is a barrier — reach for it only when a stage needs *all* prior results.
   - No `Date.now()` / `Math.random()` (they break resume) — stamp times after the run; vary randomness by index.
3. **Make it a KeepSave workflow, not a generic one.** Every new recipe must keep the ADLC gates (classify → threat → verify → review) and the guardrail block above. A workflow that fans out without an adversarial-verify phase is not appropriate for this repo's sensitive surfaces.
4. **New recipe → new template, not a fork of the gates.** Add a `templates/<recipe>.workflow.js` for a new shape (e.g. `audit-subsystem`, `migrate-callsites`), reuse the schemas and guardrails, and document the trigger in this `description`.

---

## See also

- [`docs/ADLC.md`](../../../docs/ADLC.md) — the lifecycle this skill executes.
- [`docs/HARNESS_ENGINEERING.md`](../../../docs/HARNESS_ENGINEERING.md) — the pattern menu (pipeline, adversarial verify, budgets).
- [`.claude/skills/SKILL.md`](../SKILL.md) — multi-agent team planner, for long-lived multi-phase efforts.
- `templates/keepsave-task.workflow.js` — the starter script this skill specializes.


---

## Frameworks: AIDLC by default, pluggable, bring-your-own

This skill is **framework-driven**. A *framework* is a markdown file in `frameworks/` that
defines how a task moves from intake to done — its ordered stages, the exit gate per stage, which
stages are human-gated, and the hard gates that block the run. The skill reads the selected
framework and sequences the work (single agent or a multi-agent `Workflow`) to match it.

### Selecting a framework
- **Default:** invoking this skill with no framework uses `aidlc` (the file with `default: true`),
  which encodes **this repo's** real gates — so the default behaviour follows the project's AIDLC.
- **Switch per task:** `framework=<name>` (e.g. `framework=sdlc`, `framework=lightweight`) or just
  ask in words ("run this as SDLC", "do this lazy/lightweight").
- The skill resolves `<name>` → `frameworks/<name>.md`, **validates** it has `## Stages` and
  `## Hard gates`, and follows it. A framework missing either is refused (fail-closed — a
  framework with no gates is not a framework).

### Bundled frameworks
- `aidlc` — **default.** Full gates for anything non-trivial or security-sensitive in this repo.
- `sdlc` — classic requirements→design→build→test→deploy→maintain phase gates.
- `lightweight` — small, low-risk, reversible changes; skips heavy gates, keeps the safety floor.

### Bring your own framework
Copy `frameworks/TEMPLATE.md` to `frameworks/<your-name>.md`, fill the four sections, and run
`framework=<your-name>`. No code change — the skill discovers it by filename. See
`frameworks/README.md` for the file format and the full rules. (Pairs well with the `ponytail`
skill for keeping each stage's output minimal.)

### Optional external project-management sync
Off by default. The in-repo tracker stays the source of truth. If a team wants run/stage status
mirrored into **ClickUp** or **GitHub Issues/Projects**, a framework's `## Tracker sync` section
opts in and names the stage→status mapping (uses the ClickUp / GitHub MCP tools when connected).
Nothing syncs unless a framework enables it and the user asks. Details in `frameworks/README.md`.


## Model & orchestration

Workflow agents **inherit the session model by default** (usually correct). You can pin the model
per agent (`agent(prompt, { model: 'opus', effort: 'high' })`) or per phase (apply the same model
to every `agent()` in it); `meta.phases[].model` is a display hint, not an enforced override. A
script can *set* the model but can't read it back — the resolved model shows in `/workflows` and in
`workflowProgress[].model`.

For self-narrating runs, wrap `agent()` in a `runAgent({ role, model, phase })` helper that
`log()`s `phase · role · model` before each call, so a live run tells you which model and role each
step uses. To **auto-pick** the model, use a deterministic `pickModel(role)` policy (mechanical →
`sonnet`, judgment/verify → `opus`) — it must honor this repo's model policy (see
`frameworks/aidlc.md` Hard gates; Governance = never Haiku).

Full guidance, the orchestration-layers table (script vs main-loop vs `workflow()` nesting vs agent
rosters), and a worked example: **`ORCHESTRATION.md`** in this directory.
