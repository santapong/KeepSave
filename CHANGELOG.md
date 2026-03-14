# Changelog

All notable changes to KeepSave will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [0.6.0] - 2026-03-14

### Added

- **Completed Roadmap** - Extended project roadmap from 9 to 12 phases with detailed task breakdowns
  - **Phase 10: Security Hardening Deep Dive** - CSRF protection, security headers, secure token storage, per-endpoint rate limiting, password complexity, API key lifecycle management, audit log integrity
  - **Phase 11: AI Agent Experience** - Just-in-time secret access, agent activity dashboard, advanced sandboxing with granular scopes, natural language secret query, agent SDK with caching and circuit breaker
  - **Phase 12: Platform Ecosystem** - Event-driven architecture (NATS/Redis Streams), plugin system for secret providers and notifications, GraphQL API with subscriptions, secret policies engine, multi-region disaster recovery
- **idea.md** - Comprehensive ideas document with 36 improvement proposals across security, developer experience, AI agent integration, infrastructure, and architecture categories
  - **16 security improvements** prioritized by severity (critical, high, medium, low) based on codebase audit
  - **8 feature ideas** for developer experience and AI agent integration
  - **4 architecture proposals** for event-driven design, plugin system, GraphQL API, and edge caching
- Updated milestone summary table with Phase 10-12

---

## [0.5.0] - 2026-03-13

### Added

- **Phase 5: Hardening and Operations** - Production-grade reliability, security, and deployment features
  - **Rate limiting middleware** - Per-IP token bucket rate limiter with configurable burst capacity and automatic refill
  - **Health check endpoints** - `/healthz` (liveness) and `/readyz` (readiness with database connectivity check) for Kubernetes probes
  - **Structured JSON logging** - Structured log entries with timestamps, levels, and fields; Gin request logging middleware with latency, status codes, user IDs
  - **Key rotation mechanism** - DEK rotation per-project with automatic re-encryption of all secrets across all environments; bulk rotation for all user projects; encryption verification endpoint
  - **Secret versioning** - Historical version tracking for secrets with version numbering, pruning of old versions (keep N), and version retrieval API
  - **Webhook notifications** - Configurable webhook endpoints per project with event filtering, HMAC-SHA256 signature verification, retry with exponential backoff (3 attempts), and delivery logging
  - **CI/CD pipeline** - GitHub Actions workflow with backend lint (go vet, gofmt), backend tests with race detection, frontend TypeScript check, frontend tests, Docker image builds, and govulncheck security scanning
  - **Helm chart** - Kubernetes deployment chart with backend/frontend deployments, services, ingress, secrets management, HPA autoscaling, liveness/readiness probes, and configurable resource limits
  - **Security audit checklist** - Complete OWASP Top 10 review with mitigation status for each category and production deployment recommendations
- Database migration for `secret_versions`, `webhook_configs`, and `webhook_deliveries` tables
- New API endpoints:
  - `POST /api/v1/projects/:id/rotate-keys` - Rotate project encryption keys
  - `GET /api/v1/projects/:id/verify-encryption` - Verify all secrets can be decrypted
  - `POST /api/v1/rotate-keys` - Rotate keys for all user projects
  - `GET /api/v1/projects/:id/secrets/:secretId/versions` - List secret version history
  - `GET /api/v1/projects/:id/secrets/:secretId/versions/:version` - Get specific version
  - `POST /api/v1/projects/:id/webhooks` - Register webhook endpoint
  - `GET /api/v1/projects/:id/webhooks` - List project webhooks
  - `DELETE /api/v1/projects/:id/webhooks` - Remove project webhooks
  - `GET /api/v1/webhook-deliveries` - View webhook delivery log
  - `GET /healthz` - Liveness probe
  - `GET /readyz` - Readiness probe with DB check
- Comprehensive test suite expansion (49+ backend tests):
  - Rate limiter tests: burst capacity, refill, client isolation, concurrency safety, middleware integration
  - Health endpoint tests: liveness response validation
  - Structured logging tests: level filtering, JSON format, nil fields, newline separation
  - Webhook service tests: registration, removal, event delivery, filtering, wildcard events, HMAC signatures, delivery logging
  - Key rotation tests: service construction, rotation result fields, encrypt-decrypt cycle simulation, DEK uniqueness
  - **High load tests**: concurrent request handling (5000 requests, 100 goroutines), rate limiter under stress (23K+ RPS), 200 concurrent client isolation, memory stability (10K clients), sequential latency (<2µs avg)
  - **Crypto benchmarks**: concurrent encryption (250K ops/sec), envelope encryption load test, large payload throughput (64B-64KB, up to 1.3 GB/s), parallel encrypt benchmark, DEK generation benchmark

