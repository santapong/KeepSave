# Proposed feature specifications and backlog

All F01–F09 entries are proposed, not implemented by this planning task.
Proposed API names must be reviewed against the maintained `/api/v1` contract
before implementation. Owners are roles; reviewers have not been assigned.

Every slice requires a permission matrix, complete user journey, failure tests,
additive migration where needed, OpenAPI/types, audit assertions, capability flag,
rollback/runbook and acceptance-ledger entry. Default new flags off until their
journey is accepted. Authentication/authority/custody/audit changes follow the
repository ADR and independent review requirements.

## F01 — Task-based setup and operator doctor (P0/P1)

**Journey:** an operator identifies what is missing before exposing the app; a
developer creates a named workspace and performs the first permitted read using
tested instructions. This improves the existing setup, not the vault engine.

- Identity exposes enabled sign-in methods and safe public readiness; an
  operator-only application service aggregates database/migration/baseline,
  canonical-origin, key-provider, backup-age and compatibility evidence.
- Proposed CLI: `keepsave doctor --format=json`; privileged readiness endpoint
  `GET /api/v1/operator/readiness`. Return component state, `checked_at`, evidence
  level, safe reason and remediation link. Never return DSNs, credentials or
  root-key data. Liveness remains independent of optional readiness checks.
- Query stored/explicitly invoked read-only evidence. Diagnostics disclose all
  external reads; no silent key creation, provider write or backup restore.
  Any canary check runs only through Vault and cannot return decrypted material.
- A checklist distinguishes configured, locally checked and operationally
  accepted; OAuth client secrets present is not real consent acceptance.
- **Acceptance:** fresh PostgreSQL setup, wrong key/missing baseline, bad origin,
  unconfigured provider, old/unverified backup and database outage produce safe
  states. The documented install → explicit workspace → synthetic secret →
  authorized read → isolated recovery journey completes without undocumented
  operator fixes. No real provider calls in CI.
- **Dependency/owner:** existing Identity/Vault/worker; Backend + Platform.

## F02 — Safe denial explanations (P1)

**Journey:** a denied developer sees a permitted next step; an authorized admin
can investigate the same correlation ID without exposing another tenant.

- Policy adds typed internal reasons such as inactive identity, expired parent,
  insufficient capability and unavailable authority. A separate projection
  decides which details the caller may see. Never expose raw database/crypto
  errors or internal policy objects in ordinary responses.
- Preserve existing HTTP error semantics. Add optional safe code/correlation
  fields after client compatibility tests. Proposed human-only
  `POST /api/v1/access/explain` accepts an operation and already inspectable
  resource reference; it returns safe diagnosis, not a new allow/grant.
- Expired/revoked human sessions receive a safe 401 and must reauthenticate
  before any protected explanation. A diagnostic endpoint cannot bypass
  session enforcement; only an active caller may inspect a permitted grant.
- Foreign/missing resources and out-of-scope secret names stay indistinguishable.
  Do not create an arbitrary-ID lookup oracle. Rate-limit diagnostics and audit
  bounded diagnostic requests without values or tokens.
- **Acceptance:** same public denial shape for foreign versus nonexistent
  resources; expired own session/grant, changed membership, denied environment,
  disabled capability and unavailable DB. No unauthorized name/member/policy
  disclosure; ordinary SDK callers continue interpreting errors correctly.
- **Dependency/owner:** stored-authority Policy and core error contracts; Backend.

## F03 — Searchable audit and safe receipt export (P1)

**Journey:** an administrator finds one incident window or later one broker run,
pages its events reliably and exports the permitted evidence.

- Audit owns a bounded read model with project/actor/action/time and later run
  filters. Ordinary browsing is live, using a deterministic `(created_at,id)`
  cursor; current authorization still applies on every page. A timestamp upper
  bound is not a consistent snapshot: a later commit can carry an older
  PostgreSQL transaction timestamp. The export worker materializes the selected
  permitted safe projection in a real repeatable-read snapshot, then publishes
  that bounded artifact. Download still requires current authorization.
- Extend project audit browsing additively with `cursor`, `limit` (maximum 100)
  and approved filters. Proposed export: `POST /api/v1/projects/:id/audit/exports`
  creates an audited job, returning metadata; download rechecks current access.
- Export selected safe metadata/receipt references, not secret values, tokens,
  request bodies or arbitrary detail blobs. Limit date window, rows and bytes;
  use expiring caller-bound downloads, not public storage URLs.
