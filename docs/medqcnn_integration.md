# KeepSave + MedQCNN Integration Guide

This guide covers how to use KeepSave to manage secrets, host MCP servers, and provide authentication for [MedQCNN](https://github.com/santapong/MedQCNN) — a hybrid quantum-classical CNN for medical image diagnostics.

## Table of Contents

1. [Overview](#overview)
2. [What is MedQCNN?](#what-is-medqcnn)
3. [Secret Management for MedQCNN](#secret-management-for-medqcnn)
4. [MCP Server Hub: Hosting MedQCNN Tools](#mcp-server-hub-hosting-medqcnn-tools)
5. [OAuth 2.0: Identity Provider for MedQCNN](#oauth-20-identity-provider-for-medqcnn)
6. [Environment Promotion](#environment-promotion)
7. [Agent Workflows](#agent-workflows)
8. [Deployment](#deployment)
9. [Security Considerations](#security-considerations)

---

## Overview

KeepSave serves as the security and infrastructure backbone for MedQCNN deployments:

| KeepSave Role | What It Does for MedQCNN |
|--------------|--------------------------|
| **Secret Vault** | Encrypts and stores database URLs, API keys, JWT secrets, Kafka credentials |
| **MCP Hub** | Hosts MedQCNN's diagnostic tools for AI agent discovery |
| **OAuth Provider** | Issues authentication tokens for MedQCNN API access |
| **Promotion Engine** | Manages config transitions from dev → staging → production |
| **API Key Manager** | Issues scoped, time-limited keys for agent access |
| **Audit System** | Tracks who accessed which secrets and when |

---

## What is MedQCNN?

MedQCNN is a **hybrid quantum-classical neural network** for medical image classification:

```
Medical Image (224x224)
    │
    ▼ ResNet-18 (frozen, feature extraction)
    │
    ▼ FC Projector → L2 Normalization → R^256
    │
    ▼ Amplitude Encoding → |ψ(z)⟩ (8 qubits)
    │
    ▼ Hardware-Efficient Ansatz (Ry/Rz/CZ gates, 4 layers)
    │
    ▼ Pauli-Z Measurement → 8 expectation values
    │
    ▼ Classifier → Diagnosis (Benign/Malignant)
```

**Key facts:**
- Quantum circuit: 8 qubits, PennyLane simulation
- Datasets: MedMNIST (breast, pathology, dermatoscopy, organ, blood)
- API: Litestar REST server on port 8000
- MCP tools: `diagnose`, `model_info`, `list_datasets`
- Edge target: Raspberry Pi 5 clusters

**MedQCNN secrets that need protection:**

| Secret | Purpose | Risk |
|--------|---------|------|
| `DATABASE_URL` | PostgreSQL connection | DB credential leak |
| `JWT_SECRET_KEY` | API token signing | Token forgery |
| `OPENAI_API_KEY` | LangChain agent | Billing abuse |
| `KAFKA_BOOTSTRAP_SERVERS` | Message broker | Unauthorized access |
| `MEDQCNN_API_KEY` | Edge API key | Unauthorized inference |
| `CHECKPOINT_PATH` | Model file location | Model theft |

---

## Secret Management for MedQCNN

### Create a MedQCNN Project

```bash
# Via KeepSave CLI
keepsave login --api-url http://localhost:8080 --email admin@example.com

# Via API
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "medqcnn", "description": "Quantum Medical Diagnostics"}'
```

### Import MedQCNN's .env File

```bash
# Import entire .env file at once
keepsave import --project <medqcnn-id> --env alpha --file /path/to/MedQCNN/.env
```

### Environment-Specific Configuration

| Key | Alpha (dev) | UAT (test) | PROD |
|-----|-------------|------------|------|
| `DATABASE_URL` | `sqlite:///medqcnn.db` | `postgres://...@uat-db/medqcnn` | `postgres://...@prod-db/medqcnn` |
| `N_QUBITS` | `4` | `4` | `8` |
| `MEDQCNN_AUTH_DISABLED` | `1` | `0` | `0` |
| `KAFKA_BOOTSTRAP_SERVERS` | — | `kafka-uat:9092` | `kafka-prod:9092` |
| `CHECKPOINT_PATH` | — | `checkpoints/model_v1.pt` | `checkpoints/model_prod.pt` |

### Issue API Key for MedQCNN Runtime

```bash
# Read-only key scoped to medqcnn project, alpha environment
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "medqcnn-runtime-alpha",
    "project_id": "<medqcnn-id>",
    "scopes": ["read"],
    "environment": "alpha"
  }'
```

### MedQCNN Uses KeepSave Python SDK

MedQCNN fetches secrets at startup using the KeepSave Python SDK:

```python
from keepsave import KeepSaveClient
import os

client = KeepSaveClient(
    base_url=os.environ["KEEPSAVE_URL"],
    api_key=os.environ["KEEPSAVE_API_KEY"],
)
env = os.environ.get("MEDQCNN_ENV", "alpha")
project_id = os.environ["KEEPSAVE_PROJECT_ID"]

for secret in client.list_secrets(project_id, env):
    os.environ.setdefault(secret["key"], secret["value"])
```

---

## MCP Server Hub: Hosting MedQCNN Tools

### Register MedQCNN as an MCP Server

MedQCNN exposes three MCP tools that AI agents can use for medical diagnostics:

| Tool | Description | Input |
|------|-------------|-------|
| `diagnose` | Run quantum inference on a medical image | `image_path: string` |
| `model_info` | Get model architecture and parameter counts | (none) |
| `list_datasets` | List available MedMNIST benchmark datasets | (none) |

Register in the Hub:

```bash
curl -X POST http://localhost:8080/api/v1/mcp/servers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "medqcnn",
    "description": "Quantum-classical medical image diagnostics — diagnose images, inspect model architecture, browse datasets",
    "github_url": "https://github.com/santapong/MedQCNN",
    "github_branch": "main",
    "entry_command": "uv run python scripts/mcp_server.py",
    "transport": "stdio",
    "is_public": true,
    "env_mappings": {
      "DATABASE_URL": "<project-id>/alpha/DATABASE_URL",
      "JWT_SECRET_KEY": "<project-id>/alpha/JWT_SECRET_KEY",
      "CHECKPOINT_PATH": "<project-id>/alpha/CHECKPOINT_PATH"
    }
  }'
```

### How env_mappings Works

The `env_mappings` field maps environment variable names to KeepSave secret references in the format `<project-id>/<environment>/<key>`. When an AI agent calls a MedQCNN tool through the gateway:

1. KeepSave resolves each mapping to the decrypted secret value
2. Injects the values as environment variables into the MCP server process
3. MedQCNN reads `DATABASE_URL`, `JWT_SECRET_KEY`, etc. from the environment as normal
4. The agent never sees the raw credentials

### Browse MedQCNN in the Marketplace

```bash
# List public MCP servers (includes MedQCNN)
curl http://localhost:8080/api/v1/mcp/servers/public

# Install MedQCNN
curl -X POST http://localhost:8080/api/v1/mcp/installations \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"mcp_server_id": "<medqcnn-server-id>"}'

# List all tools across installed servers
curl http://localhost:8080/api/v1/mcp/gateway/tools \
  -H "Authorization: Bearer $TOKEN"
```

### Call MedQCNN Tools via Gateway

```bash
# Diagnose a medical image
curl -X POST http://localhost:8080/api/v1/mcp/gateway \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "diagnose",
      "arguments": {"image_path": "/data/scans/breast_001.png"}
    }
  }'

# Response includes quantum analysis:
# {
#   "prediction": 0,
#   "label": "Benign",
#   "confidence": 0.82,
#   "probabilities": {"Benign": 0.82, "Malignant": 0.18},
#   "quantum_expectation_values": [-0.123, 0.456, ...]
# }
```

---

## OAuth 2.0: Identity Provider for MedQCNN

### Register MedQCNN as an OAuth Client

```bash
curl -X POST http://localhost:8080/api/v1/oauth/clients \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "MedQCNN Diagnostic API",
    "redirect_uris": [
      "http://localhost:8000/auth/callback",
      "http://localhost:3000/auth/callback"
    ],
    "scopes": ["read", "write"],
    "grant_types": ["authorization_code", "client_credentials"]
  }'
```

### Authentication Flows

**For AI agents** (machine-to-machine):
```bash
curl -X POST http://localhost:8080/api/v1/oauth/token \
  -d '{"grant_type": "client_credentials", "client_id": "...", "client_secret": "..."}'
# Returns: {"access_token": "...", "token_type": "bearer", "expires_in": 3600}
```

**For web dashboard users** (authorization code + PKCE):
```
GET /oauth/authorize?response_type=code&client_id=...&redirect_uri=...&scope=read&code_challenge=...
POST /oauth/token  (exchange code for tokens)
```

---

## Environment Promotion

### MedQCNN Promotion Strategy

```
Alpha (dev)                 UAT (test)                PROD (production)
──────────────────         ──────────────────        ──────────────────
N_QUBITS=4                 N_QUBITS=4               N_QUBITS=8
SQLite                     PostgreSQL               PostgreSQL
Auth disabled              Auth enabled             Auth enabled + API keys
No Kafka                   Kafka optional           Kafka required
Demo checkpoint            Test checkpoint          Production checkpoint

        ──── promote ────►        ──── promote ────►
        (instant, audited)        (requires approval)
```

### Promote Secrets

```bash
# Preview changes
curl -X POST http://localhost:8080/api/v1/projects/<id>/promote/diff \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"source_environment": "alpha", "target_environment": "uat"}'

# Apply promotion
curl -X POST http://localhost:8080/api/v1/projects/<id>/promote \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "source_environment": "alpha",
    "target_environment": "uat",
    "override_policy": "skip",
    "notes": "MedQCNN v1.0 UAT deployment"
  }'
```

---

## Agent Workflows

### LangChain Agent with KeepSave + MedQCNN

An AI agent can use both KeepSave (for secrets) and MedQCNN (for diagnostics) through KeepSave's MCP gateway:

```
Agent Request: "Analyze the chest X-ray at /data/scan.png"
    │
    ▼ KeepSave MCP Gateway
    │
    ├──► Resolve MedQCNN server
    ├──► Inject secrets (DATABASE_URL, CHECKPOINT_PATH)
    ├──► Start MedQCNN MCP server process
    ├──► Forward: tools/call → diagnose(image_path="/data/scan.png")
    │
    ▼ MedQCNN MCP Server
    │
    ├──► Load model (checkpoint from CHECKPOINT_PATH)
    ├──► Preprocess image (224x224, grayscale, normalize)
    ├──► Forward: ResNet → Projector → Quantum Circuit → Classifier
    │
    ▼ Result
    │
    └──► {"label": "Normal", "confidence": 0.94, "quantum_values": [...]}
```

### Multi-Tool Agent Example

```bash
# An agent can combine KeepSave secret management with MedQCNN diagnostics:

# 1. List available diagnostic models
curl -X POST http://localhost:8080/api/v1/mcp/gateway \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"model_info","arguments":{}}}'

# 2. Check available datasets
curl -X POST http://localhost:8080/api/v1/mcp/gateway \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_datasets","arguments":{}}}'

# 3. Run diagnosis
curl -X POST http://localhost:8080/api/v1/mcp/gateway \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"diagnose","arguments":{"image_path":"/data/scan.png"}}}'
```

---

## Deployment

### Docker Compose (Full Stack)

See `docker-compose.keepsave.yml` in the MedQCNN repository for a combined deployment:

```bash
# Start KeepSave + MedQCNN
export MASTER_KEY=$(openssl rand -base64 32)
export JWT_SECRET=$(openssl rand -base64 32)
docker-compose -f docker-compose.yml -f docker-compose.keepsave.yml up -d

# Verify both services
curl http://localhost:8080/healthz  # KeepSave
curl http://localhost:8000/health   # MedQCNN
```

### Kubernetes (Helm)

Deploy KeepSave with MedQCNN secrets pre-configured:

```bash
# Deploy KeepSave
helm install keepsave ./helm/keepsave \
  --set config.masterKey="$(openssl rand -base64 32)" \
  --set config.jwtSecret="$(openssl rand -base64 32)"

# MedQCNN connects to KeepSave via service DNS
# KEEPSAVE_URL=http://keepsave.default.svc.cluster.local:8080
```

---

## Application Dashboard Integration

MedQCNN can be registered as a managed application in KeepSave's Application Dashboard, providing a centralized view alongside other services:

### Register MedQCNN as an Application

```bash
curl -X POST http://localhost:8080/api/v1/applications \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "MedQCNN",
    "url": "http://localhost:8000",
    "description": "Hybrid quantum-classical CNN for medical image diagnostics. Supports BreastMNIST, PathMNIST, and custom DICOM images.",
    "icon": "🧬",
    "category": "AI & ML"
  }'
```

Once registered, MedQCNN appears in the KeepSave Application Dashboard alongside all your other homelab services. You can:
- Quick-link to the MedQCNN frontend dashboard
- Mark it as a favorite for easy access
- Search and filter by category (AI & ML)
- Track all registered services in one place

This replaces the need for a separate application-dashboard project — all service management is now unified within KeepSave.

---

## Security Considerations

| Concern | KeepSave Mitigation |
|---------|-------------------|
| Secrets in .env files | Replaced with encrypted vault; only `KEEPSAVE_URL` and `KEEPSAVE_API_KEY` needed on host |
| API key sprawl | Centralized key management with expiration and usage tracking |
| Cross-environment leaks | Environment-locked API keys prevent alpha keys from accessing prod |
| MCP server credential exposure | Secrets injected as env vars by gateway; agents never see raw values |
| Audit compliance | Full audit trail for secret access, promotion, and tool calls |
| Model checkpoint access | `CHECKPOINT_PATH` stored as encrypted secret, not hardcoded |
| JWT secret rotation | KeepSave manages rotation; MedQCNN auto-refreshes via SDK |
