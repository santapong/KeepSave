# KeepSave Deployment Readiness Audit — 2026-05-18

**Audit team:** Security, Backend, Frontend, Infra/Deploy, Governance (5 parallel reviewers)
**Scope:** Readiness to deploy KeepSave to **UAT on Vercel (frontend) + Neon (Postgres)** with a separate-hosted backend, and to scale to production.
**Baseline:** `docs/THREAT_MODEL.md` v1.2.x, `docs/audits/SECURITY_AUDIT_2026-05-15.md`, `docs/FOLLOWUPS.md` Phase A.
**Status:** Report-only. No code changes proposed in this document — fixes are tracked in `docs/FOLLOWUPS.md` and the ADRs listed below.
**Companion docs:** `docs/DEPLOYMENT_PLAN.md` (target architecture + cutover runbook), `docs/adr/0016-deployment-topology.md` (proposed).

---

## 1. Executive verdict

| Question | Answer |
|---|---|
| Can the frontend deploy to Vercel? | **Yes.** Vite SPA + embed widget are a natural fit. Three config changes required (see Frontend §). |
| Can Neon host the database for UAT and PROD? | **Yes** for both. Neon's branch databases map well to per-env / per-PR previews. Connection lifetime tuning required. |
| Can Vercel host the **backend** for production? | **No.** KeepSave's Gin server is long-running and stateful (audit-log pruner, webhook dispatcher, event bus, background goroutines). Vercel's serverless Go runtime is per-request with hard timeouts (10s Hobby / 60s Pro / 300s Enterprise). A rewrite to fit serverless is not a deploy task — it is a re-architecture and would invalidate ADR-0011 (graceful shutdown). **Recommendation: backend lives on a container platform forever; only the frontend goes on Vercel.** |
| Can we proceed to UAT cutover today? | **No.** 8 BLOCKER and 7 HIGH findings — all already tracked in `FOLLOWUPS.md` Phase A — gate the cutover. See §6 Pre-cutover gates. |

---

## 2. Findings by domain

The five auditors produced overlapping evidence on a handful of issues. The table below consolidates by ID; duplicates from a different lens are noted in *Cross-refs*.

### 2.1 Security & crypto (Security Engineer)