- Preserve immutable identity and old audit-chain format. Correlation/run
  references belong to new records/read models; never rewrite old hashes.
- **Acceptance:** timestamp ties, late commits during pagination/export,
  consistent export contents, concurrent writers, changed membership between
  pages, expired download, bounds, redaction, deletion/restart continuity and
  required export-audit rollback. Audit remains KeepSave-specific.
- **Dependency/owner:** transaction-aware audit and trusted jobs; Backend.

## F04 — Credential ownership and renewal reminders (P1)

**Journey:** a team knows who renews a credential and sees an actionable reminder
before its declared expiry; renewal clears obsolete notices.

- Vault owns `secret_lifecycle_metadata` and immutable metadata changes:
  project/secret IDs, responsible member, provider label, declared expiry,
  rotation-due date, verification origin/time and metadata revision. Avoid
  arbitrary credential-containing notes. Existing ciphertext format stays.
- Proposed `GET/PATCH /api/v1/projects/:id/secrets/:secretId/lifecycle` uses an
  expected metadata revision; update requires existing permitted management
  authority, active resource and current valid owner membership.
- A declared expiry is a reminder, not proof of provider validity or automatic
  deletion/revocation. Later provider observations are labeled separately.
- Metadata change, immutable record, audit and outbox commit together. Worker
  notifications carry IDs/revisions, not values; recheck current metadata,
  recipient membership and visibility before delivery/read.
- Proposed `GET /api/v1/account/notifications` and owned acknowledgment. Start
  in-app; default reminder windows 30/7/1 days, operator configurable. Unique
  intent keys prevent duplicate visible notifications across retries.
- Add explicit lifecycle fields to a new versioned recovery bundle schema with
  compatible readers and a reviewed rollout; keep AES-GCM unchanged. Old bundles
  without metadata yield unknown dates/owner, not invented history. Isolated
  recovery preserves declared dates/provenance and historical owner references
  as inactive metadata. Live restore requires explicit owner mapping to current
  authorized target members; it imports no source identity or authority and
  cannot silently enroll stale recipients. Scheduling needs target acceptance.
- **Acceptance:** UTC boundaries, overdue/unknown dates, concurrent update,
  duplicate leased jobs, stale revision, removed owner/recipient, tombstone and
  no secret names leaking into another user's inbox, bundle round-trip/legacy
  handling and owner-remap denial. Inventory/reminders need no decryption.
- **Dependency/owner:** Vault metadata transaction, jobs and current Identity;
  Backend + client. External email needs later configured delivery handling.

## F05 — Invitations and organization-scoped offboarding (P1)

**Journey:** an admin adds a teammate without exchanging UUIDs, then removes
that teammate with a clear receipt of the organization's affected authority.

- Identity owns invitations: tenant, inviter ID, intended role, target verified
  contact or confirmed immutable user, random handle hash, expiry (default 24h),
  consumed/revoked state. Return a copyable link once; no stored plaintext proof.
- Proposed create/revoke under `/api/v1/organizations/:id/invitations` and
  authenticated `POST /api/v1/invitations/:invitationId/accept`, using a nonsecret
  ID and proof in a nonlogged JSON body. Copied links carry proof in the browser
  fragment, clear it before navigation, and use no-referrer/no-store behavior.
  Disable proof capture in logs, traces and client telemetry; never echo it.
  Recheck current inviter
  authority, organization, role and owner protection at acceptance; atomically
  consume proof and write membership/audit/outbox.
- **Prerequisite:** a reviewed verified-contact service. Password signup alone
  does not prove email ownership. Neither an email match nor a bearer invitation
  may silently link social accounts or confer global authority. Before verified
  contacts exist, use an admin-confirmed immutable-user join request; do not
  enable email-targeted automatic acceptance.
- Offboarding preview lists only authorized organization resources and owners
  requiring reassignment. Proposed `POST .../members/:userId/offboarding-preview`
  and `POST .../members/:userId/offboarding` use an expected authority revision
  and caller-scoped idempotency key.
- Define stored `member_authority_state(org_id,user_id)` with active state and
  epoch. Membership/role/accepted-invite and subject-grant changes lock and
  advance this boundary; every subject grant mint locks it and revalidates
  current authority. Minting invalidates an earlier inventory preview. Existing
  parent locks/expiry checks still apply; an epoch never substitutes for them.
  Extend every enabled grant entry point before exposing atomic offboarding.
- The transaction establishes organization denial, invalidates its affected
  keys/leases/approvals and later broker runs/workloads, and records audit/outbox
  plus an immutable summary. Reject owner removal until explicit succession.
  Recheck mint/offboard races against the same current authority boundary.
