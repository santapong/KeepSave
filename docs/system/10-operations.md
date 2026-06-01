# Operations

> Part of the **[KeepSave System Documentation](./README.md)**.

This chapter is the operator's view of KeepSave: on-call incident procedures, key-rotation cadences, the known crash risks and single-points-of-failure surfaced by the reliability audits, the graceful-shutdown / DB-timeout behaviour of the running process, and what to watch in production. It summarizes the canonical runbook and audit docs inline and links them for the full procedures. Throughout, **implemented behaviour is distinguished from drills and aspirational/ADR-decided-but-not-yet-wired work**, because several hardening items are decided in ADRs but only partially in code.

For how the service is packaged and deployed see [infrastructure](./09-infrastructure.md); for the crypto/auth properties an incident must protect see [security](./05-security.md).

---

## 1. Incident procedures (on-call)

Severity taxonomy from [RUNBOOK](../RUNBOOK.md): **P0** = data loss or production outage; **P1** = security incident with contained blast radius; **P2** = degraded service. Each procedure below is a two-sentence summary — **read [RUNBOOK](../RUNBOOK.md) for the full step-by-step**, which is the source of truth.

| # | Scenario | Sev | Summary | Status |
|---|----------|-----|---------|--------|
| 1 | **Lost master key** | P0 | Symptom: startup fails or all decrypts return `message authentication failed`. Put the service read-only, roll the KMS key alias back to the last-known-good version, confirm DEK unwrap against a sample project; if truly lost, restore from the most recent encrypted backup (which carries its own DEK wrapped by the old master) — post-mortem mandatory. | Procedure documented; backup-restore relies on backup tooling |
| 2 | **Compromised API key** | P1 | Symptom: anomaly service raises `UnusualKeyAccess` or a leak is reported. Within 5 min: revoke the key (`keepsave api-key revoke`), revoke dependent leases, and rotate any secrets the key could read (`POST /rotate-keys`); then export the key's audit window and notify owners via webhooks. | Revoke + rotate implemented; lease-revoke + webhook-notify partial |
| 3 | **Database failover** | P0 | Symptom: `/readyz` returns 503, logs show `driver: bad connection`. Confirm the primary is down, promote a replica, update the `DATABASE_URL` secret and `kubectl rollout restart` the backend, then verify the migration version row before resuming traffic. | Procedure documented; pool-lifetime tuning (§4) reduces the dead-conn variant |
| 4 | **Rotate-everything drill** | P2 | Quarterly exercise proving the rotation plumbing works end-to-end: rotate the master key (new KMS data key), roll pods one at a time, rotate per-project keys, rotate API keys older than 90 days, rotate the JWT secret, and verify a fresh backup restores on staging. | **Drill** (quarterly) |
| 5 | **govulncheck regression** | P2 | Symptom: CI `security-scan` fails on a previously-green branch. Review the report artifact (90-day retention); if the vuln is *called*, pin/upgrade in the same PR; if *not called*, annotate `errorlog.md` and open a Dependabot PR for the next cycle (do not merge without team agreement). | Implemented gate (blocking CI job) |
| 6 | **Deploy rollback drill** | P2 (drill) | Quarterly: capture the deployed image digest, deploy a deliberately-broken image, confirm `/readyz` 503s within 30 s, `helm rollback` (or `kubectl rollout undo`), verify recovery and **clean state** (no stuck migrations, no half-written audit rows, no orphaned `pending` promotions). Pass criteria: recovery < 5 min using the *same* runbook procedure, not a special drill version. | **Drill** (quarterly) |
| 7 | **Break-glass production secret read** | procedure | Recovering from a P0 where an operator must read a production K8s Secret to diagnose. **Two-person operation**: page Tech Lead + Security, open a break-glass-labelled incident issue *before* the read, read via an audited bastion (`kubectl get secret -o yaml`), and **rotate the secret afterward** — the cloud-provider audit log is the record (KeepSave's own log may be what's being recovered). | Procedure documented |

