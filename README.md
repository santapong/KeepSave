# KeepSave

Secure environment variable storage, OAuth 2.0 identity provider, and Central MCP Server Hub for AI Agents and development teams.

## Problem

AI Agents and CI/CD pipelines need access to environment variables (API keys, database URLs, feature flags), but storing them in `.env` files, code, or chat logs exposes sensitive data. Promoting configurations between environments (Alpha -> UAT -> PROD) is manual and error-prone. MCP servers need API tokens to function, but distributing those tokens securely across multiple servers is a challenge.

## Solution

KeepSave provides:

- **Encrypted vault** - AES-256-GCM encryption at rest for all secret values
- **Environment promotion** - One-click promotion pipeline: Alpha -> UAT -> PROD with diff review, audit trail, and rollback
- **API key access for AI Agents** - Scoped API keys let agents fetch secrets without exposing them in prompts or code
- **OAuth 2.0 Provider** - Full identity provider (like Okta) with authorization code, client credentials, PKCE, and refresh token flows
- **Central MCP Server Hub** - Register MCP servers from GitHub, discover and install from a marketplace, and route tool calls through a unified gateway with automatic secret injection
- **Embeddable widget** - A `<keepsave-widget>` Web Component that integrates into any website via a single `<script>` tag

## System Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        Frontend (React + TypeScript)              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────────┐  │
│  │Dashboard │ │ MCP Hub  │ │  OAuth   │ │  Embeddable Widget │  │
│  │(Projects,│ │(Market-  │ │ Clients  │ │ (<keepsave-widget>)│  │
│  │ Secrets, │ │ place,   │ │ Manage-  │ │                    │  │
│  │ Promote) │ │ Install) │ │  ment)   │ │                    │  │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └─────────┬──────────┘  │
└───────┼─────────────┼────────────┼─────────────────┼─────────────┘
        │             │            │                  │
        └─────────────┴─────┬──────┴──────────────────┘
                            │ HTTPS / REST API
┌───────────────────────────▼──────────────────────────────────────┐
│                    Backend (Go + Gin)                             │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                     API Layer (Gin Handlers)                 │ │
│  │  Auth │ Projects │ Secrets │ Promotion │ OAuth │ MCP Hub    │ │
│  └──┬────┴────┬─────┴────┬────┴─────┬─────┴───┬───┴─────┬─────┘ │
│     │         │          │          │         │         │        │
│  ┌──▼─────────▼──────────▼──────────▼─────────▼─────────▼─────┐ │
│  │                    Service Layer                             │ │
│  │                                                             │ │
│  │  AuthService    SecretService    PromotionService           │ │
│  │  ProjectService OAuthService     MCPService                 │ │
│  │  APIKeyService  MCPBuilderService                           │ │
│  │  OrgService     TemplateService  EnvFileService             │ │
│  │  SSOService     ComplianceService BackupService             │ │
│  │  LeaseService   AgentAnalyticsService                       │ │
│  └──┬──────────────────────────────────────────────────────┬───┘ │
│     │                                                      │     │
│  ┌──▼───────────────────┐  ┌───────────────────────────────▼──┐  │
│  │   Crypto Layer       │  │      MCP Gateway / Proxy         │  │
│  │                      │  │                                   │  │
│  │  AES-256-GCM         │  │  • Routes tool calls to servers  │  │
│  │  Envelope Encryption │  │  • Injects secrets as env vars   │  │
│  │  Per-project DEKs    │  │  • JSON-RPC 2.0 protocol         │  │
│  │  Master key from KMS │  │  • Request logging & stats       │  │
│  └──────────┬───────────┘  └───────────┬──────────┬───────────┘  │
│             │                          │          │              │
│  ┌──────────▼──────────────────────────▼────┐     │              │
│  │         Repository Layer (SQL)            │     │              │
│  │  PostgreSQL │ MySQL │ SQLite              │     │              │
│  └──────────────────────┬───────────────────┘     │              │
└─────────────────────────┼─────────────────────────┼──────────────┘
                          │                         │
              ┌───────────▼───────────┐   ┌────────▼──────────┐
              │     Database          │   │   MCP Servers      │
              │                       │   │                    │
              │  • Users & Orgs       │   │  ┌──────────────┐  │
              │  • Projects & Secrets │   │  │ Gmail Server │  │
              │  • OAuth Clients      │   │  │ (from GitHub)│  │
              │  • MCP Registry       │   │  └──────────────┘  │
              │  • Audit Log          │   │  ┌──────────────┐  │
              │  • Gateway Log        │   │  │ Slack Server │  │
              │                       │   │  │ (from GitHub)│  │
              └───────────────────────┘   │  └──────────────┘  │
                                          │  ┌──────────────┐  │
                                          │  │ Custom MCP   │  │
                                          │  │ (from GitHub)│  │
                                          │  └──────────────┘  │
                                          └────────────────────┘
