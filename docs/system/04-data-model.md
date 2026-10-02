# 4. Data model and migrations

Part of the [system documentation](README.md), reconciled 2026-10-02.

SQL source under `backend/migrations/{postgres,sqlite,mysql}` is authoritative.
PostgreSQL is the current platform database; SQLite supports local/legacy tests
and a bounded MySQL 8.4 legacy identity/session/scoped-vault/workspace-role
exercise passed separately. This does not establish journal/recovery parity. Journal/recovery/durable job
semantics are PostgreSQL-only until parity is proven. Stored values and project
keys are encrypted; resource metadata and authoritative permissions are not all
ciphertext.

## Current ownership and lineage

Users have canonical email identity and optional verified provider identities.
Human tokens have stored session authority; immutable user-ID operator grants
are separate from organization roles. Registration does not create any tenant.
Organization creation joins its explicit owner administrator membership and
optional caller/idempotency-key result in a transaction. The retry record retains
a deleted result identity, preventing accidental resurrection.

Projects remain personally owned when `organization_id` is NULL. Assigned
projects use current organization membership for authority; their original owner
is provenance, not continuing permission. `deleted_at` excludes archived projects
from access. Assigning a personal project revokes its old keys/leases. Nonempty
organization deletion includes retained tombstones, so deletion cannot detach
credential scope through cascading behavior.

API keys store hashes/scopes/environment/expiry; agent leases and persisted token
issuance bind exact parent lineage. Current membership, parent revocation and
expiry are checked before access. These existing vault grants are not broker
repository runs and recovery never imports them as authority.

## Additive schema evolution

| Migration | Current responsibility |
|---|---|
| 001–014 | Existing accounts/projects/environments/secrets, promotion, audit and compatibility metadata. |
| 015 | Social identity/provider linkage. |
| 016 | Persisted serialized audit-chain head. |
| 017 | PostgreSQL outbox/jobs with leases/fences/attempt state. |
| 018 | Versioned vault journal, retained wrapped project key versions and snapshot references. |
| 019 | Agent-token issuance and lineage. |
| 020 | Canonical identity, tracked human sessions and operator grants. |
| 021 | Caller-scoped workspace-creation retry records. |
| 022 | PostgreSQL immutable journal/key/snapshot guards, composite tenant relationships and enrolled-value journal constraints. |
| 023 | Project tombstones and immutable audited identity references. |
| 024 | Exact promotion source artifact/revision binding. |
| 025 | Durable encrypted backup catalog and retention state. |

Applied migration files are immutable. Coordinate old-writer drainage, external
backup/key preservation, additive migrations and explicit journal baseline before
new traffic. Startup refuses unenrolled active projects. Existing values become
a labeled baseline; missing historical edits are never invented.

## Vault consistency and recovery

A vault mutation writes current encrypted state, append-only revision, required
audit and outbox together. Rotation retains referenced wrapped project keys for
current values, history and promotion snapshots. Restoration requires a current
revision precondition and appends a new current revision. Secret deletion retains
a journal tombstone; project deletion retains identity and closes active access.
No automatic history/key purge is performed in this release.

Encrypted bundles contain vault records, revisions, wrapped keys and snapshot
material with authenticated integrity metadata. Offline/isolated verification
requires external wrapping-key recovery material. The backup catalog stores safe
IDs, hashes, sizes, scheduling/retention state and timestamps, not plaintext.
The trusted CLI requires a fresh target with no non-system tables before
migration and creates a new authorized project; it does not
restore accounts, sessions, keys-as-grants or approvals. Live selected restore
uses explicit records and revision checks and does not undelete.

Audit HMAC field formats remain unchanged. Migration 023 stops foreign-key delete
actions from rewriting immutable hashed user/project IDs. The persisted head is
serialized across writers. Outbox external effects can remain uncertain; no
transaction can atomically commit a remote provider effect and PostgreSQL.
See [threat model](../THREAT_MODEL.md) and the source-specific acceptance evidence.
