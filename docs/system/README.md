# KeepSave — System Documentation

> The single, consolidated reference for the **entire KeepSave system**: backend, data model,
> API, security model, promotion engine, frontend, embeddable widget, infrastructure,
> operations, testing, governance, and SDKs.

KeepSave already has 60+ focused documents — Architecture Decision Records, a threat model, a
runbook, role charters, and more. This set does **not** replace them; it is the **front door**
that ties them together into one coherent, self-contained narrative. Each chapter is detailed
enough to understand its topic on its own, and links out to the canonical maintained doc when
you need the last mile of specifics.

## Who this is for

- **New engineers** who need to understand the whole product before touching it.
- **Security auditors** who need to see every trust boundary, control, and data flow in one place.
- **AI agents and integrators** who need a precise, grounded map of the system's surfaces.

## How to read it

Read top-to-bottom for a full tour, or jump to the chapter you need. The recommended path for
a newcomer is **01 → 02 → 04 → 05 → 06**, which walks from the big picture through the backend,
the data it stores, how that data is protected, and the promotion engine that is KeepSave's
reason to exist.

## Table of contents

| # | Chapter | What it covers |
|---|---------|----------------|
| — | **[Overview](./01-overview.md)** | What KeepSave is, the high-level architecture, request lifecycle, tech stack, key design decisions, and the repository layout. |
| 02 | **[Backend Architecture](./02-backend.md)** | Boot sequence, configuration, the middleware pipeline, the handler→service→repository layering, observability, and background workers. |
| 03 | **[API Reference](./03-api-reference.md)** | The complete `/api/v1` HTTP surface — every route group and endpoint, the error model, and rate limiting. |
| 04 | **[Data Model](./04-data-model.md)** | Every database table across migrations 001–008, the entity relationships, multi-dialect support, and how encrypted fields are stored. |
| 05 | **[Security Model](./05-security.md)** | Envelope encryption and the key hierarchy, authentication and authorization, the audit-log taxonomy, error sanitization, and the trust boundaries. |
| 06 | **[Promotion Engine](./06-promotion.md)** | The Alpha→UAT→PROD pipeline: diffing, four-eyes approval, snapshots, and rollback. |
| 07 | **[Frontend Architecture](./07-frontend.md)** | The React dashboard: routing, components, hooks, the API client, state, styling, and the build. |
| 08 | **[Embeddable Widget](./08-embed-widget.md)** | The `<keepsave-widget>` Web Component: Shadow DOM, auth modes, the postMessage protocol, and the origin allow-list. |
| 09 | **[Infrastructure](./09-infrastructure.md)** | Container images, Docker Compose, the Helm chart, the CI pipeline, and deployment topology. |
| 10 | **[Operations](./10-operations.md)** | Incident runbooks, key-rotation cadences, crash/SPOF risks, and what to watch in production. |
| 11 | **[Testing](./11-testing.md)** | The test pyramid, the negative-auth matrix, the flaky-test policy, the E2E harness, and CI gates. |
| 12 | **[Governance](./12-governance.md)** | Roles and veto power, decision classes, the full ADR index, the roadmap, and the "not building" list. |
| 13 | **[SDKs & Integrations](./13-sdks-integrations.md)** | The Go/Node/Python SDKs, Terraform, the MCP hub and gateway, OAuth/SSO, and external integrations. |

## System at a glance

| Aspect | Summary |
|--------|---------|
| **Purpose** | A secure vault + promotion pipeline so AI agents and teams can use secrets without putting them in `.env` files, code, or chat logs. |
| **Backend** | Go + Gin REST API, layered `api → service → crypto`/`repository` with no upward dependencies. |
| **Storage** | SQL (PostgreSQL primary; MySQL/SQLite also supported), treated as untrusted at rest — all secret values are encrypted before insertion. |
| **Encryption** | AES-256-GCM envelope encryption: a master key (from KMS/env) wraps per-project Data Encryption Keys, which wrap per-secret values. |
| **Auth** | JWT for humans, scoped API keys for agents, plus an OAuth 2.0 provider. |
| **Promotion** | Forward-only Alpha→UAT→PROD with diff review, audit trail, four-eyes approval for PROD, and rollback. |
| **Frontend** | React + TypeScript dashboard, and a Shadow-DOM `<keepsave-widget>` that runs on third-party origins as its own trust domain. |
| **Delivery** | Docker images, Docker Compose for dev, a Helm chart for Kubernetes, GitHub Actions CI with least-privilege permissions. |

## Relationship to the rest of `docs/`

This set is the **consolidated overview**. The following remain the **canonical, authoritative**
sources for their domains, and each chapter links to them where relevant:

- [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) — the package dependency map and trust boundaries.
- [`docs/THREAT_MODEL.md`](../THREAT_MODEL.md) — the full STRIDE pass with file:line references.
- [`docs/adr/`](../adr/) — Architecture Decision Records (the "why" behind each decision).
- [`docs/RUNBOOK.md`](../RUNBOOK.md) — incident procedures.
- [`docs/ROLES.md`](../ROLES.md) and [`Roadmap.md`](../../Roadmap.md) — governance and direction.

When the system changes, update **both** the canonical doc and the affected chapter here in the
same PR — an out-of-date overview is worse than none.

## Provenance

This documentation set was produced by a documentation pass over the codebase and then verified
for completeness against the source and the existing docs. The completeness audit trail is
recorded under [`docs/audits/`](../audits/).
