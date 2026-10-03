# Local setup and operational qualification

Use PostgreSQL for the platform; SQLite/MySQL remain bounded legacy compatibility.
Do not point synthetic validation scripts at an operator database. Do not copy
development keys into a production installation.

## Configuration and startup

The API/worker read exported process configuration, not dotenv files implicitly.
Examples are in `deploy/self-hosted/runtime.env.example`; keep the actual file
outside the checkout. All new flags default false:

| Setting | Purpose |
|---|---|
| KEEPSAVE_APPLICATION_ORIGIN | Exact HTTPS application origin; must match configured social origin |
| KEEPSAVE_IDENTITY_ENABLED | Account-method/contact/team proof services |
| KEEPSAVE_TEAM_VAULT_ENABLED | Lifecycle/reminders and safe audit controls |
| KEEPSAVE_MCP_ENABLED | New delegated OAuth and stateless MCP transport |
| KEEPSAVE_RUNS_ENABLED | Tool-platform issuance/execution availability |
| KEEPSAVE_RUN_ADMISSION_ENABLED | New-run/operation admission kill switch |
| KEEPSAVE_BROKER_DISPATCH_ENABLED | External broker dispatch kill switch |
| KEEPSAVE_SMTP_ACCEPTED | Operator-recorded delivery acceptance; not auto-detected |

SMTP host, port, authenticated username/password and sender must be complete.
The trusted worker processes proof-delivery and audit-publication jobs. SMTP
proof endpoints stay unavailable until acceptance is explicitly configured.
Do not interpret a queued proof or configured host as successful delivery.

Apply migrations 001–033 with one controlled new binary after old writers are
drained and an external recovery bundle/material is retained. Existing active
vault projects require explicit baseline enrollment. Changes are additive:
026 authority epochs;027 identity proofs/team records/session auth method;
028 delegated OAuth;029 immutable artifacts/connections/runs/operations;
030 lifecycle/audit exports;031 bound offboarding preview;032 export publication;
033 immutable admitted run-scope snapshots. Existing missing scope remains
unknown and cannot authorize provider work; it is not backfilled from a mutable
current binding.
Legacy non-PostgreSQL additions are no-op adapters, not parity claims.

Run `keepsave doctor` against the read-only operator readiness route using a
current operator-granted account. Readiness distinguishes configured components
from acceptance. Public signup or workspace ownership grants no global role.
Keep logs/commands free of tokens and private files.

## GitHub, profiles and clients

1. Register independent development/production Google and GitHub sign-in apps
   using [SOCIAL_LOGIN_SETUP](../../SOCIAL_LOGIN_SETUP.md). Complete consent,
   repeat sign-in, explicit linking and revocation on the exact application origin.
2. Separately create a GitHub App for the broker with repository contents read
   permission. Install on disposable repositories. Store its private key only
   through the authorized connection API/UI. Configure one explicit nonproduction
   repository ID/owner/name/ref/environment binding. Creating a connection is local.
3. A connection check explicitly discloses installation-token minting and upstream
   repository/reference reads before execution. Do not use real repositories for
   synthetic CI. Required canaries must stay absent from returned data and logs.
4. Create an instruction-only review source, a portable profile and the native
   candidate packages. A different eligible current administrator approves the
   exact profile and package digests. Approval is bounded to24 hours and current
   authority epochs. An expired/revoked/changed dependency requires new approval.
5. Export into a dedicated Codex workspace and separate Hermes profile. Do not
   modify installed Hermes0.21.3, its chosen non-Anthropic provider, sessions or
   memory. Follow [PROTOCOL](PROTOCOL.md) for exact candidate versions and native
   packaging/check commands. Metadata checks never establish interoperability.
6. Register only the two configured public clients with exact callbacks:
   Codex `http://127.0.0.1:17701/callback`; Hermes
   `http://127.0.0.1:17702/callback`. Check port availability first. Use S256;
   do not add dynamic registration, fallback callbacks or weaker auth.
