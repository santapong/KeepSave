# ADR-0016: Deployment topology — Vercel frontend, container backend, Neon Postgres

- **Status:** Proposed
- **Date:** 2026-05-18
- **Authors:** Audit team (Security, Backend, Frontend, Infra, Tech Lead — 5 reviewers; see `docs/audits/AUDIT_2026-05-18_DEPLOYMENT_READINESS.md`)
- **Reviewers required:** Tech Lead; Security Engineer; Platform Engineer
- **Supersedes:** none
- **Related:** ADR-0011 (graceful shutdown + DB timeouts), ADR-0012 (KMS auto-unseal), `docs/DEPLOYMENT_PLAN.md`, `docs/SECRET_SOURCES.md`

---

## Context

KeepSave has no recorded decision about where it runs. The repository ships:

- a long-running Gin backend (`backend/cmd/server/main.go`) with background goroutines (audit-log pruner at `main.go:199`, webhook dispatcher, event bus),
- a Vite + React frontend with a separate embeddable Web Component (`vite.embed.config.ts`),
- a `docker-compose.yml` that is dev-only (committed `MASTER_KEY`, `sslmode=disable`, `CORS_ORIGINS=*`),
- a `helm/keepsave/` chart that is production-shaped (autoscaling, KMS hooks, TLS toggle) but unbound to a specific cluster choice.

The team wants to deploy a UAT environment, with Vercel and Neon as the stated targets, and to scale to production afterwards. The question driving this ADR is *both* "where does each tier live in UAT?" *and* "is the UAT shape the same shape we run in production?"

Constraints we cannot violate:

- **Crypto continuity (Type-1, ADR-0001/0004):** the running process must hold the master key in memory at startup to decrypt project DEKs. This rules out compute that may freeze or evict its memory mid-request.
- **Audit-trail completeness (CLAUDE.md):** every state-mutating handler must emit an audit event. This requires the audit-log pruner goroutine and the same process model long-term.
- **Cross-tenant isolation (ADR-0005, proposed):** `requireProjectAccess` enforcement assumes a single multi-tenant DB. Topology must not preempt or contradict that.

Threat-model implication: splitting the frontend (Vercel) and backend (separate platform) onto different origins widens the trust boundary to include public-internet hops between them. CORS policy and TLS termination become part of the surface area. This is acceptable but must be reflected in `docs/THREAT_MODEL.md`.

## Options considered

### Option A — Frontend on Vercel; backend on a container platform (Fly.io UAT → Cloud Run or K8s PROD); Neon for the database

- **How it works:** Vite SPA + embed widget build → Vercel (preview + production deploys per PR / branch). Gin backend → container image → Fly.io machines (UAT) or Cloud Run revisions (PROD), behind HTTPS with `CORS_ORIGINS` allowlisting the Vercel URLs. PostgreSQL → Neon, sslmode=require, pooled endpoint when previews fan out branches. KMS → HashiCorp Vault dev sidecar in UAT (flagged dev-only), GCP KMS or AWS KMS in PROD.
- **Pros:**
  - Frontend gets Vercel's strengths (global CDN, preview-per-PR, automatic TLS, zero config) without forcing the backend into a serverless mold it was not built for.
  - Backend stays a normal long-running container, preserving ADR-0011, the audit-log pruner, and the webhook dispatcher.
  - Neon's branch databases compose 1-to-1 with Vercel preview deployments — a natural fit.
  - The Helm chart in `helm/keepsave/` remains the production scaling path: no architecture change to move from Cloud Run to GKE/EKS when ops budget arrives.
  - Same shape UAT and PROD; only tier size differs.
- **Cons:**
  - Two providers in the deploy story (Vercel + a container platform) increases the cognitive surface for ops.
  - Cross-origin frontend↔backend means CORS, preflight requests, and a wider trust boundary (must be reflected in `docs/THREAT_MODEL.md`).
  - Neon's idle-connection termination (~5 min) requires `ConnMaxLifetime` tuning (B-H1 / S-H6); a missed config produces cascading 500s every 5 minutes.
  - Vault dev sidecar in UAT is a real footgun if it leaks to production — requires a runbook caveat and the `KEEPSAVE_ENV=production` check (gate G10).