| ID | Sev | Area | File:Line | Required change | Cross-refs |
|---|---|---|---|---|---|
| **S-B1** | BLOCKER | Audit trail | `internal/service/{secret,project,apikey}_service.go` and matching handlers | Emit audit events on all Create/Update/Delete paths per `docs/AUDIT_LOG_COVERAGE.md`. Test must assert audit row. | B-B2 (same gap from backend lens); FOLLOWUPS #0 |
| **S-B2** | BLOCKER | IDOR | `internal/api/handlers_{secret,envfile,promotion,keyrotation,enterprise,dependency}.go` | Implement `requireProjectAccess(c, projectID)` middleware. Any authenticated user can currently CRUD any project's secrets. | ADR-0005 (proposed); FOLLOWUPS #0a |
| **S-B3** | BLOCKER | API-key scope | `internal/api/middleware.go:107-112` + 50+ handlers | Middleware *sets* `api_key_project_id` / `api_key_environment` but handlers ignore them. A scoped `ks_` key can access any project. Enforce inside `requireProjectAccess`. | ADR-0005 |
| **S-B4** | BLOCKER | Plaintext leak | `internal/models/models.go:142-150`, `internal/service/promotion_service.go:131,140` | `/projects/:id/promote/diff` returns plaintext values in JSON. Replace `SourceValue`/`TargetValue` with per-project HMAC hashes. | THREAT_MODEL §A01 |
| **S-B5** | BLOCKER | RCE | `internal/api/handlers_mcp_gateway.go:327-339`, `handlers_mcp.go:39` | MCP gateway `exec.Command()` runs user-registered entry commands with decrypted secrets in env. Allowlist binaries, reject shell metachars, sandbox per ADR-0010. | ADR-0010 (proposed) |
| **S-B6** | BLOCKER | Embed origin | `frontend/src/embed/auth.ts:21-26, 33` | `postMessage` listener accepts auth from any origin; outbound targets `'*'`. Implement per-project allow-list per `docs/EMBED_ORIGIN_POLICY.md`. | ADR-0006 |
| **S-B7** | BLOCKER (prod) | KMS not wired | `cmd/server/main.go:250-251` | AWS/GCP KMS adapters return "not implemented". Acceptable to use Vault provider in UAT *if* runbook flags it as dev-only. PROD must have real KMS. | ADR-0012; FOLLOWUPS #1 |
| **S-B8** | BLOCKER (prod) | Dev MASTER_KEY committed | `docker-compose.yml:25` | Key `43uH/WMSJG…` is in git history — treat as permanently leaked. Add startup check that rejects this key when `KEEPSAVE_ENV=production`. | — |
| **S-H1** | HIGH | Weak JWT secret allowed | `internal/config/config.go:76-79` | Reject `JWT_SECRET` shorter than 32 bytes when `KEEPSAVE_ENV=production`. | — |
| **S-H2** | HIGH | SSRF | `internal/service/webhook_service.go:136,158` | Pre-flight DNS resolve; reject RFC1918, loopback, `169.254.0.0/16`, k8s service IPs. HTTPS-only. | ADR-0013 |
| **S-H3** | HIGH | Approver = requester | `internal/service/promotion_service.go:215-240` | Enforce app-level check + DB CHECK constraint. | ADR-0007 |
| **S-H4** | HIGH | PKCE not S256 | `internal/service/oauth_service.go:109-121,249-255` | Public clients must use S256; drop `plain` from metadata. | — |
| **S-H5** | HIGH | Error leak | `internal/api/handlers_*.go` | Adopt `docs/ERROR_HANDLING_STANDARD.md`; never return `err.Error()` to clients. | B-B1 (same finding from backend lens) |
| **S-H6** | HIGH | Neon pool config | `internal/repository/db.go` | Neon terminates idle connections at ~5min. Set `ConnMaxLifetime` and `ConnMaxIdleTime`. Use Neon's pgbouncer endpoint if using branches. | B-H1 |
| **S-H7** | HIGH | No per-account lockout | `internal/service/auth_service.go:64-75` | Add per-email failure counter; soft-lock after 10 failures. | — |
| **S-M1..M8** | MEDIUM | Various (cascade-delete, query-log redaction, neg-auth tests, no API-key expiry, CORS default, audit-log tamper, user enumeration, root Dockerfile) | See full security report | Hardening — do before GA, not strictly before UAT cutover. | ADR-0009 (key expiry) |
| **S-L1..L5** | LOW | `MustGet` footgun, dead CSRF middleware, dev-key reuse risk, TLS topology, `lucide-react` version oddity | — | Polish, post-UAT. | — |

### 2.2 Backend / API container readiness (Backend Engineer)

| ID | Sev | Area | File:Line | Required change |
|---|---|---|---|---|
| **B-B1** | BLOCKER | Error responses | `internal/api/handlers_*.go` (160+ `RespondError(c, code, err.Error())` sites) | Adopt `httperror` package per CLAUDE.md. Lint-enforce in CI. *(Same as S-H5; backend lens confirms scope.)* |
| **B-B2** | BLOCKER | Audit-log emits missing | `internal/service/{secret,project,apikey}_service.go` | Wire `auditRepo` dependency; emit canonical events. *(Same as S-B1.)* |
| **B-B3** | BLOCKER | No graceful shutdown | `cmd/server/main.go` | Add SIGTERM/SIGINT handler + `http.Server.Shutdown(ctx)` with timeout. Without it, rolling deploys drop in-flight requests. | ADR-0011 |
| **B-H1** | HIGH | DB pool unconfigured | `internal/repository/db.go:32-45` | `SetConnMaxLifetime(5*time.Minute)` and `SetConnMaxIdleTime(2*time.Minute)`. *(Same as S-H6.)* |
| **B-H2** | HIGH | Version mismatch | `health.go:30,49` reports `0.5.0`; `cmd/server/main.go:27` reports `1.1.0` | Read version from a single constant or env var. Health endpoints lie to ops today. |
| **B-H3** | HIGH | Migration story unclear | `internal/repository/migrate.go` + `migrations/{postgres,mysql,sqlite}/` | Postgres path is canonical; MySQL/SQLite trees are partial. Either complete or remove, with an ADR. |
| **B-M1** | MEDIUM | Panic recovery | `internal/api/router.go:30` | Gin.Recovery() only logs; add middleware that returns structured 500. |
| **B-M2** | MEDIUM | Body double-read | `internal/metrics/middleware.go`, `internal/logging/gin_middleware.go` | Confirm body buffering; Gin bodies auto-consume. |
| **B-L1** | LOW | DB pool metrics | `internal/metrics/metrics.go` | Export `db.Stats()` as Prometheus gauges for alerting. |
| **B-L2** | LOW | Pruner goroutine leak | `cmd/server/main.go:199,329-333` | Add stop channel; cancel on SIGTERM. |

