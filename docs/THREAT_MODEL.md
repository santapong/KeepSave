# KeepSave threat model — core and harness-neutral local candidate

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
broker credentials, OAuth families, proof delivery material, ephemeral results and enrolled runner identity. Identity and resource metadata can remain plaintext;
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

## Harness-neutral additions — implemented controls, pending qualification

The following source controls now exist in the local working tree. The
[current acceptance ledger](validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
records synthetic evidence; they do not establish deployed mitigations.

| Threat | Local control | Remaining boundary |
|---|---|---|
| Token replay/resource substitution | New opaque OAuth path: exact canonical resource/client/issuer/callback, S256, atomic code consumption, refresh-family rotation/replay revocation, current browser parent checks | Exact native client negotiation and real consent remain unqualified; no dynamic registration |
| Removed/rejoined member retains authority | Sorted subject/project barriers and member authority epochs; revision-bound scoped offboard snapshot/execution; full dependent grants/runs cascade | Unrelated org/personal authority remains; old returned data cannot be recalled |
| Last login method removed / proof theft | Remaining-method recent successful auth, hashed256-bit purpose/account proofs, expiry/attempt bounds; successful recovery atomically invalidates delegated/browser/link authority | Real SMTP acceptance gated; verified contact does not attest a person |
| Proof leakage / repeated uncertain send | Vault-encrypted ephemeral delivery; ID-only jobs; clear browser fragments; STARTTLS certificate-verified SMTP; uncertain sends not automatically replayed | SMTP accepted is not delivered; operator mailbox/SMTP trust remains |
| Audit/export exfiltration | Safe typed-reference projection, current admission/download, repeatable-read bounded snapshot, fenced publication, one-hour expiry | Raw legacy journal access is separately authorized; no immutable external notarization |
| Wrong repository / altered profile | Stored binding/commit/client-family grant, independently approved source/profile/package digests, current policy/epoch checks at admission and broker redemption | Minimal fixed pilot profile; no universal harness/device enforcement |
| Provider credential escape | Broker-held GitHub App keys/tokens; structured allowlisted tree/UTF8 file requests; no arbitrary upstream URL/header; bounded safe results and canary tests | API/broker/control-host compromise can obtain keys; live GitHub custody UAT still required |
| Stolen runner ticket / duplicate dispatch | Direct TLS1.3 verified enrolled client certificate, short ticket bound to attempt/fence/run/digests/nonce; atomic redemption; current revalidation | Certificate identifies supervisor, not hardware; external effects are not exactly-once |
| Connector host/network escape | Rootless Podman reference, readonly filesystem, Unix-only relay, networknone, CPU/memory/PID/scratch limits, no host credentials or engine socket | Actual kernel/runtime enforcement is unproven here; current host refuses missing CPU delegation |
| Revocation followed by delivery | Current-authority result read, encrypted run-bounded result storage, separate external outcome/publication state; explicit cancel | Already-admitted provider calls may finish; downloaded content remains available to recipient/model |
| Instruction/package tampering | Immutable instruction-only source, native tree manifests and digest-bound independent approval; no executable skill scripts | Native copies/metadata checks do not attest device or prove harness behavior |

GitHub social sign-in and GitHub App connections have separate identities and
credentials. Model-provider credentials and subscription sessions are not held
by this broker. Self-hosting KeepSave does not make the harness's selected model
local; permitted repository content still reaches that model.

Lifecycle metadata is separate from credential-value revisions and contains
only declared dates/provenance/responsibility. Recovery v2 adds encrypted lifecycle
metadata with explicit current-member or isolated-custodian mapping. It imports
no source identity/delegation/run/approval/session authority or ephemeral results.
Legacy missing lifecycle metadata remains unknown, and retained key/history
references prevent speculative purge.

## Review and operational gates

Independent Security Engineer/Tech Lead review is pending for Type-1 changes.
Before production require real Google/GitHub UAT, coordinated migration/session/
journal cutover, independent external-backup isolated recovery, deployment TLS/
origin validation, outage/restart/upgrade tests and supported-client contracts.
No unmeasured residual-risk score, throughput, uptime or exactly-once claim is
made. See [architecture](ARCHITECTURE.md), [core ADR](adr/0028-core-identity-and-vault-release.md)
and [self-hosted reference](../deploy/self-hosted/README.md).
