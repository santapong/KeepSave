# KeepSave Roadmap

## Vision

A secure, self-hosted vault for environment variables with a controlled promotion pipeline (Alpha -> UAT -> PROD) and an embeddable frontend widget that integrates into any website.

---

## Phase 1 - Foundation (Core Backend + Storage)

**Goal:** Encrypted secret storage with CRUD API and basic auth.

### Backend
- [x] Project scaffold (Go + Gin, PostgreSQL, Docker Compose)
- [x] Database schema: `projects`, `environments`, `secrets`, `audit_log`
- [x] AES-256-GCM encryption layer with envelope encryption
- [x] CRUD API for projects (`POST/GET/PUT/DELETE /api/v1/projects`)
- [x] CRUD API for secrets (`POST/GET/PUT/DELETE /api/v1/projects/:id/secrets`)
- [x] Environment scoping (Alpha, UAT, PROD per project)
- [x] JWT authentication (register/login)
- [x] API key generation for AI Agent access
- [x] Request validation and error handling
- [x] Unit tests for crypto and service layers

### Deliverables
- Running API that stores and retrieves encrypted secrets
- API key auth so an AI Agent can read secrets without exposing them in code

---

## Phase 2 - Environment Promotion Engine

**Goal:** Move secrets safely between Alpha -> UAT -> PROD.

### Backend
- [x] Promotion endpoint (`POST /api/v1/projects/:id/promote`)
- [x] Promotion rules engine (which keys to promote, override policy)
- [x] Diff view: show what will change in target environment
- [x] Audit log for every promotion event
- [x] Rollback support (restore previous secret set)
- [x] Optional approval workflow for PROD promotions
- [x] Integration tests for promotion flows

### Deliverables
- One-command promotion of secrets between environments
- Full audit trail of who promoted what and when

---

## Phase 3 - Frontend Dashboard

**Goal:** Web UI for managing projects, secrets, and promotions.

### Frontend
- [x] Project scaffold (React 18 + TypeScript + Vite)
- [x] Auth pages (login, register)
- [x] Project list and detail views
- [x] Secret management UI (add, edit, mask/reveal, delete)
- [x] Environment switcher (Alpha / UAT / PROD tabs)
- [x] Promotion wizard (select env, review diff, confirm)
- [x] Audit log viewer
- [x] API key management page
- [x] Responsive layout

### Deliverables
- Fully functional dashboard to manage secrets and promotions via browser

---

## Phase 4 - Embeddable Widget

**Goal:** A `<keepsave-widget>` Web Component that third-party sites can embed.

### Frontend (Embed SDK)
- [x] Web Component shell using Shadow DOM
- [x] Configurable via HTML attributes (`project-id`, `api-url`, `theme`)
- [x] Auth flow via postMessage handshake with parent page
- [x] Read-only and read-write modes
- [x] Theming support (light/dark, custom CSS variables)
- [x] NPM package + CDN script for easy integration
- [x] Integration guide and example host page

### Deliverables
- Single `<script>` tag inclusion for any website
- Isolated widget that does not leak styles or clash with host page

---

## Phase 5 - Hardening and Operations

**Goal:** Production-grade reliability and security.

- [x] Rate limiting and request throttling
- [x] Key rotation mechanism (re-encrypt secrets with new master key)
- [x] Secret versioning (keep N previous values)
- [x] Webhook notifications on promotion events
- [x] Health check and readiness endpoints
- [x] Structured JSON logging
- [x] Helm chart for Kubernetes deployment
- [x] CI/CD pipeline (GitHub Actions: lint, test, build, push image)
- [x] Security audit checklist (OWASP top 10 review)

### Deliverables
- Production-ready deployment with Kubernetes Helm chart
- Comprehensive CI/CD pipeline with security scanning
- Rate limiting, structured logging, and health checks for operational monitoring
- Key rotation and secret versioning for security compliance
- Webhook notifications for event-driven integrations

---

## Phase 6 - Advanced Features

**Goal:** CLI tooling, RBAC, multi-tenant SaaS, templates, integrations, and dependency analysis.