---

## [0.3.0] - 2026-03-13

### Added

- **Frontend Dashboard** - React 18 + TypeScript + Vite web application for managing secrets and promotions
  - **Auth pages** - Login and registration forms with JWT token management and localStorage persistence
  - **Project management** - List, create, and delete projects from the dashboard
  - **Secret management UI** - Full CRUD for secrets with environment switcher (Alpha / UAT / PROD tabs)
  - **Mask/reveal toggle** - Secret values masked by default with one-click reveal
  - **Inline editing** - Edit secret values directly in the table view
  - **Promotion wizard** - Step-by-step workflow: select promotion path, configure override policy, preview diff, confirm
  - **Diff review** - Checkbox-based key selection with add/update/no_change action indicators
  - **Promotion history** - View all promotions with approve, reject, and rollback actions
  - **Audit log viewer** - Table of all project audit events with timestamps, actions, and JSON details
  - **API key management** - Create, list, and delete scoped API keys with one-time raw key display
  - **Responsive navigation** - Top nav bar with project/API key links and user session controls
- Docker support with Nginx reverse proxy for frontend container
- Updated `docker-compose.yml` with frontend service
- Vite proxy configuration for local development (`/api` → backend)
- Comprehensive frontend test suite (27 tests across 5 test files)
  - API client tests: auth state management, request/response handling, error handling, 204 responses
  - SecretsPanel tests: environment switching, reveal/hide, CRUD operations, empty and error states
  - PromotionWizard tests: configure step, diff preview, promotion execution, PROD approval flow
  - LoginPage tests: form rendering, successful login, error display, register link
  - ProjectsPage tests: list rendering, empty state, project creation

---

## [0.2.0] - 2026-03-13

### Added

- **Environment Promotion Engine** - Promote secrets between environments (Alpha -> UAT -> PROD)
  - `POST /api/v1/projects/:id/promote` - Request promotion of secrets between environments
  - `POST /api/v1/projects/:id/promote/diff` - Preview what will change before promoting
  - `GET /api/v1/projects/:id/promotions` - List all promotion requests for a project
  - `GET /api/v1/projects/:id/promotions/:promotionId` - Get promotion request details
  - `POST /api/v1/projects/:id/promotions/:promotionId/approve` - Approve a pending PROD promotion
  - `POST /api/v1/projects/:id/promotions/:promotionId/reject` - Reject a pending promotion
  - `POST /api/v1/projects/:id/promotions/:promotionId/rollback` - Rollback a completed promotion
  - `GET /api/v1/projects/:id/audit-log` - View audit trail for a project
- **Promotion rules engine** - Filter which keys to promote, choose override policy (`skip` or `overwrite`)
- **Diff view** - See exactly what will be added or changed in the target environment before promoting
- **Rollback support** - Automatic snapshots of target environment secrets before promotion; one-click restore
- **Approval workflow** - PROD promotions require explicit approval; non-PROD promotions execute immediately
- **Audit logging** - Every promotion request, approval, rejection, completion, and rollback is logged with full context
- Database migration for `promotion_requests` and `secret_snapshots` tables
- Secret upsert support for promotion operations
- Table-driven unit tests for promotion validation logic

---

## [0.1.0] - 2026-03-13

### Added

- **Project scaffold** - Go 1.22+ backend with Gin framework, PostgreSQL 16, Docker Compose
- **Database schema** - `users`, `projects`, `environments`, `secrets`, `api_keys`, `audit_log` tables
- **AES-256-GCM encryption** - Envelope encryption with per-project data encryption keys (DEK)
- **Project CRUD API** - `POST/GET/PUT/DELETE /api/v1/projects`
- **Secret CRUD API** - `POST/GET/PUT/DELETE /api/v1/projects/:id/secrets`
- **Environment scoping** - Automatic Alpha, UAT, PROD environments per project
- **JWT authentication** - Register and login endpoints with token-based auth
- **API key generation** - Scoped API keys for AI Agent access (`ks_` prefixed, SHA-256 hashed)
- **Dual auth middleware** - Endpoints accept JWT tokens or API keys
- **Request validation** - Input validation with Gin binding tags
- **Unit tests** - Tests for crypto layer, auth service, and JWT validation
