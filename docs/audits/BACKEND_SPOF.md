# Backend SPOF & Reliability Audit

**Auditor role:** Backend SPOF & Reliability Auditor
**Date:** 2026-05-15
**Scope:** `backend/**/*.go`, `docker-compose.yml`, `backend/Dockerfile`, `cmd/server/main.go`
**Mode:** Read-only. No code modified.

---

## TL;DR (report-back snapshot)

| Question | Answer |
|---|---|
| Critical-severity findings | **5** |
| Biggest single SPOF | **No graceful shutdown + no per-query context timeouts**: any SIGTERM during a long DB query truncates in-flight requests, and any slow DB query holds a goroutine forever, exhausting the 25-conn pool. |
| Graceful shutdown wired? | **No** — `cmd/server/main.go:224` uses `router.Run(":"+port)` and `cmd/server/main.go:217` uses `srv.ListenAndServeTLS(...)`. Neither installs `signal.Notify` for SIGTERM/SIGINT, neither calls `srv.Shutdown(ctx)`. No `defer db.Close()` is reachable on signal (only on normal return, which never happens). |
| DB pool tuning? | **Partial** — `SetMaxOpenConns(25)` and `SetMaxIdleConns(5)` are set (`internal/repository/db.go:32-33`, `:44-45`). **`SetConnMaxLifetime` and `SetConnMaxIdleTime` are NOT set** → connections live forever, breaking failover and silent NAT drops. |
| HTTP server timeouts? | **Partial** — only `ReadHeaderTimeout` is set (`cmd/server/main.go:215`, `:286`); `ReadTimeout`, `WriteTimeout`, `IdleTimeout` are **all absent**. The plaintext path uses `router.Run` (line 224) which gives the default `http.Server` with **no timeouts at all** → Slowloris-trivial. |

---

## 1. Methodology

I scanned the following categories across the listed scope:

### 1.1 Database layer SPOFs
- Verified `database/sql` connection pool tuning: confirmed `SetMaxOpenConns` / `SetMaxIdleConns` in `internal/repository/db.go:32-55`. Searched for `SetConnMaxLifetime` / `SetConnMaxIdleTime` — **zero matches**.
- Counted DB call sites in the repository layer: **182** `Query`/`QueryRow`/`Exec` calls (non-test). Counted context-aware variants (`QueryContext`/`QueryRowContext`/`ExecContext`): **0**. Every DB call hangs on a slow query until the OS times out the socket.
- Reviewed all repositories under `internal/repository/` for transaction usage: only `migrate.go:77` uses `db.Begin()`. Multi-step writes such as `executePromotion` (`internal/service/promotion_service.go:272-367`) and `RotateProjectKey` (`internal/service/keyrotation_service.go:36-108`) are **not wrapped in a transaction** despite mutating multiple tables across many secrets.
- Looked for N+1 patterns in hot paths: confirmed `KeyRotationService.RotateProjectKey` and `PromotionService.executePromotion` both do "list envs → for env { list secrets → for secret { read + write } }" with one `UPDATE`/`Upsert` per secret.

### 1.2 External-call SPOFs
- All HTTP clients audited via `grep`: `internal/service/webhook_service.go:55`, `internal/service/ai_provider.go:125,185,243,301`, `internal/crypto/keyprovider/vault.go:41`, `cmd/keepsave/main.go:37`. All have explicit `Timeout`, **but none use a per-request context**, so a hung TLS handshake during connect still blocks for the full timeout (10-120s) per goroutine.
- Vault provider (`internal/crypto/keyprovider/vault.go:54-92`) is called once at startup with a 15s ctx (`cmd/server/main.go:53`). **No retry** — if Vault is degraded at start, the pod crash-loops and pollutes the deployment.
- KMS providers (`internal/crypto/keyprovider/kms_aws.go`, `kms_gcp.go`) — interfaces only; main.go bails out with "not implemented" error at `cmd/server/main.go:247-248`, so production cannot today use AWS/GCP KMS without a custom build.
- Webhook delivery retries 3× with backoff (`webhook_service.go:153-156`) but **the request is reused across retries**: `req.Body` is a `bytes.NewReader` — only readable once. Retries 2 and 3 send empty bodies. Found at `webhook_service.go:136-170`.