### Backend
- [x] Multi-tenant SaaS mode with organization accounts
- [x] RBAC with custom roles (viewer, editor, admin, promoter)
- [x] Secret templates (predefined sets for common stacks)
- [x] CLI tool (`keepsave pull`, `keepsave push`, `keepsave promote`)
- [x] Integration plugins (GitHub Actions, GitLab CI, Terraform provider)
- [x] Import/export from `.env` files
- [x] Secrets dependency graph (detect references between secrets)
- [x] Unit tests for organization, RBAC, dependency, and env file services

### Frontend
- [x] Organizations management page (create, members, roles)
- [x] Templates page (create, apply builtin/custom templates)
- [x] Updated navigation with Organizations and Templates links
- [x] API client extended with all Phase 6 endpoints

### CLI
- [x] `keepsave pull` - Pull secrets as env/json/table format
- [x] `keepsave push` - Push .env file to project environment
- [x] `keepsave promote` - Promote secrets between environments
- [x] `keepsave export` - Export secrets as .env content
- [x] `keepsave import` - Import secrets from .env file
- [x] `keepsave projects` - List all projects
- [x] `keepsave login` - Authenticate with API

### Integrations
- [x] GitHub Actions action (pull secrets, export as env/file/json)
- [x] GitLab CI templates (pull, pull-to-file, promote)
- [x] Terraform provider (external data source + local file)

### Deliverables
- CLI tool for AI agents and CI/CD pipelines
- Multi-tenant organization management with role-based access control
- Secret templates for rapid project bootstrapping
- CI/CD integration plugins for GitHub Actions, GitLab CI, and Terraform
- .env file import/export for easy migration
- Dependency graph analysis for secret reference tracking

---

## Milestone Summary

| Phase | Name                  | Key Outcome                              |
|-------|-----------------------|------------------------------------------|
| 1     | Foundation            | Encrypted secret storage + API key auth  |
| 2     | Promotion Engine      | Alpha -> UAT -> PROD pipeline            |
| 3     | Frontend Dashboard    | Full web UI for secret management        |
| 4     | Embeddable Widget     | `<keepsave-widget>` for third-party sites |
| 5     | Hardening             | Production readiness                     |
| 6     | Advanced Features     | CLI, RBAC, SaaS, integrations            |
| 7     | Observability & Monitoring | Metrics, dashboards, alerting        |
| 8     | SDK & Developer Experience | Language SDKs, plugin ecosystem      |
| 9     | Enterprise Features   | SSO, compliance, disaster recovery       |
| 10    | Security Hardening    | CSRF, security headers, token hardening  |
| 11    | AI Agent Experience   | JIT access, sandboxing, activity dashboard |
| 12    | Platform Ecosystem    | Event bus, plugin system, GraphQL        |

---

## Phase 7 - Observability & Monitoring

**Goal:** Full observability stack for production operations.

- [x] Prometheus metrics endpoint (`/metrics`) for request latency, error rates, and encryption throughput
- [x] Grafana dashboard templates for KeepSave monitoring
- [x] OpenTelemetry tracing for request flows across services
- [x] Alerting rules for failed promotions, encryption errors, and rate limit breaches
- [x] Secret access analytics (who accessed what, frequency heatmaps)
- [x] Admin dashboard with system health overview

### Deliverables
- Production monitoring with Prometheus + Grafana
- Distributed tracing for debugging complex promotion flows
- Proactive alerting for security-relevant events

---

## Phase 8 - SDK & Developer Experience

**Goal:** Language SDKs and ecosystem tooling for seamless integration.

- [x] Official SDKs: Python, Node.js, Go, Rust
- [x] SDK auto-generation from OpenAPI spec
- [x] VS Code extension for secret browsing and insertion
- [x] Docker init plugin (inject secrets at container startup without .env files)
- [x] Secret linting (detect hardcoded secrets in code and suggest KeepSave references)
- [x] Interactive API documentation (Swagger UI / Redoc)
- [x] Webhook marketplace (Slack, PagerDuty, Datadog integrations)

### Deliverables
- Drop-in SDKs for major languages
- IDE integration for developer productivity
- Self-documenting API with interactive explorer

---

## Phase 9 - Enterprise Features

**Goal:** Enterprise-grade security, compliance, and disaster recovery.

