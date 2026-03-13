# Changelog

All notable changes to KeepSave will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
