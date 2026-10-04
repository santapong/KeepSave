# KeepSave incident and recovery runbook

![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

Reconciled 2026-10-04 against the harness-neutral source candidate. The canonical
project is `/mnt/data/company/apps/KeepSave`. This runbook describes implemented
controls and required operator exercises; it does not record a production deployment.
See the [documentation hub](README.md), [deployment plan](DEPLOYMENT_PLAN.md) and
[dated acceptance ledger](validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).

Severity: **P0** is data loss or service outage, **P1** is a credential/authority
incident, and **P2** is a bounded degradation. Assign real incident contacts when
installing KeepSave; no PagerDuty, chat channel or staffed on-call service is
configured by the repository.

## 1. Establish the affected installation

Record source revision, actual image digests, migration level, affected resource
IDs, incident time and last verified external backup. Keep credentials, raw proof
URLs, provider tokens and secret values out of reports. Inspect `/healthz`,
`/readyz` and private metrics. An operator-granted account can use
`GET /api/v1/operator/readiness` or `keepsave doctor` for read-only diagnostics.
Configured components and operationally accepted components are separate states.

The reference topology uses one control host with API, frontend, trusted worker,
PostgreSQL and private storage, plus a separate enrolled runner host. Its
application origin is `https://app.keepsave.draveniq.dev`; the published landing
at `https://keepsave.draveniq.dev` is separate. Do not route incident traffic to
the landing or treat it as a backend health check.

## 2. Stop new controlled-tool work

Set `KEEPSAVE_RUN_ADMISSION_ENABLED=false` and
`KEEPSAVE_BROKER_DISPATCH_ENABLED=false` in the affected installation's private
configuration, then perform its authorized coordinated restart. These are
process settings, not a live admin-toggle API. New platform flags default off.

Verify new issuance/admission/dispatch is unavailable while current authorized
status, receipts, cancellation, revocation, audit and recovery remain reachable.
Do not assume a disabled flag revokes an existing record. Revoke affected
sessions, delegation families, grants, runs, bindings or workloads explicitly.
Reenabling a flag must not resurrect revoked authority.

A database outage denies protected operations. Do not introduce a cached allow,
weaker identity check or a runner fallback to bypass it. There is no implemented
universal `ENABLE_WRITES` switch; for a vault-wide incident, drain writers and
restrict traffic using the installation's reviewed operational procedure.

## 3. Compromised browser, API key or delegated authority

1. Identify the stored actor and credential family without copying the token.
2. Revoke the owned session through Account or
   `DELETE /api/v1/account/sessions/:sessionId`; current logout uses
   `POST /api/v1/auth/logout`. Failed commits are not successful server logout.
3. Revoke affected API keys/leases/agent issuance through their supported
   management routes. For delegated tool access, revoke the family and affected
   run/grant. Use [OpenAPI](../backend/internal/api/openapi/core.json) for exact
   paths and authority requirements; no undocumented revoke CLI is assumed.
4. Inspect safe audit metadata and receipts for the affected window. Authorized
   exports are bounded, expire and reauthorize downloads; they contain no values.
5. Rotate the **upstream credential** if its plaintext may have been read.
   Rewrapping/project-key rotation alone does not change that provider credential.

Revocation denies admissions after its database commit and denies subsequent
protected result retrieval. Already-admitted provider requests may finish, and
returned data cannot be recalled. Preserve the external outcome separately from
whether KeepSave is still permitted to deliver its result.

Successful password recovery atomically changes the password, consumes its proof,
revokes browser/linking authority, expires retained API keys, revokes related
leases and denylists persisted agent issuance. Delegated OAuth also checks its
revoked browser parent. An unavailable revocation dependency refuses recovery;
there is no partial-success reset. Unrelated accounts keep their authority.

## 4. Organization offboarding

Create a current organization-scoped preview, inspect its exact impact and
execute with the matching preview/revision and a stable caller idempotency key.
Keep the durable receipt. A changed impact requires a fresh preview; do not
silently execute against different resources.

Removal invalidates the member's old organization authority epoch and dependent
approvals, grants, connections/workloads and runs. Rejoining creates new authority
and does not revive those grants. Unrelated organizations, personal projects and
global browser sessions remain intact. Use separate session/account controls
when the incident requires broader containment.

## 5. Lost or compromised wrapping material

Stop writers and external dispatch. Check the configured key source and retained
wrapped material through the operator's private key-provider procedure.
**Vault Transit is the implemented production reference.** AWS/GCP adapters are
not wired in this binary. Do not substitute a new wrapping key, delete retained
versions or regenerate historical ciphertext to make authentication errors disappear.

An encrypted bundle is recoverable only with its required independent wrapping
material. Losing both ciphertext dependencies and their keys cannot be repaired
by a database restore. Retain Transit key versions and the wrapped master material
needed by current values, revisions, snapshots and external backups. Reprovision
upstream secrets only if recovery is impossible and document the loss honestly.

For project rotation, use the authorized project rotation route. Verify current,
historical and promotion-snapshot reads on approved fixtures and create a new
verified external backup. Signing-key changes and browser-session cutover require
a coordinated operation; do not roll an old session-unaware binary back into traffic.

## 6. External vault recovery

1. Obtain an independently retained encrypted bundle and its documented key
   dependencies. Keep both outside the served frontend tree and checkout.
2. On a trusted host, load the private instance configuration without printing it.
   Run `keepsave-vault -action verify -bundle <external-file>`; this authenticates
   the bundle and returns metadata, not restored authority.
3. Select a genuinely fresh **isolated PostgreSQL database**. Set its private DSN
   in that trusted process, then use `keepsave-vault -action recover-isolated
   -bundle <external-file> -target-name <explicit-name>
   -confirm-empty-isolated-database`.
4. For v2 lifecycle records, supply the explicit
   `-map-lifecycle-owners-to-isolated-custodian=<source UUIDs>` mapping. A missing
   or wrong mapping imports no vault records, but locally created scaffolding
   remains; retry with the corrected mapping in another fresh target.
5. Inspect recovered metadata, selected authorized values, retained history and
   rotated key continuity. Recovery imports no source users, sessions, tokens,
   grants, approvals, operation results or reminder authority.
6. For live restoration, preview and choose explicit records with expected current
   value/metadata revisions and current-member owner mappings. Restoration appends
   revisions; it does not replace a whole project or undelete records.

Record installation-specific evidence before setting
`KEEPSAVE_RECOVERY_VERIFIED=true`. Scheduled backups become due at 02:00 UTC;
retain 30 verified scheduled bundles and at least two. Manual/pre-upgrade bundles
require explicit deletion. Retention failures must remain visible. Local storage
alone is not disaster recovery: maintain independent private external copies.

## 7. Database, worker or runner failure

Drain traffic/writers before a database migration or recovery. Restore authoritative
state through the reviewed installation procedure, then verify migrations, audit
continuity, current revocation and readiness before admitting users. No generic
replica-promotion or DNS failover command is supplied by the reference bundle.

Durable work uses leases/fences and bounded retries. Inspect attempts and outcomes
before retrying. An uncertain external provider or SMTP effect is not blindly
replayed. Explicit provider retries create linked attempts; explicit email resend
invalidates the old proof. Record **SMTP accepted**, not delivered.

A failed runner preflight, broker outage, deadline, denial or cancellation must
refuse/tear down execution. The current development-host observation lacks CPU
cgroup delegation; `cgroup_cpu_missing` is expected refusal, not a reason to
weaken isolation. Inspect the [separate runner reference](../deploy/runner/README.md).

## 8. Promotion kill switch

`KEEPSAVE_PROMOTIONS_ENABLED=false` plus an authorized restart denies new promotion
requests and approvals. Reject, rollback, diff, permitted listing and audit remain
available. Verify `POST /api/v1/projects/:id/promote` is refused before reenabling.
This switch does not undo a completed promotion. Promotion rollback uses the
retained snapshot and normal current-authority/versioned-vault path.

## 9. Upgrade, rollback and break-glass gates

Retain a verified external pre-upgrade bundle and compatible image digests. Drain
old vault/session writers, apply additive migrations with one controlled process,
explicitly baseline existing active projects and validate readiness. New readers
accept v1/v2 recovery bundles; that does not mean old readers accept v2.

Rollback uses a reviewed **compatible** binary. Binaries without enrolled-vault,
tracked-session or admitted-scope guarantees are unsupported rollback targets.
Never reverse immutable journal migrations or restore source authority from a vault
bundle. Measure staging recovery time rather than promising a five-minute result.

If an operator must access production key material, obtain the installation's
required second-person authorization, use its externally audited key/storage path,
record the incident and rotate affected credentials afterward. Do not print keys
in terminal transcripts, chat, tickets or documents. The repository does not
configure an external break-glass audit service.

Independent Security Engineer/Tech Lead reviews, real provider/SMTP/native-client
acceptance, actual runner isolation and full outage/upgrade drills remain release
gates. Historical May deployment steps are retained in dated audits and ADRs;
they are not the current self-hosted procedure.