- [x] SSO integration (SAML 2.0, OIDC) for enterprise identity providers
- [x] IP allowlisting and geo-restriction for API access
- [x] Compliance reports (SOC 2, GDPR data export/deletion)
- [x] Automated secret expiration and rotation policies
- [x] Cross-region replication for disaster recovery
- [x] Backup and restore tooling with encrypted snapshots
- [x] Breakglass access for emergency secret retrieval with enhanced audit
- [x] Custom approval chains (N-of-M approvals for PROD promotions)

### Deliverables
- Enterprise SSO and compliance tooling
- Automated secret lifecycle management
- Disaster recovery with cross-region replication

---

## Phase 10 - Security Hardening Deep Dive

**Goal:** Address all critical and high-priority security gaps identified in the security audit for production readiness.

### HTTP Security
- [x] CSRF token validation middleware for all state-changing endpoints (POST/PUT/DELETE)
- [x] Security headers middleware (HSTS, X-Content-Type-Options, X-Frame-Options, CSP, Referrer-Policy, Permissions-Policy)
- [x] Request body size limits (`MaxBytesReader` - 1MB default, configurable per endpoint)
- [x] CORS lockdown: validation and whitelisting of specific origins for production

### Authentication Hardening
- [x] Migrate frontend token storage from `localStorage` to `httpOnly` + `secure` + `sameSite=strict` cookies
- [x] Implement refresh token rotation with short-lived access tokens (15 min) and long-lived refresh tokens (7 days)
- [x] Token revocation support (server-side token blacklist with Redis/DB backing)
- [x] Password complexity enforcement (uppercase, lowercase, digit, special character requirements)
- [x] Breached password detection via Have I Been Pwned k-anonymity API
- [x] Force re-authentication for sensitive operations (key rotation, PROD promotion, API key creation)

### Rate Limiting Improvements
- [x] Per-endpoint rate limiting with stricter limits on auth endpoints (5/min per IP for login/register)
- [x] Rate limit by API key for authenticated endpoints (not just IP-based)
- [x] Trusted proxy configuration via `SetTrustedProxies()` to prevent X-Forwarded-For spoofing
- [x] `Retry-After` header on 429 responses

### API Key Security
- [x] Default 90-day expiration for new API keys (with opt-in override for no expiration)
- [x] API key last-used timestamp tracking for stale key detection
- [x] Expiration warning notifications 7 days before key expires
- [x] API key usage analytics and anomaly detection

### Frontend Security
- [x] Replace all `innerHTML` usage in embed widget with `textContent` and DOM API
- [x] Disable source maps in production builds
- [x] Add Content Security Policy to Shadow DOM widget
- [x] Password strength meter on registration and password change forms

### Audit & Logging
- [x] Log failed authentication attempts with IP, user agent, and timestamp
- [x] Log rate limit violations as security events
- [x] Audit log integrity checksums (hash chain) to detect log tampering
- [x] Session management: concurrent session limits, idle timeout

### Deliverables
- Production-hardened HTTP layer with CSRF protection and security headers
- Secure token lifecycle with refresh rotation and revocation
- Granular rate limiting that resists proxy-based evasion
- Comprehensive security event logging with tamper detection

---

## Phase 11 - AI Agent Experience

**Goal:** Purpose-built features for AI agents that consume secrets safely and efficiently.

### Just-In-Time Access
- [x] Temporary secret checkout: agent requests time-limited access (e.g., 1 hour) to specific secrets
- [x] Automatic secret lease expiration and revocation
- [x] Checkout audit trail: which agent checked out what, when, and for how long

### Agent Activity Dashboard
- [x] Real-time view of agent secret access patterns across all projects
- [x] Usage frequency heatmaps per API key and per secret
- [x] Anomaly detection: alert when an agent accesses unusual secrets or at unusual times
- [x] Agent session timeline: chronological view of all agent operations

### Advanced Sandboxing
- [x] Granular API key scopes: read-only, write-only, promote-only, admin
- [x] Environment-locked keys (e.g., agent can only access Alpha, never PROD)
- [x] Secret-level access control (whitelist specific keys an agent can access)
- [x] Automatic scope downgrade: if an agent hasn't used a permission in 30 days, revoke it