```

### OAuth 2.0 Flow

```
┌──────────┐     ┌──────────────┐     ┌──────────────┐
│  Client  │     │   KeepSave   │     │   Resource   │
│  App     │     │  OAuth IdP   │     │   Server     │
└────┬─────┘     └──────┬───────┘     └──────┬───────┘
     │                  │                     │
     │  1. GET /oauth/authorize               │
     │  (client_id, redirect_uri, scope)      │
     ├────────────────►│                      │
     │                  │                     │
     │  2. Authorization Code                 │
     │◄────────────────┤                      │
     │                  │                     │
     │  3. POST /oauth/token                  │
     │  (code, client_id, client_secret)      │
     ├────────────────►│                      │
     │                  │                     │
     │  4. Access Token + Refresh Token       │
     │◄────────────────┤                      │
     │                  │                     │
     │  5. API Request (Bearer token)         │
     ├────────────────────────────────────────►
     │                  │                     │
     │  6. Response                           │
     │◄────────────────────────────────────────
```

### MCP Gateway Flow

```
┌──────────┐     ┌──────────────┐     ┌──────────────┐     ┌───────────┐
│  Claude  │     │  KeepSave    │     │  KeepSave    │     │   MCP     │
│  Agent   │     │  MCP Gateway │     │  Secret Vault│     │  Server   │
└────┬─────┘     └──────┬───────┘     └──────┬───────┘     └─────┬─────┘
     │                  │                     │                   │
     │  1. tools/call   │                     │                   │
     │  (tool_name,     │                     │                   │
     │   arguments)     │                     │                   │
     ├────────────────►│                      │                   │
     │                  │                     │                   │
     │                  │  2. Resolve secrets  │                   │
     │                  │  for env_mappings    │                   │
     │                  ├────────────────────►│                   │
     │                  │                     │                   │
     │                  │  3. Decrypted        │                   │
     │                  │  secret values       │                   │
     │                  │◄────────────────────┤                   │
     │                  │                     │                   │
     │                  │  4. Execute tool call with              │
     │                  │  secrets as env vars  │                  │
     │                  ├─────────────────────────────────────────►
     │                  │                     │                   │
     │                  │  5. Tool result      │                  │
     │                  │◄─────────────────────────────────────────
     │                  │                     │                   │
     │  6. Result       │                     │                   │
     │◄────────────────┤                      │                   │
```

### Environment Promotion Pipeline

```
Alpha ──(promote)──► UAT ──(promote)──► PROD
  │                   │                   │
  │  Diff preview     │  Diff preview     │  Multi-party
  │  Instant apply    │  Instant apply    │  approval required
  │  Audit logged     │  Audit logged     │  Audit logged
  │                   │                   │  Rollback supported
```

## Tech Stack

| Layer        | Technology                         |
|-------------|-------------------------------------|
| Backend     | Go 1.24+ with Gin framework         |
| Database    | PostgreSQL 16, MySQL, SQLite        |
| Encryption  | AES-256-GCM (envelope encryption)   |
| Auth        | JWT + API keys + OAuth 2.0 Provider |
| Frontend    | React 18 + TypeScript + Vite        |
| Embed SDK   | Web Components (Shadow DOM)         |
| MCP Hub     | GitHub integration + process runner |
| Container   | Docker + Docker Compose + Helm      |
| Observability | Prometheus metrics + OpenTelemetry |

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

# Promote alpha -> uat
curl -X POST http://localhost:8080/api/v1/projects/:id/promote \
  -H "Authorization: Bearer <jwt-token>" \
  -d '{"source_environment": "alpha", "target_environment": "uat", "override_policy": "skip"}'
```

### OAuth 2.0 Provider