### 1.3 Process lifecycle
- Searched for `signal.Notify`, `SIGTERM`, `SIGINT`, `srv.Shutdown` across all `backend/**/*.go`: **zero matches**. The only safety net is `defer db.Close()` at `cmd/server/main.go:43`, which never fires because the listener call (line 217 or 224) blocks until `os.Exit(1)`.
- Identified background goroutines that need shutdown awareness:
  - `cmd/server/main.go:196` — `startAuditLogPruner`, infinite ticker, no quit channel.
  - `cmd/server/main.go:209` — `startHTTPRedirect`, blocks forever on `ListenAndServe`.
  - `internal/api/ratelimit.go:37` — `cleanupLoop`, infinite ticker, no quit channel.
  - `internal/events/eventbus.go:91-94` — handler goroutines fired with no `WaitGroup` or context.
  - `internal/service/webhook_service.go:113` — `go ws.deliver(event, config)` — one new goroutine per webhook *per event*, no bound.
  - `internal/plugins/plugin.go:179` — `NotifyAll` spawns one goroutine per plugin per event, no bound.
  - `internal/api/handlers_mcp.go:47,163` — `go h.builderService.BuildServer/RebuildServer` — kicks off git clone + `npm install` / `pip install` / `go build` in an unbounded background goroutine triggered by API call.
- Docker compose runs a single replica of `api` (no `deploy.replicas` set) and there is no leader-election logic for the in-process schedulers (audit pruner, rate-limiter cleanup), so today single-replica is the only safe topology.

### 1.4 Capacity / unbounded resources
- `internal/api/ratelimit.go:14` — `clients map[string]*bucket`. Cleanup runs every 5 min and only removes entries idle > 5 min. Burst attacker with rotating IPs can grow the map without bound between cleanups.
- `internal/service/webhook_service.go:38` — `eventLog []WebhookDelivery` is capped at 1000 (`webhook_service.go:191-193`) — OK.
- `internal/tracing/tracing.go:38` — `maxSpans = 10000`. Reasonable cap but no enforcement code is shown in the slice append path; needs verification.
- `internal/metrics/metrics.go` — counters indexed by label key strings. If labels include high-cardinality values (user IDs, project IDs, paths with UUIDs), the map grows unbounded. Need to audit which labels are emitted by `GinMiddleware`.
- `internal/api/router.go:32` — `RequestSizeLimitMiddleware(1 << 20)` = 1 MiB. OK for JSON, **but `env-import` endpoint** (`pm.POST("/env-import")` at router.go:104) is also capped at 1 MiB — likely too small for some .env imports and the cap silently truncates bodies (returns `request body too large` from custom reader — see `security_headers.go:46-48`).
- No `ReadTimeout`/`IdleTimeout` on HTTP server → Slowloris. Confirmed `cmd/server/main.go:211-216` only sets `ReadHeaderTimeout`.

### 1.5 Observability gaps
- `/healthz` (liveness) at `internal/api/health.go:26` — never fails. Good for k8s.
- `/readyz` (readiness) at `internal/api/health.go:35` — does `db.Ping()` with **no timeout context**. If the DB is slow but not down, `/readyz` hangs and k8s eventually marks pod unhealthy. Should be `PingContext(ctxWithTimeout)`.
- `/readyz` leaks DB error details in `error` field (`health.go:41`) — operational nuisance, but also a small information-leak surface.
- Only **4** structured log calls (`logger.Error/Info/Warn`) across all of `internal/**`. Hot paths (handlers, services) do not log on error — they wrap with `fmt.Errorf` and return. The middleware logger at `internal/logging/gin_middleware.go` is the only request-level logging. There is no service-layer structured logging for failed DB writes, failed crypto operations, or failed audit appends.
- `log.Printf` used directly in `internal/repository/migrate.go:112` — bypasses the structured logger.

### 1.6 Config hazards
- `MASTER_KEY` is validated at startup (`internal/config/config.go:62-74`) when `KEEPSAVE_KEY_PROVIDER=env` — good.
- KMS/Vault credentials are NOT validated at startup beyond a non-empty check; only at `resolveMasterKey` time which is in `main.go:54`. If `KEEPSAVE_KEY_PROVIDER=vault` is set but `VAULT_TOKEN` is missing, the error fires at startup — fail-fast. OK.
- `docker-compose.yml:25` ships `MASTER_KEY: 43uH/WMSJGjGgaJseq39Mt0h5eAoGgElK3k53ddRZMM=` and `JWT_SECRET: dev-jwt-secret-change-me` as defaults — fine for dev, but the compose file is the source of truth for many local quickstarts and there's no separate `.env.example`. Less of a SPOF, more of an ops-hazard.
- Trusted-proxy middleware at `internal/api/middleware.go:16-42` **unconditionally trusts** `X-Real-IP` and `X-Forwarded-For` from any source. If KeepSave is exposed without a stripping reverse proxy, the rate-limiter and audit log will record attacker-controlled IPs. The middleware does not check `gin.SetTrustedProxies`.

---

