# Backend platform validation — 2026-09-28

Scope: local M0 fixes and internal M1 components. **Not M0–M5 release acceptance.** Base `9e2b3ba`, uncommitted implementation in `feat/backend-secure-tool-access-20260928`. Accepted UI baseline copied from KeepSave-landing and preserved there.

Evidence directory: `/mnt/data/keepsave-platform-implementation-2026-09-28/`.

| Check | Evidence | Observed result / limit |
|---|---|---|
| Original behavior, real PostgreSQL/router | `m0-before.log` | Failure reproduced: PostgreSQL lease array encoding; private MCP server read returns 200 to another user |
| Original behavior, shipped SQLite schema/router | `m0-before-sqlite.log` | Lease creation reproduces the API-key foreign-key failure |
| Authorization, audit and queue contracts | `platform-final.log`, `backend-final.log`, `ci-platform-script.log` | Passing PostgreSQL tests, including independent repository instances; synthetic fixtures only |
| Backend race + shuffled suite and vet | `backend-final.log` | Passed after role/custody fixes; any subsequent scoped reruns listed below |
| SQLite compatibility | `sqlite-and-vulnerability-final.log` | Passed against shipped migrations; PostgreSQL-only vault/job tests intentionally skip |
| Scope matcher fuzz | `fuzz-vulnerability.log` | 690,781 executions over ten seconds; no failure |
| Dependency vulnerability check | `sqlite-and-vulnerability-final.log` | No reachable vulnerable functions reported; one imported-package and five module-level advisories remain classified unreachable by this run |
| Production container build | `container-build-final.log` | Local image built after final source edits; not deployed or published |
| Frontend | Baseline manifest | No new frontend changes in this implementation; frontend tests/build were not rerun |
| MySQL | Code compatibility inspection only | Not executed; new vault/jobs capabilities remain PostgreSQL-only |
| GitHub / Codex / runner | None | No live-provider UAT, protocol integration or isolation claim |

## Regression coverage

- Correct key identity, allowed lease/mint/read flow and audit actor.
- Rejection of sibling keys, wider key selection, wildcard escalation, excessive parent lifetime, wrong environment and revoked/narrowed/deleted parents.
- Explicit revocation denies sibling keys and unrelated projects using a recorded issuance.
- Persisted token denial visible through a separately constructed repository without cache refresh.
- Viewer credential routes, editor administration/approval routes, removed membership and template service role checks.
- Private MCP visibility/install denial, foreign project binding denial and installer-owned mutation.
- Disabled production builder and HTTP execution routes.
- Concurrent audit appenders, rollback of mutation/audit/head, verification after rollback.
- Single job claim, advancing fence, rejected stale acknowledgment, no automatic replay of uncertain external dispatch, persisted effect semantics independent of worker-supplied metadata.
- Versioned-vault baseline labeling, current/historical reads through key rotation, promotion-snapshot key retention, stale/concurrent restoration, deletion without resurrection, and failed-audit rollback.
- External encrypted backup file verified with independently reconstructed recovery material; wrong keys and corrupted bundles rejected. This is **not** a restore into a fresh application database.

## Reproduction

`./scripts/test-platform-postgres.sh` runs `TestPlatform*` with the race detector on a disposable PostgreSQL 16 target. It uses a fixed synthetic database identity, no published port, and a unique schema per fixture. It cannot accept a production DSN. If it created its test container it removes it on exit; a preexisting named test container is reused and retained. Ordinary `go test ./...` also exercises the SQLite compatibility path.

The new `platform-postgres` CI job runs this script for `develop` and `main` pull requests/pushes. Existing race/vet/fuzz/dependency/container checks remain. GitHub-hosted CI has not been run from this uncommitted checkout.

## Remaining acceptance

See [implementation checkpoint](../../design/2026-09-28-backend-platform/IMPLEMENTATION.md). Required independent Security/Tech Lead review is pending. New journal schemas and tested component APIs do not establish full history integration, operational recovery, OAuth/MCP interoperability, GitHub broker custody or enforced runner isolation.
