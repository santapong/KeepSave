# 10. Operations and recovery

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

Start with the [control-host reference](../../deploy/self-hosted/README.md),
[platform setup](../design/2026-10-02-harness-neutral-platform/SETUP.md) and
[runner reference](../../deploy/runner/README.md). The canonical checkout is
`/mnt/data/company/apps/KeepSave`; private runtime/recovery files and raw evidence
belong outside Git. Read-only `keepsave doctor` distinguishes configuration from
operator acceptance and does not send mail or probe providers implicitly.

## Coordinated cutover

Drain incompatible API/worker writers. Preserve an external encrypted backup and
separate recovery material. Resolve canonical-email collisions explicitly, apply
additive migrations 001–033 and run `keepsave-vault -action baseline` for old active
projects. Baseline enrolls current ciphertext without inventing past history.
Startup refuses unenrolled active projects. Legacy untracked human JWTs must
reauthenticate. Do not roll back to old session/vault writers or an incompatible
bundle reader; rollback requires a reviewed compatible binary.

Operator enrollment uses immutable user ID. Register exact configured Google/
GitHub sign-in callbacks separately from the GitHub App broker. SMTP requires
certificate-verified authenticated STARTTLS and installation acceptance before
contact/invitation/recovery endpoints become available. Do not label SMTP accepted
as delivered. Account recovery invalidates sessions and retained delegated
credentials atomically; missing revocation wiring refuses the reset.

## Encrypted recovery and retention

Verify an external bundle with `keepsave-vault -action verify` and independently
recoverable wrapping material. `recover-isolated` requires explicit confirmation
of a genuinely fresh PostgreSQL database, creates a local custodian/project and
never imports source accounts, sessions, grants or approvals. Recovery v2 carries
lifecycle metadata; map source owners explicitly to the isolated custodian using
`--map-lifecycle-owners-to-isolated-custodian`. Readers accept v1 with unknown
missing fields; deploying a compatible reader precedes v2 writers.

An incorrect/missing lifecycle mapping imports no vault records or authority but
leaves locally created target scaffolding. Correct it and use another fresh
isolated target; the failed target is no longer empty. Inspect recovered history,
keys/snapshots and metadata before selected live restoration. Live restore checks
current value/metadata revisions and explicit currently permitted owner mapping;
it does not replace a whole project or undelete records.

`KEEPSAVE_RECOVERY_VERIFIED` remains false until installation-specific recovery,
key dependencies and private/external storage pass. Then the trusted worker
schedules daily 02:00 UTC verified encrypted bundles, retaining30 scheduled copies
and always at least two. Manual/pre-upgrade copies require explicit deletion.
Durable jobs use leases/fences and stable artifact identity; retention commits
catalog/audit delete-pending before unlink and records failures. Tombstones,
historical/snapshot ciphertext and referenced keys have no automatic purge.
Audit retention retains its existing 365-day default and re-anchor semantics.

## Revocation and uncertain outcomes

Disable new admission/dispatch first while preserving authorized status,
receipts, cancellation, revocation, audit and recovery. Revoke the specific
session/family/grant/run/workload/binding under current authority. Later admissions
and result retrieval deny after commit, but an admitted upstream call may finish.
Report external outcome separately from denied delivery. Already returned content
cannot be recalled; credentials disclosed by ordinary vault clients may require
provider-side rotation.

Operations reconcile lost replies by caller/run/request key and canonical digest;
changed arguments conflict. Explicit retry creates a linked attempt, not silent
reexecution of uncertainty. Explicit proof resend invalidates old proofs; an
uncertain SMTP send is not automatically repeated. Native client interruption is
not evidence of durable cancellation—use the owned operation/run control.
Promotion's kill switch independently gates new promotion/approval and preserves
permitted rejection/rollback/status.

Authoritative database/Transit outages fail protected access closed. Never restore
stale allows, rewrite audit hashes to hide corruption or resurrect revoked grants
from recovery. Record restart/failure/drill evidence with exact source/image/host
versions. Bounded two-API revocation and external CLI recovery passed locally;
full deployed faults/upgrades and measured capacity remain gates in the
[acceptance ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).