7. The human connects each client through browser consent. An administrator issues
   explicit profile/package/binding/workload/developer/client access. Start a run
   using that developer's owned delegation family and a stable request identifier.
   Reference resolution happens before the commit-bound grant becomes usable.
8. Use distinct runs for each client. Review permitted A; attempt denied B; revoke
   and test new admissions **and result retrieval**. Inspect durable receipts.
   Record exact binary/image/client digests and actual negotiated protocol.

## Separate runner and private listener

Install the [runner reference](../../../deploy/runner/README.md) on a separate Linux
host. Prove cgroups v2 CPU/memory/PID delegation, rootless Podman, seccomp and
Unix-only relay restrictions using the selected kernel/runtime/image. The current
development host fails the CPU delegation check; no permissive fallback is allowed.

Use an independently issued TLS client certificate and enroll its exact SHA256
fingerprint plus a reviewed connector image digest. Supervisor configuration holds
that enrollment key; the container never receives it. The control host runs a
dedicated TLS1.3 listener requiring a verified client chain. Never derive runner
identity from forwarded certificate headers or terminate its TLS at the public SPA
proxy. Mount only restricted runner routes on that listener.

The optional `deploy/self-hosted/runner-listener.compose.yml` adds a port bound to
an explicit private control-host IP and readonly server-key/runner-CA directory.
Use it with the main reference only after enrollment/isolation review. Permit only
the enrolled runner-host network at the operator firewall. Actual keys must be
readable by the API UID65532, private to that installation and outside the checkout.
Validation uses synthetic paths with `config --quiet`; it does not start services.

## Incident and recovery procedures

- A successful account password recovery invalidates all of that account's
  browser sessions, API keys, related leases and persisted agent issuance in
  one transaction with proof consumption and required audit/outbox. OAuth
  delegation continues to check the now-revoked parent browser session. Retain
  these records for audit; obtain a new sign-in session before creating fresh
  credentials. Unrelated accounts are unaffected. A failed reset commits none
  of these changes, and a missing legacy-revocation port refuses the operation.

- Stop admission/dispatch first. Keep current authorized status, cancellation,
  revocation, audit and recovery available. No stale cached allow survives a DB outage.
- Revoke the relevant session/family/grant/run/workload/binding under current
  authority. An admitted upstream call may finish; report its outcome separately
  from denied result delivery. Do not claim returned content was recalled.
- Uncertain provider/SMTP attempts remain visible. Inspect receipts; create an
  explicit linked retry or resend. Never automatically repeat an uncertain effect.
- Verify the external encrypted bundle using independently recoverable material.
  Restore into a fresh isolated PostgreSQL target first. Version2 lifecycle owners
  require explicit mapping to the newly created isolated custodian, supplied via
  `--map-lifecycle-owners-to-isolated-custodian=<source UUIDs>`.
  A missing or incorrect mapping commits no imported vault records or source
  authority, but the isolated CLI target retains its locally created custodian
  and project scaffolding. Correct the mapping and retry into another genuinely
  fresh isolated target; the failed target cannot be reused as an empty target.
- Inspect metadata. Selected live recovery includes expected value/metadata
  revisions and explicit current-member mapping when lifecycle is restored.
  It does not undelete records, replace a whole project or import source authority.
- Record the recovery drill before enabling daily02:00UTC scheduling. Retain30
  verified scheduled bundles and at least two; manual/pre-upgrade bundles require
  explicit deletion. Preserve referenced keys and independent external copies.
- Drain incompatible readers/writers before new bundle-format rollout. Rollback
  means a reviewed compatible binary; old untracked-session/unenrolled-vault
  binaries cannot return to service. Retain external backups before upgrades.

## Required remaining operational exercises

Use one API/one runner fixed hardware, then two actual API processes. Exercise
database/Transit outages, worker/supervisor crashes, duplicate job delivery,
migration contention, restart, upgrade/compatible rollback and isolated external
recovery. Measure queue/admission latency, denial and recovery results. Service
instances in a unit test are useful concurrency evidence but do not replace these
host/process/network drills. No availability/throughput claim is published here.
