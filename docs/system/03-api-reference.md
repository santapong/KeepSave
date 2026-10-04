# 3. API reference

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

The sole maintained management contract is
[`core.json`](../../backend/internal/api/openapi/core.json), served at `/api/docs`.
It contains **112 paths, 141 operations and 177 schemas** in this candidate.
Actual-router tests verify response envelopes and mounted paths; generated
frontend wire types live in [`coreTypes.ts`](../../frontend/src/api/coreTypes.ts).
From the canonical repository root, run
`node scripts/generate-core-api-types.mjs --check` to detect drift.

## Interface groups

Paths below begin with `/api/v1` unless an absolute protocol/probe path is shown.
The OpenAPI file, not this summary, defines request fields and exact responses.

| Group | Routes and purpose |
|---|---|
| Sign-in | `/auth/register`, `/auth/login`, `/auth/providers`, `/auth/social/{provider}/start`, `/complete`, `/auth/logout`. |
| Account | `/account/sessions`, `/account/connections`, `/account/methods`, `/account/contacts`, `/account/contact-proofs`, `/account/proofs`, `/account/notifications`, `/account/delegations`. |
| Recovery identity | `/auth/recovery/request`, `/auth/recovery/confirm`; generic initiation and nonlogged body proofs. |
| Teams | `/organizations`, members, explicit project attachment, invitations and member offboarding preview/execution. |
| Vault | `/projects/{id}/secrets`, `/batch`, `/{secretId}`, `/versions`, explicit historical reads and `/restore`. |
| Lifecycle | `/projects/{id}/secret-lifecycle`, `/secrets/{secretId}/lifecycle`; metadata independent of value revision. |
| Encrypted recovery | `/projects/{id}/backups`, `/verify`, `/preview`, `/restore`. |
| Existing workflows | Project env import/export, promotion/diff/approval/rejection/rollback, rotation, templates, API keys, leases and agent tokens. |
| Audit | `/projects/{id}/audit-log` compatibility; `/audit`, `/audit/exports`, export status and authorized download. |
| Tool management | `/projects/{id}/tool-platform`: catalog, artifacts, profiles, packages, connections, bindings, workloads, grants, runs, operations and receipts. |
| Consent | `/mcp/consent` preview/decision and owned delegation-family revocation. |
| Operator | `/operator/readiness`, operator-ID gated; `/capabilities` reports profile availability. |
| MCP/OAuth | `/mcp`, `/.well-known/oauth-protected-resource/mcp`, `/.well-known/oauth-authorization-server`, `/oauth/mcp/authorize`, `/token`, `/revoke`. |
| Probes | `/healthz`, `/readyz`; `/metrics` requires private exposure. |

The private `/api/v1/runner/operations/{claim,execute,status}` surface is mounted
only on a dedicated verified mTLS listener. It is not a public frontend route and
cannot derive identity from forwarded headers.

## Contracts that matter to clients

- Human sign-in preserves `{user, token}` and Bearer authentication with a
  database-revocable 24-hour session. A valid signature or known ID alone is
  insufficient. Browser sessions and opaque MCP delegation tokens are distinct.
- Batch accepts `{environment, keys}` with 1–100 selected keys. The exact POST is
  classified as a read; out-of-scope records are concealed in `missing_keys`.
- History lists and recovery previews return metadata. Explicit vault value
  reads and env export return permitted plaintext. Restore appends a revision
  using an expected-current-revision condition.
- Run creation accepts approved scope and an owned client delegation. A
  `preparing` run has no usable grant until reference resolution/revalidation.
  Operation creation is asynchronous and uses a stable request key; changed
  arguments under the same key conflict. Status does not include result data.
- Results reauthorize current authority separately from provider-call outcome.
  Client-owned run/operation IDs are handles, not authorization.

Errors retain safe existing envelopes; contract variants are specified per
response. Stale revisions conflict; unavailable platform adapters/authoritative
state fail closed. Raw SQL, provider and crypto errors are not public contracts.
General body limits and exact larger encrypted-recovery limits are enforced by
the router; MCP separately limits request payloads and full response envelopes.

## Availability and compatibility

New identity/team/MCP/tool controls require PostgreSQL and their default-off
flags. SMTP-dependent flows also require installation acceptance. The legacy
OAuth server, API-host MCP execution/builds, unfinished AI/SSO/compliance,
metadata-only policy enforcement and nondurable webhook automation remain
unavailable in the restricted profile. Source presence does not expand support.
Use the [acceptance ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
and [protocol receipt](../design/2026-10-02-harness-neutral-platform/PROTOCOL.md)
for exact tested lanes and external qualification gaps.
