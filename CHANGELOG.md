# Changelog

All notable changes to KeepSave will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

**Project governance and 30-day plan execution.** Docs-only work plus
one CI hardening change; no API or behavior changes. Establishes the
roles, decision-making process, and artifacts the team needs to keep
shipping KeepSave with the rigor a secrets product demands.

### Added — governance artifacts

- **`docs/ROLES.md`** - 9-role operating model (Tech Lead, Security
  Engineer with veto on crypto/auth/promotion, Backend, Frontend,
  DevOps, QA, PM, UX, Tech Writer). Each role mandate is anchored to
  owned artifacts already in the repo, not abstract titles. Includes
  Type-1/2/3 decision classification, phased team composition, hiring
  filters, and an anti-pattern list.
- **`docs/ROLES_30_60_90.md`** - per-role 30/60/90 action plan for
  Phase A (MVP hardening). Interim owners named for the un-staffed
  roles. Critical-chain dependencies + explicit "not building" list.

### Added — ADRs (Architecture Decision Records)

- **`docs/adr/`** - directory with README (lifecycle + numbering),
  `0000-template.md`, and four backfilled ADRs documenting decisions
  already in the codebase:
  - `0001-envelope-encryption.md` - AES-256-GCM with two-level envelope;
    rejects CBC+HMAC and XChaCha20-Poly1305 with reasons.
  - `0002-auth-model.md` - JWT for humans, API keys for agents;
    rejects JWT-everywhere and API-keys-everywhere.
  - `0003-promotion-engine.md` - decrypt-and-rewrap with PROD approval
    gate; explicitly rejects verbatim-ciphertext copy on nonce-reuse
    grounds.
  - `0004-key-hierarchy.md` - two-level hierarchy (master KEK +
    per-project DEK); rejects three-level and per-secret-DEK.
- Each ADR includes file:line references to the actual code, alternative
  options with explicit rejection rationale, consequences, and rollback
  plan.

### Added — 30-day execution artifacts

- **Architecture and security**
  - `docs/ARCHITECTURE.md` - one-page dependency map with three trust
    boundaries (network, identity, key custody); "no upward edges" rule.
  - `docs/THREAT_MODEL.md` re-baselined to v1.2.0 with file:line refs
    and a "Findings new" block surfacing five critical/high gaps the
    v1.1.0 model missed.
  - `docs/VETO_LIST_AUDIT.md` - 90-day commit triage in protected paths
    + proposed CODEOWNERS + commit-message convention.
- **Backend specs**
  - `docs/AUDIT_LOG_COVERAGE.md` - canonical event taxonomy, service
    pattern, test obligations, read-path deferral note.
  - `docs/ERROR_HANDLING_STANDARD.md` - `httperror` package design,
    handler/service pattern, migration plan, lint rule to forbid
    `err.Error()` in `c.JSON`.
- **Frontend specs**
  - `docs/EMBED_STATE.md` - widget state machine, audit-log requirement
    per transition, auto-clear policy.
  - `docs/EMBED_ORIGIN_POLICY.md` - per-project origin allow-list spec;
    forbids wildcard `'*'` postMessage; framing guard; integrator CSP.
- **DevOps**
  - `docs/SECRET_SOURCES.md` - dev/staging/prod source map per secret;
    rotation cadence; break-glass principles.
  - `docs/CI_PERMISSIONS.md` - job-by-job permission map + branch
    protection requirements.
  - `docs/RUNBOOK.md` extended with §6 deploy-rollback drill and §7
    break-glass production secret read procedure.
- **QA**
  - `tests/PYRAMID.md` - package-by-package census; flags the
    "inverted pyramid" shape.
  - `tests/NEGATIVE_AUTH_PLAN.md` - 12-endpoint × 11-attacker-case
    matrix; every cell missing today.
  - `tests/FLAKY.md` - triage SLA + fix patterns + quarantine etiquette.
- **PM / UX / Tech Writer (interim Tech Lead)**
  - `docs/ROADMAP_NOT.md` - explicit non-goals + exception process.
  - `docs/UX_STATE_INVENTORY.md` - per-screen state inventory; rule
    that destructive actions must use typed confirmation, not
    `window.confirm`.
  - `docs/DOCS_SANITIZATION_AUDIT.md` - example-value rules + detector
    script spec.

### Added — code / tooling

- `.github/workflows/ci.yml` - top-level
  `permissions: contents: read` (least-privilege CI default).

