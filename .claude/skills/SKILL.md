---
name: multi-agent-team-planner
description: Design and bootstrap a multi-agent team for a Claude Code project — pick the right shape (single-PM, federated, hierarchical), generate a lean three-agent core (PM + agent-designer + general-worker), and lay down state-management scaffolding that prevents context drift and information mismatch. Use this skill whenever the user wants to set up `.claude/agents/`, plan agents for a new project, design a multi-agent workflow, recruit specialized agents, scale or slim down an existing agent team, or asks "how many roles do I need", "what agents should I have", "how do I structure my .claude/agents", or anything about orchestrator-worker patterns. Trigger generously — even when the user just describes a new project's complexity, this skill helps right-size the team rather than over-engineering with a fixed roster of 8 roles upfront.
---

# Multi-Agent Team Planner

Right-sizes a multi-agent team for a Claude Code project. Produces:

1. A **team plan** (`AGENTS-PLAN.md`) at the project root for review.
2. A **lean core** of three universal agents in `.claude/agents/`: PM, agent-designer, general-worker.
3. **State-management scaffolding**: `CLAUDE.md` invariants, `.claude/decisions/`, `.claude/state.md`, `.claude/designs/`.
4. A **failure-mode checklist** matched to the chosen team shape.

## Core principle: lean core + on-demand recruitment

Most projects don't need 8 specialist roles upfront. The lean core gives you:

- **`project-manager`** — orchestrates phases, dispatches workers.
- **`agent-designer`** — meta-agent. Given a role description, surveys the codebase, reads sibling agents to absorb local style, and writes a project-specific `.claude/agents/<role>.md`. This is the recruiter.
- **`general-worker`** — generalist fallback for one-off tasks too small to justify a custom agent.

When the PM hits a task that needs domain depth (security audit, vendor API research, CI rewrite), it dispatches `agent-designer` to recruit. The new specialist is project-aware from day one because the designer mirrors local patterns. PM then dispatches the new agent.

This avoids two failure modes: pre-defining roles that never get used (over-engineering) and inventing roles from scratch on every task (inconsistency).

## Workflow

### Step 1 — Survey the project

Ask the user (skip questions whose answer is obvious from context):

1. What is the project doing? (one paragraph)
2. Approximate size (LOC, modules, deploy targets)?
3. Are there clear vertical slices (frontend/backend/ML/infra) or one cohesive codebase?
4. Critical risk areas (auth, threading, security, numerical correctness, real-time)?
5. Any existing `.claude/agents/` files? If yes, read them.
6. Language/stack and lint/style conventions in use?

If the user can't answer something, mark it `TBD` and proceed.

### Step 2 — Pick the team shape

```
Is the project < ~10K LOC AND one cohesive codebase?
├── YES → SHAPE: single-PM (lean core only; recruit on demand)
└── NO
    └── 2+ truly independent vertical slices (separate repos/deploys/stacks)?
        ├── YES → SHAPE: federated-PMs (one team per slice; user coordinates)
        └── NO  → SHAPE: single-PM with extended roster (lean core +
                  pre-recruit 2-4 specialists for the known risk areas)
```

**Reject hierarchical** (PM → senior → workers) unless the user provides specific evidence (two prior failures of the same type) that simpler shapes won't work. Adding management layers is rarely the right move for agent teams — it stacks latency without adding decision authority that the PM didn't already have.

### Step 3 — Generate the lean core

