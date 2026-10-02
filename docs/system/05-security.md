# 5. Security model

Part of the [system documentation](README.md), reconciled 2026-10-02.
The current [threat model](../THREAT_MODEL.md) is the detailed control/limit
matrix; [ADR0028](../adr/0028-core-identity-and-vault-release.md) records the local
core implementation decision. Independent Security Engineer/Tech Lead review
has not occurred.

AES-256-GCM seals credential values with fresh nonces and per-project data keys.
An external wrapping key protects those keys; Vault Transit is the production
reference source. The encrypted database/backups require documented external
recovery material. Rotation preserves retained journal and snapshot key
references. Crypto and audit HMAC format changes require separate review; this
slice keeps those formats. Database encryption cannot contain an API/worker
process that already has key custody, and metadata is not all encrypted.

Human sign-in returns the compatible Bearer token shape with a tracked session.
HS256 verification alone is insufficient: sid/jti, stored token hash, expiry and
revocation are checked on admission. Sessions have a 24-hour maximum and no
human refresh flow. Logout and individual owned revocation close new admissions;
unavailable authoritative state fails closed. Already-admitted work can finish.
Google/GitHub social flows use verified provider identities, one-use state/PKCE
and exact configured callbacks. Email collisions require explicit recent
same-session linking; signup never grants operator authority.

Personal projects grant their stored owner administrator authority. An assigned
project uses its organization's current membership for everyone, including its
original owner and delegated API keys. Viewer can inspect permitted metadata;
editor reads/writes values; promoter approves eligible promotion; admin manages
configuration. API-key and agent scopes intersect current role, stored project,
environment, selected keys and parent expiry/revocation. Parent authority cannot
be widened. Organization ownership cannot be demoted/removed; cross-org project
transfer is refused in v1.

Private workspace templates use current membership for reads/list/application
and current admin for metadata mutation; a removed creator has no bypass.
Personal templates belong to their creator. Metadata mutation requires a tracked
human session and a required audit/local outbox transaction. Core global
publication is refused. Template defaults are ordinary config/placeholders,
not encrypted credential storage. Lease and agent-token mutations likewise join
required audit/outbox with current session/parent/membership checks.

Core vault mutations join required audit/outbox and immutable revision in the
same PostgreSQL transaction. Failed audit denies success. Secret/project
DELETE retains tombstones and referenced keys/history; this is not erasure.
Immutable audited identity fields survive deletion. Verification and isolated
restore precede selected live recovery; restoration excludes authority and
rejects stale current revisions or undelete attempts.

The restricted composition refuses unfinished AI, policy metadata, enterprise
SSO, old OAuth issuance, webhook automation, event replay, plugin mutation and
MCP execution. Current vault APIs intentionally return authorized plaintext.
GitHub credential confinement belongs to the future broker, not social sign-in
or an ordinary secret read. Future broker calls must check live authority twice
and only the broker makes the authenticated structured GitHub request.

The accepted managed-Codex plan does not claim administrator-proof devices or
remote attestation. A skill, prompt, tool annotation or self-reported harness
cannot grant authority. A separate restricted runner must prove its actual file,
network and resource boundaries before M3 acceptance. Self-hosting also does
not make Codex's configured model local or recall already-returned data.

Production configuration refuses leaked development key material, short signing
secret, wildcard origin and insecure PostgreSQL settings; trusted proxies are
explicit. The reference TLS proxy restricts metrics and exact recovery body
limits. These source controls are not a completed deployment/availability review.
See [acceptance ledger](../validation/2026-10-01-core-release/ACCEPTANCE.md) for
executed negative tests, real-provider UAT and operational release gates.