### Changed — `docs/FOLLOWUPS.md`

Re-shaped from the v1.1.0 audit delta into a persistent tracker with
owner + due date per item. Top 10 Phase A items are ordered by leverage;
Phase B items captured but explicitly deferred. New P0 / P1 items
discovered during the 30-day audit are slotted ahead of the original
list:
- Secret/Project/API-key mutations are not audit-logged today.
- Multiple handlers return `err.Error()` directly, leaking
  `pgx`/`pq`/`crypto/cipher` text to clients.
- The embed widget accepts auth from any origin
  (no `ev.origin` check, outbound target origin `'*'`).
- No handler-level negative-auth tests exist anywhere.
- Approver-cannot-be-requester invariant is unverified at the DB layer.

### Fixed — in this branch's earlier commits

- Backend: `gofmt` clean (13 files reformatted); fixed a password-policy
  test typo (`wantErr: false` on a too-short password); fixed the
  Prometheus histogram renderer which was emitting invalid format
  (`name{label="x"}_count` instead of `name_count{label="x"}`).
- Frontend: cleared every `npm audit` finding (high `picomatch` ReDoS
  advisory and follow-on `esbuild`/`vitest` chain); bumped `vitest` 2 → 4.1.6.
- Backend deps: bumped `x/net`, `x/crypto`, `x/text`, `x/sys`,
  `protobuf`, `gin`, `validator`, `lib/pq`, `mysql`, `sqlite3`.

### Notes

- All changes are docs and one workflow line; no API or runtime
  behavior changes.
- CI was blocked on a GitHub Actions quota during development; the
  changes have been verified locally (`go vet`, `gofmt`, `go test`,
  `npm test`, `npm audit`, `tsc --noEmit`, `npm run build`).

---

## [1.1.0] - 2026-04-19

**Phase 16 - Production Hardening.** Closes every open item in
`SECURITY_AUDIT.md` "Recommendations for Production Deployment". See
`PHASE16_CHANGELOG.md` for the full summary.

### Added

- **Pluggable MasterKeyProvider** (`backend/internal/crypto/keyprovider/`)
  - `Provider` interface with `Name / GetMasterKey / Rotate`
  - `EnvProvider` (default) wraps the existing `MASTER_KEY` base64 path
  - `AWSKMSProvider` and `GCPKMSProvider` accept narrow decrypter
    interfaces so SDK wiring stays out of this package
  - `VaultProvider` calls HashiCorp Vault Transit via stdlib `net/http`
  - Table-driven tests with fakes for every provider
  - Selected at runtime via `KEEPSAVE_KEY_PROVIDER=env|awskms|gcpkms|vault`
- **In-app TLS** - `TLS_CERT_FILE` + `TLS_KEY_FILE` enable TLS 1.2+
  listener; `TLS_REDIRECT=true` runs an 80->443 redirect goroutine;
  `TLS_CIPHER_SUITES` takes a comma-separated IANA name list
- **Dependabot config** for gomod, npm (frontend + nodejs SDK), pip
  (python SDK), docker (backend + frontend), and github-actions
- **GitHub secret scanning config** (`.github/secret_scanning.yml`)
- **Frontend npm audit** CI job; blocks `docker-build` on high+ findings
- **`govulncheck` gating** - `security-scan` is now a `docker-build`
  dependency; previously advisory
- **Audit log retention knob** - `AUDIT_LOG_RETENTION_DAYS` env var
  (default 365); nightly pruner lands in 1.1.1
- **Docs** - `docs/THREAT_MODEL.md`, `docs/RUNBOOK.md`,
  `docs/PENTEST_CHECKLIST.md`, `docs/SEIDR_INTEGRATION.md`
- **Config tests** - `backend/internal/config/config_test.go` covers
  defaults, prod lockdown, and KMS-provider skip-MASTER_KEY behavior

### Changed

- `config.Load()` now reads `KEEPSAVE_ENV`, `KEEPSAVE_KEY_PROVIDER`,
  `TLS_*`, `AUDIT_LOG_RETENTION_DAYS`, `VAULT_*`, and KMS params;
  the existing `MasterKey []byte` field remains populated only when
  `KEEPSAVE_KEY_PROVIDER=env`
