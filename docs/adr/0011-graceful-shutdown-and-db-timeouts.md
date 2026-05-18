# ADR-0011: Graceful shutdown, DB context timeouts, HTTP server timeouts, pool-lifetime tuning, and a shared `safego` helper

- **Status:** Accepted (sponsor-authorized 2026-05-18; retroactive Security/Tech Lead sign-off pending per CLAUDE.md §Type-1)
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (Backend Engineer role)
- **Reviewers required:** Tech Lead (gating sign-off); Security Engineer (recommended — touches process lifecycle but **not** crypto / auth / promotion, so no Security veto per `docs/ROLES.md` §2.2 and §3.1)
- **Supersedes:** none
- **Related:** ADR-0010 part C (`safego` for MCP goroutines — same helper), ADR-0012 (KMS auto-unseal — depends on this ADR for clean restart semantics), ADR-0015 (per-use audit emission — consumes the same `safego`), `docs/audits/BACKEND_SPOF.md` rows #1-#5, `docs/audits/BACKEND_CRASH_RISKS.md` §3.1, `docs/RUNBOOK.md` §3, `docs/FOLLOWUPS.md` 0l, FU 7

---

## Context

The 2026-05-15 SPOF audit returned **5 critical findings**. Three are process-lifecycle defects sharing the same drain-and-bound theme.

**Finding 1 — no graceful shutdown anywhere.** Verbatim from `docs/audits/BACKEND_SPOF.md` row #1 (line 77):

> *"HTTP server lifecycle (`cmd/server/main.go:217,224`) — SIGTERM → in-flight requests killed mid-write; `defer db.Close()` never runs. Blast radius: All connected clients see truncated responses; in-flight DB transactions in `executePromotion` / `RotateProjectKey` left half-committed (no tx wrap). Current mitigation: None. Severity: critical."*

**Finding 2 — 182 DB call sites with no context timeout.** Verbatim from row #3 (line 79):

> *"DB calls have **no context** (`internal/repository/**` — 182 sites). A slow query (lock contention, full table scan) holds a goroutine and a pool slot indefinitely. Blast radius: One slow tenant exhausts the 25-conn pool → app-wide outage with `/readyz` still 200 until ping itself queues. Current mitigation: None. Severity: critical."*

**Finding 3 — plaintext HTTP server has no Read/Write/IdleTimeout.** Verbatim from row #5 (line 81):

> *"Slowloris on HTTP server (`cmd/server/main.go:211-216` and `:224`) — A handful of slow-reading clients each tying up a goroutine starve the server; plaintext path has **zero** timeouts (default `http.Server` from `router.Run`). Trivial DoS — 1k connections of `GET /readyz` with 1-byte/min reads tie up all of Gin's per-conn workers. Current mitigation: `ReadHeaderTimeout` only on TLS path. Severity: critical."*

Two adjacent defects are in the same one-PR blast radius:

- **DB pool lifetimes missing.** `backend/internal/repository/db.go:32-33`, `:44-45`, `:54-55` set max-open and max-idle but neither `SetConnMaxLifetime` nor `SetConnMaxIdleTime`. After PgBouncer/RDS failover dead idle conns sit in the pool and every query returns `driver: bad connection` until the pod is rolled — the 3am pager `docs/RUNBOOK.md` §3 exists to handle reactively.
- **Goroutine no-recover** (`docs/audits/BACKEND_CRASH_RISKS.md` §3.1) overlaps: the shutdown coordinator must launch and join background goroutines, and that launch primitive should be the same `safego.Launch(ctx, fn)` drafted in ADR-0010 part C and called for in FU 0l. One helper, shared across ADR-0010 (MCP build goroutines), this ADR (shutdown coordinator + background workers), and ADR-0015 (per-use audit emission).

Every other reliability fix (KMS auto-unseal in ADR-0012, webhook retry hardening, audit-pruner correctness) is undermined by mid-flight truncation on every k8s rollout.

## Options considered

### Option A — Per-handler `context.WithTimeout` wrappers
Apply `context.WithTimeout(c.Request.Context(), 5*time.Second)` at the top of every handler and pass it down.

- **Pros:** Per-route tuning is natural.
- **Cons:** 182 DB sites reached via ~150 handlers in a fan-out — per-handler timeouts get re-derived in each service call and drift. High risk of "I forgot one" — same shape as the `MustGet` panic surface in FU 0l that the codebase already paid for once.

### Option B — Repository-layer context plumbing (chosen)
Repository methods take `ctx context.Context` as their first argument; all calls become `*Context` variants. A single Gin middleware injects a 5s default. Services pass the request context through unchanged. Per-method overrides via a small map for known-long queries.

