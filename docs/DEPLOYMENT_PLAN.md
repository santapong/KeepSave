# KeepSave Deployment Plan — UAT on Vercel + Neon, Path to Production

**Status:** Draft for review (depends on `docs/adr/0016-deployment-topology.md` acceptance)
**Date:** 2026-05-18
**Authors:** Audit team (5 reviewers, see `docs/audits/AUDIT_2026-05-18_DEPLOYMENT_READINESS.md`)
**Reviewers required:** Tech Lead; Security Engineer; Platform Engineer
**Related:** ADR-0011 (graceful shutdown), ADR-0012 (KMS auto-unseal), `docs/SECRET_SOURCES.md`, `docs/RUNBOOK.md`

---

## 0. Quick answer

> **Can Vercel host the production backend, or do we need another provider?**

Vercel hosts the **frontend** beautifully (Vite SPA + embed widget). It **cannot host the Go/Gin backend** — not for UAT, not for production, not at any scale tier. The backend is a long-running stateful process with background goroutines (audit-log pruner, webhook dispatcher, event bus, ADR-0011 graceful shutdown). Vercel's Go runtime is serverless: per-request handlers with a hard 10s/60s/300s timeout depending on plan. Forcing the backend onto Vercel is a re-architecture, not a deploy.

**The shape we recommend, both UAT and PROD:**

```
        ┌─────────────────────────┐
Browser │ Vercel (frontend + edge)│  ──HTTPS──┐
        │   - Vite SPA            │           │
        │   - keepsave-widget.js  │           │
        └─────────────────────────┘           │
                                              │
        ┌─────────────────────────────────────▼──────────────────┐
        │ Backend container platform                              │
        │   UAT:  Fly.io   (recommended) or Railway              │
        │   PROD: Cloud Run (recommended) or Fly.io scale-up     │
        │         or EKS/GKE via helm/keepsave (when ops budget) │
        │   - Gin server, long-running                            │
        │   - audit-log pruner, webhook dispatcher                │
        └─────────────────┬──────────────────────────────────────┘
                          │
                  ┌───────▼────────┐         ┌──────────────────┐
                  │ Neon Postgres  │         │ KMS              │
                  │  UAT: branch   │         │  UAT: Vault dev  │
                  │  PROD: main +  │         │  PROD: GCP KMS   │
                  │   read replica │         │   (or AWS KMS)   │
                  └────────────────┘         └──────────────────┘
```

This shape **does not change** between UAT and PROD. Only the size and durability of each tier changes.

---

## 1. Why this shape

### 1.1 Vercel is for the frontend

| Capability | Frontend needs it? | Vercel provides? |
|---|---|---|
| Static SPA hosting + global CDN | ✓ | ✓ |
| SPA fallback (route → index.html) | ✓ | ✓ (automatic) |
| Per-PR preview environments | ✓ | ✓ (GitHub app) |
| Per-environment env vars (`VITE_API_BASE_URL`) | ✓ | ✓ |
| Automatic TLS + custom domain | ✓ | ✓ |
| Long-running process / cron / background jobs | ✗ | ✗ |

### 1.2 Vercel is not for the backend

`backend/cmd/server/main.go` runs:

- A persistent `gin.Engine` listening on `:8080`
- An audit-log pruner goroutine on a 24h interval (`main.go:199`)
- A webhook dispatcher service
- An event bus
- A graceful-shutdown handler (per ADR-0011, once implemented)

None of these are expressible as a per-request serverless function. Vercel's Go runtime invokes `func Handler(w, r)` per request and may freeze or evict between invocations. There is no path to keep the pruner running, no way to hold a connection pool across invocations on cold starts, and no way to honor a SIGTERM-driven graceful shutdown.

### 1.3 Neon is for the database

Neon is a managed Postgres service with three properties that fit KeepSave:

- **Branch databases:** every PR / preview environment can spin its own DB branch off `main`, isolating destructive migrations. This composes with Vercel preview deploys.
- **Serverless compute:** scales to zero when idle (good for UAT, fine for PROD with `min_compute=1`).
- **Standard Postgres wire protocol:** the existing `lib/pq` driver works unchanged; no SDK to adopt.

The caveats are real and must be addressed:

- **Idle-connection termination at ~5 minutes** — fix by setting `db.SetConnMaxLifetime(5*time.Minute)` (B-H1 / S-H6).
- **SSL required** — change `sslmode=disable` (dev compose) to `sslmode=require` in any deployed env.
- **Connection limits per compute tier** — if PR previews open too many connections, use Neon's pgbouncer-mode endpoint.

---

## 2. Pre-cutover gates (UAT)