- Preserve unrelated organizations, personal projects and global human sessions.
  Separate later provider cleanup with pending/confirmed/failed/unsupported/
  uncertain states. A local receipt cannot certify upstream rotation.
- **Acceptance:** replay/concurrent consume, identity mismatch, inviter demotion,
  revoked invite, proof absent from logs/traces, cross-tenant IDs, viewer
  mutation, owner protection, membership/grant changes after preview, scoped
  offboard/mint race, two-API subsequent denial, unrelated access preserved and
  restart/recovery cannot revive authority. Returned data cannot be recalled.
- **Dependency/owner:** Identity/Policy/audit; Security review. Extend narrow
  Broker/Automation ports only when M3/M4 exist.

## F06 — Login-method safety and account recovery (P2)

**Journey:** a user knows which methods are usable, adds a backup method through
explicit linking and can recover without an email-based account takeover.

- First ship usable-method inventory and refusal to remove the last working
  method. Existing explicit linking stays session-bound and requires recent
  authentication; configuration presence is not proof a provider is usable.
- Design verified contacts and purpose-bound hashed one-time recovery proofs in
  Identity before email recovery. Generic initiate responses, expiry, rate and
  attempt limits, concurrent one-time consume and safe audits are mandatory.
  As with invitations, put only nonsecret IDs in paths; proofs use nonlogged
  request bodies or cleared browser fragments, never query/path logs.
- Proposed password recovery request/confirm and method-removal endpoints need
  a separate auth ADR. Canonical-email matches cannot merge identities; provider
  loss does not permit recovering a different account with the same address.
- Successful reset revokes existing human sessions/pending link proofs in the
  same transaction; it grants no project/organization/operator role. Operator
  assistance needs documented identity proof and trusted user-ID commands.
- **Acceptance:** enumeration resistance, collisions, wrong purpose/account,
  expiry/replay/concurrency, revoked origin, last-method lockout, required audit
  failure and cross-instance old-session denial. No recovery authority in backups.
- **Dependency/owner:** reviewed Identity proof/contact service; Backend + Security.

## F07 — Controlled access across harnesses (strategic M2–M5)

**Journey:** an approved developer opens a ten-minute run, reviews repository A
through a supported harness, is denied B and loses subsequent access after
revocation. Codex is the first pilot candidate. The owner's 2 October correction
requires a harness-neutral core and an extension path for future clients; it
does not establish that every harness is already supported.