### Option B — Everything on Vercel (frontend + backend as Vercel Functions; Neon database)

- **How it works:** Frontend as today; backend rewritten as Vercel Go Functions, one per route. Background work (audit-log pruner, webhook dispatcher) moved to Vercel Cron or to an external worker.
- **Pros:**
  - Single provider for compute. Single deploy story. Preview-per-PR includes the API.
  - Auto-scales to zero; cheapest at low traffic.
- **Cons:**
  - **Re-architecture, not deploy.** Vercel Go Functions are per-request handlers with 10s (Hobby) / 60s (Pro) / 300s (Enterprise) timeouts; no graceful-shutdown semantics; memory between invocations is not guaranteed.
  - Background goroutines (`main.go:199` pruner, webhook service, event bus) have no home — they would split into Vercel Cron + queue, doubling the deploy surface.
  - Master-key bootstrap (ADR-0004) on every cold start is expensive and weakens the threat model (more KMS calls, more places the key briefly resides).
  - Invalidates ADR-0011 (graceful shutdown + DB timeouts) the day it lands.
  - Connection pooling on Neon becomes harder — each function instance opens its own pool.
- **Verdict:** Rejected on Type-1 grounds — invalidates ADR-0001/0004/0011 and widens crypto attack surface.

### Option C — Everything on Kubernetes from day one (frontend self-hosted via the existing nginx Dockerfile; backend via `helm/keepsave/`; Neon or in-cluster Postgres)

- **How it works:** Skip Vercel entirely. Frontend nginx container + backend Gin container both deployed to a managed Kubernetes cluster (GKE or EKS) via the existing Helm chart.
- **Pros:**
  - Single deploy substrate. The Helm chart already exists and is production-shaped.
  - No cross-origin (frontend and backend can share a domain via Ingress path rewrites).
  - Strongest control over network policy, mTLS, secret injection (ExternalSecrets / SealedSecrets).
- **Cons:**
  - High operational floor: cluster, Ingress, cert-manager, observability stack — all required before the first request.
  - No preview-per-PR equivalent without significant tooling (Argo Rollouts + per-branch namespaces, or vcluster).
  - Loses Vercel's free CDN + analytics + previews for the frontend, which are real productivity wins.
- **Verdict:** Right answer eventually for K8s shops, wrong answer for "first UAT in this quarter". Option A leaves this door open — moving the backend from Cloud Run/Fly to the Helm chart is a deploy-target swap, not a re-architecture.

## Decision

**Option A.** Frontend on Vercel, backend on a container platform (Fly.io for UAT, Cloud Run as the default PROD recommendation with K8s-via-Helm reserved as the scale-out option), database on Neon, KMS via Vault dev sidecar for UAT only and cloud KMS for PROD.

This option wins because it (a) preserves the backend's process model and the threat-model assumptions it rests on (no re-architecture), (b) gives the frontend the best-fit hosting it has, (c) keeps the production scaling path identity-shaped to UAT (same containers, same DB, only bigger), and (d) does not preempt the eventual K8s-via-Helm choice when ops budget exists.

Conditional: if the org adopts GCP as the single cloud before UAT cutover, replace Fly.io with Cloud Run from day one — that removes the Vault-dev-sidecar caveat (KMS = GCP KMS immediately) without changing the rest of the shape.

## Rejection rationale

- **Option B (Vercel for backend):** rejected because it requires re-architecting the backend out of its current process model, invalidates ADR-0011, and widens the crypto attack surface — none of which is justified by a deploy goal.
- **Option C (K8s from day one):** rejected for the first cutover because the operational floor is higher than the team's current ops capacity. Reserved as the production scaling target when a managed cluster is already paid for.

## Consequences

**Operational:**

