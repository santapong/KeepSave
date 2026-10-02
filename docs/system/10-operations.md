# 10. Operations and recovery

Part of the [system documentation](README.md), reconciled 2026-10-02.
The [self-hosted reference runbook](../../deploy/self-hosted/README.md) is the
current operator starting point; it is not a completed production installation.

## Coordinated cutover

Drain old API/worker writers. Preserve an external encrypted backup and separate
recovery material before additive migrations. Resolve ambiguous canonical account
identities explicitly; never automatically merge them. Apply shipped migrations,
run trusted `keepsave-vault -action baseline` for old active projects and verify
the journal/audit chain. Baseline labels current encrypted values only. Startup
refuses unenrolled active projects. Admit traffic using compatible binaries and
require legacy human sessions to reauthenticate. An old binary must not resume
writing after enrollment or serve as an unsupported session/format rollback.

Operator enrollment uses `keepsave-operator` and an immutable user ID, not an
email claim. Provider configuration uses the exact application callback/origin
and private operator secrets; see [social setup](../SOCIAL_LOGIN_SETUP.md).
No live credentials or provider applications are installed by test fixtures.

## Recovery and retention

Verify an external encrypted bundle with `keepsave-vault -action verify`. An
isolated drill uses an explicitly confirmed fresh PostgreSQL database with no non-system tables and a new
project name. It must prove independent recovery material, record/key continuity
and absence of imported accounts/sessions/grants. A database with existing non-system tables is refused before migrations; an
operator advisory lock prevents concurrent recovery admission.
Live selected restore first shows metadata differences, then checks exact
selected backup/current revisions. It does not replace an entire project or
resurrect deleted records/authority.

Scheduled maintenance and retention default off. After a fresh isolated drill the operator
can enable `KEEPSAVE_RECOVERY_VERIFIED` for `keepsave-worker`; daily 02:00 UTC
backups retain thirty verified scheduled artifacts and at least two verified
copies. Manual/pre-upgrade artifacts stay held. The private 0700 directory is
trusted local storage; durable jobs use leases/fences and stable per-job file
identity. Catalog/audit delete-pending commits before filesystem unlink; failures
remain visible. Verify actual external-copy/key dependencies before production.

Secret/project deletion retains tombstones and dependent encrypted history/key
versions. There is no automatic historical/key purge. Audit retention stays 365
days by default; the chain retains its existing re-anchor semantics. Do not
recompute hashes to conceal corruption. A wrapping-key incident can render data
unrecoverable; process memory and external key custody remain trusted.

## Incident boundaries

Revoke the specific active session/key/lease or later broker run; new admissions
after commit must deny. Returned data cannot be recalled and already-admitted
work may finish. Upstream credentials previously disclosed by vault APIs need
provider-side rotation. The promotion kill switch denies new promotion/approval
while status/reject/rollback remain accessible for draining.

Database/key-provider outage fails protected access closed; no stale allow
fallback is acceptable. Check readiness and audit/key integrity after restart.
Unknown external-effect outcomes are uncertain, not blindly retried. Queue
primitives and maintenance fixtures do not establish durable webhook execution.

Production release still requires actual TLS/origin validation, provider UAT,
production key/storage recovery, host/storage failures, upgrade/rollback and broad
two-replica tests. Local external-file recovery and two running API instances'
session revocation passed; they do not measure production availability/capacity. Document exact
commands and outcomes in the [acceptance ledger](../validation/2026-10-01-core-release/ACCEPTANCE.md).
