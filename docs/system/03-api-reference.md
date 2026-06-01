# API Reference

> Part of the **[KeepSave System Documentation](./README.md)**.

This chapter is the complete inventory of KeepSave's HTTP surface. Every route is registered in one place — `backend/internal/api/router.go` — and dispatched to a handler in `backend/internal/api/handlers_*.go`. All application endpoints live under the base path **`/api/v1`**; a small set of operational/discovery endpoints live at the root. The inventory below is derived directly from `router.go` and reconciled against the read-only [`docs/audits/API_RECONCILIATION.md`](../audits/API_RECONCILIATION.md) audit (whose headline count of 126 is now stale — the live `router.go` registers **142 routes**). For the auth model in depth see the [security model](./05-security.md); for the layering behind these handlers see [backend architecture](./02-backend.md).

---

## 1. Authentication model

Three context-setting mechanisms gate the routes; they are applied **per route group** in `router.go`, not globally.

| Mechanism | Header | Used by | Sets on context |
|-----------|--------|---------|-----------------|
| **JWT bearer** (`JWTAuthMiddleware`) | `Authorization: Bearer <jwt>` | Dashboard / human routes | `user_id`, `email` |
| **API key** (`APIKeyAuthMiddleware`) | `X-API-Key: <key>` (falls back to JWT) | Agent routes: secrets, leases, `/agent`, `/applications` | `user_id`, `api_key_project_id`, `api_key_scopes`, `api_key_environment` |
| **Public** | — | health, metrics, docs, auth register/login, OAuth token/JWKS/userinfo/revoke, public MCP listing, embed-config | — |

On top of the auth middleware, every `/projects/:id/*` route also runs **`RequireProjectAccess`** ([ADR-0005](../adr/0005-require-project-access-middleware.md)). It requires the caller to be the project **owner**, a **member of the project's organization**, or an **API key scoped to that exact project** (and, when present, the matching environment). It fails closed and returns the same response for "no access" and "does not exist" to avoid leaking project existence. The implementation is in `backend/internal/api/project_access.go`.

In the tables below the **Auth** column means:

- **Public** — no auth middleware.
- **JWT** — `JWTAuthMiddleware`.
- **API key** — `APIKeyAuthMiddleware` (API key *or* JWT).
- **+PA** — additionally guarded by `RequireProjectAccess` (all `/projects/:id/*` routes).

---

## 2. Root / operational endpoints

Registered outside the `/api/v1` group.

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/healthz` | Public | Liveness — returns `status`, `uptime`, `version`. |
| GET | `/readyz` | Public | Readiness — `Ping`s the DB; 503 if disconnected. |
| GET | `/metrics` | Public | Prometheus exposition of all `keepsave_*` metrics. |
| GET | `/api/docs` | Public | Static OpenAPI 3.0.3 spec (JSON) from `openapi.go`. |
| GET | `/.well-known/openid-configuration` | Public | OIDC discovery document for the OAuth provider. |

## 3. Auth & users

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/auth/register` | Public | Create a user (email + password ≥ 8 chars); returns a JWT. |
| POST | `/api/v1/auth/login` | Public | Authenticate; returns a JWT. Locked accounts get 429. |
| GET | `/api/v1/users/lookup?email=` | JWT | Non-enumerating lookup — returns `{found: bool}` (and the user id on hit) so probing can't confirm an email. |