**Backend verdict:** ~80% container-ready. After B-B1/B2/B3 land, the binary is production-shaped for any container platform.

### 2.3 Frontend & embed (Frontend Engineer)

| ID | Sev | Area | File:Line | Required change |
|---|---|---|---|---|
| **F-B1** | BLOCKER | API base URL hardcoded | `frontend/src/api/client.ts:24` (`const BASE_URL = '/api/v1'`) | Read `import.meta.env.VITE_API_BASE_URL` with `/api/v1` fallback. Set the env var in Vercel project settings per environment. |
| **F-H1** | HIGH | nginx.conf not portable | `frontend/nginx.conf` | Vercel handles SPA fallback + gzip + caching natively. Drop the nginx layer for Vercel deploys. Backend serves `/healthz`, `/readyz`, `/metrics`, `/.well-known/` from its own origin. |
| **F-H2** | HIGH | `vercel.json` absent | (new file) | See template in `docs/DEPLOYMENT_PLAN.md` §4.2. |
| **F-H3** | HIGH | CORS for cross-origin | Backend CORS middleware | Vercel frontend is a different origin from the backend. Backend must set `Access-Control-Allow-Origin` to the Vercel project URL + custom domain, allow `Authorization` header, no credentials cookies needed (bearer tokens only). |
| **F-M1** | MEDIUM | Embed widget `api-url` attr required | `frontend/src/embed/keepsave-widget.ts:75-77` | Fallback to `window.location.origin` will be wrong on Vercel. Integrators must set `api-url` explicitly. Document this. |
| **F-M2** | MEDIUM | Build output split | `package.json:8-9` | `npm run build` → `dist/` (SPA, ships to Vercel). `npm run build:widget` → `dist-embed/` (separate distribution, e.g. npm package or CDN — out of Vercel scope). Document. |
| **F-L1** | LOW | TS `baseUrl` deprecated in 5.8 | `frontend/tsconfig.json:19` | Add `"ignoreDeprecations": "6.0"`. |

**Frontend verdict:** Vercel-ready after ~2–4h of config work.

### 2.4 Infra / deploy (Platform Engineer)

| Concern | Finding | Action |
|---|---|---|
| Vercel for backend | Vercel Go runtime is serverless (10s/60s/300s timeouts). KeepSave has a 24h-interval audit-log pruner goroutine (`main.go:199`) plus webhook service and event bus — **incompatible.** | Backend deploys to a container platform; Vercel hosts frontend only. |
| Migration runner | Runs inside Go binary at startup (`main.go:47`). | Works on Neon — first container of a deploy applies migrations transactionally. Document that schema-breaking releases need a maintenance window (ADR-0011 + a deploy guide entry). |
| Dockerfile (backend) | Multi-stage, CGO off, alpine base, no `USER` directive. | Add `USER keepsave` (also S-M8). |
| Dockerfile (frontend) | nginx-based; obsolete if Vercel hosts frontend. | Keep for self-hosted path; mark as alternative in `helm/`. |
| docker-compose | Dev-only secrets baked in (`MASTER_KEY=43uH…`, `JWT_SECRET=dev-…`, `CORS_ORIGINS=*`, `sslmode=disable`). | None for compose itself — but CI/CD must reject these strings in any non-dev env (S-B8). |
| Helm chart | Exists (`helm/keepsave/`); production-shaped (KMS hooks, autoscaling, TLS toggle). | This is the production scaling path. |
| GitHub Actions | `ci.yml` lints/tests/builds Docker images on main. | Add step: push backend image to registry (GHCR / GAR / ECR) on tag, trigger backend platform deploy. Vercel deploys automatically via its GitHub app. |

**Backend hosting comparison (UAT → PROD):**

