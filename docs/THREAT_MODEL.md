# KeepSave threat model — current core candidate

Reconciled 2026-10-02 (Asia/Bangkok) against the local unreleased implementation.
This is a source/evidence update, not an independent security sign-off or a
production risk rating. Historical May audit findings remain in
`docs/audits/`; they must not be treated as the current implementation state.
The [acceptance ledger](validation/2026-10-01-core-release/ACCEPTANCE.md) separates
executed synthetic checks from review, provider UAT and operational gates.

## Assets and trust assumptions

Protect wrapping keys, project key versions, current/historical plaintext,
encrypted backups and snapshot material, human session authority, API-key and
agent parent lineage, organization membership, immutable audit identity and
future broker credentials. Identity and resource metadata can remain plaintext;
not every database column is encrypted.

The browser, SDK, CLI, model, skill text, repository content and client-reported
harness identity are untrusted. Authentication identifies a caller; stored
resource resolution and live policy authorize an operation. The control host,
trusted recovery CLI, database service and configured wrapping-key provider are
operational trust dependencies. AES-GCM mitigates stolen ciphertext and
tampering; it does not protect against a compromised API process that has key
access. A hostile database operator can alter authoritative permissions or
remove entire history; HMAC chaining is tamper evidence within its key-custody
assumptions, not an external immutable receipt service.

The intended application uses same-origin TLS at `app.keepsave.draveniq.dev`;
marketing at `keepsave.draveniq.dev` stays static. No app production deployment,
provider application or runner isolation is established by this change.

## Current threat/control matrix

| STRIDE | Threat | Current control / source | Practical limit / remaining acceptance |
|---|---|---|---|
| S | Forged human token or legacy expire-only JWT | `auth/auth.go`, `service/session_service.go`, API middleware require tracked active sid/jti and token hash; 24h maximum. | HS256 signing secret and trusted DB remain critical. No human refresh flow; drain old binaries at cutover. |
| S/E | Replayed or revoked human session | Session list/logout/owned revoke; database checks on each admission. Identity tests cover foreign targets, unavailable DB and rollback. | Already-admitted work may complete. Provider UAT and operational outage exercise remain separate. |
| S | Social callback identity substitution | One-use state/PKCE, configured exact callbacks, Google identity verification and GitHub verified identity/email handling. | Fixtures do not prove operator configuration or real provider behavior. |
| E | Automatic email linking or canonical collision grants another account | Migration020 canonical identities refuse ambiguity; explicit provider link requires originating active recently authenticated session. | Existing collisions need operator resolution; no automatic merge. |
| E | Open signup creates operator privileges | Registration grants identity only; explicit transactional workspace create/admin membership. Immutable user-ID operator grant requires trusted CLI. | No global permission from an email claim; deprecated email-admin configuration is rejected. |
| E | Wrong project/tenant, viewer credential read, forged resource IDs | Stored resource and policy checks in `repository/authority_store.go`, project access and authorized services; actual-router denial tests. | Legacy compatibility adapters are inventoried; they cannot be assumed migrated. |
| E | Original project owner bypasses workspace demotion/removal | Personal owner authority applies only while organization is NULL; assigned project authority uses current membership for sessions, keys and policy. | Regression covers demotion/removal and personal compatibility; explicit cross-org transfer is deferred. |
| E | Workspace assignment steals foreign project or broadens old delegation | Stored owner + destination administrator, active project, same-org idempotency, cross-org refusal, source key/lease revocation in one transaction. | No implicit account/workspace/project bootstrap. |
| E | Lease or token broadens parent project/environment/keys/expiry | Parent identity and live lineage, persisted issuance and revocation, legacy scope adapter; `agent_token_service`, `policy`. | Vault leases are not future broker run grants. Granted plaintext cannot be recalled. |
| E/I | Private workspace template is exposed or changed by a removed creator | Current stored membership for read/list/apply, current admin for workspace mutation, personal creator for personal mutation, strict human session; required metadata audit/outbox transaction. | Template defaults are ordinary configuration: use placeholders, not live credentials. Core global publication is refused; builtins remain available. |
| T/R | Secret edit lacks a version or success is audited after failed mutation | PostgreSQL vault transaction joins current mutation, immutable revision, required audit and outbox; fixtures force audit failure. | PostgreSQL-only guarantees; unsupported versioned history/recovery refuse 503. No old writer may resume after enrollment. |
| T | Concurrent restore/edit silently loses a newer value | Project serialization and expected current revision; restore appends rather than overwrites history. | Callers must supply explicit restore preconditions; stale revision returns conflict. |
| T/I | Rotation strands history or promotion snapshots | Versioned wrapped project keys retained while ciphertext/snapshot references remain; journal adapters and rotation/promotion tests. | Recovery material must be retained externally. No speculative key purge. |
| T/R | DELETE changes hashed audit identities through FK actions | Migration 023 preserves immutable audit project/user IDs; project tombstones retain resource identity. | Applied migration and restart-chain verification are required; never recompute prior hashes to hide a mismatch. |
| I | Delete treated as erasure or restore resurrects a deleted credential | Secret/project tombstones deny active authority; no undelete in selected/version restore. | Encrypted history/key retention is indefinite in this release; revoked upstream secrets still need provider-side rotation. |
| T/I | Corrupt backup, wrong recovery key or partial recovery | Authenticated bundle verification, metadata preview, selected revision checks; CLI recovery refuses occupied/nonempty-schema targets before migration. External-file seven-check isolated recovery passed. | External key custody and production backup storage operation remain release gates. Recovery excludes authority. |
| T/R | Duplicate jobs or stale worker acknowledges another attempt | `internal/jobs` leases, fencing, bounded retries and persisted uncertain states; PostgreSQL queue tests. | External effects are not atomic with DB; uncertain attempts require reconciliation. |
| T/I | Backup retention deletes only recoverable copy or hides failure | Opt-in scheduling, verified catalog dependencies, thirty daily/minimum two, manual bundles held; audited delete-pending before unlink and failure state. | Worker defaults off until recovery confirmation; no historical key deletion. Operational storage failure drill remains separate. |
| I | Secret/token in error, response, log or metrics | Safe error envelope and logging redaction; metadata-only history list/backup preview/session catalog. | Explicit vault reads/exports contain authorized plaintext. Do not advertise credential confinement before broker delivery. |
| I/D | User-controlled connector command/build or outbound webhook | Core composition disables connector build/execute/install/config generation and unfinished webhook operations. | Legacy source remains an inventory, not an approved runtime. Restricted runner tests are M3. |
| D | Oversized request / recovery bundle exhausts API | 1 MiB general body limit; exact recovery endpoints 90 MiB, corresponding proxy limit; bounded vault selections and worker resources. | No capacity/availability numbers are measured yet; DB-backed cross-replica admission remains M5. |
| S/I | Forged proxy origin/IP or leaked application token | Explicit trusted-proxy list, allowed origins, TLS/reference security headers; proxy does not log OAuth callback query. | CORS is browser isolation, not caller authorization. Host/browser compromise can steal a valid bearer token. |
| E | Unfinished integration advertised as enforced policy | Core capability refusal for AI, policy metadata, legacy OAuth, enterprise SSO, replay and plugin execution. | SDK/widget/CLI compatibility UAT is separate. `/api/docs` covers the bounded core, not every legacy route. |