## 4. Projects

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/projects` | JWT | Create a project (generates a per-project DEK). |
| GET | `/api/v1/projects` | JWT | List projects the caller can access. |
| GET | `/api/v1/projects/:id` | JWT +PA | Get one project. |
| PUT | `/api/v1/projects/:id` | JWT +PA | Update name/description. |
| DELETE | `/api/v1/projects/:id` | JWT +PA | Delete a project (cascades to envs/secrets). |
| PUT | `/api/v1/projects/:id/embed-config` | JWT +PA | Set the embed origin allow-list; rejects `["*"]` with 422. |

## 5. Secrets & versions

API-key authenticated (agent surface), plus `RequireProjectAccess`.

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/projects/:id/secrets` | API key +PA | Create a secret (`key`, `value`, `environment ∈ {alpha,uat,prod}`). |
| GET | `/api/v1/projects/:id/secrets?environment=` | API key +PA | List secrets in an environment (decrypted values). |
| GET | `/api/v1/projects/:id/secrets/:secretId` | API key +PA | Get a single secret. |
| PUT | `/api/v1/projects/:id/secrets/:secretId` | API key +PA | Update a secret's value (snapshots the previous version). |
| DELETE | `/api/v1/projects/:id/secrets/:secretId` | API key +PA | Delete a secret. |
| GET | `/api/v1/projects/:id/secrets/:secretId/versions` | API key +PA | List historical versions of a secret. |
| GET | `/api/v1/projects/:id/secrets/:secretId/versions/:version` | API key +PA | Get one historical version. |

## 6. Promotion, key rotation, webhooks, env files, dependencies (project-scoped)

All under `/api/v1/projects/:id`, JWT + `RequireProjectAccess`.

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/promote` | JWT +PA | Promote secrets between environments; PROD may require approval. |
| POST | `/promote/diff` | JWT +PA | Preview the diff (HMAC-hashed values, never plaintext). |
| GET | `/promotions` | JWT +PA | List promotion requests. |
| GET | `/promotions/:promotionId` | JWT +PA | Get one promotion request. |
| POST | `/promotions/:promotionId/approve` | JWT +PA | Approve a pending promotion (approver ≠ requester). |
| POST | `/promotions/:promotionId/reject` | JWT +PA | Reject a pending promotion. |
| POST | `/promotions/:promotionId/rollback` | JWT +PA | Roll back a completed promotion from snapshots. |
| GET | `/audit-log?limit=` | JWT +PA | Project audit-log entries. |
| POST | `/rotate-keys` | JWT +PA | Rotate the project DEK and re-encrypt its secrets. |
| GET | `/verify-encryption` | JWT +PA | Self-test: confirm every secret decrypts. |
| POST | `/webhooks` | JWT +PA | Register a webhook (SSRF-guarded). |
| GET | `/webhooks` | JWT +PA | List webhooks. |
| DELETE | `/webhooks` | JWT +PA | Remove **all** webhooks for the project (no `:webhookId` discriminator — see reconciliation note). |
| GET | `/env-export?environment=` | JWT +PA | Export secrets as `.env` content. |
| POST | `/env-import` | JWT +PA | Import secrets from `.env` content (`overwrite` flag). |
| POST | `/dependencies/analyze?environment=` | JWT +PA | Detect `${VAR}` references between secrets. |
| GET | `/dependencies/graph?environment=` | JWT +PA | Return the secret dependency graph. |

## 7. Enterprise (backup / policy) and agent analytics (project-scoped)

Also under `/api/v1/projects/:id`, JWT + `RequireProjectAccess`.

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/backups` | JWT +PA | Create an encrypted project backup. |
| GET | `/backups` | JWT +PA | List backups. |
| GET | `/policy` | JWT +PA | Get the secret lifecycle policy. |
| PUT | `/policy` | JWT +PA | Set the secret lifecycle policy. |
| GET | `/agent-activity` | JWT +PA | Recent agent activity for the project. |
| GET | `/agent-heatmap` | JWT +PA | Access heat-map (time × key). |
| GET | `/access-policies` | JWT +PA | List access policies (time/IP/geo). |
| POST | `/access-policies` | JWT +PA | Create an access policy. |
| DELETE | `/access-policies/:policyId` | JWT +PA | Delete an access policy. |

## 8. AI Intelligence — per-project (Phase 15)