```bash
# Register an OAuth client
curl -X POST http://localhost:8080/api/v1/oauth/clients \
  -H "Authorization: Bearer <jwt-token>" \
  -d '{"name": "My App", "redirect_uris": ["https://myapp.com/callback"], "scopes": ["read"], "grant_types": ["authorization_code"]}'

# Get authorization code
curl "http://localhost:8080/api/v1/oauth/authorize?response_type=code&client_id=ks_xxx&redirect_uri=https://myapp.com/callback&scope=read" \
  -H "Authorization: Bearer <jwt-token>"

# Exchange code for tokens
curl -X POST http://localhost:8080/api/v1/oauth/token \
  -d '{"grant_type": "authorization_code", "code": "xxx", "client_id": "ks_xxx", "client_secret": "xxx", "redirect_uri": "https://myapp.com/callback"}'

# Client credentials grant (M2M)
curl -X POST http://localhost:8080/api/v1/oauth/token \
  -d '{"grant_type": "client_credentials", "client_id": "ks_xxx", "client_secret": "xxx"}'

# OIDC Discovery
curl http://localhost:8080/.well-known/openid-configuration
```

### MCP Server Hub

```bash
# Register an MCP server from GitHub
curl -X POST http://localhost:8080/api/v1/mcp/servers \
  -H "Authorization: Bearer <jwt-token>" \
  -d '{"name": "my-mcp-server", "github_url": "https://github.com/user/mcp-server", "github_branch": "main", "is_public": true}'

# Browse public marketplace
curl http://localhost:8080/api/v1/mcp/servers/public

# Install an MCP server
curl -X POST http://localhost:8080/api/v1/mcp/installations \
  -H "Authorization: Bearer <jwt-token>" \
  -d '{"mcp_server_id": "uuid-here"}'

# Call a tool through the gateway
curl -X POST http://localhost:8080/api/v1/mcp/gateway \
  -H "Authorization: Bearer <jwt-token>" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "tool_name", "arguments": {}}}'

# List all available tools across installed servers
curl http://localhost:8080/api/v1/mcp/gateway/tools \
  -H "Authorization: Bearer <jwt-token>"

# Get MCP config for Claude Desktop/Code
curl http://localhost:8080/api/v1/mcp/config \
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

## Project Structure

```
keepsave/
├── backend/                  # Go (Gin) REST API
│   ├── cmd/server/           # Entry point
│   ├── internal/
│   │   ├── api/              # HTTP handlers & middleware
│   │   ├── auth/             # JWT + API keys
│   │   ├── crypto/           # AES-256-GCM encryption
│   │   ├── models/           # Domain models (OAuth, MCP, etc.)
│   │   ├── repository/       # Database access (multi-dialect)
│   │   ├── service/          # Business logic
│   │   ├── events/           # Event bus
│   │   ├── plugins/          # Plugin registry
│   │   ├── metrics/          # Prometheus metrics
│   │   └── tracing/          # Distributed tracing
│   └── migrations/           # SQL migrations (PG, MySQL, SQLite)
├── frontend/                 # React + TypeScript + Vite
│   ├── src/
│   │   ├── pages/            # Route pages (MCP Hub, OAuth, etc.)
│   │   ├── components/       # UI components
│   │   ├── api/              # API client
│   │   ├── types/            # TypeScript interfaces
│   │   └── embed/            # Embeddable widget SDK
├── sdks/                     # Client SDKs (Node.js, Go, Python)
├── integrations/             # Terraform, GitHub Actions, GitLab CI
├── helm/                     # Kubernetes Helm chart
└── docker-compose.yml        # Full stack orchestration
```

## Feature Phases

| Phase | Feature | Status |
|-------|---------|--------|
| 1-5   | Core: Auth, Secrets, Promotion, Widget | Done |
| 6     | Organizations, Templates, Dependencies | Done |
| 7     | Observability (Metrics, Tracing) | Done |
| 8     | OpenAPI Specification | Done |
| 9     | Enterprise (SSO, Compliance, Backups) | Done |
| 10    | Security Hardening (Rate Limit, CSRF) | Done |
| 11    | AI Agent Experience (Leases, Analytics) | Done |
| 12    | Platform Ecosystem (Events, Plugins) | Done |
| 13    | **OAuth 2.0 Provider + MCP Server Hub** | **Done** |

## Security

- All secret values encrypted using AES-256-GCM with envelope encryption
- Master key never stored in the database
- API keys are scoped per-project and per-environment
- OAuth 2.0 with PKCE support for public clients
- Full audit log for all secret access and promotion events
- Rate limiting (100 req/s, 200 burst)
- Security headers (X-Frame-Options, CSP, etc.)
- CORS restrictions for the embeddable widget

## Project Documentation

| Document                  | Description                          |
|---------------------------|--------------------------------------|
| [CLAUDE.md](./CLAUDE.md)  | Development guide and conventions    |
| [Roadmap.md](./Roadmap.md)| Phased delivery plan                 |

## License

MIT