The source paths are intentionally file-level because this working tree changes
rapidly. Exact executed evidence and dates belong in the acceptance ledger rather
than stale line-number claims.

## Broker, runner and skill threats — future required controls

M2–M5 are unimplemented acceptance programs, not current mitigations. Their
security contract remains: audience/resource/client-bound OAuth, exact PKCE and
callback, atomic authorization-code consumption, refresh replay detection,
installation-qualified tool identity, schema/artifact digests and real Codex
contract checks. Public dynamic registration is deferred.

Provider use must be opt-in through an approved binding and attenuated run. A
run ID is not bearer authority. At admission and broker use, intersect current
membership, organization/project policy, parent grant, approved artifact/profile,
repository/commit/action and expiry. Deny overrides allow; unavailable authority
denies. The broker receives structured identifiers and makes the authenticated
GitHub request; no arbitrary upstream URL or provider token reaches the connector
or model.

The separate runner must prove read-only filesystem, limited scratch/resources,
no host home/socket/database/vault keys, broker-only network, signed tenant-bound
identity, replay denial, cancellation, timeout and crash recovery. Skill text and
repository instructions cannot expand grants. Immutable instruction-only skills
and profiles bind approval to exact digests and policy revisions. Administrator
Codex requirements are verified on the supported version; editable defaults and
self-reported harness names are not enforcement or device attestation.

Revocation denies new operations after commit; already-dispatched work may
complete. Repository data returned to Codex reaches its configured model and
cannot be recalled. Self-hosting does not imply a local model or an administrator-
proof developer device.

## Review and operational gates

Independent Security Engineer/Tech Lead review is pending for Type-1 changes.
Before production require real Google/GitHub UAT, coordinated migration/session/
journal cutover, independent external-backup isolated recovery, deployment TLS/
origin validation, outage/restart/upgrade tests and supported-client contracts.
No unmeasured residual-risk score, throughput, uptime or exactly-once claim is
made. See [architecture](ARCHITECTURE.md), [core ADR](adr/0028-core-identity-and-vault-release.md)
and [self-hosted reference](../deploy/self-hosted/README.md).
