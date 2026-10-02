![KeepSave — Your secrets. In the right orbit.](docs/assets/keepsave-banner.png)

# KeepSave

[![Release](https://img.shields.io/github/v/release/santapong/KeepSave?style=flat-square&color=8b5cf6&labelColor=0b0a12)](https://github.com/santapong/KeepSave/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/santapong/KeepSave/ci.yml?branch=develop&style=flat-square&label=CI&color=4fe3b8&labelColor=0b0a12)](https://github.com/santapong/KeepSave/actions)
[![Go](https://img.shields.io/badge/Go-1.27.1-00ADD8?style=flat-square&labelColor=0b0a12)](https://go.dev)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat-square&labelColor=0b0a12)](https://react.dev)
[![License](https://img.shields.io/badge/license-MIT-a78bfa?style=flat-square&labelColor=0b0a12)](#license)

**Your secrets. In the right orbit.**

KeepSave stores encrypted project credentials and controls access to them across
Alpha, UAT and production. The current backend work focuses on a dependable
core: sign-in, revocable human sessions, team workspaces, scoped credential
access, immutable secret history and recoverable encrypted backups.

This is the **unreleased `v1.4.0-rc.1` core candidate**; see its
[candidate notes](docs/releases/v1.4.0-rc.1.md). Local implementation and synthetic
verification do not establish production readiness. The
[acceptance ledger](docs/validation/2026-10-01-core-release/ACCEPTANCE.md) records
what is verified, what is preserved for compatibility, and what remains gated.
The existing static landing page is at [keepsave.draveniq.dev](https://keepsave.draveniq.dev/).
`app.keepsave.draveniq.dev` is the intended application origin; this backend work
has not deployed it.

## Core behavior

| Capability | Behavior and limits |
|---|---|
| Sign-in | Password registration/login, plus Google and GitHub when operator configuration is present. Provider identity is verified; an email collision requires explicit linking from the existing account. |
| Human sessions | Database-backed 24-hour sessions, metadata listing, logout and individual revocation. Legacy untracked human JWTs are refused by the running application. There is no human refresh-token flow in this slice. |
| Workspaces | A user explicitly creates a named organization and becomes its administrator. Creation joins membership and audit in one transaction; an optional `Idempotency-Key` makes retries safe. Registration creates no default projects or global permissions. |
| Project access | Personal projects remain supported. Attaching one requires its stored owner and a destination administrator. Cross-organization transfer is refused; successful first attachment ends existing delegated keys and leases. Assigned projects then use current workspace membership, including for the original owner. |
| Vault | AES-256-GCM envelope encryption and project keys. Credential access checks current stored ownership, membership, scope and expiry. API-key reads deliberately return authorized values to the caller. |
| History and rotation | PostgreSQL journal adapters capture core mutation paths, retain referenced key versions and append restoration as a new revision with a concurrency precondition. Existing values require explicit baseline enrollment. |
| Encrypted recovery | PostgreSQL backup download/catalog, verification, metadata preview and selected-record restoration. A trusted worker schedules private encrypted backups only after operator-confirmed recovery acceptance. Deleted records and authorization state are not resurrected. |
| Audit | Core mutations join required audit and PostgreSQL outbox writes in their transaction. The audit-chain head is persisted and serialized in the database. |
| Templates | Private workspace templates require current membership to read/apply and current admin to change. Personal templates belong to their creator. Metadata changes require a current human session and required audit; core global publication is refused. Defaults are ordinary config: store examples/placeholders, not live credentials. |

The repository also contains promotion, import/export, templates, scoped API
keys and agent leases. Their acceptance status is recorded individually in the
ledger. SQLite supports the legacy/local profile. A disposable MySQL 8.4 exercise
passed the bounded legacy identity/session/scoped-vault and workspace-role
profile; it does not establish journal/recovery parity. The versioned vault and
durable jobs are **PostgreSQL-only** until parity is tested. Unsupported core history/recovery
returns 503; authorized ciphertext corruption returns a safe 500.

The restricted application profile refuses experimental AI, enterprise SSO,
policy metadata, legacy OAuth issuance, event replay, plugin execution and
webhook automation. MCP execution and API-host connector builds remain disabled.
Their source and routes are retained for incremental migration.

## Architecture and next features

The target remains a Go/Gin **modular monolith** with PostgreSQL transactions:
identity, policy, vault, promotion, broker, MCP, automation, audit and jobs.
Handlers translate requests; authorized services own decisions and mutations.
Typed dependency wiring replaces the growing positional router constructor,
with a compatibility adapter retained for old callers.

The approved next journey is controlled repository review through Codex:
one explicitly approved GitHub repository, a resolved commit, a ten-minute run,
a vetted connector on a separate restricted runner, and GitHub App credentials
held by the broker. A connector requests a structured operation; the broker
makes the authenticated provider request. **This broker, standards-based `/mcp`,
private skills and managed Codex profiles are planned, not implemented here.**
GitHub social sign-in is separate from a GitHub App provider connection.

See the [architecture proposal](docs/design/2026-09-28-backend-platform/README.md),
[implementation checkpoint](docs/design/2026-09-28-backend-platform/IMPLEMENTATION.md),
[core release ADR](docs/adr/0028-core-identity-and-vault-release.md) and
[delivery dependencies](docs/design/2026-09-28-backend-platform/DELIVERY.md).

The [research-based product plan](docs/design/2026-10-02-product-roadmap/README.md)
prioritizes setup, safe denial explanations, searchable audit, renewal ownership
and team onboarding/offboarding, with proposed contracts, acceptance gates and
primary-source evidence. These additions are planned; the selected Codex/GitHub
broker program remains the strategic delivery path.
The older [architecture views](docs/ARCHITECTURE_VIEWS.md) describe legacy
components; they do not prove runner isolation or current release acceptance.

## Local development

**Prerequisites:** Docker and Docker Compose. CI and container builds pin Go
**1.27.1** and Node.js **24.21.0**. The Go module minimum is now 1.26.0.

```bash
git clone https://github.com/santapong/KeepSave.git
cd KeepSave
cp .env.example .env
# Configure disposable local keys and PostgreSQL settings in .env.
docker compose up --build
```

The development Compose bundle exposes the API at `http://localhost:8080` and
the frontend at `http://localhost:3002`. It is not the planned production
self-hosted bundle. Keep recovery material outside the database and back it up
separately; loss of the wrapping key can make encrypted values unrecoverable.

To run the services directly, export the process configuration. Copying an
`.env` file alone does not configure a Go process; Compose reads dotenv values
for its own interpolation, while `config.Load()` reads `os.Getenv`. Source only
a trusted local shell-compatible file:

```bash
cd backend
cp .env.example .env
# Configure disposable local database/key settings in this trusted file.
# Go reads exported process variables; it does not load .env automatically.
set -a
. ./.env
set +a
go run ./cmd/server
```

```bash
cd frontend
npm ci
npm run dev
```

Follow [social sign-in setup](docs/SOCIAL_LOGIN_SETUP.md) to register Google and
GitHub applications and configure exact callbacks on the application origin.
Provider buttons reflect configuration availability. No provider secrets,
provider applications, DNS records or production services are created by tests.

Existing PostgreSQL installations need a coordinated journal cutover. Drain
older API/worker processes, preserve an external backup and its recovery key,
apply additive migrations, and run the trusted-host `keepsave-vault -action
baseline` command before admitting traffic with the new binary. This command
labels current encrypted values as a baseline; it does not invent old history.
Startup refuses active projects that remain unenrolled. Migration 023 preserves
project tombstones and immutable audit identity fields; 024 binds promotion
approval to the captured source artifact; 025 adds the durable backup catalog.
Do not resume old writers after enrollment. The new container includes operator binaries, invoked explicitly
through an alternate entrypoint; they are never HTTP endpoints.

`keepsave-vault -action verify -bundle <path>` verifies an external encrypted
bundle and prints metadata. `recover-isolated` requires an explicitly confirmed
fresh PostgreSQL database with no non-system tables and a new project name. Use separate target configuration
and documented recovery material; it refuses existing non-system tables before migration and does not
restore source accounts, grants or sessions. Live recovery instead uses the
metadata preview and explicit selected records with current revision checks.

## API contract

The maintained core OpenAPI source is
[core.json](backend/internal/api/openapi/core.json), served at `/api/docs`.
It describes identity, sessions, workspaces, projects, secret reads/writes,
history, encrypted recovery, imports/exports, promotion, rotation, templates,
audit and client delegation. Actual-router tests check mounted paths and
response schemas. Compatibility surfaces outside this contract remain explicitly
inventoried in the acceptance ledger. The core contract covers 52 paths,
70 operations and 87 schemas, validated against actual handler responses.

| Area | Paths |
|---|---|
| Sign-in | `/api/v1/auth/register`, `/login`, `/providers`, `/social/:provider/start`, `/social/:provider/complete`, `/logout` |
| Sessions and linking | `/api/v1/account/sessions`, `/account/connections` |
| Workspaces | `/api/v1/organizations`, members and explicit personal-project attachment |
| Projects and credentials | `/api/v1/projects`, `/:id/secrets`, `/:id/secrets/batch` |
| History | `/:id/secrets/:secretId/versions`, explicit version reads and `/restore` |
| Recovery | `/:id/backups`, `/verify`, `/preview`, `/restore` |
| Imports and exports | `/:id/env-import`, `/:id/env-export` |
| Key continuity | `/:id/rotate-keys`, `/:id/verify-encryption` |
| Promotion | `/:id/promote`, `/:id/promote/diff`, `/:id/promotions`, approval/reject/rollback |
| Templates | `/api/v1/templates`, `/builtin`, `/:templateId`, `/:templateId/apply` |
| Client delegation | `/api/v1/api-keys`, `/:id/leases`, `/:id/agent-token` and revocation |
| Audit | `/:id/audit-log` |
| Capability metadata | `/api/v1/capabilities` |
| Operations | `/healthz`, `/readyz`, `/metrics` |

Restore requests supply the expected current revision. List-history and recovery
preview responses contain metadata; explicit current or historical secret reads
contain values. Error responses use the existing safe `{error: ...}` envelope.

Generate frontend wire types after changing the contract:

```bash
node scripts/generate-core-api-types.mjs
node scripts/generate-core-api-types.mjs --check
```

## Verification

```bash
# Creates only unique disposable Docker resources, with no published DB port.
# Ignores an operator DATABASE_URL; cleanup removes only this run's resources.
./scripts/test-platform-postgres.sh

cd backend
go test -race -shuffle=on ./...
go vet ./...

cd ../frontend
npm ci
npm test -- --maxWorkers=2
npm run lint
npm run build:all
```

The PostgreSQL harness applies the shipped migrations and runs authorization,
transaction, concurrency, identity/session and API-contract fixtures. CI retains
race, vet, formatting, fuzz, dependency, frontend, container, SAST, CodeQL and
Robot checks on `develop` and `main`. A workflow file is not evidence that remote
CI ran; exact executed checks and remaining gates belong in the ledger.

## Recovery and security boundaries

Project and secret deletion revoke active authority and retain tombstones,
encrypted history and dependent keys. This slice does not automatically purge
history or key versions. Audit retention remains 365 days. The intended daily
backup policy is 02:00 UTC, 30 verified daily bundles and at least two verified
copies, with manual/pre-upgrade bundles held separately. The trusted worker
implements this schedule and retention through durable jobs and a metadata
catalog. `KEEPSAVE_RECOVERY_VERIFIED` defaults to false; automatic scheduling and retention
must stay disabled until private storage, verified key dependencies and an
operator recovery drill are configured. Failed retention stays visible.

The [control-host reference bundle](deploy/self-hosted/README.md) places TLS,
frontend, API, PostgreSQL and the trusted worker on one host without publishing
database/API ports. It uses the existing Vault Transit key integration and
private operator-supplied files. Configuration validation is not production
deployment or M5 acceptance; the connector runner remains a separate milestone.

KeepSave controls operations routed through it. Revocation denies later
admissions after its database commit; already admitted work may finish and
returned values cannot be recalled. The future broker prevents provider-token
release for its own integrations; ordinary vault clients still receive the
values they are permitted to read. Managed Codex settings will add local
restrictions but cannot attest to an unrestricted device administrator.

Relevant authorization, custody, key continuity and audit changes require the
repository's independent Security/Tech Lead review before integration/release.
User approval of implementation does not imply those reviews have occurred.
See [development rules](CLAUDE.md), [threat model](docs/THREAT_MODEL.md),
[security audit](SECURITY_AUDIT.md) and [secret sources](docs/SECRET_SOURCES.md).

## Repository layout

```text
backend/                 Go/Gin services, policy, vault, jobs and repositories
backend/migrations/      Additive embedded PostgreSQL, SQLite and MySQL migrations
frontend/                Accepted KeepSave identity, workspace and auth UI
sdks/                    Existing Go, Node.js and Python adapters
integrations/            Existing CI/Terraform adapters
scripts/                 Isolated PostgreSQL tests and contract type generation
docs/                    Decisions, architecture, acceptance and operator notes
```

SDK, widget and integration source availability does not establish end-to-end
compatibility with the core candidate. See [integrations](docs/INTEGRATIONS.md)
and the acceptance ledger before using them.

## License

The existing repository documentation labels KeepSave MIT. This checkout does
not include a standalone license file.