## 2. SPOF inventory

| # | Component | Failure mode | Blast radius | Current mitigation | Severity | Suggested hardening |
|---|---|---|---|---|---|---|
| 1 | HTTP server lifecycle (`cmd/server/main.go:217,224`) | SIGTERM → in-flight requests killed mid-write; `defer db.Close()` never runs | All connected clients see truncated responses; in-flight DB transactions in `executePromotion` / `RotateProjectKey` left half-committed (no tx wrap) | None | **critical** | Use `srv.Shutdown(ctx)` with `signal.NotifyContext(SIGTERM,SIGINT)` and a 30s drain; same for redirect listener |
| 2 | DB connection pool — no `SetConnMaxLifetime` (`internal/repository/db.go:32-55`) | Idle connection silently killed by NAT/PgBouncer/RDS → next query returns `driver: bad connection`; pool fills with dead conns | Cascading 500s; in some configs (PgBouncer transaction pooling) the pool deadlocks until process restart | None | **critical** | `db.SetConnMaxLifetime(30*time.Minute)`, `db.SetConnMaxIdleTime(5*time.Minute)` |
| 3 | DB calls have **no context** (`internal/repository/**` — 182 sites) | A slow query (lock contention, full table scan) holds a goroutine and a pool slot indefinitely | One slow tenant exhausts the 25-conn pool → app-wide outage with `/readyz` still 200 until ping itself queues | None | **critical** | Migrate all repos to `*Context` variants; thread `c.Request.Context()` from handlers; add per-query 5s timeout middleware |
| 4 | No graceful shutdown of background goroutines (`cmd/server/main.go:196,209`; `ratelimit.go:37`) | On SIGTERM the goroutines abandon work mid-flight; audit-log pruner can leave a half-deleted batch | Data integrity issues if the pruner is mid-DELETE; missed cleanup on restart loops | `defer db.Close()` (not reachable) | **critical** | Pass a context to each goroutine, select on `ctx.Done()` |
| 5 | Slowloris on HTTP server (`cmd/server/main.go:211-216` and `:224`) | A handful of slow-reading clients each tying up a goroutine starve the server; plaintext path has **zero** timeouts (default `http.Server` from `router.Run`) | Trivial DoS — 1k connections of `GET /readyz` with 1-byte/min reads tie up all of Gin's per-conn workers | `ReadHeaderTimeout` only on TLS path | **critical** | Set `ReadTimeout`, `WriteTimeout`, `IdleTimeout` on both `srv` instances; eliminate `router.Run` and use explicit `&http.Server{}` for both TLS and plain paths |
| 6 | Webhook retry resends empty body (`webhook_service.go:136-170`) | After 1st failure, retry 2 & 3 POST an empty body; remote integration sees a malformed event and treats it as success (200) or rejects (4xx) | Webhook integrations silently lose data on transient downstream failures | Retry count = 3, exponential backoff | **high** | Re-build `req.Body` each iteration (e.g., `req.Body = io.NopCloser(bytes.NewReader(payload))` per attempt) or use `req.GetBody` |
| 7 | Webhook unbounded goroutine spawn (`webhook_service.go:113`) | Notify spawns one goroutine per config per event with no semaphore | Event storm × N configs = goroutine explosion; each holds a 10s HTTP client timeout | `http.Client.Timeout = 10s` per call | **high** | Bounded worker pool or buffered channel + worker goroutines; drop or queue on overflow |
| 8 | Event bus fire-and-forget handlers (`events/eventbus.go:91-95`) | Handler goroutine panics → Recovery in Gin doesn't catch it because it's outside the request | Process crash on any handler panic; lost subscribers across restart | None — no panic recovery, no retry, no DLQ | **high** | Wrap with `defer recover()`; persist failed handler results; expose `/platform/events/replay` (already present but reactive only) |
| 9 | MCP build runs unbounded subprocess from API call (`handlers_mcp.go:47,163`; `mcp_builder_service.go:111,144`) | `git clone`, `npm install`, `pip install`, `go build` run with no timeout, no concurrency limit, no resource cap | Any user with MCP scope can pin CPU/disk by registering many large repos; build dir `/tmp/keepsave-mcp-builds` grows unbounded | None | **high** | `exec.CommandContext` with 5-min timeout, per-user concurrency cap, disk quota check, dedicated builder pod |
| 10 | `/readyz` ping has no timeout (`internal/api/health.go:36`) | If DB is slow but reachable, `Ping()` may take >30s | `/readyz` blocks the k8s probe thread; probe times out and k8s misclassifies pod | None | **high** | `PingContext(ctxWithTimeout(2*time.Second))` |
| 11 | Single API replica + in-process state (`docker-compose.yml`, `ratelimit.go`, `events/eventbus.go:40`) | Rate limiter and event-bus handlers live in process memory; scaling to N replicas: each gets its own bucket map → 100rps limit becomes Nx100rps; subscribers attached on one pod don't see events on another | When ops scales horizontally, security control silently loosens | None | **medium** | Move rate-limiter state to Redis (or accept per-pod limits and document; halve the per-pod limit) |
| 12 | Audit pruner errors swallowed silently (`cmd/server/main.go:309-313`) | Pruner logs and continues. If DB is read-only during DR, pruner runs anyway — gets failed deletes, but no alerting metric is incremented | Operator unaware that retention is silently failing | Error log only | **medium** | Increment a Prometheus counter `audit_pruner_failures_total`; fail readyz after N consecutive failures |
| 13 | Trusted-proxy middleware accepts unverified headers (`middleware.go:16-42`) | Without a verified upstream stripping proxy, `X-Real-IP` is attacker-controlled | Rate-limit bypass; falsified audit-log IPs | Sets `client_ip` from headers | **medium** | Use `gin.SetTrustedProxies` with allowlist from env var; if empty, fall back to `c.RemoteAddr` only |
| 14 | Unbounded request rate-limiter map (`ratelimit.go:14,37`) | Attacker rotates source IPs; map grows up to 5-min cleanup interval | Memory pressure under churn (10k IPs/sec × 300s = 3M entries) | 5-min sweep | **medium** | Cap map size with LRU; reject when full |
| 15 | `executePromotion` not transactional (`promotion_service.go:272-367`) | Crash or DB blip between Upserts leaves target env partially promoted | Inconsistent prod environment; manual reconciliation required | Audit-log entry only; rollback is best-effort via snapshots | **medium** | Wrap copy in `sql.Tx`; commit/rollback atomically with snapshot writes |
| 16 | `RotateProjectKey` not transactional (`keyrotation_service.go:36-108`) | Crash mid-rotation → some secrets re-encrypted with new DEK, others still on old; project's DEK pointer is updated last but secrets that already succeeded are wedged | Partial corruption requiring manual `VerifyProjectEncryption` and re-rotation | Verify endpoint available (`/verify-encryption` at router.go:99) | **medium** | Single transaction per project; or implement two-phase (write all encrypted-with-new alongside old, swap pointer, drop old) |
| 17 | `gin.Recovery()` is the only panic guard (`router.go:29`) | Panics in goroutines started by handlers (`go h.builderService...`) crash the process | A poisoned MCP repo → process crash → restart loop | None outside Gin | **medium** | Wrap every `go func()` with `defer recover()` |
| 18 | Webhook delivery records appended without bound under contention (`webhook_service.go:182`) | The lock is held while appending; high-rate notify under contention serializes all webhook deliveries' record-keeping | Lock contention slows webhook delivery throughput | Slice capped at 1000 entries | **low** | Use a ring buffer with copy-out; lock only the writer index |
| 19 | Plaintext HTTP fallback uses `router.Run` (`cmd/server/main.go:224`) | Gin's `Run` creates a default `http.Server` with no timeouts at all (worse than the TLS path) | Slowloris on plain port; can't tune any limit | None | **low** (plain mode shouldn't be used in prod) | Replace with explicit `&http.Server{}` with all timeouts |
| 20 | MCP gateway runs arbitrary user-defined entry commands (`handlers_mcp_gateway.go:332`) | `cmd.Output()` with no timeout, no cgroup, no ulimit | DoS or RCE-via-config for any user with MCP scope | None | **low** (auth-gated) | `exec.CommandContext` with hard timeout; restrict to allowlist of binaries; container sandbox |
| 21 | Migrations on every startup (`migrate.go`, called from `main.go:47`) | Long migration on rolling deploy holds startup latency; if it fails, pod crashes and gets cycled by k8s | Restart loops on any bad migration; no separate migrate job | Idempotent check via `schema_migrations` | **low** | Provide a separate `migrate` subcommand and a Helm `Job`; decouple from boot |
| 22 | Master-key env-var stored in plain compose (`docker-compose.yml:25`) | Developer copies values into production by accident | Confidentiality loss | docs say "change me" | **info** | Reject `dev-jwt-secret-change-me` literal at boot when `KEEPSAVE_ENV=production` |

---

## 3. Reliability gap analysis (by category)

### 3.1 Database layer

**Critical — No per-query context:**
- `internal/repository/secret_repo.go:25,51,63,88,113,125,155,178` — all calls use `db.Query`/`db.QueryRow`/`db.Exec`, never `*Context`.
- Same pattern in every repo: `oauth_repo.go:46,61,75,128,163,177`; `audit_repo.go:26,41,71`; `apikey_repo.go`; `promotion_repo.go:34,51,62,71,84,141`; `mcp_repo.go:47,141,162,197,233,259,268`; etc.
- 182 call sites total. **Zero** `Context`-variant calls.
- Impact: a single slow query holds one of the 25 conns until the OS TCP timeout, which on Linux is ~15 minutes by default. 25 slow queries → entire app wedged with `/readyz` still returning 200 (until the ping itself queues).

**Critical — No `SetConnMaxLifetime` / `SetConnMaxIdleTime`:**
- `internal/repository/db.go:32-55` sets only max-open and max-idle.
- Without lifetime caps, conns sit idle indefinitely. RDS/PgBouncer/cloud LBs will silently kill them; next query returns `driver: bad connection`. The Go sql pool *does* retry once, but if the dead conn count exceeds 1, the retry hits another dead conn → repeated failures and the pool quickly fills with dead conns.

**High — Multi-row writes not transactional:**
- `internal/service/promotion_service.go:307-349` (`executePromotion`) — loops over `srcSecrets` and does `Upsert` per key. Each Upsert is its own auto-commit transaction. If the process dies (OOM kill, SIGTERM) between iteration 50 of 100, target env is half-promoted.
- `internal/service/keyrotation_service.go:69-101` (`RotateProjectKey`) — same shape. Partial state is recoverable via `VerifyProjectEncryption` but requires operator action.
- Mitigation present: `auditRepo.Create` emits a `promotion_completed` only after the loop. Reasonable, but doesn't make the data state consistent.

**Medium — N+1 in rotation:**
- `keyrotation_service.go:69-95` — for each env, list secrets, decrypt per-row, write per-row. A project with 1000 secrets and 3 envs = 3000 round-trips. With 25 conns and a 10ms per round-trip, this still pegs one conn for ~30s.
- Better: bulk read + `COPY` / batched writes.

**Low — `migrate.go:53` checks each migration's existence one at a time:**
- Acceptable since list-of-migrations is small. Mentioned only for completeness.

### 3.2 External-call resilience

**High — Webhook retry sends empty body:**
- `webhook_service.go:136-170` builds the `http.Request` once and reuses it. `req.Body` is a `bytes.Reader`; first `Do` consumes it. Retries 2 and 3 send `Content-Length: 0`. Plus the signature header is still set for the full payload → downstream verification fails on retries.
- Combined with the fire-and-forget goroutine spawn (`webhook_service.go:113`), failed deliveries are silently mis-retried.

**High — Vault startup has no retry:**
- `cmd/server/main.go:53-59` calls `resolveMasterKey` once with a 15s timeout. If Vault is having a hiccup, pod crashes. K8s will restart, eventually succeed — but during the gap, the deployment has 0 healthy replicas.
- Recommend: exponential backoff with cap (e.g., 5 attempts, 1s/2s/4s/8s/16s) before bailing.

**Medium — AI provider HTTP clients use plain `http.Client.Timeout` only:**
- `internal/service/ai_provider.go:125,185,243,301`. Single timeout but no `Transport.DialContext` timeout, no `TLSHandshakeTimeout`. A misbehaving upstream (TCP accepts, never replies) hits the wall at full timeout, blocking the calling goroutine. Recommend a tuned `http.Transport` per provider.

**Medium — Ollama Available() check makes a blocking HTTP call (`ai_provider.go:306-313`):**
- Called at every Chat dispatch (`ai_provider.go:77-83`). If Ollama is slow, every NLP query waits up to 120s just to find out Ollama is down. Should cache the result with TTL.

**Info — `cmd.Output()` with no context in MCP gateway (`handlers_mcp_gateway.go:339`):**
- A subprocess that hangs holds the request goroutine. Use `exec.CommandContext`.

### 3.3 Process lifecycle

**Critical — No graceful shutdown:**
- `cmd/server/main.go:217` (TLS) and `:224` (plain) both block on `ListenAndServe`. Process can only exit via `os.Exit(1)` or signal-induced kill.
- Standard pattern (`signal.NotifyContext(SIGTERM,SIGINT)` → `srv.Shutdown(ctx)` → close DB) is missing entirely.
- In k8s, when the deployment rolls, pods receive SIGTERM and get killed after `terminationGracePeriodSeconds` (default 30s). Any in-flight write that exceeds that — including a 200-secret promotion to PROD — is truncated mid-transaction. Combined with the no-transaction finding above, this means **a PROD rollover during a PROD promotion can corrupt the target env**.

**High — Background goroutines do not honour shutdown:**
- `cmd/server/main.go:196` `startAuditLogPruner` — loops on `ticker.C` forever. If `repo.DeleteOlderThan` is mid-execution when SIGTERM arrives, it's killed mid-DELETE.
- `cmd/server/main.go:209` `startHTTPRedirect` — never returns.
- `internal/api/ratelimit.go:41-54` `cleanupLoop` — never returns.

**Medium — Single-replica assumption baked in:**
- `internal/api/ratelimit.go:14` — in-process map of clients. 2 replicas → effective rate doubles silently.
- `internal/events/eventbus.go:40` — handler map per-process. Subscribers registered on pod A won't fire on pod B.
- `internal/service/webhook_service.go:36` — same per-process `configs` map. Webhooks registered via API on pod A are invisible to pod B (configs are not loaded from DB on startup).

This last one is particularly bad — `webhook_service.go` only persists nothing; `RegisterWebhook` is purely in-memory. If the pod restarts, all webhook subscriptions are lost. Cross-reference `handlers_webhook.go:Register` to confirm.

### 3.4 Capacity & unbounded resources

**Critical — No request body timeout / Slowloris:**
- `cmd/server/main.go:211-216` sets only `ReadHeaderTimeout: 10s` on TLS path. Once headers are read, the body can stream for ever.
- `cmd/server/main.go:224` (`router.Run`) creates the default Gin server with **no timeouts at all**.
- Even a single connection holding `Content-Length: 1048576` and writing 1 byte / 10s can hold a goroutine for ~3 hours; 1000 such connections wedges the server.

**Medium — Unbounded goroutine spawns per request:**
- `handlers_mcp.go:47,163` — every `RegisterServer` / `RebuildServer` API call spawns a builder goroutine. A user with API access can flood the box with `npm install` jobs.
- `webhook_service.go:113` — per-event per-config goroutine.

**Medium — Rate-limit map unbounded between sweeps:**
- `ratelimit.go:14` — no cap. Worst case (rotating-IP DDoS over a 5-minute window): millions of entries.

**Low — MCP build dir grows without GC:**
- `mcp_builder_service.go:24` — `/tmp/keepsave-mcp-builds`. Only cleaned by `RebuildServer` (`mcp_builder_service.go:93`) or explicit `CleanupBuild` (`:103`). No periodic sweep for orphaned dirs.

### 3.5 Observability gaps

**High — `/readyz` lacks a query timeout:**
- `internal/api/health.go:36` — `h.db.Ping()`. Use `PingContext(ctx)` with 2s timeout so k8s probes complete in bounded time.

**Medium — Almost no service-layer structured logging:**
- Only 4 calls to `logger.Error/Info/Warn` across `backend/internal/`. Hot paths return errors wrapped with `fmt.Errorf("ctx: %w", err)` and rely on the request middleware to log them. This means:
  - Background goroutine errors (webhook deliveries, audit pruner, event-bus handlers) are silently swallowed.
  - Crypto failures are not logged at the source.
  - The `executePromotion` partial-failure case logs nothing.

**Medium — `log.Printf` in migrations bypasses structured logger:**
- `migrate.go:112` — uses stdlib `log`. Output line "Applied migration: ..." isn't JSON, so log aggregators may not parse it.

**Low — Metrics endpoint has no authentication:**
- `router.go:47` — `r.GET("/metrics", metricsHandler.Metrics)` is on the unauthenticated router. Confirm whether this is intentional (Prometheus scrape from within cluster) or a leak. Not strictly a SPOF, but operationally relevant.

### 3.6 Config hazards

**Medium — KMS providers refuse to load:**
- `cmd/server/main.go:247-248` — AWS/GCP KMS returns "requires the SDK adapter" error. Operators choosing `KEEPSAVE_KEY_PROVIDER=awskms` get a startup crash with a docs link. This is a fail-fast (good) but means there is no working KMS path despite ADR-0004 suggesting one. Worth flagging that the "expected" prod posture is unrealised today.

**Low — Trusted-proxy headers honoured unconditionally:**
- `middleware.go:18-27`. Without a stripping proxy, `X-Real-IP` is attacker-set; combined with the per-IP rate-limit (`ratelimit.go`), it's a trivial bypass. Gin's `engine.SetTrustedProxies` is never called.

---

## 4. Top recommendations (priority order)

### Recommendation 1 — Wire graceful shutdown
**Fix lands in:** `cmd/server/main.go:198-228`

Wrap the listener in `srv.Shutdown(ctx)` triggered by `signal.NotifyContext(SIGTERM, SIGINT)`. Cascade context cancellation to:
- `db.Close()` (currently at `:43`)
- `startAuditLogPruner` (pass `ctx` and select on `ctx.Done()` in the for-range-ticker loop)
- `startHTTPRedirect` (use the same pattern)
- `RateLimiter.cleanupLoop` (`internal/api/ratelimit.go:41-54`)
- All `go ws.deliver(...)` and `go h(event)` calls (wrap in a `sync.WaitGroup` owned by their service)

Target drain time: 30s (matches default k8s grace period).

**Why first:** Without this, every other reliability fix is undermined by mid-flight truncation on every deploy.

### Recommendation 2 — Add full HTTP server timeouts
**Fix lands in:** `cmd/server/main.go:211-228`

Replace `router.Run(":"+cfg.Port)` (`:224`) with an explicit `&http.Server{}`, and on both that server and the TLS server (`:211-216`) set:
- `ReadTimeout: 15 * time.Second`
- `ReadHeaderTimeout: 10 * time.Second` (already set on TLS path)
- `WriteTimeout: 30 * time.Second` (longer than slowest expected response)
- `IdleTimeout: 60 * time.Second`
- `MaxHeaderBytes: 1 << 14` (16 KiB)

**Why second:** Closes a trivial DoS vector that's exploitable today.

### Recommendation 3 — Thread `context.Context` through every DB call
**Fix lands in:** all of `internal/repository/*.go` (182 call sites), `internal/service/*.go`, and add a middleware in `internal/api/router.go` that injects a 5s deadline.

Steps:
1. Change repository method signatures to accept `ctx context.Context` as the first arg.
2. Change `db.Query/QueryRow/Exec` to `*Context` variants.
3. Plumb `c.Request.Context()` from each Gin handler through the service layer.
4. Add a Gin middleware that wraps `c.Request = c.Request.WithContext(ctxWithTimeout)`.

**Why third:** This is large but unlocks bounded latency, proper request cancellation, and correct behaviour under shutdown. Combined with Rec 1, in-flight work actually finishes during drain.

### Recommendation 4 — Set DB pool lifetimes
**Fix lands in:** `internal/repository/db.go:32-55`

Add to each branch (postgres, mysql, sqlite):
```go
db.SetConnMaxLifetime(30 * time.Minute)
db.SetConnMaxIdleTime(5 * time.Minute)
```

**Why fourth:** Trivial one-line fix that prevents the "all 25 conns are dead and the pool deadlocks at 3 AM" failure mode mentioned in RUNBOOK §3 (DB failover).

### Recommendation 5 — Fix the webhook retry payload + bound the goroutines
**Fix lands in:** `internal/service/webhook_service.go:113,129-177`

- Rebuild the request body inside the retry loop (`bytes.NewReader(payload)` per attempt), or use `req.GetBody`.
- Replace `go ws.deliver(event, config)` with submission to a bounded worker pool (e.g., a buffered channel + N=8 workers, drop or 429 on overflow).
- Persist webhook config to DB on register so multi-replica works (separate concern but co-located fix).

**Why fifth:** Restores correctness of an existing security/operational signal (promotion notifications) and removes one of two known goroutine-explosion vectors.

---

## 5. What's already good

Several reliability patterns *are* in place and should be preserved through any refactor.

- **TLS cipher selection and HSTS** — `cmd/server/main.go:256-275` enforces TLS 1.2+, lets ops pin cipher suites, and sets HSTS at `internal/api/security_headers.go:21`.
- **Master-key validation at startup** — `internal/config/config.go:62-74` requires `MASTER_KEY` is exactly 32 bytes when `KEEPSAVE_KEY_PROVIDER=env`. Fail-fast is the correct posture.
- **Production config guards** — `internal/config/config.go:82-87` rejects `CORS_ORIGINS=*` and `sslmode=disable` in production.
- **Docker-compose healthchecks** — both `db` (`docker-compose.yml:12-17`) and `api` (`:32-37`) define proper health probes and `depends_on.condition: service_healthy`. The dependency ordering is correct.
- **Request size limit** — `internal/api/router.go:32` caps request body to 1 MiB via `RequestSizeLimitMiddleware` (`internal/api/security_headers.go:28-69`).
- **Rate limiting** — `internal/api/router.go:37` (100 req/s burst 200) is applied globally before auth, so unauthenticated floods are bounded.
- **Webhook signature** — HMAC-SHA256 of the body with per-config secret (`webhook_service.go:147-149`). Downstream can verify.
- **Webhook delivery log capped** — `webhook_service.go:191-193` keeps at most 1000 entries.
- **Audit-log retention pruner** — `cmd/server/main.go:298-331` cleans the table on a 24h schedule with explicit retention from env var.
- **Health/readiness endpoints exist** — `internal/api/health.go:26-51` and routed at `internal/api/router.go:45-46`. They need a `PingContext` retrofit but the wiring is there.
- **Structured JSON logger** — `internal/logging/logger.go` produces single-line JSON entries, lock-protected. Gin middleware integration at `internal/logging/gin_middleware.go`.
- **Metrics + tracing wired into Gin** — `internal/api/router.go:38-43` registers `metrics.GinMiddleware` and `tracing.GinMiddleware` globally. Endpoint at `/metrics` (`:47`).
- **Dialect-aware migrations with transactional apply** — `internal/repository/migrate.go:77-110` wraps each migration in a `tx`; statement parsing handles quoted strings and comments. Per-dialect subdir resolution at `:14-21`.
- **Recovery middleware** — `internal/api/router.go:29` catches Gin handler panics (does NOT catch goroutine panics — see finding #17).
- **Auth ordering** — security middleware sits before auth middleware (`router.go:30-43` → group-level `Use(JWTAuthMiddleware(...))` ), so size limits / rate limit are applied to unauth traffic.
- **PROD promotion approval requirement** — `internal/service/promotion_service.go:187-196` separates request from execution for `prod` target, providing a meaningful approval gate.
- **API key expiry honoured** — `internal/api/middleware.go:101-105`.

These patterns are the bones of a defensible service. The gaps above are mostly about *bounded latency, shutdown, and pool lifetime* — none of which require redesign; they require ~200 lines of plumbing changes.

---

## Appendix A — Files inspected

- `/home/user/KeepSave/backend/cmd/server/main.go` — entry point, server lifecycle
- `/home/user/KeepSave/backend/cmd/keepsave/main.go` — CLI client (used for HTTP client patterns)
- `/home/user/KeepSave/backend/internal/repository/db.go` — DB pool config
- `/home/user/KeepSave/backend/internal/repository/migrate.go` — migration runner
- `/home/user/KeepSave/backend/internal/repository/secret_repo.go` — representative repo
- `/home/user/KeepSave/backend/internal/repository/audit_repo.go` — audit + retention pruner
- `/home/user/KeepSave/backend/internal/api/router.go` — routes, middleware order
- `/home/user/KeepSave/backend/internal/api/middleware.go` — TrustedProxy, JWT, APIKey
- `/home/user/KeepSave/backend/internal/api/security_headers.go` — security headers, body limit
- `/home/user/KeepSave/backend/internal/api/ratelimit.go` — rate limiter, cleanup loop
- `/home/user/KeepSave/backend/internal/api/health.go` — health/readiness
- `/home/user/KeepSave/backend/internal/api/handlers_mcp.go` — async build goroutine launchers
- `/home/user/KeepSave/backend/internal/api/handlers_mcp_gateway.go` — subprocess exec
- `/home/user/KeepSave/backend/internal/service/secret_service.go` — secret crypto path
- `/home/user/KeepSave/backend/internal/service/promotion_service.go` — multi-write promotion
- `/home/user/KeepSave/backend/internal/service/keyrotation_service.go` — N+1 + non-tx rotation
- `/home/user/KeepSave/backend/internal/service/webhook_service.go` — retry, goroutine, in-memory state
- `/home/user/KeepSave/backend/internal/service/lease_service.go` — DB call patterns
- `/home/user/KeepSave/backend/internal/service/anomaly_service.go` — query patterns
- `/home/user/KeepSave/backend/internal/service/ai_provider.go` — external HTTP clients
- `/home/user/KeepSave/backend/internal/service/mcp_builder_service.go` — exec.Command, no timeout
- `/home/user/KeepSave/backend/internal/crypto/keyprovider/vault.go` — Vault HTTP client
- `/home/user/KeepSave/backend/internal/crypto/keyprovider/kms_aws.go` — KMS interface only
- `/home/user/KeepSave/backend/internal/events/eventbus.go` — fire-and-forget goroutines
- `/home/user/KeepSave/backend/internal/plugins/plugin.go` — NotifyAll goroutines
- `/home/user/KeepSave/backend/internal/logging/logger.go` — structured logger
- `/home/user/KeepSave/backend/internal/tracing/tracing.go` — trace span buffer
- `/home/user/KeepSave/backend/internal/metrics/metrics.go` — metrics types
- `/home/user/KeepSave/backend/internal/config/config.go` — startup config validation
- `/home/user/KeepSave/backend/Dockerfile` — container build
- `/home/user/KeepSave/docker-compose.yml` — local dev topology
- `/home/user/KeepSave/docs/RUNBOOK.md` — operational context (read for failure-mode framing only)
