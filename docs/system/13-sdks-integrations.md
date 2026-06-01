# SDKs & Integrations

> Part of the **[KeepSave System Documentation](./README.md)**.

This chapter documents the surfaces through which other systems talk to KeepSave: the first-party client SDKs (Go, Node.js, Python), the CI/CD and Terraform integrations, the MCP hub & gateway (registering MCP servers and executing their tools with decrypted secrets), the OAuth/OIDC provider that downstream platforms use for SSO, the external project integrations (Seidr, Nexus, MedQCNN, Grovernance), and the outbound webhook surface with its SSRF guard. Each is grounded in the actual code/config and linked to its canonical doc.

For the HTTP API these clients call see [backend](./02-backend.md) and [API reference](./03-api-reference.md); for the auth model they use see [security](./05-security.md).

---

## 1. Client SDKs

KeepSave ships three first-party SDKs. All three wrap the same `/api/v1` REST surface and share an **identical auth model**: an **API key takes precedence** (sent as `X-API-Key`), otherwise a JWT is sent as `Authorization: Bearer <token>`. `login()`/`register()` capture the returned JWT into the client. All three also implement the same resilience trio — **automatic retry with exponential backoff** (retry on network error, 429, or 5xx), a **circuit breaker** (CLOSED/OPEN/HALF_OPEN, default threshold 5, reset 30 s), and an **in-memory TTL cache** (default 60 s) for `listSecrets`, invalidated on writes/promotions/rotations.

| SDK | Location | Package / version | Runtime deps | HTTP layer |
|-----|----------|-------------------|--------------|------------|
| **Go** | `sdks/go/keepsave.go` | `package keepsave` (doc'd v2.0.0) | stdlib only (single file, no `go.mod`) | `net/http` |
| **Node.js / TS** | `sdks/nodejs/src/index.ts` | `@keepsave/sdk` v2.1.0 | none at runtime (TS dev dep only) | `fetch` |
| **Python** | `sdks/python/keepsave/client.py` | `keepsave` v2.1.0, `python_requires>=3.9` | stdlib only (`urllib`) | `urllib.request` |

### 1.1 Surface

Each SDK covers projects, secrets (incl. **batch fetch** via `POST /projects/:id/secrets/batch` and `refreshSecrets` to bypass cache), promotions (`promote` + `promote/diff`), key rotation (`rotateKeys`, which invalidates the local cache), `.env` import/export, API keys, and the Application Dashboard (`/applications`). The Go and Python clients expose the circuit state (`CircuitState()` / `circuit_state`) and `ClearCache()`/`clear_cache()`.

### 1.2 Basic usage

**Go:**
```go
client := keepsave.NewClient("http://localhost:8080", keepsave.WithToken("jwt"))
secrets, err := client.ListSecrets(ctx, "project-id", "alpha")
```

**Node.js:**
```ts
import { KeepSaveClient } from '@keepsave/sdk';
const client = new KeepSaveClient({ baseUrl: 'http://localhost:8080', apiKey: 'ks_...' });
const secrets = await client.listSecrets('project-id', 'alpha');
```

**Python:**
```python
client = KeepSaveClient("http://localhost:8080")
client.login("user@example.com", "password")
secrets = client.list_secrets("project-id", "alpha")
```

Dependabot tracks the Node.js (`/sdks/nodejs`, npm) and Python (`/sdks/python`, pip) SDKs for updates (see [infrastructure](./09-infrastructure.md) §4.3). The Go SDK is a single dependency-free file.

---

## 2. CI/CD and IaC integrations

These live under `integrations/` and are thin wrappers around the same REST surface, designed for the API-key auth path so an agent/runner reads secrets without embedding them in code.

| Integration | Location | What it does |
|-------------|----------|--------------|
| **GitHub Action** | `integrations/github-action/` (`action.yml`) | Pulls secrets from KeepSave and injects them as env vars or files; inputs `api-url`, `api-key`, project/environment |
| **GitLab CI** | `integrations/gitlab-ci/` (`keepsave.gitlab-ci.yml`) | Equivalent include for GitLab pipelines |
| **Terraform** | `integrations/terraform/` (`main.tf`) | Secrets-as-IaC |

### 2.1 Terraform (`integrations/terraform/main.tf`)

The Terraform module reads secrets via an `external` data source: it `curl`s `GET /api/v1/projects/:id/secrets?environment=...` with the JWT (`var.keepsave_token`, marked `sensitive`), maps the response into `{key: value}`, and can render them into a `.env.<environment>` file written with `file_permission = "0600"`. It exports `secret_keys` and `secrets_count` outputs. Configuration is via `TF_VAR_keepsave_*` env vars (`integrations/terraform/README.md`). Note this is a **data-source consumer**, not a full Terraform *provider* (there is no resource CRUD for managing secrets from Terraform state).

---

## 3. MCP hub & gateway

KeepSave acts as an **MCP (Model Context Protocol) server marketplace** for AI agents (Phase 13). The routes are in `backend/internal/api/router.go` under `/api/v1/mcp` (JWT-authenticated, plus a public `/mcp/servers/public` listing).

### 3.1 Hub — register / build / install (`MCPHubHandler`)

Authenticated users can register an MCP server from a GitHub URL (`POST /mcp/servers`), which clones and **builds** it (the build runs in a background goroutine), then rebuild (`POST /mcp/servers/:id/rebuild`), list, update, delete, and install/uninstall servers (`/mcp/installations`).

### 3.2 Gateway — tool execution (`MCPGatewayHandler`)

`POST /mcp/gateway` executes a tool exposed by an installed MCP server. The gateway:

- decrypts the project's secrets and passes them to the spawned process **as environment variables** (the agent's tool runs with the secrets it needs in `env`);
- builds `cmd.Env` from the mapped vars only — it does **not** inherit `os.Environ()`, so the API server's own secrets are not leaked into the child (`handlers_mcp_gateway.go`);
- validates the entry command, **rejecting shell metacharacters**, and runs it via `exec.CommandContext` with a timeout.