- **Pros:** One plumbing change covers all 182 call sites. Uniform cancellation — when `srv.Shutdown(ctx)` fires, every in-flight DB call sees `ctx.Done()` and unwinds.
- **Cons:** Repository signature change is invasive across `internal/service/**`. Override map is a foot-gun if not documented.

### Option C — Drop HTTP/1.1 keepalive
- **Pros:** One-line Slowloris fix.
- **Cons:** Breaks browser clients and the `keepsave-widget` host page. Addresses only one of three criticals.

### Option D — Sidecar reverse proxy
- **Pros:** Familiar defense-in-depth.
- **Cons:** KeepSave ships self-hostable; half our deployments are `docker-compose up` on a single VM. We can't make hardening conditional on a sidecar we don't ship.

## Decision

Adopt Option B as the spine and bundle four adjacent fixes. Five rollback-independent parts:

**Part 1 — Graceful shutdown via `signal.NotifyContext`.** Replace the blocking `ListenAndServeTLS` (`backend/cmd/server/main.go:217`) and `router.Run` (`:224`) with an explicit `&http.Server{}` plus `signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)`. On signal: invoke `srv.Shutdown(ctx)` with a 30s drain budget (configurable via `KEEPSAVE_SHUTDOWN_TIMEOUT`, matches k8s `terminationGracePeriodSeconds`). After Shutdown returns, in order: stop background goroutines (pass ctx to the audit pruner at `:196`, the rate-limiter cleanup at `backend/internal/api/ratelimit.go:37`, the HTTPS redirect listener at `:209`, the event bus, the webhook delivery pool), wait on a `sync.WaitGroup`, **flush nonce-collision metrics per FU 7** (so the 2^28 alert isn't lost on the last second), flush the logger, then `db.Close()`.

**Part 2 — Repository-layer DB context plumbing.** All repository methods accept `ctx context.Context` first and call `*Context` variants of `database/sql`. New middleware in `backend/internal/api/middleware.go` wraps `c.Request = c.Request.WithContext(ctxWithTimeout)` with a 5s default. Per-method overrides via a `map[string]time.Duration` on each repo struct, consulted only when the default is too tight (promotion replay, key rotation, analytical queries).

**Part 3 — HTTP server timeouts on both listeners.**

```
ReadHeaderTimeout: 10 * time.Second
ReadTimeout:       15 * time.Second
WriteTimeout:      30 * time.Second
IdleTimeout:       120 * time.Second
MaxHeaderBytes:    1 << 14
```

The plaintext fallback at `:224` is converted from `router.Run` to an explicit `&http.Server{}` so the same struct receives the same hardening. All values overridable by env var to permit Part-3-only rollback.

**Part 4 — DB pool lifetime tuning.** In each branch of `backend/internal/repository/db.go` (`:32-33` postgres, `:44-45` mysql; `:54-55` sqlite is a no-op since SQLite is single-conn):

```
db.SetConnMaxLifetime(30 * time.Minute)
db.SetConnMaxIdleTime(10 * time.Minute)
```

Closes the failover failure mode `docs/RUNBOOK.md` §3 catalogues.

**Part 5 — `safego.Launch(ctx, fn)` helper, shared with ADR-0010 part C and ADR-0015.** New file `backend/internal/runtime/safego.go`. Wraps `go fn()` with `defer recover()` that logs panic + stack via the structured logger and decrements an injected `sync.WaitGroup` so the shutdown coordinator can join. **One helper, three consumers** (this ADR's shutdown coordinator, ADR-0010 part C MCP build goroutines, ADR-0015 per-use audit emission) — explicitly cross-referenced to avoid drift.

This bundle is Type-1 (process lifecycle touches every request path) but does not modify crypto, auth, or the promotion engine, so per `docs/ROLES.md` §3.1 the Tech Lead is the gating reviewer; Security Engineer review is recommended (shutdown ordering matters for crypto-flush correctness) but not veto-bearing.

## Rejection rationale

- **Option A** lost because FU 0l (58 sites) already taught us that "copy this pattern into every handler" produces drift; 182 callsites is worse.
- **Option C** lost because it breaks the embed widget and every browser client while solving only one of three criticals.
- **Option D** lost because half our deployments have no sidecar to push hardening into.

## Consequences

- **Operational.** k8s rolling updates become non-truncating; `terminationGracePeriodSeconds: 30` is now load-bearing (must be ≥ `KEEPSAVE_SHUTDOWN_TIMEOUT`). New env vars: `KEEPSAVE_SHUTDOWN_TIMEOUT`, `KEEPSAVE_DB_QUERY_TIMEOUT`, `KEEPSAVE_HTTP_READ_TIMEOUT`, `KEEPSAVE_HTTP_WRITE_TIMEOUT`, `KEEPSAVE_HTTP_IDLE_TIMEOUT`. Runbook §3 gets a new step (pool turnover handles dead conns automatically; rollout-restart no longer required after failover).
- **Security.** Trust-boundary shape unchanged. New surface: the per-method override map could let an inattentive PR raise a deadline to mask a slow query. ADR requires overrides carry a comment justifying why the default is insufficient. No threat-model entry change.
- **Migration.** No schema change. No data migration. Repository signatures change — every caller in `internal/service/**` requires recompilation. Wide diff but mechanical; test fixture builder adds `context.Background()` where the test doesn't care.
- **Reversibility.** Each of the 5 parts is independently reversible without data loss. No ciphertext rewrite, no migration.

## Rollback plan

Every part is independently rollback-able. **No part requires a schema change or data migration**, so every rollback is a `git revert` plus redeploy.

- **Part 1 (graceful shutdown):** revert `cmd/server/main.go`; back to immediate-kill SIGTERM (operationally bad but functionally unbreaking).
- **Part 2 (DB context):** revert repository changes; queries fall back to no-timeout. The middleware-injected ctx is harmless on its own if left in place.
- **Part 3 (HTTP timeouts):** set the env vars to `0` to disable each timeout individually without code change. No deploy needed.
- **Part 4 (pool tuning):** no rollback needed — longer-lived conns aren't broken, just sub-optimal. Set `KEEPSAVE_DB_CONN_MAX_LIFETIME=0` to restore unbounded lifetime.
- **Part 5 (`safego`):** the helper can be removed; bare `go fn()` works again.

## Open questions

1. **Shutdown-timeout tiering.** Is a single 30s default acceptable, or should we tier — 15s for idempotent reads, 60s for `executePromotion` / `RotateProjectKey` where mid-flight cancel risks the very partial-state problem SPOF row #1 names? Owner: Backend Engineer + Tech Lead. Due: before Part 1 lands.
2. **DB context timeout granularity.** Repository-layer one-size-fits-all 5s, or per-method override map? What about analytical / reporting queries (`internal/service/anomaly_service.go`, verify-encryption sweep) that legitimately take 30+ seconds? Proposal is the override map but we haven't enumerated the overrides. Owner: Backend Engineer. Due: before Part 2 lands.
3. **`safego.Launch` responsibility surface.** Does the helper own (a) panic logging, (b) nonce-collision-monitor flush per FU 7, (c) shutdown-WaitGroup integration — or are some separate concerns layered on by the caller? Recommendation: helper owns (a) and (c); (b) is a separate shutdown step because nonce-collision flush is not goroutine-local. Owner: Tech Lead. Due: jointly with ADR-0010 part C review.

## References

- `backend/cmd/server/main.go:196` — audit-log pruner goroutine, accept ctx, exit on cancel
- `backend/cmd/server/main.go:209` — HTTPS redirect listener, needs ctx and timeouts
- `backend/cmd/server/main.go:211-216` — TLS server config, missing `ReadTimeout` / `WriteTimeout` / `IdleTimeout`
- `backend/cmd/server/main.go:217` — blocking `ListenAndServeTLS`
- `backend/cmd/server/main.go:224` — blocking `router.Run` (no timeouts at all)
- `backend/internal/repository/db.go:32-55` — add `SetConnMaxLifetime` + `SetConnMaxIdleTime`
- `backend/internal/repository/*.go` — add `ctx` parameter; migrate to `*Context` variants
- `backend/internal/api/middleware.go` — request-context timeout middleware
- `backend/internal/api/ratelimit.go:37` — cleanup loop, needs ctx
- New: `backend/internal/runtime/safego.go` — shared `safego.Launch` helper
- New: `tests/integration/graceful_shutdown_test.go` — SIGTERM during long request drains correctly; SIGTERM mid-DB-query rolls back rather than truncates
- `docs/audits/BACKEND_SPOF.md` rows #1, #2, #3, #4, #5
- `docs/audits/BACKEND_CRASH_RISKS.md` §3.1 — overlapping goroutine-no-recover findings
- `docs/RUNBOOK.md` §3 — DB failover, current dead-conn reactive procedure
- `docs/FOLLOWUPS.md` 0l — `safego` + `getUserID` refactor (shares this ADR's helper)
- `docs/FOLLOWUPS.md` 7 — nonce-collision monitoring per DEK (flushed at shutdown by Part 1)
- ADR-0010 part C — MCP build goroutine `safego` adoption (same helper)
- ADR-0012 — KMS auto-unseal (depends on graceful restart semantics from this ADR)
- ADR-0015 — per-use audit emission (consumes the same `safego`)