Two additional operational notes from [RUNBOOK](../RUNBOOK.md) §7–§8 worth internalizing:

- **UAT cutover** has its own quick-reference checklist (provision Neon, generate per-env secrets with `openssl rand`, deploy backend to Fly.io, configure Vercel, smoke-test) — long-form in [DEPLOYMENT_PLAN](../DEPLOYMENT_PLAN.md) §4.
- **The dev `MASTER_KEY` is permanently leaked** (`docker-compose.yml`). It is in git history and on every contributor's machine; if it ever reaches a non-dev environment, treat that environment as compromised and rotate immediately. The `KEEPSAVE_ENV=production` startup check refuses it by SHA-256 hash (`backend/internal/config/config.go`) — the check is the safety net, not the policy.

---

## 2. Key-rotation cadence

Rotation cadences and drill cadences are owned by [SECRET_SOURCES](../SECRET_SOURCES.md):

| Secret | Rotation cadence | Drill cadence |
|--------|------------------|---------------|
| `MASTER_KEY` | on suspicion of compromise; otherwise annual | quarterly table-top; semi-annual live (staging) |
| `JWT_SECRET` | 90 days; immediate on incident (forces re-auth) | per rotation event |
| `DATABASE_URL` | when credentials expire (≤ 90 days with policy) | per rotation event |
| OAuth client secrets | per-provider; minimum annual | per rotation event |
| TLS private key | cert-manager automatic (~60 days) | watch monitoring; alert if not renewed |

Operational facts that bound rotation:

- **`POST /api/v1/projects/:id/rotate-keys`** rotates a project's DEK and re-encrypts that project's secrets (see [security](./05-security.md) / `internal/service/keyrotation_service.go`). The reliability audit notes this rotation is **not yet wrapped in a single transaction** (`docs/audits/BACKEND_SPOF.md` #16), so a crash mid-rotation can leave some secrets on the old DEK; `/verify-encryption` exists to detect partial state and the rotation can be re-run.
- **DEK rotation API and master-key in-process rotation are not fully built.** The production `MasterKeyProvider` must be Vault/KMS (not `EnvProvider`, whose `Rotate()` returns `ErrUnsupported`), so any rotate-everything drill against `EnvProvider` fails by design. Tracked as `FOLLOWUPS` #1 and #3.

---

## 3. Known crash risks & single-points-of-failure

Two read-only audits (2026-05-15) catalog the reliability surface. **Both describe the pre-hardening state**; some findings have since been addressed (§4), others remain open and ADR-decided. Read them for the full tables.

### 3.1 Crash / panic risks — [BACKEND_CRASH_RISKS](../audits/BACKEND_CRASH_RISKS.md)

35 findings (6 critical, 7 medium, 4 low, 18 info). The dominant class is **`goroutine-no-recover`**: `gin.Recovery()` (`internal/api/router.go`) catches panics on the request goroutine and returns HTTP 500, but **goroutines spawned from handlers do not inherit that recovery** and a panic there takes the whole process down. The critical sites are the MCP build goroutines (`handlers_mcp.go`), the webhook delivery goroutine (`webhook_service.go`), and the event-bus / plugin-notify goroutines (latent — no production subscribers today). The audit's headline remediation is a one-line `defer recover()` on every `go` statement plus a `getUserID(c)` helper to replace the (then) 58 `MustGet("user_id").(uuid.UUID)` panic sites.

**Status:** the `getUserID` helper landed (61 of 62 sites migrated, `internal/api/project_access.go`, per [FOLLOWUPS](../FOLLOWUPS.md) §0l). The shared `safego.Launch` goroutine-recover harness specified by [ADR-0010](../adr/0010-mcp-gateway-command-execution-hardening.md) part C / [ADR-0011](../adr/0011-graceful-shutdown-and-db-timeouts.md) part 5 is **decided but not yet in the tree** (no `backend/internal/runtime/` package exists), so the goroutine-no-recover risk class is still open for the MCP/webhook/event-bus paths.

### 3.2 SPOF & reliability — [BACKEND_SPOF](../audits/BACKEND_SPOF.md)

5 critical findings. The biggest SPOF as originally found was the combination of **no graceful shutdown** plus **no per-query context timeouts**: a SIGTERM during a long DB query truncated in-flight requests, and a single slow query held a goroutine and a pool slot until the OS socket timeout (~15 min on Linux), so 25 slow queries could wedge the whole 25-connection pool. Other criticals: missing DB pool-lifetime caps (dead idle conns after failover), no `Read/Write/IdleTimeout` (Slowloris), and unbounded background-goroutine spawn.

**Status (what landed vs. what is still open):**

| Finding | Status |
|---------|--------|
| Graceful shutdown (signal handling, drain, `db.Close()`) | **Landed** — `cmd/server/main.go` now uses `signal.Notify(SIGINT, SIGTERM)` + `srv.Shutdown(ctx)` with a 30 s grace period |
| DB pool lifetimes (`SetConnMaxLifetime`/`SetConnMaxIdleTime`) | **Landed** — `internal/repository/db.go` (postgres + mysql branches) |
| DB pool gauges at `/metrics` | **Landed** — `keepsave_db_open/in_use/idle_connections` polled by `metrics.StartDBPoolUpdater` |
| HTTP `ReadHeaderTimeout` | **Landed** (10 s); full `Read/Write/IdleTimeout` per ADR-0011 part 3 are decided but only `ReadHeaderTimeout` is set today |
| Per-query `context` plumbing (182 call sites → `*Context`) | **Not yet** — repositories still use bare `db.Query/Exec`; decided in [ADR-0011](../adr/0011-graceful-shutdown-and-db-timeouts.md) part 2 |
| `/readyz` timeout (`PingContext`) | **Not yet** — `internal/api/health.go` still calls `db.Ping()` without a context |
| Webhook retry empty-body bug + SSRF guard | SSRF guard **landed at registration** (`url_safety.go`); body-buffered retries + delivery-time re-resolve decided in [ADR-0013](../adr/0013-webhook-emission-with-ssrf-guard.md) |
| MCP build budget / interpreter allowlist | Exec hardening **partially landed** (`exec.CommandContext`, scrubbed env, metacharacter reject); interpreter allowlist + build-concurrency budget decided in [ADR-0010](../adr/0010-mcp-gateway-command-execution-hardening.md) |

The audit also reaffirms the **single-replica assumptions**: the in-process rate-limiter map and event-bus/webhook subscriber maps are per-process, so scaling the backend horizontally silently changes those behaviours (rate limits multiply per replica; in-memory webhook registrations are not shared). Until those move to shared state, document the per-pod limit or run a single replica for those controls.

---

## 4. Graceful shutdown & DB timeouts ([ADR-0011](../adr/0011-graceful-shutdown-and-db-timeouts.md))

The running process (`backend/cmd/server/main.go`) implements an orderly lifecycle:

1. **Boot:** load + validate config → connect DB → run migrations → resolve master key (15 s context) → build crypto/JWT services → wire repos/services/handlers → start the HTTP server in a goroutine.
2. **Background workers** share a `context.Context` cancelled at shutdown: the **audit-log pruner** (`startAuditLogPruner`, deletes `audit_log` rows older than `AUDIT_LOG_RETENTION_DAYS` on boot then every 24 h) and the **DB pool gauge updater** (`metrics.StartDBPoolUpdater`, every 15 s). When TLS redirect is enabled, an HTTP→HTTPS redirect listener on `:80` is also context-bound.
3. **Shutdown:** `signal.Notify` catches `SIGINT`/`SIGTERM`; the server runs `srv.Shutdown(ctx)` with a **30 s grace period** (`shutdownGracePeriod`, matching k8s `terminationGracePeriodSeconds`), then cancels the background context. `defer db.Close()` runs on the normal return path.

**DB pool** (`internal/repository/db.go`): Postgres/MySQL pools are tuned to `MaxOpenConns=25`, `MaxIdleConns=5`, plus `ConnMaxLifetime`/`ConnMaxIdleTime` (the ADR-0011 part-4 fix) so Neon/PgBouncer idle eviction doesn't leave dead connections — this is what closes the §3.2 failover failure mode. SQLite is single-connection.

**Caveat (be precise as an operator):** the ADR specifies more than has landed. Per-query context timeouts (part 2), full HTTP `Read/Write/IdleTimeout` (part 3), and the `safego` helper (part 5) are **decided but not yet wired**. So today a *slow* query is not bounded by a per-request deadline, and the audit pruner's `prune()` is not yet wrapped in a panic-recover. Plan around the implemented subset.

---

## 5. Observability in operations

KeepSave exposes Prometheus metrics at the **unauthenticated** `/metrics` endpoint (intended for in-cluster scrape), alongside `/healthz` (liveness) and `/readyz` (readiness). The metric set is defined in `backend/internal/metrics/metrics.go`. What to watch:

| Signal | Metric / source | Alert guidance |
|--------|-----------------|----------------|
| **DB pool saturation** | `keepsave_db_in_use_connections` / `keepsave_db_open_connections` | alert when `in_use/open` > 0.8 for 5 min |
| **DB pool churn** (Neon idle eviction) | `keepsave_db_open_connections` | alert when `open` oscillates > 25% in a 1-min window — re-check `ConnMaxLifetime` |
| **Rate-limit rejections** | `keepsave_rate_limit_hits_total` | spike = abusive client or an under-sized limit (global 100 rps / burst 200) |
| **Auth failures** | `keepsave_auth_failures_total` / `keepsave_auth_attempts_total` | sustained high ratio = credential-stuffing / misconfigured client |
| **Errors / latency** | `keepsave_http_errors_total`, `keepsave_http_request_duration_seconds` | standard SLO alerting |
| **Crypto throughput** | `keepsave_secrets_encrypted_total`, `keepsave_secrets_decrypted_total` | anomalies can indicate bulk exfiltration |
| **Promotions / rotations** | `keepsave_promotions_total`, `keepsave_key_rotations_total` | unexpected PROD promotions warrant audit-log review |

> **Audit-emit failure is logged, not (yet) a metric.** When an audit write fails, `internal/service/audit_helper.go` emits `log.Printf("audit emit failed action=%s err=%v", ...)`. There is **no** `audit_emit_failures_total` counter today — operators must watch logs (or ship them to a SIEM) for that line. A dedicated metric would be a natural hardening follow-up; do not assume one exists when building alerts.

Two more operational observability gaps from [BACKEND_SPOF](../audits/BACKEND_SPOF.md) §3.5 to keep in mind: service-layer structured logging is sparse (background-goroutine errors can be swallowed), and the audit pruner increments no failure metric — a silently-failing pruner is not currently alertable. The nonce-collision-per-DEK counter (alert at 2²⁸, [ADR-0001] safety boundary) is tracked as `FOLLOWUPS` #7 and not yet wired.

Distributed tracing is wired into Gin (`internal/tracing`) and surfaced alongside metrics. Phase 7 also ships Grafana dashboard templates and alerting rules (see `Roadmap.md` Phase 7).

---

## See also

- [RUNBOOK](../RUNBOOK.md) — full incident procedures (the source of truth for §1)
- [SECRET_SOURCES](../SECRET_SOURCES.md) — rotation cadences and break-glass
- [BACKEND_CRASH_RISKS](../audits/BACKEND_CRASH_RISKS.md), [BACKEND_SPOF](../audits/BACKEND_SPOF.md) — the reliability audits
- [DEPLOYMENT_PLAN](../DEPLOYMENT_PLAN.md) — UAT/PROD cutover runbook
- [ADR-0011](../adr/0011-graceful-shutdown-and-db-timeouts.md), [ADR-0012](../adr/0012-kms-auto-unseal.md) — lifecycle and KMS decisions
- [Infrastructure](./09-infrastructure.md) — images, Helm, topology, config
- [Security](./05-security.md) — encryption / auth properties incidents protect