This command-execution path is the highest-risk surface in KeepSave (authenticated-RCE class). Its hardening is governed by [ADR-0010](../adr/0010-mcp-gateway-command-execution-hardening.md): allowlist of interpreters (`node`/`python`/`go-binary`) with argv arrays instead of free-form strings, a scrubbed environment, output bounded via `io.LimitReader`, a `safego` goroutine recover-harness, and a per-build concurrency/disk budget.

> **Implementation state (be precise):** the *scrubbed-env*, *metacharacter reject*, and *`exec.CommandContext` + timeout* parts of [ADR-0010](../adr/0010-mcp-gateway-command-execution-hardening.md) (part B) are in code. The **interpreter allowlist with argv arrays** (part A), the **`safego` harness** (part C — no `backend/internal/runtime/` package exists), and the **build budget** (part D) are decided but not yet fully wired. Treat the MCP exec surface as partially hardened. See [operations](./10-operations.md) §3.

---

## 4. OAuth / OIDC provider & SSO

KeepSave is itself an **OAuth 2.0 / OIDC identity provider** (Phase 13), used by downstream platforms (Nexus, MedQCNN, Grovernance) for SSO. Routes in `backend/internal/api/router.go`:

| Surface | Endpoint(s) |
|---------|-------------|
| Discovery | `GET /.well-known/openid-configuration`, `GET /api/v1/oauth/.well-known/jwks.json` |
| Token | `POST /api/v1/oauth/token` (grants: `authorization_code`, `client_credentials`, `refresh_token`) |
| Userinfo | `GET /api/v1/oauth/userinfo` |
| Revoke | `POST /api/v1/oauth/revoke` |
| Authorize + client mgmt (JWT-gated) | `GET /oauth/authorize`, `POST/GET/DELETE /oauth/clients` |

The `/userinfo` response is the contract downstream consumers branch on: human-user tokens return `sub`, `email`, `scopes`, and `groups` (org memberships as `"<org-slug>:<role>"`); `client_credentials` tokens return `token_type: "service_account"` and `scopes` (see [Grovernance integration](../grovernance_integration.md)).

> **Signing-algorithm caveat:** KeepSave currently signs JWTs with **HS256** (shared `JWT_SECRET`). The JWKS endpoint returns an **empty key set** with a note that RS256 support is *planned* (`handlers_oauth.go`). The move to **RS256 + JWKS with `kid` rotation** is [ADR-0008](../adr/0008-rs256-jwks-rotation.md), which is **Proposed** — so external token verification via JWKS is not yet possible. Enterprise SSO config (per-org SSO providers) is wired via `/organizations/:orgId/sso` (`EnterpriseHandler`).

---

## 5. External project integrations

KeepSave is the security/secrets backbone for several sibling projects. Each integration doc pins the contract.