- `KEEPSAVE_ENV=production` refuses `CORS_ORIGINS=*`
- `KEEPSAVE_ENV=production` refuses `sslmode=disable` in `DATABASE_URL`
- `Content-Security-Policy` drops `'unsafe-inline'` from `style-src`
  and adds `frame-ancestors 'none'`, `base-uri 'self'`,
  `form-action 'self'`, `connect-src 'self'`
- `Strict-Transport-Security` gains `preload`
- Helm chart bumped to 1.1.0 with KMS env plumbing, optional TLS volume
  mount, and sslmode=require default `DATABASE_URL`
- Frontend bumped to 1.1.0; Go/Node/Python SDKs bumped 2.0.0 -> 2.1.0
  (SDK line stays on 2.x to avoid a downgrade for existing consumers)

### Security

- **SECURITY_AUDIT.md "Recommendations for Production Deployment" fully
  closed.** Each of the 8 items now shows a tick with a link to the
  commit, config file, or docs page that closed it.
- Master key now cached in memory only; never written to disk
- TLS 1.2+ enforced at the application tier when in-app TLS is enabled

### Known follow-ups

- AWS/GCP KMS SDK adapters (pending `go mod tidy` with real SDK deps)
- Phase 15 service unit tests (feature-complete but untested)
- Backup tamper-detection test formalizing AEAD-auth guarantee
- Audit-log pruner consumer wiring in `main.go`
- Roadmap.md Phase 15 checkboxes and README.md feature-phases table

---

## [1.0.0] - 2026-03-14

### Added

- **Phase 7: Observability & Monitoring** - Full observability stack
  - **Prometheus metrics** - Custom metrics collector with counters, gauges, and histograms; `/metrics` endpoint in Prometheus exposition format
  - **Metrics middleware** - Per-request tracking of latency, error rates, in-flight requests, rate limit hits
  - **Application metrics** - `keepsave_http_requests_total`, `keepsave_secrets_encrypted_total`, `keepsave_auth_attempts_total`, `keepsave_promotions_total`, and 10 more metrics
  - **Distributed tracing** - Request tracing with trace/span ID propagation via `X-Trace-ID`/`X-Span-ID` headers
  - **Tracing middleware** - Automatic span creation for every HTTP request with status, method, and IP attributes
  - **Admin dashboard** - System health overview page with traces, events, plugins, and security event views
  - **Test suite** - 8 metrics tests (counters, gauges, histograms, concurrency, formatting) + 6 tracing tests (spans, propagation, limits, eviction)

- **Phase 8: SDK & Developer Experience** - Multi-language SDKs and API documentation
  - **Python SDK** (`sdks/python/`) - Full-featured client with auth, projects, secrets, promotions, key rotation, import/export
  - **Node.js SDK** (`sdks/nodejs/`) - TypeScript client with complete type definitions and all API endpoints
  - **Go SDK** (`sdks/go/`) - Idiomatic Go client with context support, functional options pattern, and error types
  - **OpenAPI 3.0 specification** - Interactive API documentation served at `/api/docs` with schemas for all resources
  - All SDKs support both JWT token and API key authentication

- **Phase 9: Enterprise Features** - Enterprise-grade security and compliance
  - **SSO integration** - OIDC/SAML provider configuration per organization with encrypted client secret storage
  - **IP allowlisting** - CIDR-based IP restrictions per organization and per project
  - **Compliance reports** - SOC 2, GDPR, and PCI report generation with automated security posture assessment
  - **Secret lifecycle policies** - Configurable max age, rotation reminders, and mandatory rotation per project
  - **Encrypted backups** - Full/incremental backup snapshots with AES-256-GCM encryption
  - **Security event logging** - Dedicated security events table with severity levels and detailed context
  - Database migration `005_phase7_12.sql` with 11 new tables for all Phase 7-12 features

- **Phase 10: Security Hardening Deep Dive** - Production security hardening
  - **CSRF protection** - Single-use CSRF tokens for all mutation endpoints; API key requests exempted
  - **Security headers middleware** - HSTS, X-Content-Type-Options, X-Frame-Options, CSP, Referrer-Policy, Permissions-Policy
  - **Request body size limits** - 1MB default limit to prevent payload DoS
  - **Password complexity** - Configurable policy requiring uppercase, lowercase, digits, and special characters
  - **Password strength scoring** - 0-4 strength score for UI feedback
  - **Session token tracking** - Database-backed session tokens for revocation support
  - **Test suite** - Password policy validation tests and strength scoring tests

