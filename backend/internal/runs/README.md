# Durable controlled tool runs

Source audit: October 4, 2026. This is the default-off PostgreSQL source
candidate under [ADR0029](../../../docs/adr/0029-harness-neutral-platform.md).
See the [documentation hub](../../../docs/README.md),
[architecture](../../../docs/ARCHITECTURE.md), [branding](../../../docs/BRANDING.md)
and [acceptance ledger](../../../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md). Synthetic
GitHub, OAuth, PostgreSQL and runner tests are local evidence. Independent
review, real GitHub/harness use and an isolated runner-host drill remain gates.

## Authority and wiring

Composition supplies `AuthorizeTx`, required transactional audit, the encrypted
Broker, `LockProjectSubjects`, `LockMaintenanceSubjects`, `RequireGrantTx`,
`FamilyPrincipalTx` and an `automation.NativeExporter`. The native adapter lives
in `internal/harness`; this module has no harness-specific configuration types
or candidate names. Only Broker opens GitHub credential material. Result spools
are encrypted by Broker custody and excluded from vault backups.

A management human must have a current tracked session and stored project
membership. New capabilities use explicit mappings: admins manage; editors
start approved runs; promoters or admins approve another person's immutable
profile/package. Viewers inspect metadata. Personal vault projects remain
compatible, but the team approval journey needs an organization and another
eligible approver.

Bindings select one stored project environment from `alpha`, `uat`, or
`development`; other environments are refused in this pilot. Grant authority
binds developer, issuer, current membership epochs, approved source/profile/
package/catalog digests, OAuth client, binding, runner certificate and connector
image. Profile/package approvals expire within 24 hours, further limited by the
approver's session authority. Revoked versions and changed epochs cannot be
reapproved in place: create and independently approve another immutable version.

Browser-created runs select an owned current OAuth family, not merely a client
name. OAuth calls must match the stored run family and original session. The
original access token is pinned in admitted runs and operations; refreshing it
requires a newly authorized run. Human owners may inspect and cancel their runs
from another current session. Departed ancestors block protected operations but
do not hide a current member's own metadata or receipts.

Acquire the full sorted subject barrier, stored organization/project and
membership/session state, then OAuth parent authority before run/operation
locks. Management revocation holds the project mutation barrier. Maintenance
uses the separate reducing-authority barrier, including tombstoned projects and
inactive ancestors; it never creates authority or opens credential/result data.

## Durable lifecycle

Run preparation commits its resolution attempt, required audit and ID-only
`tool.event` before any credential access or provider HTTP request. Broker
resolves the approved reference to a commit and tree SHA. Activation rechecks
current authority and the resolution deadline. Each provider call repeats
admission and charges an actual HTTP call, including installation-token minting.

Migration 033 captures repository ID/name, environment ID/name, installation ID
and requested reference at preparation. The snapshot and reference cannot be
rewritten; discovery, listing and status return these admitted values. A changed
binding, installation or environment denies subsequent operation/result access
without substituting new scope. Older rows without a snapshot remain unknown
and cannot authorize provider work.

`Request` binds caller/run/request-key to a normalized kind/argument digest.
Reusing the key with changed parameters conflicts. Status returns metadata only;
`Result` independently checks current authority before opening the spool.

`Claim` requires an enrolled certificate SHA256 and exact approved image digest.
It issues a short-lived one-use ticket bound to operation, run, grant, attempt,
fence, request digest and nonce. `Execute` accepts only the identical typed
request through the dedicated mTLS router. Its required dispatch receipt and
consumed ticket commit before Broker work. Completion atomically records the
fenced outcome, encrypted spool, byte charge, audit and outbox.

The initial bounds are ten minutes per run, 100 provider HTTP calls including
resolution, 32 MiB cumulative serialized result data, two active attempts, and
at most 30 seconds per attempt. Broker payloads reserve room for the MCP text
and structured-data envelopes; the final transport independently enforces the
four-MiB ceiling. Spool expiry never exceeds operation/run/parent authority.

Cancellation marks dispatched work as cancellation-requested and blocks further
broker admissions/publication. It cannot recall already returned data or prove
that an in-flight upstream read stopped. External outcome remains separate from
delivery authority; revoked result retrieval does not rewrite a completed call
as cancelled. Deadline recovery records dispatched
attempts as uncertain and never replays them. Expired undispatched leases become
an explicit pre-dispatch failure; safe retry is a separate authorized operation
with a new fence. Periodic maintenance deletes expired spools and retains audit
receipts. No automatic connector builds or arbitrary provider URLs exist.

`Flags.Admission` and `Flags.Dispatch` deny new work. Structural readiness keeps
metadata, cancellation, revocation, offboarding and maintenance available even
when `Flags.Enabled` is disabled. Reenabling flags never resurrects revoked
stored grants. Flags are deployment configuration; durable grant/session/epoch
revocations are PostgreSQL authority shared across API instances.

## Interfaces and diagnostics

Management routes are under `/api/v1/projects/:id/tool-platform`. The maintained
core OpenAPI contract includes their actual DTOs. Runner routes exist only on
the dedicated mTLS listener under `/api/v1/runner/operations/{claim,execute,status}`;
caller headers do not authenticate a runner. See the [runner reference](../../../deploy/runner/README.md).

Connection checks require explicit acknowledgments for external metadata reads
and authorization-token creation. They perform exactly the bounded token/repo/
commit workflow, retain durable uncertain outcomes and never write a repository.
They return IDs/status/commit metadata, never provider credentials.

Actual runner execution is not accepted on the current host: read-only Podman
preflight reports `cgroup_cpu_missing`, `isolation_accepted:false`. The supervisor
refuses execution rather than weakening its restrictions.
