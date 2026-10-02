# 3. API reference

Part of the [system documentation](README.md), reconciled 2026-10-02.

The maintained management contract is
[`backend/internal/api/openapi/core.json`](../../backend/internal/api/openapi/core.json),
embedded and served at `/api/docs`. It contains 52 paths, 70 operations and 87
schemas for the bounded core. `openapi_contract_test.go` validates actual router
response shapes and mounted paths. The generated frontend types are
`frontend/src/api/coreTypes.ts`; regenerate/check with
`node scripts/generate-core-api-types.mjs [--check]` from the repository root.

| Group | Contract / authority |
|---|---|
| `/api/v1/auth/register`, `/login` | Public identity admission; `{user,token}`. Registration creates no workspace. |
| `/auth/providers`, `/auth/social/:provider/start`, `/complete` | Configuration-aware Google/GitHub sign-in; exact callback/state/PKCE. |
| `/auth/logout`, `/account/sessions` | Current-session logout, owned metadata and individual revoke; no token material in list. |
| `/account/connections` | Explicit same-session provider linking; no automatic account merge by email. |
| `/organizations` and members | Explicit named workspace and current administrator operations; optional caller-scoped creation `Idempotency-Key`. |
| `/organizations/:orgId/projects` | Active personal-project assignment requires stored owner and destination administrator; no cross-org transfer. |
| `/projects` and `/:id` | Authorized metadata, transactional project enrollment and retained project DELETE tombstone. |
| `/projects/:id/secrets`, `/batch`, `/:secretId` | Scoped credential CRUD and bounded POST batch; explicit reads return values. |
| `/:secretId/versions`, `/:version`, `/:version/restore` | Metadata history list, explicit historical value read, append-as-new restore with expected current revision. |
| `/projects/:id/backups` | Encrypted bundle creation and metadata catalog. |
| `/backups/verify`, `/preview`, `/restore` | Authenticated bundle verification, metadata diff and explicitly selected live restore. |
| `/env-import`, `/env-export`, `/rotate-keys`, `/verify-encryption` | Authorized import/export and versioned project key continuity; export deliberately contains permitted plaintext. |
| `/promote`, `/promote/diff`, `/promotions` and actions | Exact source artifact, current authority, eligible four-eyes approve/reject and revision-aware rollback. |
| `/templates`, `/builtin`, `/:templateId`, `/apply` | Personal creator/current workspace read/admin scope; tracked human metadata mutation/audit/outbox; global publication refused in core. Defaults are ordinary config/placeholders. |
| API keys, leases and agent tokens | Current parent/session/membership, narrowed scope/expiry, required mutation/audit/outbox; vault read delegation, not broker runs. |
| `/capabilities` | Truthful current profile/availability metadata. |

All project IDs and secret ownership are resolved from stored records. Human
routes require active stored sessions; key routes apply current project role and
parent scopes. An assigned project's former owner is governed by current
organization membership. Viewer metadata permission does not grant credential
reads. A run ID, prompt or client-reported harness cannot grant authority.

Safe errors use the existing `{error: ...}` envelope. Stale restoration is 409;
unsupported core history/recovery on non-PostgreSQL is 503. Authorized current
ciphertext corruption is a safe 500 rather than an absent-record 404. Bodies are
limited to 1 MiB generally and 90 MiB on exact encrypted recovery verification/
preview/restore paths. Raw provider/DB/crypto error details are not public contracts.

## Compatibility surfaces

`router.go` retains the existing `/api/v1` route families incrementally. Core
promotion/import/export/template/key/grant controls have actual-router contract
checks. Applications, widget, feedback and operations remain individually inventoried in the
[acceptance ledger](../validation/2026-10-01-core-release/ACCEPTANCE.md); they are
not all newly verified frontend/SDK contracts. Applications/favorites remain
metadata, not harness management.

Unfinished AI, policy metadata, enterprise SSO, webhook automation, legacy OAuth
issuance, event replay, plugin mutation and API-host MCP operations return explicit
capability refusal in the core profile. Legacy OAuth discovery is not a delivered
MCP authorization server. Standards-based `/mcp` is M2 and provider bindings/run
grants are M3. Health/readiness remain probes; `/metrics` requires private operator
exposure and the reference public TLS proxy denies it.