Under `/api/v1/projects/:id`, JWT + `RequireProjectAccess`.

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/drift` | JWT +PA | Run a drift check between two environments. |
| GET | `/drift` | JWT +PA | List drift checks. |
| POST | `/drift/schedules` | JWT +PA | Create a scheduled drift check (cron). |
| GET | `/drift/schedules` | JWT +PA | List drift schedules. |
| PUT | `/drift/schedules/:scheduleId` | JWT +PA | Update a drift schedule. |
| DELETE | `/drift/schedules/:scheduleId` | JWT +PA | Delete a drift schedule. |
| POST | `/anomalies/scan` | JWT +PA | Run anomaly detection for the project. |
| GET | `/analytics/trends?period=&days=` | JWT +PA | Usage trends. |
| GET | `/analytics/forecast?days=` | JWT +PA | Usage forecast. |
| GET | `/analytics/export` | JWT +PA | Export analytics as CSV. |
| POST | `/recommendations/generate` | JWT +PA | Generate AI recommendations. |
| GET | `/recommendations?status=` | JWT +PA | List recommendations. |
| DELETE | `/recommendations/:recId` | JWT +PA | Dismiss a recommendation. |

## 9. Leases & agent (API-key surface)

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/projects/:id/leases` | API key +PA | Create a time-limited secret lease for an agent. |
| GET | `/api/v1/projects/:id/leases` | API key +PA | List leases. |
| DELETE | `/api/v1/projects/:id/leases/:leaseId` | API key +PA | Revoke a lease. |
| GET | `/api/v1/agent/activity` | API key | Activity summary for the calling API key. |

## 10. API keys, key rotation, webhook deliveries

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/api-keys` | JWT | Issue an API key (scopes, optional env, optional expiry; defaults to +90d, capped at +365d). |
| GET | `/api/v1/api-keys` | JWT | List the caller's API keys. |
| DELETE | `/api/v1/api-keys/:id` | JWT | Revoke an API key. |
| POST | `/api/v1/rotate-keys` | JWT | Rotate DEKs and re-encrypt secrets across **all** projects (admin op). |
| GET | `/api/v1/webhook-deliveries` | JWT | List webhook delivery attempts. |

## 11. Organizations & members

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/organizations` | JWT | Create an organization. |
| GET | `/api/v1/organizations` | JWT | List organizations. |
| GET | `/api/v1/organizations/:orgId` | JWT | Get an organization. |
| PUT | `/api/v1/organizations/:orgId` | JWT | Update an organization. |
| DELETE | `/api/v1/organizations/:orgId` | JWT | Delete an organization. |
| POST | `/api/v1/organizations/:orgId/members` | JWT | Add a member (role: viewer/editor/admin/promoter). |
| GET | `/api/v1/organizations/:orgId/members` | JWT | List members. |
| PUT | `/api/v1/organizations/:orgId/members/:userId` | JWT | Change a member's role. |
| DELETE | `/api/v1/organizations/:orgId/members/:userId` | JWT | Remove a member. |
| POST | `/api/v1/organizations/:orgId/projects` | JWT | Assign a project to the org. |
| GET | `/api/v1/organizations/:orgId/projects` | JWT | List the org's projects. |

### Organization enterprise & quota sub-resources

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/organizations/:orgId/sso` | JWT | Configure an SSO provider (OIDC/SAML); client secret encrypted. |
| GET | `/api/v1/organizations/:orgId/sso` | JWT | List SSO configs. |
| DELETE | `/api/v1/organizations/:orgId/sso/:provider` | JWT | Delete an SSO config. |
| POST | `/api/v1/organizations/:orgId/compliance` | JWT | Generate a compliance report (SOC2/GDPR/PCI). |
| GET | `/api/v1/organizations/:orgId/compliance` | JWT | List compliance reports. |
| GET | `/api/v1/organizations/:orgId/quota` | JWT | Get usage quota. |
| PUT | `/api/v1/organizations/:orgId/quota` | JWT | Set usage quota. |

## 12. Templates

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/templates` | JWT | Create a secret template. |
| GET | `/api/v1/templates` | JWT | List templates. |
| GET | `/api/v1/templates/builtin` | JWT | List built-in templates for common stacks. |
| GET | `/api/v1/templates/:templateId` | JWT | Get a template. |
| PUT | `/api/v1/templates/:templateId` | JWT | Update a template. |
| DELETE | `/api/v1/templates/:templateId` | JWT | Delete a template. |
| POST | `/api/v1/templates/:templateId/apply` | JWT | Apply a template's keys to a project/environment. |

