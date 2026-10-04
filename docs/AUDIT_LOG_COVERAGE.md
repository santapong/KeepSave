# KeepSave audit coverage and transaction contract

![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

Reconciled 2026-10-04 against the harness-neutral source candidate. This replaces
the May/June implementation proposal as the current coverage guide. Historical
findings and their original closeouts remain in [audits](audits/) and
[FOLLOWUPS](FOLLOWUPS.md). Independent review is pending; local executed scope is
in the [acceptance ledger](validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).

## Required behavior

For enabled PostgreSQL security/vault paths, the local mutation, immutable revision
when applicable, required audit event and outbox entry share one database
transaction. A failed required audit/outbox write aborts the operation. Return a
successful mutation or newly issued credential only after commit. The old advice
to log an audit failure and still return success is superseded.

Admissions record the permitted read/dispatch before decryption or external work.
Do not sample sensitive read admissions or defer `secret.read` on enabled vault
paths. Network calls occur outside authority transactions: persisted admission,
external outcome and permitted delivery are distinct records. A required outcome
publication failure cannot be reported as a successfully delivered result.

The persisted audit-chain head is locked **last** in the shared authority order.
Concurrent writers serialize chain updates across processes. Immutable audited
user/project IDs remain stable through deletion; tombstones and migration 023
avoid foreign-key actions rewriting previously hashed content. Do not recompute
old hashes to conceal a mismatch. The existing chain format is unchanged.

## Event inventory by owner

The following is a source-backed event-family index, not a claim that every
legacy route is enabled or operationally qualified. Exact details are defined by
the owning implementation; actor/project/environment fields also live in the
audit row. Avoid inventing fields from the historical taxonomy.

| Owner | Representative current event names | Safe metadata |
|---|---|---|
| Identity/login/session adapters | `auth.register`, `auth.login`, `auth.login_failed`, `auth.session_created`, `auth.session_revoked` | Method, session ID, success/failure context; no signed token/hash/password |
| Organization/project/API-key adapters | `org.created`, `org.member_added`, `org.member_role_updated`, `org.member_removed`, `project.created`, `project.deleted`, `apikey.created`, `apikey.deleted` | Immutable resource IDs, roles, permitted scope metadata; no key plaintext |
| Vault | `vault.baseline.created`, `secret.created`, `secret.updated`, `secret.deleted`, `secret.read`, `secret.restored`, `key.dek_rotated` | Secret ID, revision, key identifier and environment; no value or ciphertext key material |
| Promotion | `promotion_requested`, `promotion_approved`, `promotion_completed`, `promotion_rejected`, `promotion_rollback` | Exact promotion/snapshot/version metadata; legacy names retained |
| Lifecycle | `secret.lifecycle_updated`, `secret.lifecycle_restored`, `secret.reminder_created` | Record and metadata/lifecycle revision, reminder count; no credential value |
| Recovery/backups | `backup.created`, `backup.verified`, `backup.restore.previewed`, `backup.restore.admitted`, `backup.isolated_recovery.admitted`, `backup.isolated_recovery.completed`, `backup.scheduled`, `backup.stored`, `backup.retention.requested` | Bundle/job IDs, counts/integrity/revision metadata; no recovery key or authority payload |
| Identity proofs and methods | `identity.proof_requested`, `identity.proof_rejected`, `identity.proof_consumed`, `identity.method_removed` | Proof ID, purpose, method and account ID; no proof string or raw proof URL |
| SMTP delivery | `identity.delivery_dispatched`, `identity.delivery_completed`, `identity.delivery_uncertain`, `identity.delivery_cancelled`, `identity.delivery_recovered` | Delivery/proof IDs, state and safe reason code; accepted is not delivered |
| Invitations/offboarding | `identity.invitation_created`, `identity.invitation_revoked`, `identity.invitation_accepted`, `identity.offboarding_preview_created`, `identity.member_offboarded` | Organization/member/preview/receipt IDs; no invitation proof |
| Delegated OAuth | `mcp.token_issued`, `mcp.token_refreshed`, `mcp.refresh_replay`, `mcp.token_revoked`, `mcp.delegation_revoked` | Client, consent/family IDs; no authorization code/access/refresh token |
| Artifacts and connections | `tool.artifact.created`, `tool.profile.created`, `tool.profile.approved`, `tool.package.created`, `tool.package.approved`, `tool.connection.created`, `tool.binding.created`, `tool.workload.enrolled` and scoped revocations | Exact IDs/digests/repository target; no GitHub private key or installation token |
| Disclosed diagnostics | `tool.connection.check.admitted`, `tool.connection.check.access`, `tool.connection.check.completed`, `tool.connection.check.expired` | Check IDs, declared external read/token mint, commit/status; no upstream raw response |
| Runs/operations | `tool.grant.issued`, `tool.operation.queued`, `tool.attempt.dispatched` and run/attempt/receipt lifecycle events | Client/run/operation/attempt/fence/receipt IDs and safe outcome; no result content |
| Safe exports | `audit.export_created`, `audit.export_ready`, `audit.export_failed` | Export ID, bounded row count/publication state; no raw journal payload |

New names use dotted entities and snake_case sub-actions; legacy `promotion_*`
names remain compatible. Existing event string literals are the runtime source;
there is no claimed universal generated event-constants registry.

## Application implementation pattern

The transport handler authenticates/translates and calls an authorized service.
The service resolves stored ownership, acquires ordered authority locks, checks
current policy and mutates through its own repository. Its transaction-aware
`event` helper calls `AuditRepository.CreateTx` and enqueues identifier/safe-metadata
outbox work before `Commit`.

[vault/service.go](../backend/internal/vault/service.go),
[identity/service.go](../backend/internal/identity/service.go),
[runs/service.go](../backend/internal/runs/service.go) and
[mcpauth/service.go](../backend/internal/mcpauth/service.go) show the concrete
patterns. Required audit dependency absence denies the operation. Retained
compatibility services using the historical best-effort helper do not establish
the new transaction guarantee; their availability and migration need explicit review.

Outbox payloads contain only identifiers/safe metadata. Email delivery material
and operation results use separate ephemeral encrypted custody. A fenced worker
acknowledges only its current attempt. Uncertain SMTP/provider work is recorded
and requires explicit resend/retry/reconciliation; no exactly-once external-effect
claim follows from a database transaction.

## Safe browsing and export

`auditview` projects a limited typed-reference allowlist rather than rendering
arbitrary `details`. It checks current access, uses deterministic
`(created_at,id)` cursors and caps browsing at 100 rows. Exports materialize safe
metadata in a real repeatable-read snapshot: defaults are a 31-day window,
10,000 rows, 20 MiB and one-hour download expiry. Trusted publication is fenced;
status distinguishes pending, ready and failed. Downloads reauthorize current
sessions/member access. Failed publication cannot become ready through a stale job.

Safe exports exclude values, proofs, tokens, raw request/provider payloads and
operation results. Older raw journal access is separately authorized and is not
the safe projection. Current audit retention defaults to 365 days. Retention of
chain/key dependencies must preserve verifiability; an HMAC chain is tamper evidence
under control-host/key trust, not an external immutable notarization service.

## Verification and operations

For each enabled mutation, sensitive credential read and dispatch contract,
exercise the actual router and
real PostgreSQL with success, cross-tenant/role denial, missing dependency,
audit/outbox failure, concurrency and restart scenarios. Force an audit failure
and assert no partial mutation, issued authority, revision, consumed ticket or
misleading success survives. Verify chain continuity across resource deletion and
process restart. Use synthetic values only.

The dated local receipts prove bounded scenarios, including concurrent writers,
rollback, two-process session revocation, export terminal failure and broker
admission canaries. They do not replace live provider/SMTP/runner qualification
or a full operational fault drill. Monitor chain verification, failed/uncertain
jobs, export publication, backup retention and refusal rates. Do not claim an
alert integration or metric exists unless its shipped configuration is checked.

See [THREAT_MODEL](THREAT_MODEL.md), [RUNBOOK](RUNBOOK.md),
[architecture](ARCHITECTURE.md) and the sole [management contract](../backend/internal/api/openapi/core.json).