- **Phase 11: AI Agent Experience** - Purpose-built features for AI agents
  - **Just-in-time secret leases** - Time-limited access grants (1 min to 24 hours) with automatic expiration
  - **Lease management** - Create, list active, and revoke leases via API
  - **Agent activity tracking** - Per-action logging with API key, project, environment, secret key, and IP
  - **Activity analytics** - Action summaries grouped by type with last-used timestamps
  - **Access heatmaps** - Secret access frequency data bucketed by hour for the last 7 days
  - **Lease and activity tables** - `secret_leases` and `agent_activities` tables with proper indexing

- **Phase 12: Platform Ecosystem** - Extensible platform architecture
  - **Event bus** - In-process event bus with persistent event log, pub/sub pattern, wildcard subscribers, and event replay
  - **Event types** - 16 defined event types: `secret.created`, `promotion.completed`, `key.rotated`, `lease.created`, etc.
  - **Plugin system** - Registry for secret providers, notification senders, and validators with interface contracts
  - **Plugin management** - Database-backed plugin registry with enable/disable toggle
  - **Access policies** - Time-window, IP-restriction, and geo-restriction policy types per project
  - **Platform API** - Events listing, event replay, plugin registration, and access policy CRUD
  - **Test suite** - 5 event bus tests (pub/sub, wildcards, payloads) + 5 plugin registry tests (providers, senders, validators)

- **Frontend updates**
  - Admin Dashboard page with 5 tabs: Overview, Traces, Events, Plugins, Security
  - Navigation updated with Admin link
  - API client extended with 20+ new endpoint functions for all Phase 7-12 features

### Fixed

- **API key error display** - Frontend showed `[object Object]` instead of actual error message when API key creation failed; fixed error extraction from nested `{ error: { code, message } }` response format
- **API key scopes parsing** - Empty scopes input no longer sends `[""]` to the backend; added `.filter(Boolean)` to scope splitting
- **API key empty projects guard** - Create form now shows informational message when no projects exist instead of rendering a broken form
- **API key environment validation** - Backend now rejects invalid environment values with binding tag `oneof=alpha uat prod`
- **API key scope validation** - Backend now validates each scope value must be `read` or `write`
- **API key project authorization** - Backend verifies project ownership before creating API key; returns 403 for unauthorized access and 404 for missing projects
- **API key delete authorization** - Delete query now filters by `user_id` to prevent users from deleting other users' API keys; returns error when key not found or not owned

### Changed

- **Router** - Extended with 30+ new endpoints across admin, enterprise, agent, and platform route groups
- **Main entry point** - Wired all new services, repositories, and handlers for Phases 7-12
- **Health endpoint** - Version bumped to 1.0.0

---

## [0.6.0] - 2026-03-14

### Added

- **Completed Roadmap** - Extended project roadmap from 9 to 12 phases with detailed task breakdowns
- **idea.md** - Comprehensive ideas document with 36 improvement proposals
- Updated milestone summary table with Phase 10-12

---

## [0.5.0] - 2026-03-13

### Added

- **Phase 5: Hardening and Operations** - Rate limiting, key rotation, secret versioning, webhooks, health checks, structured logging, Helm chart, CI/CD pipeline, OWASP security audit checklist
- Database migration for `secret_versions`, `webhook_configs`, and `webhook_deliveries` tables
- Comprehensive test suite expansion (49+ backend tests)

---

## [0.3.0] - 2026-03-13

### Added

- **Frontend Dashboard** - React 18 + TypeScript + Vite web application with auth, project management, secret CRUD, environment switcher, promotion wizard, diff review, audit log viewer, API key management
- Docker support with Nginx reverse proxy for frontend container
- Comprehensive frontend test suite (27 tests across 5 test files)

---

## [0.2.0] - 2026-03-13

### Added

- **Environment Promotion Engine** - Promote secrets between environments (Alpha -> UAT -> PROD) with diff preview, rollback, approval workflow, and audit logging
- Database migration for `promotion_requests` and `secret_snapshots` tables

---

## [0.1.0] - 2026-03-13

### Added

- **Project scaffold** - Go 1.22+ backend with Gin framework, PostgreSQL 16, Docker Compose
- **Database schema** - `users`, `projects`, `environments`, `secrets`, `api_keys`, `audit_log` tables
- **AES-256-GCM encryption** - Envelope encryption with per-project data encryption keys (DEK)
- **Full CRUD API** for projects and secrets with JWT auth and API keys
- **Unit tests** - Crypto layer, auth service, and JWT validation
