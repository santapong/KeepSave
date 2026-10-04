# 2. Backend architecture

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

## Composition and process ownership

[`cmd/server`](../../backend/cmd/server/) loads exported configuration, opens SQL,
applies additive migrations, obtains the wrapping key and constructs typed
[`api.Dependencies`](../../backend/internal/api/router.go). The positional
`SetupRouter` remains a legacy/fixture adapter. PostgreSQL startup refuses active
vault projects without explicit journal baseline enrollment. The restricted
application composition keeps API-host connector execution/builds disabled.

| Binary | Responsibility |
|---|---|
| `server` | Public application API/MCP; optional dedicated private mTLS runner listener. |
| `keepsave-worker` | Trusted proof delivery, audit publication, reminders and opted-in backup maintenance. |
| `keepsave-operator` | Operator grants keyed by immutable user ID; no email-based global authority. |
| `keepsave-vault` | Trusted-host baseline, bundle verification and isolated recovery. |
| `keepsave` | Existing API CLI plus read-only operator `doctor`. |
| `keepsave-harness` | Native package export, metadata/digest check and safe unpack. |
| `keepsave-runner` | Enrolled supervisor on a separate rootless Linux host. |
| `keepsave-connector` | Bounded connector using only its per-attempt Unix relay. |

Operator/recovery binaries are explicit trusted-host commands, not remote admin
endpoints. The supervisor holds its enrollment key and container-engine access
outside connector containers.

## Module boundaries

| Package | Ownership |
|---|---|
| `internal/policy`, `internal/authority` | Shared decision vocabulary, stored authority and ordered database barriers. |
| `internal/identity`, existing auth adapters | Accounts, sessions, verified contacts, method safety, proofs and scoped team changes. |
| `internal/vault` | Value/key custody, revisions, lifecycle, recovery and ephemeral encrypted payloads. |
| `internal/service/promotion_vault.go` | Existing promotion adapted to authorized vault transactions. |
| `internal/auditview`, `internal/jobs` | Safe audit read/export and durable fenced work. |
| `internal/mcpauth`, `internal/mcpgateway` | Delegated OAuth and stateless MCP translation. |
| `internal/runs`, `internal/broker` | Grants, budgets, attempts/results and typed provider calls. |
| `internal/automation`, `internal/harness` | Immutable source/profile/package governance and native rendering. |
| `internal/runner` | Restricted supervisor/relay; no database, vault or provider credentials. |

Legacy `internal/service` and `internal/repository` adapters remain where a
capability has not fully moved. Narrow ports preserve compatibility without a
second permission evaluator. Handlers translate requests; transport and harness
adapters do not perform SQL, decryption or process execution. Existing exceptions
are bounded by [`boundaries_test.go`](../../backend/internal/architecture/boundaries_test.go)
and cannot expand.

## Admission and concurrency

Middleware validates transport, origin, body limits, logging, proxies and
credentials. Authorized services reconstruct resource ownership and check current
session, membership, scope, parent lineage, expiry and revocation under database
barriers. The lock contract orders known humans, organizations/projects,
membership state, sessions/delegations, provider/workload records, runs/attempts,
approvals/vault and finally audit head. Intended lock modes are selected before
acquisition; discovery is revalidated without network work in the transaction.
Ownership drift currently fails closed; bounded automatic rediscovery is future
work, not a delivered retry guarantee.

`config.Load()` reads process environment, not dotenv implicitly. See
[`config`](../../backend/internal/config/), [setup](../design/2026-10-02-harness-neutral-platform/SETUP.md)
and the [control-host reference](../../deploy/self-hosted/README.md). Production
refuses known development keys, short JWT secrets, wildcard CORS, insecure
PostgreSQL settings and deprecated administrator-email configuration. New
capabilities default off; configured components are not accepted installations.
