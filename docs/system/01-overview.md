# 1. System overview

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

KeepSave is a self-hosted credential vault and harness-neutral access platform for
developer teams. The integrated source candidate contains reliable identity and
vault services plus team lifecycle controls, delegated MCP/OAuth, portable review
profiles, client-bound runs, a GitHub broker and a separate runner reference.
**Source availability is not operational acceptance.** The package remains
`1.4.0-rc.1`, unreleased and untagged; use the
[current acceptance ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
for executed checks and remaining gates.

## User journeys and scope

A user signs in with password or an operator-configured Google/GitHub provider,
explicitly creates a named workspace, manages credentials, inspects history,
restores a revision and verifies encrypted recovery material. Registration
creates no sample projects, secrets or global permission. Personal projects stay
personal until their stored owner and a destination administrator authorize
attachment. Assigned projects use current workspace membership for everyone.

The tool-access candidate adds a separate journey: an administrator approves an
instruction-only review profile and exact package digests, binds a GitHub App to
one nonproduction repository, and authorizes a developer/client. The developer
starts a bounded run whose reference resolves to a commit before activation.
MCP reads queue operations; result delivery checks authority again. The broker
makes authenticated GitHub calls, keeping the provider token out of the connector
and model. Codex and Hermes use separate delegations and runs for the same
portable profile; neither is yet qualified as a supported real client.

| Surface | Boundary |
|---|---|
| Ordinary vault | Authorized clients receive permitted plaintext values. |
| Controlled provider tools | Broker retains provider credentials and returns permitted repository content. |
| Skills and native configuration | Instructions and packaging; they grant no server authority or device attestation. |
| Revocation | Denies later admissions and protected result retrieval after commit; admitted calls may finish. |
| Recovery | Imports encrypted vault data into explicitly authorized targets, never source identity/grants. |

## Architecture and product identity

Go/Gin remains a modular monolith with PostgreSQL and a trusted worker. Shared
policy and authority services serve REST, MCP and compatible clients. Local
mutations join required audit, immutable revisions and outbox writes in one
transaction. The connector supervisor belongs on a separate Linux host and has
no database or vault access. New platform guarantees are PostgreSQL-only;
SQLite and MySQL retain bounded legacy compatibility.

The accepted visual identity is **Field Twist / Event Horizon**, with violet and
mint accents and legible task icons. KeepSave's black-hole imagery is a visual
metaphor, not a security guarantee. See [branding](../BRANDING.md),
[current architecture](../ARCHITECTURE.md) and
[architecture views](../ARCHITECTURE_VIEWS.md).

The canonical local project is `/mnt/data/company/apps/KeepSave`.
`keepsave.draveniq.dev` serves the separate static landing; the same-origin full
application at `app.keepsave.draveniq.dev` remains a deployment target. All new
platform flags default off. Real sign-in/SMTP/GitHub/harness acceptance, runner
isolation, independent reviews and full operational drills remain required.