### Natural Language Secret Query
- [x] NLP endpoint: agents describe what they need in plain English
- [x] Map natural language to project/environment/key lookups
- [x] Fuzzy matching for key names (e.g., "database URL" matches `DATABASE_URL`)
- [x] Context-aware suggestions based on project type and technology stack

### Agent SDK Enhancements
- [x] Automatic secret refresh: SDK detects rotated secrets and re-fetches
- [x] Local encrypted cache: agents cache secrets locally with TTL and AES encryption
- [x] Circuit breaker: graceful degradation if KeepSave API is unreachable
- [x] Batch secret fetch: retrieve multiple secrets in a single API call

### Deliverables
- Time-limited secret access reducing blast radius of compromised agents
- Visibility into agent behavior with anomaly alerting
- Fine-grained access control beyond project/environment scoping
- Agent-friendly SDK with caching, refresh, and resilience patterns

---

## Phase 12 - Platform Ecosystem

**Goal:** Transform KeepSave from a tool into an extensible platform with event-driven architecture and integrations.

### Event-Driven Architecture
- [x] Event bus integration (NATS / Redis Streams) for decoupled event processing
- [x] Event types: `secret.created`, `secret.updated`, `secret.deleted`, `promotion.requested`, `promotion.completed`, `key.rotated`
- [x] Event replay for debugging and disaster recovery
- [x] Consumer groups for reliable event delivery to multiple subscribers

### Plugin System
- [x] Plugin API for custom secret providers (HashiCorp Vault, AWS Secrets Manager, Azure Key Vault, GCP Secret Manager)
- [x] Custom validation plugins (e.g., verify database URLs are reachable, validate API key formats)
- [x] Notification plugins (Slack, PagerDuty, Datadog, Microsoft Teams, Discord)
- [x] Plugin marketplace with community contributions

### GraphQL API
- [x] GraphQL endpoint alongside REST for flexible querying
- [x] Subscriptions for real-time secret change notifications via WebSocket
- [x] Batched queries to reduce API call overhead for dashboard and agents
- [x] Schema-first design with auto-generated documentation

### Secret Policies Engine
- [x] Time-based access windows (e.g., PROD secrets only during business hours)
- [x] IP-based restrictions per API key or user
- [x] Geolocation-based access control with region allowlisting
- [x] Automated secret rotation policies with cloud provider integrations (AWS RDS, GCP Cloud SQL)

### Advanced Secret Features
- [x] Secret references and interpolation: `${DATABASE_HOST}:${DATABASE_PORT}` resolved at read time
- [x] Circular reference detection and validation
- [x] Cross-project secret sharing with access policies
- [x] Secret tagging and search (filter by tags: `database`, `api-key`, `feature-flag`)

### Multi-Region & DR
- [x] Active-passive replication across cloud regions
- [x] Region-aware routing for latency optimization
- [x] Encrypted cross-region sync for secret data
- [x] Automated failover with health-based routing

### Deliverables
- Event-driven architecture decoupling secret management from notification delivery
- Extensible plugin system for secret providers and notification channels
- GraphQL API for flexible data access and real-time subscriptions
- Policy engine for time, location, and role-based access control
- Multi-region deployment for global availability and disaster recovery

---

## Getting Started

This section walks you through setting up KeepSave from scratch for local development or production deployment.

### Prerequisites

| Tool              | Version  | Purpose                          |
|-------------------|----------|----------------------------------|
| Docker            | 20.10+   | Container runtime                |
| Docker Compose    | v2+      | Multi-service orchestration      |
| Go                | 1.22+    | Backend development (local only) |
| Node.js           | 20+      | Frontend development (local only)|
| PostgreSQL        | 16       | Database (or use Docker)         |
| Git               | 2.30+    | Source control                   |

### Option 1: Docker Compose (Recommended)

The fastest way to get the full stack running:

```bash
# 1. Clone the repository
git clone https://github.com/santapong/KeepSave.git
cd KeepSave

# 2. (Optional) Generate a secure master key for non-dev use
#    The docker-compose.yml ships with a dev key that works out of the box
export MASTER_KEY=$(openssl rand -base64 32)

# 3. Start all services (PostgreSQL + API + Frontend)
docker-compose up --build

# 4. Verify the stack is healthy
curl http://localhost:8080/healthz    # API health check
curl http://localhost:8080/readyz     # API readiness (DB connected)
```