## 13. Admin, agent summary, platform

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/api/v1/admin/dashboard` | JWT | System-health overview (links to metrics/traces/health). |
| GET | `/api/v1/admin/traces` | JWT | Most recent 100 trace spans. |
| GET | `/api/v1/platform/events` | JWT | List events from the `event_log`. |
| POST | `/api/v1/platform/events/replay` | JWT | Replay historical events to handlers. |
| GET | `/api/v1/platform/plugins` | JWT | List registered plugins. |
| POST | `/api/v1/platform/plugins` | JWT | Register a plugin. |
| PUT | `/api/v1/platform/plugins/:pluginId` | JWT | Enable/disable a plugin. |

## 14. AI Intelligence — global (Phase 15)

Under `/api/v1/ai`, JWT.

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/providers` | JWT | List configured AI providers and availability. |
| POST | `/query` | JWT | Natural-language secret search. |
| POST | `/converse` | JWT | Multi-turn NLP conversation. |
| GET | `/anomalies?project_id=&status=` | JWT | List anomalies. |
| PUT | `/anomalies/:anomalyId/acknowledge` | JWT | Acknowledge an anomaly. |
| PUT | `/anomalies/:anomalyId/resolve` | JWT | Resolve an anomaly. |
| POST | `/rules` | JWT | Create an anomaly alert rule. |
| GET | `/rules` | JWT | List alert rules. |
| PUT | `/rules/:ruleId` | JWT | Update an alert rule. |
| DELETE | `/rules/:ruleId` | JWT | Delete an alert rule. |
| POST | `/drift/run-scheduled` | JWT | Operator/cron trigger to run due drift schedules. |

## 15. OAuth 2.0 provider

KeepSave is itself an OAuth 2.0 / OIDC provider. Public token-plane endpoints under `/api/v1/oauth`; client-management endpoints require JWT. OAuth access/refresh tokens are **opaque random strings** (stored hashed, validated by DB lookup), **not JWTs**; KeepSave's own session JWTs are signed with **HS256**, and the JWKS endpoint currently returns an **empty key set** — RS256 + JWKS rotation is [ADR-0008](../adr/0008-rs256-jwks-rotation.md) (Proposed, not implemented). See the [security model](./05-security.md) and [SDKs & integrations](./13-sdks-integrations.md) for the matching detail.

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/oauth/token` | Public | Token endpoint (authorization_code w/ PKCE, refresh). |
| GET | `/api/v1/oauth/userinfo` | Public (token-checked) | OIDC UserInfo. |
| POST | `/api/v1/oauth/revoke` | Public | Revoke an access/refresh token. |
| GET | `/api/v1/oauth/.well-known/jwks.json` | Public | JWKS public keys. |
| GET | `/api/v1/oauth/authorize` | JWT | Authorization (consent) endpoint — issues an auth code. |
| POST | `/api/v1/oauth/clients` | JWT | Register an OAuth client. |
| GET | `/api/v1/oauth/clients` | JWT | List the caller's OAuth clients. |
| DELETE | `/api/v1/oauth/clients/:clientId` | JWT | Delete an OAuth client. |

## 16. MCP hub & gateway

The MCP (Model Context Protocol) hub lets users register MCP servers and proxy tool calls. Gateway command execution is hardened per [ADR-0010](../adr/0010-mcp-gateway-command-execution-hardening.md).

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/api/v1/mcp/servers/public` | Public | List public MCP servers (catalog). |
| POST | `/api/v1/mcp/servers` | JWT | Register an MCP server (GitHub source). |
| GET | `/api/v1/mcp/servers` | JWT | List the caller's MCP servers. |
| GET | `/api/v1/mcp/servers/:serverId` | JWT | Get an MCP server. |
| PUT | `/api/v1/mcp/servers/:serverId` | JWT | Update an MCP server. |
| DELETE | `/api/v1/mcp/servers/:serverId` | JWT | Delete an MCP server. |
| POST | `/api/v1/mcp/servers/:serverId/rebuild` | JWT | Rebuild a server's tool definitions from source. |
| POST | `/api/v1/mcp/installations` | JWT | Install an MCP server for the caller. |
| GET | `/api/v1/mcp/installations` | JWT | List installations. |
| PUT | `/api/v1/mcp/installations/:installId` | JWT | Update an installation. |
| DELETE | `/api/v1/mcp/installations/:installId` | JWT | Uninstall an MCP server. |
| POST | `/api/v1/mcp/gateway` | JWT | Proxy a JSON-RPC MCP tool call to the target server. |
| GET | `/api/v1/mcp/gateway/tools` | JWT | List tools available across the caller's servers. |
| GET | `/api/v1/mcp/gateway/stats` | JWT | Gateway usage stats. |
| GET | `/api/v1/mcp/config` | JWT | Generate the MCP client config JSON for the user. |