| Option | UAT fit | PROD fit | Notes |
|---|---|---|---|
| **Vercel** | Frontend only | Frontend only | Not viable for the Gin backend — no path without a re-architecture. |
| **Fly.io** | ✓ Excellent (free tier, regional) | ✓ Good up to mid-scale; native Postgres or Neon | Recommended UAT default. Multi-region, persistent volumes, fast deploys, no cold start. |
| **Railway** | ✓ Excellent (GitHub-native, simple) | ≈ OK up to mid-scale | Easiest devex; pay-as-you-go. Pricing climbs at scale. |
| **Google Cloud Run** | ✓ Generous free tier | ✓ Strongest PROD scale option; native KMS aligns with ADR-0012 | Cold start 1–2s; min-instances=1 removes that. Best fit when KMS = GCP KMS. |
| **AWS App Runner / ECS Fargate** | ≈ Heavier setup | ✓ Strong with AWS KMS | Pick if KMS = AWS KMS. |
| **EKS / GKE via existing Helm chart** | Overkill for UAT | ✓ Best for "we already run K8s" orgs | Helm chart is real; use this when ops budget exists. |

### 2.5 Governance (Tech Lead)

| Gate | Status today | Required for UAT |
|---|---|---|
| ADR-0005 (project-access middleware) | Proposed | Accepted + implemented |
| ADR-0006 (embed origin allow-list) | Proposed; partially landed per PR #50 | Verify enforcement matrix in tests |
| ADR-0010 (MCP exec hardening) | Proposed | Accepted + implemented |
| ADR-0011 (graceful shutdown + DB timeouts) | Proposed | Accepted + implemented |
| ADR-0012 (KMS auto-unseal) | Proposed | Accepted; KMS provider wired (UAT may use Vault provider with runbook caveat) |
| ADR-0013 (webhook SSRF guard) | Proposed | Accepted + implemented |
| **ADR-0016 (deployment topology)** | **Missing — proposed in this PR** | Accepted before cutover |
| Security Engineer sign-off (per CLAUDE.md §Type-1) | Not recorded on 0005, 0009, 0013, 0016 | Recorded |
| `docs/SECRET_SOURCES.md` updated | Out of date for Neon + chosen backend platform | Updated in same PR as ADR-0016 acceptance |
| `docs/RUNBOOK.md` cutover checklist | No UAT cutover section yet | Added |

**Governance verdict:** Phase A items already track every blocker. The deployment-shaped work this PR adds is **ADR-0016 + the cutover plan**, not new engineering scope.

---

## 3. Cross-cutting themes

1. **Two auditors flagged the same audit-trail gap** (S-B1 / B-B2) and the same error-handling gap (S-H5 / B-B1). Both are CLAUDE.md-mandated and already in `FOLLOWUPS.md` Phase A item #0. Fix once, satisfy two reviewers.
2. **Connection lifetime for Neon** (S-H6 / B-H1) is a 4-line change with severe failure mode if missed (cascading 500s every 5 minutes). Highest fix-effort-to-impact ratio.
3. **Vercel/Neon discovery is asymmetric.** Vercel adoption requires only frontend config + a `VITE_API_BASE_URL`. Neon adoption requires DB pool tuning + `sslmode=require` + a migration story for branch databases. The frontend is easy; the data tier is where the careful work lives.
4. **The dev `MASTER_KEY` in `docker-compose.yml` is in git history.** It is not a deploy-blocker in itself, but a leaked-credential class issue — treat it as a known IOC for any future incident.

---

## 4. Counts

| Severity | Security | Backend | Frontend | Infra | Governance | Total |
|---|---|---|---|---|---|---|
| BLOCKER | 8 | 3 | 1 | 0 | 4 gates | **8 unique** (after dedupe) |
| HIGH | 7 | 3 | 3 | — | — | **10 unique** |
| MEDIUM | 8 | 2 | 2 | — | — | 12 |
| LOW | 5 | 2 | 1 | — | — | 8 |

---

## 5. Status (this audit)

Read-only. No code or doc changes in `internal/`, `migrations/`, or `helm/` proposed here. All remediation work is tracked in `docs/FOLLOWUPS.md` Phase A and the ADR set 0005–0016.

---

## 6. Phase 1 closure (appended 2026-05-18)