**Do not deploy until each of these is closed.** All items are already tracked in `docs/FOLLOWUPS.md` Phase A or the ADR set; the gate is verification, not new scope.

| # | Gate | Owner | Tracked in |
|---|---|---|---|
| G1 | `requireProjectAccess` middleware lands and is mounted on all `/projects/:id/*` routes (fixes cross-tenant IDOR) | Backend + Security | ADR-0005, FOLLOWUPS #0a |
| G2 | API-key scope (`api_key_project_id`, `api_key_environment`) enforced by handlers | Backend + Security | ADR-0005 |
| G3 | `/promote/diff` no longer returns plaintext values; replaced with per-project HMAC hashes | Backend + Security | THREAT_MODEL §A01 |
| G4 | MCP gateway `exec.Command` is allowlisted; no shell metachars accepted | Backend + Security | ADR-0010 |
| G5 | Embed widget origin allow-list enforced per ADR-0006 (verify post-PR-#50 enforcement matrix) | Frontend + Security | ADR-0006 |
| G6 | Audit events emitted on **every** state-mutating handler (secret, project, api-key, promotion); tests assert audit rows | Backend | CLAUDE.md §Audit log; FOLLOWUPS #0 |
| G7 | Handlers stop returning `err.Error()` — `httperror` package adopted, lint enforced | Backend | `docs/ERROR_HANDLING_STANDARD.md` |
| G8 | Graceful shutdown (SIGTERM → `http.Server.Shutdown`) implemented | Backend | ADR-0011 |
| G9 | DB pool tuned for Neon (`ConnMaxLifetime=5m`, `ConnMaxIdleTime=2m`) | Backend | B-H1 |
| G10 | `KEEPSAVE_ENV=production` rejects dev MASTER_KEY hash + short JWT secrets | Backend + Platform | S-B8, S-H1 |
| G11 | Webhook SSRF guard live (block RFC1918, link-local, metadata IPs) | Backend + Security | ADR-0013 |
| G12 | Approver ≠ requester invariant (app + DB CHECK) | Backend + Security | ADR-0007 |
| G13 | KMS provider wired — for UAT, Vault provider is acceptable **only if** runbook tags this as dev-only and PROD has real GCP/AWS KMS in scope | Platform + Security | ADR-0012, FOLLOWUPS #1 |
| G14 | ADR-0016 (this topology) accepted with Tech Lead + Security sign-offs | Tech Lead + Security | `docs/adr/0016-deployment-topology.md` |

Production cutover adds:

| # | Additional PROD gate | Owner |
|---|---|---|
| G15 | Per-account login lockout shipped (S-H7) | Backend + Security |
| G16 | Audit-log nightly export to immutable store (GCS/S3) for tamper-evidence | Platform |
| G17 | Negative-auth test matrix in CI (401/403 on wrong project, expired JWT, revoked key) | Backend + QA |
| G18 | KMS rotation drill performed in staging | Platform + Security |
| G19 | Helm chart values reviewed for prod (TLS termination, autoscaling, secrets via ExternalSecrets) | Platform |

---

## 3. UAT environment

### 3.1 Tier picks

| Tier | Provider | Plan | Rationale |
|---|---|---|---|
| Frontend | **Vercel** | Pro ($20/seat/mo) | Preview deploys per PR; team collaboration; analytics. |
| Backend | **Fly.io** | Shared-CPU-1x, 256MB, 2 machines | Lowest-friction container deploy. No cold start. ~$5–10/mo per service at this size. |
| Database | **Neon** | Launch ($19/mo) or Free for solo UAT | Branch DBs match PR previews; sslmode=require by default. |
| KMS | **HashiCorp Vault (Fly Volume)** *(UAT-only)* | Dev binary on a sidecar | Bootstraps key hierarchy; runbook **must** flag this is not prod. |
| Object storage (audit-log export) | Skip in UAT | — | Defer to PROD gate G16. |

Alternative if the org standardizes on GCP early: backend on **Cloud Run** (free tier 2M req/mo) + **GCP KMS** from day one. This skips the Vault-dev sidecar — recommended if a GCP project is already available.

### 3.2 Per-environment configuration

| Variable | UAT value | Notes |
|---|---|---|
| `DATABASE_URL` | `postgres://<user>:<pass>@<neon-host>/<db>?sslmode=require&channel_binding=require` | Use Neon's **pooled** endpoint if PR previews are enabled. |
| `MASTER_KEY` | Unique per env (32 bytes, base64) — **not** the dev key | Stored in Fly secrets / Cloud Run secret; never in git. |
| `JWT_SECRET` | Unique per env, ≥ 32 bytes | Same. |
| `PORT` | `8080` | Fly maps to its internal port. |
| `CORS_ORIGINS` | Comma-separated list of Vercel URLs: `https://keepsave-uat.vercel.app,https://keepsave-uat-*-yourteam.vercel.app` | Wildcard the preview subdomain pattern; never `*`. |
| `KEEPSAVE_ENV` | `uat` (or `production` once G10 enforcement is in place — your call) | Drives the prod-only validation in `internal/config/config.go`. |
| `VITE_API_BASE_URL` (frontend) | `https://api-uat.keepsave.example/api/v1` | Set in Vercel project settings, scoped to Preview + Production. |

---

## 4. Cutover runbook (UAT)

### 4.1 Order of operations

1. **Prerequisites verified.** All G1–G14 gates closed. Tag the merge commit `uat-cutover-001`.
2. **Provision Neon.** Create project `keepsave-uat`, get pooled DSN, store in `1Password` (or your team's vault) as `keepsave/uat/database_url`.
3. **Generate per-env secrets.**
   ```bash
   openssl rand -base64 32   # MASTER_KEY
   openssl rand -base64 48   # JWT_SECRET
   ```
   Store under `keepsave/uat/master_key` and `keepsave/uat/jwt_secret`.
4. **Deploy backend to Fly.io.**
   - `fly launch --no-deploy --image-label uat-cutover-001`
   - `fly secrets set DATABASE_URL=... MASTER_KEY=... JWT_SECRET=... CORS_ORIGINS=... KEEPSAVE_ENV=uat`
   - `fly deploy --strategy rolling --wait-timeout 300`
   - Verify `/readyz` returns 200 (migrations applied transactionally on first boot).
   - Attach Fly Volume + run Vault dev sidecar **only if** KMS gate uses Vault path.
5. **Configure Vercel project.**
   - Connect GitHub repo, set **Root Directory** to `frontend`.
   - Add env var `VITE_API_BASE_URL = https://api-uat.keepsave.example/api/v1` for both Preview and Production scopes.
   - Add `vercel.json` (template below).
6. **Push deploy.** Vercel auto-builds on push; first deploy promotes to UAT URL.
7. **Smoke-test checklist.**
   - [ ] Frontend loads, no console errors.
   - [ ] Login round-trips a JWT.
   - [ ] Create project → audit row exists (verifies G6).
   - [ ] Create secret → encrypted in DB, decryptable via API (verifies KMS + crypto).
   - [ ] Cross-tenant probe: second user attempts to read first user's project → 403 (verifies G1/G2).
   - [ ] `/promote/diff` shows hashes, never plaintext (verifies G3).
   - [ ] Embed widget on a test page from a non-allowlisted origin → rejected (verifies G5).
   - [ ] SIGTERM the backend → in-flight requests complete; `/readyz` flips to 503 before exit (verifies G8).
   - [ ] Idle backend for 6 minutes → next request succeeds (verifies G9 / Neon idle-conn handling).
8. **Update `docs/RUNBOOK.md` §Cutover** with the URLs, secret references, and rollback steps.
9. **Announce.** UAT URL + smoke-test summary to the team channel.

### 4.2 `vercel.json` template

```json
{
  "buildCommand": "npm run build",
  "outputDirectory": "dist",
  "cleanUrls": true,
  "trailingSlash": false,
  "rewrites": [
    { "source": "/(.*)", "destination": "/index.html" }
  ],
  "headers": [
    {
      "source": "/assets/(.*)",
      "headers": [
        { "key": "Cache-Control", "value": "public, max-age=31536000, immutable" }
      ]
    },
    {
      "source": "/index.html",
      "headers": [
        { "key": "Cache-Control", "value": "public, max-age=0, must-revalidate" }
      ]
    }
  ]
}
```

Place at `frontend/vercel.json`. The embed widget (`npm run build:widget` → `dist-embed/`) is a separate distribution (npm package or CDN); it is **not** shipped via Vercel — Vercel builds the SPA only.

### 4.3 Frontend code change

```ts
// frontend/src/api/client.ts:24
const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api/v1';
```

The `??` fallback preserves local-dev behavior (Vite proxy still routes `/api` to `localhost:8080` per `vite.config.ts:16-20`).

### 4.4 Backend CORS change

`CORS_ORIGINS` already exists as config. Set it to the Vercel project's URLs (production + the preview wildcard pattern). Never `*` in a deployed env. The middleware must allow the `Authorization` header and not require credentials (KeepSave uses bearer tokens, not cookies, so `Access-Control-Allow-Credentials` stays `false`).

### 4.5 Rollback

- **Frontend:** Vercel → Deployments → Promote previous build. One click.
- **Backend:** `fly releases list` → `fly deploy --image registry.fly.io/keepsave-uat:<prev-tag>`.
- **Database:** Neon → Branches → restore from PITR to T-5min. Migrations run transactionally inside the binary; partial-state rollback is bounded by the migration runner.

---

## 5. Path to production

### 5.1 Architectural shape

**Same as UAT.** No new tiers. Only changes:

| Tier | UAT | PROD |
|---|---|---|
| Frontend | Vercel Pro | Vercel Pro (or Enterprise for SSO + DDoS) — same project, Production scope |
| Backend | Fly.io shared-CPU-1x × 2 machines | Cloud Run min-instances=2 (or Fly performance-2x × 3 across regions; or Helm chart on GKE/EKS for K8s shops) |
| DB | Neon Launch tier, single compute | Neon Scale tier, primary + read replica, `min_compute_units=1` to remove cold-start |
| KMS | Vault dev sidecar (UAT-only) | GCP KMS or AWS KMS, key per env, rotation drilled |
| Audit-log export | Skipped | Nightly dump to GCS/S3 (Object Lock / Bucket Lock) |
| Observability | Fly metrics + Vercel analytics | Add tracing (Cloud Trace / Honeycomb), structured-log shipping (Cloud Logging / Datadog), Prometheus scrape of `/metrics` |

### 5.2 Production backend picks

Pick **one**, decided in ADR-0016:

| Option | Pick this when… | Cost shape | Operational burden |
|---|---|---|---|
| **Cloud Run** (recommended default) | You want GCP KMS, autoscaling for free, no ops team | $0.000017/vCPU-sec + req fee | Lowest |
| **Fly.io performance tier, multi-region** | Latency from non-US regions matters; you like Fly's UX | $30–80/mo per region per machine | Low |
| **EKS/GKE via `helm/keepsave`** | The org already runs Kubernetes; you need pod-level network policies | Cluster cost + per-pod | Medium-high (worth it only if K8s is already paid for) |
| **AWS App Runner / ECS Fargate** | AWS-only org; KMS = AWS KMS | Higher per-request than Cloud Run | Low-medium |

The Helm chart in `helm/keepsave/` is real and production-shaped (autoscaling, KMS hooks, TLS toggle). Picking K8s is not a re-platform — it's a deploy target swap.

### 5.3 Production cutover gates (additive over UAT G1–G14)

See §2 gates G15–G19.

### 5.4 Cost ballpark (USD/month, order of magnitude)

| Component | UAT | PROD (modest, ~10k req/day) | PROD (busy, ~1M req/day) |
|---|---|---|---|
| Vercel | 20 (1 seat Pro) | 20 | 20+ (analytics add-ons) |
| Backend (Fly.io / Cloud Run) | 5–15 | 30–60 | 150–400 |
| Neon | 0–19 | 19–69 | 69–199 |
| KMS | 0 (Vault dev) | 1–5 | 5–20 |
| Observability | 0 | 0–50 (Datadog/Honeycomb dev tiers) | 100–500 |
| **Total** | **~$25–35** | **~$70–200** | **~$350–1100** |

Numbers are intentionally rough — they shift with provider promos and per-tier discounts. Treat as "is this in the right order of magnitude?", not as a quote.

---

## 6. What we are **not** doing

- **Not** putting the backend on Vercel. Re-confirmed in §0; no scaling tier of Vercel changes this answer.
- **Not** introducing per-customer DBs at this stage. Single multi-tenant Neon DB with `requireProjectAccess` enforcement (G1).
- **Not** building a custom KMS. Vault for UAT-only convenience; cloud KMS in PROD.
- **Not** rewriting the embed widget for Vercel. It already supports `api-url` attribute (F-M1).
- **Not** committing any prod secrets to git. All secrets live in Fly secrets / Vercel env / cloud KMS. The dev `MASTER_KEY` already in `docker-compose.yml` is treated as permanently leaked.

---

## 7. References

- `docs/audits/AUDIT_2026-05-18_DEPLOYMENT_READINESS.md` — full audit punch list (5 reviewers)
- `docs/adr/0016-deployment-topology.md` — proposed ADR for this shape
- `docs/adr/0005-require-project-access-middleware.md` — IDOR fix gating UAT
- `docs/adr/0011-graceful-shutdown-and-db-timeouts.md` — backend SIGTERM + Neon pool
- `docs/adr/0012-kms-auto-unseal.md` — production KMS path
- `docs/SECRET_SOURCES.md` — needs update (see §A.1 in audit doc)
- `docs/RUNBOOK.md` — needs `Cutover` section appended
- `docs/FOLLOWUPS.md` Phase A — every gate above is already tracked there
