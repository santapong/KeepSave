# CLAUDE.md - KeepSave Development Guide

## Project Overview

**KeepSave** is a secure environment variable storage and promotion system designed for AI Agents and development teams. It prevents exposure of sensitive environment parameters by providing encrypted storage, role-based access, and a controlled promotion pipeline across environments (Alpha -> UAT -> PROD).

## Architecture

```
keepsave/
├── backend/                  # Go (Gin) REST API
│   ├── cmd/server/           # Entry point
│   ├── internal/
│   │   ├── api/              # HTTP handlers & middleware
│   │   ├── auth/             # Authentication (JWT + API keys)
│   │   ├── crypto/           # AES-256-GCM encryption layer
│   │   ├── models/           # Domain models
│   │   ├── repository/       # Database access (PostgreSQL)
│   │   ├── service/          # Business logic
│   │   └── promotion/        # Environment promotion engine
│   ├── migrations/           # SQL migrations
│   └── Dockerfile
├── frontend/                 # React + TypeScript
│   ├── src/
│   │   ├── components/       # UI components
│   │   ├── hooks/            # Custom React hooks
│   │   ├── api/              # API client
│   │   ├── pages/            # Route pages
│   │   └── embed/            # Embeddable widget SDK
│   └── Dockerfile
├── docker-compose.yml
├── CLAUDE.md
├── Roadmap.md
└── README.md
```

## Tech Stack

| Layer       | Technology                        |
|-------------|-----------------------------------|
| Backend     | Go 1.22+ with Gin framework       |
| Database    | PostgreSQL 16                      |
| Encryption  | AES-256-GCM (envelope encryption) |
| Auth        | JWT tokens + API keys for agents   |
| Frontend    | React 18 + TypeScript + Vite       |
| Embed SDK   | Web Components (Shadow DOM)        |
| Container   | Docker + Docker Compose            |

## Key Design Decisions

### Security Model
- All secret values are encrypted at rest using AES-256-GCM with per-project envelope keys
- Master key is derived from a KMS or env-provided root key (never stored in DB)
- API keys for AI Agents are scoped per-project and per-environment
- Promotion between environments requires explicit approval (configurable)

### Environment Promotion Pipeline
```
Alpha ──(promote)──> UAT ──(promote)──> PROD
```
- Promotion copies encrypted values between environment scopes
- Audit trail is written for every promotion event
- PROD promotions can require multi-party approval

### Embeddable Frontend
- The frontend exposes a `<keepsave-widget>` Web Component
- Integrators include a single `<script>` tag and configure via attributes
- Shadow DOM isolates styles from the host page
- Communication via postMessage for cross-origin embedding

## Development Commands

```bash
# Backend
cd backend && go run ./cmd/server         # Run API server
cd backend && go test ./...                # Run all tests

# Frontend
cd frontend && npm install                 # Install dependencies
cd frontend && npm run dev                 # Dev server
cd frontend && npm run build               # Production build
cd frontend && npm test                    # Run tests

# Full stack
docker-compose up --build                  # Run everything
```

## Environment Variables (for the app itself)

| Variable              | Description                              |
|-----------------------|------------------------------------------|
| `DATABASE_URL`        | PostgreSQL connection string             |
| `MASTER_KEY`          | Root encryption key (base64, 32 bytes)   |
| `JWT_SECRET`          | JWT signing secret                       |
| `PORT`                | API server port (default: 8080)          |
| `CORS_ORIGINS`        | Allowed origins for embed widget         |

## Coding Conventions

- **Go**: Follow standard Go project layout. Use `internal/` for non-exported packages. Error wrapping with `fmt.Errorf("context: %w", err)`.
- **TypeScript**: Strict mode enabled. Use functional components with hooks. No `any` types.
- **Tests**: Table-driven tests in Go. React Testing Library for frontend.
- **Commits**: Conventional commits (`feat:`, `fix:`, `docs:`, `chore:`).
- **Branches**: Feature branches off `main`. PRs required for `main`.
