# Deploying KeepSave on the Vercel Ecosystem (Three-Project Topology)

**Status:** Runbook — companion to [`DEPLOYMENT_PLAN.md`](DEPLOYMENT_PLAN.md) (which holds the full rationale and ADR-0016 topology decision).
**Scope:** KeepSave's role in the three-project deployment: **KeepSave + Grovernance-Platfrom + Cooker**, all fronted by Vercel.

## Topology recap

Vercel hosts the **frontend only**. The Go/Gin backend is a long-running stateful process (audit-log pruner, DB-pool updater, token-denylist maintainer, in-memory rate limiter) and runs on a container host — Fly.io / Railway for UAT, Cloud Run / Fly for PROD, per `DEPLOYMENT_PLAN.md` §0–1. Postgres is Neon (available through the Vercel Marketplace).

KeepSave is the **first service to bring up** in the three-project stack, because the other two depend on it:

- **Grovernance-Platfrom** verifies user/service JWTs against KeepSave's JWKS endpoint (`GET /api/v1/oauth/.well-known/jwks.json`). Signing keys are persisted in Postgres (`migrations/013_jwt_keys.sql`), so backend restarts/redeploys do not rotate them unexpectedly.
- **Cooker** uses KeepSave as its secrets backend (`COOKER_SECRETS_BACKEND=keepsave`) via a project-scoped API key, and presents a KeepSave service token with the `governance.authorize` scope when calling Grovernance's deploy gate.

## 1. Vercel project (frontend)

| Setting | Value |
|---|---|
| Root Directory | `frontend` |
| Framework | Vite |
| Build Command | `npm run build` |
| Output Directory | `dist` |
| Env var | `VITE_API_BASE_URL=https://<keepsave-api-origin>/api/v1` |

Notes:

- `frontend/vercel.json` is checked in (SPA rewrite, cache headers, CSP). The CSP `connect-src` directive contains the placeholder origin `https://api.keepsave.example` — **replace it with your real backend origin** before/at first deploy, or browser calls to the API will be blocked by CSP even if CORS is correct.
- All API clients honor `VITE_API_BASE_URL` (`frontend/src/api/client.ts` exports the resolved `BASE_URL`; `ai.ts` reuses it).
- Vercel preview deploys work out of the box: the backend CORS middleware supports wildcard origin patterns such as `https://keepsave-*-<team>.vercel.app`.

## 2. Backend (container host)

Build from `backend/Dockerfile` (distroless, non-root; it copies `migrations/` into the image — required, since migrations are read from the filesystem at boot by `internal/repository/migrate.go`). Single replica initially: the rate limiter and token-denylist cache are per-process.

Required env:

| Variable | Value / note |
|---|---|
| `DATABASE_URL` | Neon **pooled** connection string. PROD rejects `sslmode=disable`. Pool sizing in `repository/db.go` already assumes Neon-style idle eviction. |
| `MASTER_KEY` | base64 32 bytes (or `KEEPSAVE_KEY_PROVIDER=vault` + `VAULT_*`). PROD refuses the known dev key. |
| `JWT_SECRET` | strong random secret |
| `KEEPSAVE_ENV` | `production` (or `uat`) |
| `CORS_ORIGINS` | `https://<your-vercel-domain>,https://keepsave-*-<team>.vercel.app` — no `*` in prod |
| `KEEPSAVE_PLATFORM_ADMIN_EMAILS` | admin allowlist (empty ⇒ `/admin` fail-closed) |
| `PORT` | `8080` (host default usually fine) |

## 3. Outputs the other two projects consume

After the backend is up, record these values — they are inputs to the Grovernance and Cooker deployments:

| Consumer | What it needs from KeepSave |
|---|---|
| Grovernance | `GOVERNANCE_JWKS_URL=https://<keepsave-api>/api/v1/oauth/.well-known/jwks.json`, plus issuer/audience values for `GOVERNANCE_JWT_ISSUER` / `GOVERNANCE_JWT_AUDIENCE` |
| Cooker (secrets) | `COOKER_SECRETS_KEEPSAVE_URL=https://<keepsave-api>`, a KeepSave project ID and a project/environment-scoped API key |
| Cooker (deploy gate) | a KeepSave **service token** with the `governance.authorize` scope, used as `COOKER_GOVERNANCE_CALLER_TOKEN` |

## 4. Bring-up order (whole stack)

1. Neon: create the three databases (keepsave / grovernance / cooker).
2. **KeepSave backend** → verify `GET /api/v1/oauth/.well-known/jwks.json` returns keys.
3. Grovernance backend (points its JWKS URL here).
4. Cooker backend (points at both).
5. The three Vercel frontend projects; add each Vercel origin to the matching backend's CORS allowlist.
6. Smoke: log in on the Vercel frontend, create a project + secret, confirm the audit row; then verify Cooker can read a secret and Grovernance accepts a KeepSave-issued token.

## Out of scope here

Multi-replica hardening (shared rate limiting/denylist), KMS auto-unseal (ADR-0012), and provider provisioning steps — see `DEPLOYMENT_PLAN.md`.
