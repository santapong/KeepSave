# Backend platform implementation — local checkpoint

Reconciled 2026-10-02 (Asia/Bangkok). The full M0–M5 release is not complete.
This checkpoint supersedes the September 28 statement that vault HTTP adapters
and backup maintenance were unwired. It does not claim provider UAT, production
operation, independent review or a usable GitHub/Codex broker pilot.

## Baseline and scope

Implementation checkout: `/mnt/data/company/apps/KeepSave-backend-platform`,
branch `feat/backend-secure-tool-access-20260928`, base `9e2b3ba`. The accepted
dirty frontend and backend baseline is preserved. The 2 October candidate
preparation names this slice `v1.4.0-rc.1` and aligns version metadata; see the
[candidate notes](../../releases/v1.4.0-rc.1.md). Integration, tagging, push,
production migration, credential configuration and deployment remain pending.
Original preserved patch/manifest live under
`/mnt/data/keepsave-platform-implementation-2026-09-28/`.

The owner approved self-hosted teams, Linux Codex, read-only GitHub App, one
repository/commit per ten-minute run, broker-held credentials and managed team
configuration. Those choices remain settled. The current bounded release is
open-registration login plus a reliable vault, with revocable human sessions
and explicit workspaces. [ADR0028](../../adr/0028-core-identity-and-vault-release.md)
records sponsor authorization; independent Security/Tech Lead signoff is pending.

## Delivery state

| Milestone | Current implementation | Remaining acceptance gate |
|---|---|---|
| M0 authority | Principal/policy ports; parent lineage and revocation; role/project/MCP ownership fixes; typed router dependencies; restricted capability composition. | Review inventory/legacy exceptions and complete compatibility/client acceptance. |
| Core identity | Canonical account identities; password and social fixture flows; server-revocable sessions; explicit transactional workspaces; immutable-ID operator enrollment. | Real Google/GitHub applications/UAT and operational deployment acceptance. |
| M1 reliable vault | PostgreSQL journal integrated into core mutations, history/rotation/promotion/restoration, encrypted backup/catalog/verify/preview/selected restore, isolated recovery CLI and trusted maintenance worker. Local seven-check external-file fresh-target recovery passed. | Production storage/key/runbook acceptance, independent review and coordinated cutover. |
| M2 MCP/OAuth | Not delivered. Legacy issuance/execution refuses in core. | Official SDK, resource-bound consent/OAuth, replay tests, bounded synthetic tool and real supported Codex exercise. |
| M3 broker/runner | Not delivered. | GitHub bindings/runs, broker-only custody, enrolled separate runner, isolation/canary tests and dedicated disposable repository pilot. |
| M4 skills/profiles | Not delivered. | Immutable manifests/approvals, instruction-only review skill, profiles, native install/compatibility/managed requirements and negotiated extension. |
| M5 self-hosting | Core control-host Compose/TLS reference exists; not deployed or a full M5 bundle. | Separate runner, two APIs, admission/worker failures, coordinated upgrades, measured capacity/recovery and team runbooks. |

## Current invariants

- Human admission requires a current stored sid/jti session and token hash. A
  legacy untracked JWT must reauthenticate after a coordinated cutover; no human
  refresh flow is implied by older OAuth code.
- Registration grants identity only. Explicit organization creation joins the
  administrator membership, required audit, PostgreSQL outbox and optional
  caller-scoped idempotency record in one transaction. No sample secrets/default
  projects/global permission. Owner demotion/removal and nonempty deletion fail.
- Personal project attachment requires its stored owner and destination admin,
  active state and no other organization. Same-org retries are safe; first
  attachment revokes old delegated keys/leases. Once assigned, current membership
  governs the original owner too. Personal projects remain compatible.
- PostgreSQL credential mutations share the versioned vault transaction across
  CRUD, import/template, promotion/rollback, rotation and restoration. Required
  audit failure rolls back the mutation, revision and outbox.
- Private workspace templates require current membership for read/list/apply,
  current admin for metadata changes, and no removed-creator bypass. Personal
  templates require their creator. Metadata changes require tracked sessions
  plus audit/local outbox; core global publication is refused. Defaults are
  ordinary config and should contain placeholders, not live credentials.
- Lease/agent-token creation and revocation check current parent/membership/
  session authority and require mutation/audit/local outbox transactions. They
  remain vault read delegation, not future broker run grants.
- Keys remain while retained ciphertext/history/snapshots need them. Secret and
  project DELETE retain tombstones; hashed audit identity fields do not change
  through deletion FKs. No automatic historical purge or live undelete.
- New history and recovery are PostgreSQL-only. Unsupported core dialect paths
  refuse 503; authorized current ciphertext corruption returns safe 500.
- Recovery verifies encrypted bundles with external material, previews metadata
  and requires explicit selected live revision checks. Isolated recovery refuses
  a target containing any non-system tables before migration and never imports source accounts, sessions, grants or
  approval authority.
- Backup maintenance is a separate trusted binary. Scheduling defaults off
  until a fresh isolated drill is operator-confirmed; daily 02:00 UTC, thirty verified
  scheduled/minimum two copies, manual/pre-upgrade held. Two-phase audited
  retention makes unlink failures visible and never purges keys/history.
- Core OpenAPI is embedded from `internal/api/openapi/core.json`; actual-router
  response validation and generated frontend wire types are shipped. Older
  route inventories are compatibility source, not supported-feature evidence.

## Coordinated migration and validation

Additive migrations 015–025 cover social identity, audit head, outbox, vault,
agent issuance, canonical sessions/operator IDs, workspace retries, project
tombstones/immutable audit IDs, promotion source digest and backup catalog.
Do not edit applied migrations or let old writers continue after journal
baseline. Drain traffic/workers, preserve an external backup and recovery key,
apply migrations, explicitly baseline old active projects on a trusted host,
verify the chain/journal, then admit traffic with compatible binaries. Startup
refuses unenrolled active projects. Rollback must use a compatible reviewed
reader/writer; an old binary is not a safe cutover rollback.

The [current acceptance ledger](../../validation/2026-10-01-core-release/ACCEPTANCE.md)
records exact passing and failed commands. PostgreSQL tests use unique disposable
Docker resources, synthetic credentials and per-fixture schemas. SQLite checks
use shipped migrations; MySQL 8.4 has bounded legacy identity/session/scoped
CRUD/workspace-role acceptance, not journal/recovery parity. Historical
[September 28 evidence](../../validation/2026-09-28-backend-platform/README.md)
remains historical and cannot prove later edits.

## Next slices

Complete current review/provider/recovery/operational gates first. M2 synthetic
protocol work may then proceed alongside remaining recovery operations, using
the shared policy contract. Continue M3 → M4 → M5 in the approved order. Each
slice needs a user journey, permission matrix, contract, additive migration,
failures and runbook. The [architecture](../../ARCHITECTURE.md) and
[delivery plan](DELIVERY.md) retain the target patterns and deferred boundaries.