| Project | Doc | Role of KeepSave | Status |
|---------|-----|------------------|--------|
| **Seidr** | [SEIDR_INTEGRATION](../SEIDR_INTEGRATION.md) | Seidr's `KeepSaveSecretProvider` fetches secrets via `GET /projects/:id/secrets` with a scoped API key, caches per-TTL, refetches on `secret.rotated`, and circuit-breaks on repeated 5xx | Contract is **design-only** in the doc; the E2E harness (`tests/e2e/seidr/`) exercises the HTTP contract (see [testing](./11-testing.md) §5). A real Seidr-runtime boot is [FOLLOWUPS](../FOLLOWUPS.md) #8 |
| **Nexus** | [nexus_integration](../nexus_integration.md) | Secret vault, OAuth provider, MCP hub for agent tools, promotion engine, scoped API keys, and audit for an Agentic-AI-Company-as-a-Service platform (incl. A2A gateway auth) | Integration guide |
| **MedQCNN** | [medqcnn_integration](../medqcnn_integration.md) | Secret vault, MCP hub for diagnostic tools, OAuth provider, promotion engine, and audit for a hybrid quantum-classical medical-imaging CNN | Integration guide |
| **Grovernance** | [grovernance_integration](../grovernance_integration.md) | Identity provider: every actor token reaching Grovernance's `/authorize` is resolved against KeepSave's `GET /oauth/userinfo` (the deploy-governance gate) | Contract pinned (userinfo shape) |

The integration docs converge on the same KeepSave capabilities — scoped/expiring API keys for agents, OAuth/OIDC for actor identity, the MCP hub for tool hosting, and the promotion pipeline for dev→staging→prod config transitions — making them a good cross-check on the API surface.

---

## 6. Webhooks (with SSRF guard)

KeepSave can notify external receivers of events (e.g. `promotion_completed`, `secret.rotated`). Webhook routes are per-project: `POST/GET/DELETE /api/v1/projects/:id/webhooks` and `GET /api/v1/webhook-deliveries` (`WebhookHandler`). Deliveries are **HMAC-SHA256-signed** with a per-config secret so receivers can verify authenticity.

Because a webhook URL is user-controlled and the request originates from KeepSave's network position, it is an **SSRF** surface (an attacker could point it at cloud metadata — `169.254.169.254` — or RFC-1918 / k8s service IPs). The guard is governed by [ADR-0013](../adr/0013-webhook-emission-with-ssrf-guard.md), whose central rule is **atomicity**: webhook emission ships *only* alongside the SSRF guard, body-buffered retries, and signing-secret rotation — any subset is worse than the status quo.

> **Implementation state:** `ValidateWebhookURL` (`backend/internal/service/url_safety.go`) **is wired and called at registration** (`webhook_service.go`). It rejects non-`https` schemes (outside dev), cloud-metadata hostnames, link-local `169.254.0.0/16` (covers AWS/GCP IMDS), loopback/RFC-1918, IPv6 link-local, and k8s service CIDRs (honoring `K8S_SERVICE_CIDR`). The [ADR-0013](../adr/0013-webhook-emission-with-ssrf-guard.md) parts still pending: the **delivery-time re-resolve** (to defeat DNS rebinding between registration and delivery), **body-buffered retries** (today's retry can resend an empty body — [BACKEND_SPOF](../audits/BACKEND_SPOF.md) #6), and **disabled redirects**. The webhook service is also still in-memory (config registration is not persisted across restarts / replicas — see [operations](./10-operations.md) §3.2).

---

## See also

- [Backend](./02-backend.md) / [API reference](./03-api-reference.md) — the HTTP surface SDKs and integrations call
- [Security](./05-security.md) — auth model (JWT + API keys), OAuth, encryption
- [ADR-0010](../adr/0010-mcp-gateway-command-execution-hardening.md) — MCP gateway command-execution hardening
- [ADR-0013](../adr/0013-webhook-emission-with-ssrf-guard.md) — webhook SSRF guard
- [ADR-0008](../adr/0008-rs256-jwks-rotation.md) — RS256 + JWKS (proposed)
- [SEIDR_INTEGRATION](../SEIDR_INTEGRATION.md), [nexus_integration](../nexus_integration.md), [medqcnn_integration](../medqcnn_integration.md), [grovernance_integration](../grovernance_integration.md)
- [Testing](./11-testing.md) §5 — the Seidr E2E harness