## 17. Applications (dashboard)

Under `/api/v1/applications`, API-key authenticated.

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/applications` | API key | Register an application/service link. |
| GET | `/api/v1/applications` | API key | List applications. |
| GET | `/api/v1/applications/:appId` | API key | Get an application. |
| PUT | `/api/v1/applications/:appId` | API key | Update an application. |
| DELETE | `/api/v1/applications/:appId` | API key | Delete an application. |
| POST | `/api/v1/applications/:appId/favorite` | API key | Toggle favourite. |

## 18. Embed config

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/api/v1/embed-config/:project_id` | Public (10/min limiter) | Widget bootstrap: returns `{project_id, allowed_origins, embed_policy_enabled}`. 404 (shared shape) when missing **or** disabled, to mitigate ID enumeration. |

The authenticated counterpart, `PUT /api/v1/projects/:id/embed-config`, is listed under [§4 Projects](#4-projects). See the [embed widget](./08-embed-widget.md) chapter and [ADR-0006](../adr/0006-embed-widget-origin-allowlist.md).

---

## 19. Reconciliation notes

The endpoint inventory above is cross-checked against [`docs/audits/API_RECONCILIATION.md`](../audits/API_RECONCILIATION.md), which reconciled the backend routes against the frontend client. Highlights an auditor should know:

- **Total: 142 routes** (5 root + 137 under `/api/v1`), counted directly from `router.go`; the per-route tables above enumerate them all. (The reconciliation audit's older 126/121 figure predates the Phase-15 per-project AI routes and the org-quota/embed routes and should be refreshed.) Most are consumed by the dashboard; the remainder are machine/agent surfaces or backend-ahead-of-frontend features.
- **One known frontend-only mismatch (`F-C-001`):** the embed SDK references `POST /api/v1/projects/:id/secrets/batch`, which **has no backend route** — calling `KeepSaveAPI.batchGetSecrets` will 404.
- **`DELETE /projects/:id/webhooks` has no `:webhookId`** segment, so it removes all webhooks for the project (flagged as an API-design smell in the audit).
- A cluster of Phase-15 (drift schedules, AI rules, quotas, CSV export) and Phase-9 (webhook setup, key rotation, SSO delete) endpoints are **orphan-backend** — registered and functional but not yet wired into the dashboard UI.

---

## 20. The error model

Errors never leak raw database, crypto, or validation internals to clients. The implementation lives in `backend/internal/api/errors.go`; the standard it follows is [`docs/ERROR_HANDLING_STANDARD.md`](../ERROR_HANDLING_STANDARD.md). (The standard describes a planned `internal/api/httperror/` package; the shipped code implements the same contract in the `api` package itself.)

### Wire shape

Every non-2xx response is a single envelope:

```json
{ "error": { "code": 404, "message": "not found", "error_code": "NOT_FOUND" } }
```

- `code` — mirrors the HTTP status (kept for back-compat with older clients).
- `message` — a safe, end-user-facing string.
- `error_code` — a stable machine-readable symbol.

### Typed errors and sentinels

`HTTPError{Symbol, Status, Message, cause}` carries a safe `Message` plus an unexported `cause` that is **logged server-side and never serialised**. Handlers build on these sentinels:

| Sentinel | Symbol | Status |
|----------|--------|--------|
| `ErrInvalidInput` | `INVALID_INPUT` | 400 |
| `ErrUnauthorized` | `UNAUTHORIZED` | 401 |
| `ErrForbidden` | `FORBIDDEN` | 403 |
| `ErrNotFound` | `NOT_FOUND` | 404 |
| `ErrConflict` | `CONFLICT` | 409 |
| `ErrRateLimited` | `RATE_LIMITED` | 429 |
| `ErrInternal` | `INTERNAL` | 500 |

`Wrap(base, cause)` / `WrapMessage(base, msg, cause)` attach an internal cause to a sentinel. `WrapError(c, err)` is the single sink: it unwraps an `*HTTPError` (sending only the safe fields), maps `sql.ErrNoRows → 404`, and treats anything else as a sanitized **500 INTERNAL** with the cause logged. The `PanicRecoveryMiddleware` produces the identical 500 shape, so a panic and a returned error are indistinguishable to clients.

### Sanitization rule

Raw `pq`/`pgx`/`database/sql`/`crypto/cipher`/`validator` error strings must never reach the response body. This is enforced by `backend/internal/api/error_leak_test.go`, which asserts the envelope shape per status and that no response contains forbidden substrings. (`Readiness`/`/readyz` is a deliberate exception — it echoes the DB ping error to operators on an unauthenticated infra endpoint.)

---

## 21. Rate limiting, request size, and CORS

**Rate limiting** (`backend/internal/api/ratelimit.go`) is a per-client-IP token bucket with a background cleanup loop:

| Limiter | Rate | Burst | Scope |
|---------|------|-------|-------|
| Global (`RateLimitMiddleware`) | 100 req/s | 200 | Every route. |
| Embed-config | 10 req/min | 10 | Only `GET /api/v1/embed-config/:project_id` ([ADR-0006](../adr/0006-embed-widget-origin-allowlist.md)). |

On exhaustion the limiter returns **429** with `Retry-After: 1` and `{"error":"rate limit exceeded, please retry later"}`, and the metrics middleware increments `keepsave_rate_limit_hits_total`.

**Request size** — `RequestSizeLimitMiddleware` caps every request body at **1 MiB** (`1 << 20`).

**CORS** (`CORSMiddleware`) reflects an `Origin` that matches `CORS_ORIGINS`: `*` mirrors the origin (dev only; rejected in production at config load), an exact comma-separated match, or a single-`*` glob (for preview subdomains). `Access-Control-Allow-Credentials` stays **off** — KeepSave is bearer-token only, and enabling cookies would widen the CSRF surface (a Type-1 change). `OPTIONS` is short-circuited with 204.

---

## See also

- [Backend Architecture](./02-backend.md) — the middleware pipeline, layering, and service/repository inventory behind these endpoints.
- [Data Model](./04-data-model.md) — the tables these handlers read and write.
- [Security Model](./05-security.md) — the auth/authz model in depth and the audit-log taxonomy.
- [Promotion Engine](./06-promotion.md) — the workflow behind the `/promote*` and `/promotions/*` routes.
- [Embeddable Widget](./08-embed-widget.md) — the consumer of `/embed-config`.
- [`docs/audits/API_RECONCILIATION.md`](../audits/API_RECONCILIATION.md) — the full route-by-route frontend/backend reconciliation.
- [`docs/ERROR_HANDLING_STANDARD.md`](../ERROR_HANDLING_STANDARD.md) — the canonical error-handling standard.