Copy these files into `.claude/agents/` (use bundled templates from this skill's `templates/` folder, then specialize):

| Template | Specialize by injecting |
|---|---|
| `templates/project-manager.md` | Project name, summary, identified risks, available specialists list, key conventions |
| `templates/agent-designer.md` | Project layout (src/, tests/, docs/), local convention markers, output path |
| `templates/general-worker.md` | Tool allowlist appropriate for the project |

When specializing, **don't add fluff**. If the project doesn't have a SimBridge-like threading invariant, don't invent one.

### Step 4 — Lay down state-management scaffolding

Create these in the project root:

- **`CLAUDE.md`** — Project-wide invariants every agent inherits. Populate with: lint/format rules, naming conventions, threading or numerical invariants, "files NOT to touch", critical patterns (e.g., "frozen dataclass + `__post_init__` for value types"). Keep it tight; this is shared across every dispatch.
- **`.claude/state.md`** — Current phase, files-owned-by, blocking questions, TODO. PM writes; subagents read at the start of every dispatch.
- **`.claude/decisions/`** — Empty directory. PM writes one file per phase (`PHASE-1.md`, `PHASE-2.md`, etc.) capturing decisions made. **This is the fix for "decisions live only in PM's context."**
- **`.claude/designs/`** — Empty directory. Architects (when recruited) write designs to `phase-N.md` here, not as Agent-tool return values. **This is the fix for the 8K output cap and the phone-game problem.**

### Step 5 — Write `AGENTS-PLAN.md` at project root

Sections to include:

- **Project summary** (from Step 1)
- **Team shape chosen** (from Step 2) with one-paragraph rationale
- **Active agents** (the three core, plus any pre-recruited specialists)
- **Recruitment policy** — one-line rules for when PM should dispatch agent-designer (e.g., "any phase touching auth or network → recruit a `reviewer`")
- **Failure-mode mitigations applied** — point to `state.md`, `decisions/`, `designs/` patterns
- **Known gaps** — what this setup does NOT solve (observability, cost monitoring, prompt A/B testing, recovery from partial failure). Naming gaps prevents future-you from believing the system is complete.

### Step 6 — Walk through one realistic phase on paper

Sanity-check by tracing a realistic feature request through the team:

1. User: "implement feature X"
2. PM reads `state.md` and surveys
3. PM dispatches `agent-designer` to recruit `architect` for this domain
4. Designer writes `.claude/agents/architect.md`
5. PM dispatches architect → architect writes `.claude/designs/phase-1.md`
6. PM dispatches `general-worker` (or recruits `implementer`) → reads design from file → writes code
7. PM updates `state.md` and writes `decisions/PHASE-1.md`

If the walk-through reveals a missing piece (no test runner? no rollback story?), fix it before declaring done.

## Failure-mode checklist

The lean core + scaffolding handles these by default:

| Risk | Mitigation in this skill |
|---|---|
| Phone game (PM summarizes design, info lost) | ✅ Architect writes design to `.claude/designs/<phase>.md` |
| Decisions evaporate at session end | ✅ `decisions/PHASE-N.md` per phase |
| Stale view (file changed mid-session) | ✅ PM re-reads `state.md` at every phase boundary |
| Output cap truncation (8K) | ✅ Subagents write artifacts to file, return summaries |
| PM context saturation | ✅ Project state externalized to disk |
| Cost compounding | ✅ Sonnet default, opus only for PM/architect/reviewer |

The lean core does NOT auto-handle these — flag them in `AGENTS-PLAN.md` so the user knows:

| Risk | What to do |
|---|---|
| Concurrent write conflicts | Use `isolation: "worktree"` when dispatching parallel implementers |
| Cross-agent invariant drift | Recruit a `reviewer` if signature/contract drift is observed |
| Observability (cost, latency, failure rate) | Not solved; consider hooks (`SubagentStart`/`SubagentStop`) when needed |
| Recovery from partial failure | Not solved; rely on git for now |

## Triggering rules

Trigger this skill aggressively when the user mentions:

- Setting up `.claude/agents/`
- Planning agents for a new project
- "How many roles do I need" / "what agents should I have"
- Recruiting, hiring, or designing agent teams
- Scaling an existing agent setup
- Multi-agent workflow design
- Orchestrator-worker pattern setup

Also trigger when the user describes a new project's complexity even without explicitly asking for agents — if the work is non-trivial, suggest right-sizing a team rather than letting them default to either "no agents" or "8 roles upfront".

## When to refuse or push back

- User asks for "all 8 roles upfront, like POC-RobotArm" without specific failure evidence → push back. Suggest the lean core; recruit on demand.
- User asks for hierarchical structure (PM → seniors → workers) without two prior failures of the same type → push back. Suggest federated-PMs if scale is the issue, or single-PM with extended roster otherwise.
- User has a working agent setup and just wants tweaks → don't run the full workflow. Read their existing `.claude/agents/` and propose targeted changes.

## What this skill does NOT do

- Run evals on agent prompts. Tune by use, not by benchmark, until there's evidence prompts are weak.
- Set up CI/observability for the agent system itself.
- Migrate an existing rich roster into this leaner shape unless explicitly asked.
- Generate the user's actual project code — only the agent setup around it.
