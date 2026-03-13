# KeepSave

Secure environment variable storage and promotion system for AI Agents and development teams.

## Problem

AI Agents and CI/CD pipelines need access to environment variables (API keys, database URLs, feature flags), but storing them in `.env` files, code, or chat logs exposes sensitive data. Promoting configurations between environments (Alpha -> UAT -> PROD) is manual and error-prone.

## Solution

KeepSave provides:

- **Encrypted vault** - AES-256-GCM encryption at rest for all secret values
- **Environment promotion** - One-click promotion pipeline: Alpha -> UAT -> PROD with diff review, audit trail, and rollback
- **API key access for AI Agents** - Scoped API keys let agents fetch secrets without exposing them in prompts or code
- **Embeddable widget** - A `<keepsave-widget>` Web Component that integrates into any website via a single `<script>` tag

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  Frontend                        │
│  ┌──────────────┐    ┌────────────────────────┐  │
│  │  Dashboard   │    │  Embeddable Widget     │  │
│  │  (React SPA) │    │  (<keepsave-widget>)   │  │
│  └──────┬───────┘    └───────────┬────────────┘  │
│         │                        │               │
└─────────┼────────────────────────┼───────────────┘
          │         HTTPS          │
┌─────────▼────────────────────────▼───────────────┐
│                  Backend (Go + Gin)               │
│  ┌──────────┐ ┌───────────┐ ┌─────────────────┐  │
│  │   Auth   │ │  Secrets  │ │   Promotion     │  │
│  │  (JWT +  │ │  CRUD +   │ │   Engine +      │  │
│  │ API Key) │ │ Encryption│ │   Audit Log     │  │
│  └──────────┘ └───────────┘ └─────────────────┘  │
└──────────────────────┬───────────────────────────┘
                       │
              ┌────────▼────────┐
              │   PostgreSQL    │
              │  (encrypted     │
              │   at rest)      │
              └─────────────────┘
```

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.22+ (for local backend development)
- Node.js 20+ (for local frontend development)

### Run with Docker Compose

```bash
# Clone the repository
git clone https://github.com/santapong/KeepSave.git
cd KeepSave

# Generate a master encryption key
export MASTER_KEY=$(openssl rand -base64 32)

# Start all services
docker-compose up --build
```

The API will be available at `http://localhost:8080` and the dashboard at `http://localhost:3000`.

### Local Development

```bash
# Backend
cd backend
cp .env.example .env        # Configure database and keys
go run ./cmd/server

# Frontend
cd frontend
npm install
npm run dev
```

## API Overview

### Authentication

```bash
# Register
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "secure-password"}'

# Generate API key (for AI Agent use)
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer <jwt-token>" \
  -d '{"name": "my-agent", "project_id": "...", "scopes": ["read"]}'
```

### Secrets

```bash
# Store a secret
curl -X POST http://localhost:8080/api/v1/projects/:id/secrets \
  -H "X-API-Key: ks_xxxx" \
  -d '{"key": "DATABASE_URL", "value": "postgres://...", "environment": "alpha"}'

# Retrieve secrets (values are decrypted server-side, returned over TLS)
curl http://localhost:8080/api/v1/projects/:id/secrets?environment=alpha \
  -H "X-API-Key: ks_xxxx"
```

### Environment Promotion

```bash
# Preview what will change (diff)
curl -X POST http://localhost:8080/api/v1/projects/:id/promote/diff \
  -H "Authorization: Bearer <jwt-token>" \
  -d '{"source_environment": "alpha", "target_environment": "uat"}'

# Promote alpha -> uat (executes immediately)
curl -X POST http://localhost:8080/api/v1/projects/:id/promote \
  -H "Authorization: Bearer <jwt-token>" \
  -d '{"source_environment": "alpha", "target_environment": "uat", "override_policy": "skip"}'

# Promote uat -> prod (requires approval)
curl -X POST http://localhost:8080/api/v1/projects/:id/promote \
  -H "Authorization: Bearer <jwt-token>" \
  -d '{"source_environment": "uat", "target_environment": "prod"}'

# Approve a pending PROD promotion
curl -X POST http://localhost:8080/api/v1/projects/:id/promotions/:promotionId/approve \
  -H "Authorization: Bearer <jwt-token>"

# Rollback a completed promotion
curl -X POST http://localhost:8080/api/v1/projects/:id/promotions/:promotionId/rollback \
  -H "Authorization: Bearer <jwt-token>"

# View audit log
curl http://localhost:8080/api/v1/projects/:id/audit-log \
  -H "Authorization: Bearer <jwt-token>"
```

## Embedding the Widget

Add the widget to any website:

```html
<script src="https://your-keepsave-host/embed/keepsave-widget.js"></script>
<keepsave-widget
  api-url="https://your-keepsave-host"
  project-id="your-project-id"
  theme="dark">
</keepsave-widget>
```

## Project Documentation

| Document                  | Description                          |
|---------------------------|--------------------------------------|
| [CLAUDE.md](./CLAUDE.md)  | Development guide and conventions    |
| [Roadmap.md](./Roadmap.md)| Phased delivery plan                 |

## Security

- All secret values are encrypted using AES-256-GCM with envelope encryption
- Master key never stored in the database
- API keys are scoped per-project and per-environment
- Full audit log for all secret access and promotion events
- CORS restrictions for the embeddable widget

## License

MIT
