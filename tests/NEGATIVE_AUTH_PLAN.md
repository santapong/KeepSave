# KeepSave negative-authority coverage plan

Source audit: October 4, 2026. The original Phase-A 12×11 proposal is historical;
its claim that all middleware cases are untested is obsolete. Actual-router,
PostgreSQL and compatibility fixtures now cover many denials. This document maps
current sources and remaining qualification rather than marking every proposed
cell complete without execution evidence.

## Current routes and test sources

| Surface | Required denials | Source |
|---|---|---|
| `/auth/login`, protected browser routes | Bad signature, expiry, missing/revoked/hash-mismatched SID, database outage, generic unknown-user failures. | `identity_sessions_test.go`, `platform_authorization_test.go`, `negative_auth_test.go`, auth/service fixtures. |
| `/projects/{id}/secrets`, batch and revisions | Foreign/missing project, viewer value read, wrong environment/key scope, denied referenced dependencies, parent/sibling widening, tombstone. | `negative_auth_matrix_test.go`, `platform_vault_test.go`, `platform_authorization_test.go`, `secret_scope_test.go`. |
| `/api-keys`, project leases/agent-token | Current owner/membership/session, narrowed scope/expiry, revoked parents and concurrent recovery/mint. | `platform_lease_transactions_test.go`, `identity_recovery_delegations_test.go`, `platform_membership_authority_test.go`. |
| Organization/project changes | Owner demotion, foreign assignment, cross-org transfer, nonempty deletion, offboard/rejoin stale epochs. | `platform_workspace_test.go`, `platform_membership_authority_test.go`, `identity_platform_test.go`. |
| Promotion/rollback | Viewer, wrong project, requester self-approval, changed/expired source binding, concurrent execution and stale rollback. | `handlers_promotion_test.go`, `platform_vault_test.go`, service promotion tests. |
| Proof/recovery/method changes | Replay, wrong account/purpose, stale resend, last method, revoked inviter, missing reset revocation port. | `identity_platform_test.go`, `identity_recovery_delegations_test.go`, identity/service PostgreSQL fixtures. |
| Delegated OAuth/MCP | PKCE, callback/resource/issuer/client/Origin mismatch, code race, refresh replay, revoked parent/family. | `handlers_mcp_platform_test.go`, `mcpauth/*_test.go`, `mcpgateway/*_test.go`. |
| Runs/operations/results | Foreign client/run/key, binding/artifact drift, expiry/budget/fence/ticket misuse, revoked result delivery. | `platform_tool_router_test.go`, `runs/service_postgres_test.go`. |
| Safe audit/lifecycle | Foreign/missing parity, viewer constraints, cursor/selection limits, old-owner exports/downloads. | `platform_team_vault_test.go`, `platform_audit_publication_test.go`, `auditview/service_test.go`. |

Names in the source column are under `backend/internal/api` unless a package is
shown. The canonical API-key creation/deletion paths are `/api/v1/api-keys` and
`/api/v1/api-keys/{id}`, not the old proposed project-prefixed route. The exact POST
secret batch is **read**, accepts 1–100 keys and conceals unauthorized keys as missing.

## Test requirements

Use the actual router and stored ownership/current authority. Browser fixtures
need real tracked 24-hour SID/hash state; signature-only legacy middleware tests
cannot prove current session revocation. API keys, agent tokens, OAuth delegates
and enrolled workloads are distinct principals, not interchangeable headers.
Foreign and nonexistent resources retain indistinguishable public denials.
Validate safe body/envelope against the maintained OpenAPI contract per route;
not every endpoint shares one hardcoded error-code shape.

Force required-audit/outbox failure and assert rollback before success publication.
Use real PostgreSQL for ordered-lock races and transaction guarantees, while
keeping SQLite/MySQL compatibility fixtures separately scoped. Validate that
no denied operation reaches credential custody/provider dispatch. A stored run ID,
client name, skill or self-reported digest is never authorization.

Execution receipts must name source revision/runtime/scenarios/skips. Source
presence is not a pass, and the older 12×11 matrix is not an exhaustive inventory
of today's endpoints. Full provider/native harness/host acceptance remains open.

## Running and remaining work

```bash
# From the canonical repository root; unique disposable PostgreSQL only.
bash scripts/test-platform-postgres.sh
```

The [current ledger](../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
records executed negative/rollback/concurrency coverage. The original per-endpoint
presence checker and full automated coverage matrix remain unclaimed unless
implemented and actually run. Follow the [threat model](../docs/THREAT_MODEL.md),
[safe error standard](../docs/ERROR_HANDLING_STANDARD.md),
[architecture](../docs/ARCHITECTURE.md) and [documentation hub](../docs/README.md).