Implemented per `plans/so-let-fix-everything-cozy-tide.md`. Commits on
`claude/audit-deployment-plan-moSsk` are grouped A-H.

| ID | Severity | Status | Commit group | Notes |
|---|---|---|---|---|
| S-B1 / B-B2 | BLOCKER | **Closed** | D | `emitAudit` wired into secret/project/apikey/auth services; test asserts row written |
| S-B2 | BLOCKER | **Closed** | B | `RequireProjectAccess` middleware on all `/projects/:id/*` route groups |
| S-B3 | BLOCKER | **Closed** | B | API-key scope enforced inside same middleware |
| S-B4 | BLOCKER | **Closed** | D | `DiffEntry.SourceValue/TargetValue` removed; HMAC-SHA256 hash prefix substituted |
| S-B5 | BLOCKER | **Closed** | D | `validateMCPEntryCommand` allow-list + shell-metachar reject at register + exec; `cmd.Env` minimized; 30s context timeout |
| S-B6 | BLOCKER | **Closed** | already PR #50; F adds CORS test | Embed origin allow-list verified end-to-end |
| B-B1 / S-H5 | BLOCKER | **Closed** | C | 125 `err.Error()` leak sites migrated to `WrapError`; AST-walking regression test gates CI |
| B-B3 | BLOCKER | **Closed** | A | `signal.Notify` → `srv.Shutdown(ctx)` with 30s grace; pruner takes ctx |
| F-B1 | BLOCKER | **Closed** | F | `VITE_API_BASE_URL` fallback in `client.ts:24`; `.env.example` documents it |
| S-B7 | BLOCKER (prod) | **Partial** | D | Vault provider is supported; AWS/GCP stubs deferred per ADR-0016 |
| S-B8 | BLOCKER (prod) | **Closed** | A | `KEEPSAVE_ENV=production` rejects the leaked dev key hash |
| S-H1 | HIGH | **Closed** | A | `JWT_SECRET` < 32 bytes refused in production |
| S-H2 | HIGH | **Closed** | E | `ValidateWebhookURL` SSRF guard; tested against 169.254 / 10/8 / 127/8 / metadata.google.internal |
| S-H3 | HIGH | **Closed** | E | App-level `ErrSelfApproval` + DB CHECK constraint in migration 008 |
| S-H4 | HIGH | **Closed** | E | `verifyPKCE` accepts only `S256`; public clients must supply a challenge |
| S-H6 / B-H1 | HIGH | **Closed** | A | `ConnMaxLifetime=5m`, `ConnMaxIdleTime=2m` on Postgres + MySQL pools |
| S-H7 | HIGH | **Closed** | E | `auth_login_attempts` table; lockout after 10 failures within 15 min |
| B-H2 | HIGH | **Closed** | E | `internal/version` package; `/healthz` and `/readyz` now report 1.1.0 |
| B-H3 | HIGH | **Closed** | E | `migrations/README.md` documents the per-dialect convention; sqlite 008/009 added for parity |
| F-H1 | HIGH | **Closed** | F | `nginx.conf` header comment marks it as the Option-C self-hosted fallback |
| F-H2 | HIGH | **Closed** | F | `frontend/vercel.json` committed |
| F-H3 | HIGH | **Closed** | F | Backend CORS supports comma-list + single-glob; `Vary: Origin` emitted; no credentials cookie |

Negative-auth coverage added: `internal/api/negative_auth_test.go` (~13 cells across IDOR / API-key scope / MCP exec); `internal/service/negative_auth_test.go` (~13 cells across PKCE / lockout / self-approval / SSRF); `internal/api/cors_test.go` (6 cells); `internal/service/url_safety_test.go` (10 cells). Total ~42 cells — at the upper end of the "~30-40" budget from the plan.

ADRs flipped to Accepted under sponsor authorization (group G): 0005, 0006, 0007, 0010, 0011, 0012, 0013, 0016. ADR-0008, 0009, 0014, 0015 remain Proposed for Phase 3.

Out of scope (deferred to Phase 3 per the plan): 12 MEDIUM + 8 LOW items, AWS/GCP KMS adapters, the full 132-cell negative-auth matrix, audit-log nightly export (PROD gate G16), Helm chart updates (PROD gate G19).

Phase 2 (audit team recheck) follows next on the same branch.