**Services:**

| Service   | URL                       | Description          |
|-----------|---------------------------|----------------------|
| API       | http://localhost:8080      | Go backend           |
| Dashboard | http://localhost:3000      | React frontend       |
| Database  | localhost:5432             | PostgreSQL           |

### Option 2: Local Development (Backend + Frontend Separately)

For active development with hot-reloading:

#### 1. Start PostgreSQL

```bash
# Use Docker for the database only
docker run -d \
  --name keepsave-db \
  -e POSTGRES_USER=keepsave \
  -e POSTGRES_PASSWORD=keepsave_dev \
  -e POSTGRES_DB=keepsave \
  -p 5432:5432 \
  postgres:16-alpine
```

#### 2. Start the Backend

```bash
cd backend

# Copy and review environment config
cp .env.example .env

# Install dependencies and run
go mod download
go run ./cmd/server
```

The API starts on http://localhost:8080. Migrations run automatically on startup.

#### 3. Start the Frontend

```bash
cd frontend

# Install dependencies
npm install

# Start dev server (proxies API requests to localhost:8080)
npm run dev
```

The dashboard starts on http://localhost:5173 (Vite dev server).

### First Steps After Setup

```bash
# 1. Register a user account
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com", "password": "your-secure-password"}'

# 2. Log in and get a JWT token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com", "password": "your-secure-password"}'
# Save the returned token

# 3. Create a project
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer <your-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app", "description": "My application secrets"}'

# 4. Store a secret in Alpha
curl -X POST http://localhost:8080/api/v1/projects/<project-id>/secrets \
  -H "Authorization: Bearer <your-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"key": "DATABASE_URL", "value": "postgres://...", "environment": "alpha"}'

# 5. Generate an API key for AI Agent access
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer <your-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-agent-key", "project_id": "<project-id>", "scopes": ["read"]}'
```

Or simply open the **Dashboard** at http://localhost:3000 and do everything through the UI.

### Using the CLI

```bash
# Build the CLI tool
cd backend
go build -o keepsave ./cmd/keepsave

# Authenticate
./keepsave login --api-url http://localhost:8080 --email admin@example.com

# List projects
./keepsave projects

# Pull secrets as environment variables
./keepsave pull --project <project-id> --env alpha --format env

# Import from a .env file
./keepsave import --project <project-id> --env alpha --file .env

# Promote Alpha -> UAT
./keepsave promote --project <project-id> --from alpha --to uat
```

### Running Tests

```bash
# Backend tests (with race detection)
cd backend && go test -race ./...

# Frontend tests
cd frontend && npm test

# Full CI pipeline locally (lint + test + build)
cd backend && go vet ./... && go test -race ./...
cd frontend && npx tsc --noEmit && npm test && npm run build
```

### Embedding the Widget

Add KeepSave to any website with a single script tag:

```html
<script src="http://localhost:3000/embed/keepsave-widget.js"></script>
<keepsave-widget
  api-url="http://localhost:8080"
  project-id="your-project-id"
  theme="light">
</keepsave-widget>
```

### Environment Variables Reference

| Variable         | Required | Default | Description                              |
|------------------|----------|---------|------------------------------------------|
| `DATABASE_URL`   | Yes      | —       | PostgreSQL connection string             |
| `MASTER_KEY`     | Yes      | —       | Base64-encoded 32-byte encryption key    |
| `JWT_SECRET`     | Yes      | —       | Secret for signing JWT tokens            |
| `PORT`           | No       | 8080    | API server listen port                   |
| `CORS_ORIGINS`   | No       | *       | Comma-separated allowed origins          |

### Production Deployment

For Kubernetes deployments, use the included Helm chart:

```bash
helm install keepsave ./helm/keepsave \
  --set config.masterKey="$(openssl rand -base64 32)" \
  --set config.jwtSecret="$(openssl rand -base64 32)" \
  --set config.databaseUrl="postgres://user:pass@db-host:5432/keepsave?sslmode=require"
```

See `helm/keepsave/values.yaml` for all configurable options.
