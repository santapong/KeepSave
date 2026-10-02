# 2. Backend architecture

Part of the [system documentation](README.md), reconciled 2026-10-02.

## Composition and startup

`backend/cmd/server/main.go` loads validated configuration, opens the selected
SQL database, applies additive migrations from the on-disk tree or embedded fallback, resolves the wrapping key,
constructs repositories/services and enables database-backed human sessions.
On PostgreSQL it wires the versioned vault and fails startup when active projects
have not been explicitly baselined. `api.NewRouter(api.Dependencies{...})` is the
typed composition contract; `SetupRouter` remains a fixture/legacy adapter.
CoreRelease is enabled by the application composition.

The core profile deliberately refuses unfinished intelligence, enterprise SSO,
policy metadata, legacy OAuth issuance, event replay, plugin mutation, webhook
automation and MCP execution. Connector building is disabled. Retained source
files do not imply the production API executes arbitrary tools.

The server has lifecycle goroutines for existing audit retention, database-pool
metrics and token-denylist maintenance. Scheduled backups use the separate
`cmd/keepsave-worker` binary, not an API goroutine. `cmd/keepsave-operator` enrolls
an immutable user ID for operator authority; email-derived administration is
rejected. `cmd/keepsave-vault` performs explicit baseline, encrypted verification
and isolated recovery on a trusted host. None is an HTTP administration endpoint.

## Request and authority path

Middleware enforces transport/body limits, request logging/redaction, trusted
proxies, origins, rate limits, metrics and tracing. Authentication resolves the
person or API key. Human admission checks current stored session authority;
legacy untracked human JWTs require reauthentication. Project and scope guards
reject early; the authorized vault service rechecks stored resource authority
before decryption or mutation. A valid token or known resource ID is insufficient.

Current project roles come from personal ownership only while organization is
NULL, otherwise current organization membership. API keys use their stored owner
and project scope; agent tokens use live parent lineage. Database/session/policy
failure never produces a cached allow fallback. Already-admitted work may finish.

## Module direction

`internal/policy` is transport-independent; `internal/vault` and `internal/jobs`
use ports and own their invariants. Existing `internal/service` and
`internal/repository` are migration adapters. Typed actions and stored-resource
resolution are shared across transports. Only vault/broker custody may obtain
new credential material; only the future isolated runner executes connectors.
Legacy exceptions are explicit in the architecture boundary test and credential
inventory, and must not expand.

The current source has not been completely reorganized into the target modules.
Move a capability when its authorized service, transaction boundary, handler
contract and real-database failure tests are complete. Do not introduce a generic
repository/interface for every model or duplicate authorization in MCP/CLI/jobs.
See [architecture](../ARCHITECTURE.md) for the ownership map and future slices.

## Configuration and operational limits

`config.Load()` reads exported process environment and does not parse `.env`.
A copied dotenv file alone is insufficient for direct Go startup; use a trusted
shell-compatible environment loader or explicit exported variables. Compose
handles its own interpolation/env files. Use `backend/.env.example`, `backend/internal/config/config.go`,
`docs/SOCIAL_LOGIN_SETUP.md` and the
[self-hosted reference](../../deploy/self-hosted/README.md) as the current source
for exact variables. Production rejects the committed development key, short JWT
secret, wildcard CORS, insecure PostgreSQL DSN and deprecated administrator-email
configuration. Vault Transit is the reference production wrapping-key source;
environment keys are for disposable development. AWS/GCP adapters remain unwired.
These checks do not prove TLS deployment, least-privilege database operation,
capacity, outage recovery or two-replica correctness.
