# KeepSave + NEXUS Integration Guide

This guide covers how to use KeepSave to manage secrets, provide OAuth 2.0 authentication, and host MCP servers for [NEXUS](https://github.com/santapong/Nexus) — an Agentic AI Company-as-a-Service platform.

## Table of Contents

1. [Overview](#overview)
2. [What is NEXUS?](#what-is-nexus)
3. [Secret Management for NEXUS](#secret-management-for-nexus)
4. [OAuth 2.0: Identity Provider for NEXUS](#oauth-20-identity-provider-for-nexus)
5. [MCP Server Hub: Hosting NEXUS Agent Tools](#mcp-server-hub-hosting-nexus-agent-tools)
6. [Environment Promotion](#environment-promotion)
7. [API Key Management for NEXUS Agents](#api-key-management-for-nexus-agents)
8. [A2A Protocol Integration](#a2a-protocol-integration)
9. [Deployment](#deployment)
10. [Security Considerations](#security-considerations)

---

## Overview

KeepSave serves as the **security and secrets backbone** for NEXUS deployments:

| KeepSave Role | What It Does for NEXUS |
|--------------|------------------------|
| **Secret Vault** | Encrypts and stores LLM API keys, database URLs, JWT secrets, Kafka credentials, Redis passwords |
| **OAuth Provider** | Issues authentication tokens for NEXUS API and A2A gateway access |
| **MCP Hub** | Hosts NEXUS agent tools for external discovery via the MCP marketplace |
| **Promotion Engine** | Manages config transitions from dev → staging → production |
| **API Key Manager** | Issues scoped, time-limited keys for NEXUS agents to fetch secrets at runtime |
| **Audit System** | Tracks who accessed which secrets and when — critical for multi-tenant compliance |

### Integration Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  NEXUS Platform                                                     │
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │
│  │ CEO      │  │ Engineer │  │ Analyst  │  │  A2A Gateway      │  │
│  │ Agent    │  │ Agent    │  │ Agent    │  │  (External Agents)│  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬──────────┘  │
│       │              │              │                 │             │
│       └──────────────┴──────┬───────┴─────────────────┘             │
│                             │                                       │
│                    ┌────────▼────────┐                              │
│                    │  Settings.py    │                              │
│                    │  (reads secrets │                              │
│                    │   at startup)   │                              │
│                    └────────┬────────┘                              │
└─────────────────────────────┼───────────────────────────────────────┘
                              │ HTTPS (KeepSave Python SDK)
┌─────────────────────────────▼───────────────────────────────────────┐
│  KeepSave Platform                                                  │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ Secret Vault │  │ OAuth 2.0    │  │ MCP Server Hub           │  │
│  │ (AES-256-GCM)│  │ Provider     │  │ (Agent tool marketplace) │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ Environment  │  │ API Key      │  │ Audit Trail              │  │
│  │ Promotion    │  │ Manager      │  │ (who accessed what/when) │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## What is NEXUS?

NEXUS is an **Agentic AI Company-as-a-Service** platform where every department is staffed by an AI agent:

| Agent | Role | LLM Model |
|-------|------|-----------|
| **CEO** | Task orchestration & decomposition | Claude Sonnet |
| **Engineer** | Code generation & debugging | Claude Sonnet |
| **Analyst** | Research & data analysis | Gemini Pro |
| **Writer** | Content & email drafting | Claude Haiku |
| **QA** | Output quality review | Claude Haiku |
| **Prompt Creator** | Meta-agent: improves system prompts | Claude Sonnet |

**Key infrastructure:**
- **Event Bus**: Apache Kafka (KRaft mode) — 24+ topics for agent communication
- **Database**: PostgreSQL 16 + pgvector for persistent memory with embeddings
- **Cache**: Redis 7 (4 database roles: working memory, cache, pub/sub, locks)
- **API**: Python Litestar (async-first REST + WebSocket)
- **External Protocol**: A2A (Agent-to-Agent) for hiring/being hired by external agents

**NEXUS secrets that need protection:**

| Secret | Purpose | Risk |
|--------|---------|------|
| `ANTHROPIC_API_KEY` | Claude LLM calls (CEO, Engineer, Writer, QA) | Billing abuse, unauthorized AI access |
| `GOOGLE_API_KEY` | Gemini LLM calls (Analyst) + embeddings | Billing abuse |
| `OPENAI_API_KEY` | OpenAI fallback provider | Billing abuse |
| `GROQ_API_KEY` | Groq fallback provider | Billing abuse |
| `MISTRAL_API_KEY` | Mistral fallback provider | Billing abuse |
| `DATABASE_URL` | PostgreSQL connection with pgvector | Full DB access, memory leak |
| `REDIS_URL` | Redis connection (working memory, locks) | Cache poisoning, lock manipulation |
| `KAFKA_BOOTSTRAP_SERVERS` | Kafka broker address | Message injection, agent impersonation |
| `JWT_SECRET_KEY` | User authentication token signing | Token forgery, account takeover |
| `LANGFUSE_SECRET_KEY` | Eval tracking service | Eval data leak |
| `TEMPORAL_HOST` | Temporal workflow engine | Workflow manipulation |

---

## Secret Management for NEXUS

### Create a NEXUS Project

```bash
# Register and login to KeepSave
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@nexus.dev", "password": "SecurePass123!"}'

TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@nexus.dev", "password": "SecurePass123!"}' | jq -r '.token')

# Create project for NEXUS
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "nexus", "description": "Agentic AI Company-as-a-Service"}'
```

### Import NEXUS Environment Variables

```bash
# Import from NEXUS .env file
keepsave import --project <nexus-id> --env alpha --file /path/to/Nexus/.env

# Or add secrets individually via API
curl -X POST http://localhost:8080/api/v1/projects/<nexus-id>/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "ANTHROPIC_API_KEY",
    "value": "sk-ant-...",
    "environment": "alpha",
    "description": "Anthropic Claude API key for CEO, Engineer, Writer, QA agents"
  }'
```

### Environment-Specific Configuration

| Key | Alpha (dev) | UAT (staging) | PROD |
|-----|-------------|---------------|------|
| `DATABASE_URL` | `postgresql+asyncpg://nexus:dev@localhost:5432/nexus` | `postgresql+asyncpg://nexus:uat@uat-db:5432/nexus` | `postgresql+asyncpg://nexus:prod@prod-db:5432/nexus` |
| `REDIS_URL` | `redis://localhost:6379` | `redis://redis-uat:6379` | `redis://redis-prod:6379` |
| `KAFKA_BOOTSTRAP_SERVERS` | `localhost:9092` | `kafka-uat:9092` | `kafka-prod:9092` |
| `DAILY_SPEND_LIMIT_USD` | `5.00` | `10.00` | `50.00` |
| `DEFAULT_TOKEN_BUDGET_PER_TASK` | `50000` | `50000` | `100000` |
| `JWT_SECRET_KEY` | dev secret | rotated monthly | rotated weekly |
| `ANTHROPIC_API_KEY` | dev key | staging key | production key |
| `GOOGLE_API_KEY` | dev key | staging key | production key |

### NEXUS Uses KeepSave Python SDK

NEXUS fetches secrets at startup using the KeepSave Python SDK. Add to `settings.py`:

```python
# nexus/settings.py — KeepSave integration
from keepsave import KeepSaveClient
import os

# Bootstrap: only KEEPSAVE_URL and KEEPSAVE_API_KEY needed in env
keepsave = KeepSaveClient(
    base_url=os.environ["KEEPSAVE_URL"],
    api_key=os.environ["KEEPSAVE_API_KEY"],
)

project_id = os.environ["KEEPSAVE_PROJECT_ID"]
env = os.environ.get("NEXUS_ENV", "alpha")

# Fetch all secrets and inject into environment before Pydantic Settings loads
for secret in keepsave.list_secrets(project_id, env):
    os.environ.setdefault(secret["key"], secret["value"])

# Now Pydantic Settings reads from env as normal
class Settings(BaseSettings):
    anthropic_api_key: str = ""
    google_api_key: str = ""
    database_url: str = "postgresql+asyncpg://nexus:nexus_dev@localhost:5432/nexus"
    redis_url: str = "redis://localhost:6379"
    kafka_bootstrap_servers: str = "localhost:9092"
    jwt_secret_key: str = ""  # No more hardcoded default!
    daily_spend_limit_usd: str = "5.00"
    # ... rest of settings
```

This approach means:
- Only `KEEPSAVE_URL`, `KEEPSAVE_API_KEY`, and `KEEPSAVE_PROJECT_ID` exist in `.env`
- All sensitive secrets live in KeepSave's encrypted vault
- No more hardcoded JWT secrets (solves NEXUS security audit finding #1)
- Environment switching is managed by changing `NEXUS_ENV`

---

## OAuth 2.0: Identity Provider for NEXUS

### Register NEXUS as an OAuth Client

```bash
curl -X POST http://localhost:8080/api/v1/oauth/clients \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "NEXUS Agentic Platform",
    "redirect_uris": [
      "http://localhost:8000/auth/callback",
      "http://localhost:5173/auth/callback"
    ],
    "scopes": ["read", "write", "admin"],
    "grant_types": ["authorization_code", "client_credentials", "refresh_token"]
  }'
```

### Authentication Flows

**For NEXUS agents** (machine-to-machine — A2A external calls):
```bash
# Client credentials flow for agent-to-service auth
curl -X POST http://localhost:8080/api/v1/oauth/token \
  -d '{
    "grant_type": "client_credentials",
    "client_id": "<nexus-client-id>",
    "client_secret": "<nexus-client-secret>"
  }'
# Returns: {"access_token": "...", "token_type": "bearer", "expires_in": 3600}
```

**For NEXUS dashboard users** (authorization code + PKCE):
```
GET /api/v1/oauth/authorize?response_type=code&client_id=<nexus-client-id>&redirect_uri=http://localhost:5173/auth/callback&scope=read+write&code_challenge=<challenge>
POST /api/v1/oauth/token (exchange code for tokens)
```

**For A2A external agents** (bearer token validation):
```bash
# External agents authenticate via KeepSave OAuth before calling NEXUS A2A gateway
curl -X POST http://localhost:8080/api/v1/oauth/token \
  -d '{"grant_type": "client_credentials", "client_id": "...", "client_secret": "..."}'

# Use the token to call NEXUS A2A
curl -X POST http://nexus:8000/a2a/tasks \
  -H "Authorization: Bearer <keepsave-oauth-token>" \
  -d '{"skill": "software-engineering", "instruction": "..."}'
```

---

## MCP Server Hub: Hosting NEXUS Agent Tools

NEXUS agent tools (web search, code execute, file operations) can be registered in KeepSave's MCP Hub, making them discoverable by other platforms:

### Register NEXUS Tools as an MCP Server

```bash
curl -X POST http://localhost:8080/api/v1/mcp/servers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "nexus-agent-tools",
    "description": "NEXUS Agentic AI tools — web search, code execution, file operations, research analysis",
    "github_url": "https://github.com/santapong/Nexus",
    "github_branch": "main",
    "entry_command": "python -m nexus.tools.mcp_server",
    "transport": "stdio",
    "is_public": true,
    "env_mappings": {
      "ANTHROPIC_API_KEY": "<project-id>/alpha/ANTHROPIC_API_KEY",
      "GOOGLE_API_KEY": "<project-id>/alpha/GOOGLE_API_KEY"
    }
  }'
```

### NEXUS Agent Tools Available via Gateway

| Tool | Description | Access |
|------|-------------|--------|
| `web_search` | Search the web and return results | Read-only |
| `web_fetch` | Fetch content from a URL | Read-only |
| `file_read` | Read file contents | Read-only |
| `code_execute` | Run code in sandboxed environment | Sandboxed |
| `file_write` | Write content to a file | Requires approval |
| `send_email` | Send an email | Requires approval |
| `hire_external_agent` | Hire an external A2A agent | Requires approval |

### Call NEXUS Tools via KeepSave Gateway

```bash
# Execute a web search through NEXUS tools
curl -X POST http://localhost:8080/api/v1/mcp/gateway \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "web_search",
      "arguments": {"query": "latest Python async patterns 2026"}
    }
  }'
```

---

## Environment Promotion

### NEXUS Promotion Strategy

```
Alpha (dev)                  UAT (staging)               PROD (production)
───────────────────         ───────────────────         ───────────────────
$5/day spend limit          $10/day spend limit         $50/day spend limit
Dev API keys                Staging API keys            Production API keys
Local Kafka                 Dedicated Kafka cluster     HA Kafka cluster
SQLite-compatible           PostgreSQL                  PostgreSQL HA
Dev JWT secret              Rotated monthly             Rotated weekly

        ──── promote ────►          ──── promote ────►
        (instant, audited)          (requires approval)
```

### Promote Secrets

```bash
# Preview changes before promoting
curl -X POST http://localhost:8080/api/v1/projects/<nexus-id>/promote/diff \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"source_environment": "alpha", "target_environment": "uat"}'

# Apply promotion
curl -X POST http://localhost:8080/api/v1/projects/<nexus-id>/promote \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "source_environment": "alpha",
    "target_environment": "uat",
    "override_policy": "skip",
    "notes": "NEXUS v1.0 UAT deployment — all 6 agents validated"
  }'

# Rollback if needed
curl -X POST http://localhost:8080/api/v1/projects/<nexus-id>/promotions/<id>/rollback \
  -H "Authorization: Bearer $TOKEN"
```

---

## API Key Management for NEXUS Agents

KeepSave API keys enable NEXUS agents to fetch secrets at runtime without exposing credentials:

### Create Scoped API Keys

```bash
# Read-only key for NEXUS runtime (alpha environment)
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "nexus-runtime-alpha",
    "project_id": "<nexus-id>",
    "scopes": ["read"],
    "environment": "alpha"
  }'

# Separate key for production with tighter scope
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "nexus-runtime-prod",
    "project_id": "<nexus-id>",
    "scopes": ["read"],
    "environment": "prod",
    "expires_at": "2026-06-01T00:00:00Z"
  }'
```

### Per-Agent API Keys (Fine-Grained Access)

For multi-tenant deployments, create separate API keys per agent role:

```bash
# Engineer agent — needs code-related secrets only
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "nexus-engineer-agent",
    "project_id": "<nexus-id>",
    "scopes": ["read"],
    "environment": "alpha"
  }'
```

---

## A2A Protocol Integration

NEXUS uses the A2A (Agent-to-Agent) protocol for external agent interoperability. KeepSave secures this channel:

### Securing A2A with KeepSave

1. **Store A2A bearer tokens in KeepSave** — no hardcoded tokens in source code
2. **OAuth 2.0 for A2A authentication** — external agents get tokens via client credentials
3. **Audit trail** — all A2A token access logged in KeepSave audit

```bash
# Store A2A bearer tokens as secrets
curl -X POST http://localhost:8080/api/v1/projects/<nexus-id>/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "key": "A2A_INBOUND_TOKEN",
    "value": "<generated-secure-token>",
    "environment": "alpha",
    "description": "Bearer token for external agents calling NEXUS A2A gateway"
  }'
```

### External Agent Discovery Flow

```
1. External agent fetches KeepSave OAuth token
   POST /api/v1/oauth/token (client_credentials)

2. External agent discovers NEXUS capabilities
   GET http://nexus:8000/.well-known/agent.json

3. External agent submits task to NEXUS
   POST http://nexus:8000/a2a/tasks
   Authorization: Bearer <keepsave-oauth-token>

4. NEXUS agents process task, fetch secrets from KeepSave as needed
   KeepSave Python SDK → list_secrets()

5. Result streamed back via SSE
   GET http://nexus:8000/a2a/tasks/{id}/events
```

---

## Deployment

### Docker Compose (KeepSave + NEXUS)

```yaml
# docker-compose.integration.yml
services:
  # === KeepSave Services ===
  keepsave-db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: keepsave
      POSTGRES_PASSWORD: keepsave_dev
      POSTGRES_DB: keepsave
    ports:
      - "5435:5432"

  keepsave-api:
    build: ../KeepSave/backend
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgresql://keepsave:keepsave_dev@keepsave-db:5432/keepsave
      MASTER_KEY: ${MASTER_KEY}
      JWT_SECRET: ${JWT_SECRET}
    depends_on:
      - keepsave-db

  keepsave-frontend:
    build: ../KeepSave/frontend
    ports:
      - "3002:3000"
    depends_on:
      - keepsave-api

  # === NEXUS Services ===
  nexus-db:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: nexus
      POSTGRES_PASSWORD: nexus_dev
      POSTGRES_DB: nexus
    ports:
      - "5432:5432"

  nexus-redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  nexus-kafka:
    image: apache/kafka:3.7.0
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://nexus-kafka:9092
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@nexus-kafka:9093
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    ports:
      - "9092:9092"

  nexus-backend:
    build: ../Nexus/backend
    ports:
      - "8000:8000"
    environment:
      KEEPSAVE_URL: http://keepsave-api:8080
      KEEPSAVE_API_KEY: ${KEEPSAVE_API_KEY}
      KEEPSAVE_PROJECT_ID: ${KEEPSAVE_PROJECT_ID}
      NEXUS_ENV: alpha
    depends_on:
      - keepsave-api
      - nexus-db
      - nexus-redis
      - nexus-kafka

  nexus-frontend:
    build: ../Nexus/frontend
    ports:
      - "5173:5173"
    depends_on:
      - nexus-backend
```

### Kubernetes (Helm)

```bash
# Deploy KeepSave first
helm install keepsave ./helm/keepsave \
  --set config.masterKey="$(openssl rand -base64 32)" \
  --set config.jwtSecret="$(openssl rand -base64 32)"

# Deploy NEXUS with KeepSave connection
kubectl apply -k k8s/overlays/dev
# Set KEEPSAVE_URL=http://keepsave.default.svc.cluster.local:8080
```

---

## Application Dashboard Integration

Register NEXUS as a managed application in KeepSave's Application Dashboard:

```bash
curl -X POST http://localhost:8080/api/v1/applications \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "NEXUS",
    "url": "http://localhost:8000",
    "description": "Agentic AI Company-as-a-Service — 6 AI agents (CEO, Engineer, Analyst, Writer, QA, Prompt Creator) with Kafka event bus, persistent memory, and A2A protocol.",
    "icon": "🤖",
    "category": "AI & ML"
  }'
```

---

## Security Considerations

| Concern | KeepSave Mitigation |
|---------|-------------------|
| LLM API keys in `.env` files | Replaced with encrypted vault; only `KEEPSAVE_URL` and `KEEPSAVE_API_KEY` needed on host |
| Hardcoded JWT secret (NEXUS audit finding #1) | JWT secret stored in KeepSave, fetched at runtime — no default value |
| Hardcoded A2A token (NEXUS audit finding #2) | A2A tokens stored as encrypted secrets, rotated via promotion pipeline |
| Multi-tenant secret isolation | Environment-locked API keys prevent dev keys from accessing prod |
| Agent cost explosion ($5/day cap) | `DAILY_SPEND_LIMIT_USD` managed per-environment via promotion |
| Kafka credential exposure | `KAFKA_BOOTSTRAP_SERVERS` encrypted at rest, never in source code |
| Cross-environment leaks | Scoped API keys per environment (alpha/uat/prod) |
| Audit compliance | Full audit trail for every secret access, promotion, and tool call |
| Secret rotation | KeepSave manages rotation; NEXUS auto-refreshes via SDK cache TTL |
| A2A external agent authentication | OAuth 2.0 client credentials flow replaces hardcoded bearer tokens |