- Two deploy systems to learn: Vercel (frontend, auto via GitHub app) and the backend platform CLI (`fly deploy` for UAT; `gcloud run deploy` or `helm upgrade` for PROD).
- `docs/RUNBOOK.md` gains a `Cutover` section per `docs/DEPLOYMENT_PLAN.md` §4.
- A `vercel.json` is added to `frontend/` (template in `DEPLOYMENT_PLAN.md` §4.2).
- A new `VITE_API_BASE_URL` env var convention is introduced and must be honored in `frontend/src/api/client.ts:24`.
- The frontend Dockerfile + `nginx.conf` are retained for Option C as the self-hosted alternative; not removed.

**Security:**

- New trust boundary: public-internet hop from Vercel frontend to backend origin. `docs/THREAT_MODEL.md` to be updated with: CORS policy as a controlled surface; bearer-token only (no cross-site cookies); preflight requests not credential-bearing.
- Dev `MASTER_KEY` in `docker-compose.yml:25` is treated as permanently leaked; `KEEPSAVE_ENV=production` gates a startup check that refuses it (S-B8).
- UAT KMS via Vault dev sidecar is a transitional risk — runbook MUST flag it as dev-only and the path to PROD KMS must be in `docs/SECRET_SOURCES.md` before UAT goes live (G13).

**Migration:**

- No data migration. New environments are net-new Neon projects with the existing migration set applied on first backend boot.
- `frontend/src/api/client.ts:24` change is the only required code edit for Vercel adoption.
- `internal/repository/db.go` connection-pool tuning (B-H1) lands as part of ADR-0011 follow-up; this ADR is a prerequisite for the UAT cutover, not a separate work item.

**Reversibility:**

- Reversible. Either tier can be re-platformed without changing the other:
  - Frontend off Vercel → rebuild from the existing nginx Dockerfile, deploy alongside backend.
  - Backend off Cloud Run/Fly → `helm install keepsave ./helm/keepsave` on any K8s cluster.
- The DB choice (Neon) is the stickiest — moving off Neon requires a Postgres dump/restore but no schema changes (standard Postgres wire protocol).

## Rollback plan

If this topology proves wrong in UAT (e.g. CORS/cross-origin pain dominates, or Vercel preview limits bite):

1. **Frontend regression:** rebuild via existing `frontend/Dockerfile` and deploy to the same container platform that hosts the backend, under a shared origin. nginx config already proxies `/api/` → backend.
2. **Backend regression:** the chosen container platform (Fly.io / Cloud Run) is a deploy-target swap. The Helm chart is the most general escape hatch — `helm install` on any K8s.
3. **Neon regression:** Neon export → restore to managed Postgres (RDS, Cloud SQL, Crunchy Bridge). Same `lib/pq` driver; only the DSN changes.

The decision is reversible at each tier independently. There is no single point that requires a coordinated migration to undo.

## Open questions

| # | Question | Owner | Due |
|---|---|---|---|
| 1 | Which cloud KMS in PROD — GCP KMS or AWS KMS? Drives whether the backend lives on Cloud Run or App Runner. | Platform + Security | Before PROD cutover |
| 2 | Do we want preview-per-PR backends too (e.g. Fly Apps per PR), or only frontend previews against a shared UAT backend? | Tech Lead | Before UAT cutover |
| 3 | Audit-log nightly export destination — GCS Bucket Lock vs. S3 Object Lock — depends on Q1. | Platform | Before PROD cutover (G16) |
| 4 | When does the K8s/Helm path become the primary PROD target rather than the fallback? Defer to operational-readiness review at the 6-month mark post-PROD cutover. | Tech Lead | 2026-Q4 |

## References

- `backend/cmd/server/main.go` — long-running process model, line 199 (pruner goroutine)
- `backend/internal/repository/db.go` — DB pool config site (B-H1 lands here)
- `frontend/src/api/client.ts:24` — `VITE_API_BASE_URL` change site (F-B1)
- `frontend/nginx.conf` — retained for self-hosted (Option C) path
- `helm/keepsave/` — production scaling path
- `docs/audits/AUDIT_2026-05-18_DEPLOYMENT_READINESS.md` — 5-reviewer audit underpinning this ADR
- `docs/DEPLOYMENT_PLAN.md` — cutover runbook
- ADR-0001, ADR-0004 (crypto continuity), ADR-0011 (graceful shutdown), ADR-0012 (KMS)
