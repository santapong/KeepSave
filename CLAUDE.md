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
| `KEEPSAVE_PROMOTIONS_ENABLED` | Promotion kill switch (default: true). `false` makes `/promote` + `/approve` return 503; see `docs/RUNBOOK.md` §8 |
| `KEEPSAVE_PLATFORM_ADMIN_EMAILS` | Comma-separated allowlist of user emails permitted to reach the cross-tenant `/admin` endpoints (dashboard, traces). Empty (default) ⇒ `/admin` rejects everyone (fail-closed) |
| `TRUSTED_PROXIES` | Comma-separated reverse-proxy CIDRs whose `X-Forwarded-For`/`X-Real-IP` headers are trusted when deriving the client IP (rate-limit key + audit IP). Empty (default) ⇒ trust NO proxy, so a forged `X-Forwarded-For` cannot spoof the client IP (CWE-348) |

## Coding Conventions

- **Go**: Follow standard Go project layout. Use `internal/` for non-exported packages. Error wrapping with `fmt.Errorf("context: %w", err)`.
- **TypeScript**: Strict mode enabled. Use functional components with hooks. No `any` types.
- **Tests**: Table-driven tests in Go. React Testing Library for frontend.
- **Commits**: Conventional commits (`feat:`, `fix:`, `docs:`, `chore:`).
- **Branches**: Feature branches off `main`. PRs required for `main`.
- **Error responses**: handlers must never return `err.Error()` directly to clients — use the `httperror` package described in [`docs/ERROR_HANDLING_STANDARD.md`](docs/ERROR_HANDLING_STANDARD.md). Lint enforced.
- **Audit log**: every state-mutating handler (secret/project/api-key/promotion) MUST emit an audit event from the canonical taxonomy in [`docs/AUDIT_LOG_COVERAGE.md`](docs/AUDIT_LOG_COVERAGE.md), and the test MUST assert the audit row was written. PRs failing either are rejected.

## Project Governance (read before non-trivial changes)

This project has explicit operating rules. Before opening a PR that touches crypto, auth, or the promotion engine, read:

- [`docs/ROLES.md`](docs/ROLES.md) — who reviews what; Security Engineer has veto power on `internal/crypto`, `internal/auth`, and the promotion engine.
- [`docs/adr/`](docs/adr/) — Architecture Decision Records. ADRs 0001-0004 backfill existing decisions; new Type-1 work needs a new ADR before implementation.
- [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md) — STRIDE pass with file:line refs. Update in the same PR if the change widens any trust boundary.
- [`docs/FOLLOWUPS.md`](docs/FOLLOWUPS.md) — known tracked work; pick from here before inventing new tasks.
- [`docs/ROLES_30_60_90.md`](docs/ROLES_30_60_90.md) — current phase action plan per role.
- [`docs/ADLC.md`](docs/ADLC.md) — **AI-Assisted Development Life Cycle.** If an AI agent is doing the work, this is the pipeline it follows: intake → classify → threat delta → design → harness plan → build → verify → review → land → post-merge, with a human gate at every irreversible or secret-touching step. No agent lands a Type-1 or a `main`/PROD change alone.

### AI-assisted development

When development is driven by an AI agent (Claude Code or similar), the work is sequenced by [`docs/ADLC.md`](docs/ADLC.md) and executed with the harness described in [`docs/HARNESS_ENGINEERING.md`](docs/HARNESS_ENGINEERING.md). For a non-trivial or security-sensitive task, drive it through the [`/workflow`](.claude/skills/workflow/SKILL.md) skill, which compiles the ADLC gates (classify → threat → build → adversarial-verify → review) and KeepSave's invariants (never surface plaintext, audit-log assertion, Security-Engineer veto on crypto/auth/promotion) into a runnable, fail-closed Workflow. The ADLC never overrides a gate — it only orders them; when it conflicts with `docs/ROLES.md` or an ADR, those win.

### Decision classes (`docs/ROLES.md` §3.1)

| Class      | Examples                                                   | Required process                                              |
|------------|------------------------------------------------------------|---------------------------------------------------------------|
| **Type-1** | Crypto scheme, key hierarchy, schema breaking change, audit-field removal, PROD key rotation | ADR + Security sign-off + Tech Lead sign-off                  |
| **Type-2** | New endpoint, new UI component, dependency upgrade          | RFC if non-trivial; standard PR review otherwise              |
| **Type-3** | Refactor within a package, doc edit                        | Standard PR review                                            |

When in doubt, treat as the next class up. Misclassification is itself a bug.

## Where to look for what

| Need                                       | Doc                                                       |
|--------------------------------------------|-----------------------------------------------------------|
| Architecture overview                      | [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)            |
| Why a decision was made                    | [`docs/adr/`](docs/adr/)                                  |
| What's open / tracked                      | [`docs/FOLLOWUPS.md`](docs/FOLLOWUPS.md)                  |
| What we're NOT building                    | [`docs/ROADMAP_NOT.md`](docs/ROADMAP_NOT.md)              |
| Incident procedures                        | [`docs/RUNBOOK.md`](docs/RUNBOOK.md)                      |
| Where secrets live (KeepSave's own)        | [`docs/SECRET_SOURCES.md`](docs/SECRET_SOURCES.md)        |
| Test strategy and coverage gates           | [`tests/PYRAMID.md`](tests/PYRAMID.md)                    |
| Negative-auth test matrix                  | [`tests/NEGATIVE_AUTH_PLAN.md`](tests/NEGATIVE_AUTH_PLAN.md) |
| Flaky-test policy                          | [`tests/FLAKY.md`](tests/FLAKY.md)                        |
| Embed widget state machine                 | [`docs/EMBED_STATE.md`](docs/EMBED_STATE.md)              |
| Embed origin / postMessage policy          | [`docs/EMBED_ORIGIN_POLICY.md`](docs/EMBED_ORIGIN_POLICY.md) |
| UX state per screen                        | [`docs/UX_STATE_INVENTORY.md`](docs/UX_STATE_INVENTORY.md) |
| CI runner permissions                      | [`docs/CI_PERMISSIONS.md`](docs/CI_PERMISSIONS.md)        |
| How AI agents develop here (lifecycle)     | [`docs/ADLC.md`](docs/ADLC.md)                            |
| Engineering the agent harness / workflows  | [`docs/HARNESS_ENGINEERING.md`](docs/HARNESS_ENGINEERING.md) |
| Running a task as a gated workflow         | [`.claude/skills/workflow/SKILL.md`](.claude/skills/workflow/SKILL.md) |