- Keep shared principals, resources, policy, approvals, grants, broker custody
  and receipts free of harness-specific types. Harness transport/configuration/
  artifact adapters and provider operation adapters evolve independently.
  Follow the [extension contract](README.md#harness-neutral-extension-contract).

- M2: official Go MCP SDK, verified exact pins/versions, stateless transport,
  resource-bound OAuth consent, pre-registered client/S256 PKCE, atomic codes,
  refresh replay detection, typed tool identity and real client acceptance.
- M3: read-only GitHub App connection/binding, pinned repository ID/commit,
  workload enrollment, durable attempts/receipts and an independent rootless
  runner. Connector requests structured operations; Broker holds the token and
  performs the authenticated request. No arbitrary upstream URLs or API-host builds.
- M4: immutable instruction-only private skills, digests/approvals/revocation,
  portable access profiles with pinned harness packages, native installation
  and tested compatibility/export. Each export has an exact format/version/
  digest; changed packaging cannot borrow an approval for another artifact.
  Separate server-enforced, administrator-configured and unverified controls.
  Refuse a package/client that cannot honor required capabilities; never
  silently drop restrictions. Client declarations cannot grant authority.
- M5: complete control/runner self-hosting, consistent multiple APIs, durable
  external effects, upgrade/fault/recovery drills and measured capacity.
- **Acceptance:** preserve the [existing pilot matrix](../2026-09-28-backend-platform/DELIVERY.md#first-pilot-review-this-repository-with-controlled-access),
  real Codex/GitHub UAT, stolen-runner/handle denial, expiry/policy changes,
  canaries absent and proven file/network/process/time/resource restrictions.
  Before advertising broad harness support, run the same profile and allowed/
  denied repository, expiry, revocation, cancellation and receipt cases through
  two independent real harnesses. Pick the second from pilot demand; a generic
  MCP fixture alone is not cross-harness acceptance. Publish a version/capability
  matrix, not an unqualified universal-support claim.
- **Dependency/owner:** existing M0/M1 contracts; Backend + Platform + Security.
  Skills do not grant authority. This is the established program, not a new
  parallel agent-hosting or model-credential product.

## F08 — Connection diagnostics and read-only drift (P2)

**Journey:** an admin distinguishes configured, checked, stale, unavailable and
unknown integration states and knows the permitted next action.

- MCP/Broker expose diagnostics only for an authorized connection/binding or
  available run. Harness checks report exact client version/settings and control
  evidence. Installed metadata alone is never healthy/enforced evidence.
- Proposed `POST /api/v1/projects/:id/connections/:connectionId/check` explicitly
  names bounded external reads. Store safe check receipts, scope and check time;
  no hidden write, credential export or arbitrary outbound URL.
- Begin with installation/repository permissions, expected artifact/schema and
  runner reachability. Later environment-contract checks assess required key
  presence and authorized references, reusing existing diff services.
- Do not claim secret value equality where a destination cannot return values.
  Unchecked or stale states remain unknown, not green. Remediation links are
  advisory; they do not automatically broaden permissions.
- **Acceptance:** missing/expired connection, wrong repository, absent runner,
  incompatible client, schema drift, database/provider outage and safe bounded
  results. Current admission authority applies again after any successful check.
- **Dependency/owner:** M2/M3 resources and F02/F03; Backend + harness/client.

## F09 — One deliberate delivery integration (later)

**Journey:** a demonstrated pilot need for CI/hosting delivery is satisfied by
one approved destination with preview, change receipts and visible uncertainty.

- Choose GitHub Actions or Vercel from user evidence; do not enable both by
  default. Register an explicitly owned target and provider permission scope.
- Broker owns credential custody; application service owns exact selected
  records/revisions/destination and approval. Reuse protected-promotion rules
  where applicable; fresh changes invalidate the affected approval.
- Worker effects are durable, idempotent where upstream supports it and fenced.
  Unknown dispatched writes require reconciliation, not blind replay. Show local
  committed intent separately from confirmed destination state.
- Plaintext delivery to a destination is an explicit custody expansion and
  needs an ADR/threat review. Do not market this as model-safe brokered reads.
- **Acceptance:** wrong owner/target, stale revision, expired approval, partial
  upstream failure, retry/double delivery, uncertain reconciliation, revocation
  before dispatch and credential-free logs/results. No hidden external writes
  in connection checks or CI fixtures.
- **Dependency/owner:** mature M3/M5 effect handling plus pilot demand; Backend +
  Platform + Security. Broad sync, arbitrary rotation and marketplaces deferred.

## First implementation queue

| Ticket | Work and dependency | Done evidence |
|---|---|---|
| CORE-ACCEPT | Finish existing provider/review/CI/operational gates. | Dated named evidence in current ledger; no invented approvals. |
| DX-01 | Define public/internal decision DTOs (F02), before client changes. | Wrong-tenant/missing/scope/outage negative matrix and contract tests. |
| DX-02 | F03 stable cursors and filters, then bounded export. | Pagination concurrency and current-access redaction checks. |
| OPS-01 | F01 safe component checks and tested setup journey. | Fresh isolated install/recovery transcript and redacted doctor output. |
| LIFE-01 | F04 metadata/revision transaction, before reminder UI. | Stale update/audit rollback/current owner tests. |
| LIFE-02 | F04 in-app notification worker/read authorization. | Deduplication/restart/stale-recipient tests. |
| TEAM-01 | F05 preview/offboard receipt on existing membership denial. | Mint race/two-API/unrelated-workspace matrix. |
| AUTH-01 | F06 verified-contact/proof ADR and method safety. | Independent review plus replay/collision/last-method tests. |
| TEAM-02 | F05 invitations using accepted contact/identity proof. | Atomic accept/inviter-revocation/role tests. |
| MCP-01 | F07 M2 synthetic transport/OAuth/diagnostics and harness-neutral adapter contracts. | Supported protocol/client matrix and cancellation; real Codex acceptance separately. |
| HARNESS-01 | F07 second independent harness adapter, selected from pilot use after MCP-01; include M4 packaging only when profiles exist. | Same authority/expiry/revoke matrix and safe capability mismatch; real harness evidence before broad compatibility claims. |
| BROKER-01 | F07 M3 synthetic provider and independent runner slice. | Custody, A/B/ref/run denial, canaries and isolation tests. |

Tickets are local proposed work items, not opened GitHub issues or assigned
engineering commitments. Re-estimate after the first accepted slice and pilot.
