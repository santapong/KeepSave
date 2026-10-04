# 4. Data model and migrations

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

[`backend/migrations`](../../backend/migrations/) is authoritative. The candidate
ships **001–033**. PostgreSQL supplies the new journal, recovery, authority,
OAuth/run and durable-work guarantees. SQLite and bounded MySQL 8.4 tests retain
legacy compatibility; no-op/new-platform adapters are not parity evidence.
Resource and permission metadata is not all encrypted.

## Ownership, identity and authority

Accounts retain display emails and a canonical identity key. Stable provider
subjects bind social identities; matching emails do not merge accounts. Browser
sessions store token hashes and current revocation/expiry. Operator roles use
immutable user IDs, separate from organization membership. Signup creates no
tenant or global role.

Organizations have explicit owner-admin membership and caller-scoped creation
idempotency. Personal projects have no organization; attachment requires stored
owner and destination-admin authority and revokes old delegated keys/leases.
Cross-organization transfer is refused. Assigned projects use current membership;
original owner identity remains provenance. Tombstoned projects are excluded from
active authorization. Nonempty organization deletion cannot detach projects.

`member_authority_state(organization_id,user_id,active,epoch)` serializes scoped
offboarding and rejoin. Fresh membership does not revive an old epoch's grants.
API keys/leases/agent issuance and new OAuth families/runs preserve distinct
lineage. Recovery bundles never reconstruct any of them as authority.

## Additive schema ownership

| Migration | Responsibility |
|---|---|
| 001–014 | Legacy accounts, projects, environments, secrets, promotion, audit and metadata. |
| 015 | Social provider linkage. |
| 016–019 | Persisted audit head, outbox/jobs, vault journal/key/snapshot continuity and agent issuance. |
| 020–021 | Canonical identities, tracked sessions, operator grants and workspace creation retries. |
| 022–025 | Journal/ownership guards, tombstones/audited identity preservation, promotion source binding and backup catalog. |
| 026 | Member authority epochs. |
| 027 | Identity proofs, contacts, invitations, offboarding and session authentication method. |
| 028 | Delegated OAuth clients, consent, code/token hashes and rotating families. |
| 029 | Immutable artifacts/profiles/packages, connections/bindings/workloads, grants/runs/attempts/results. |
| 030 | Independent secret lifecycle metadata, reminders and safe audit exports. |
| 031–032 | Exact offboarding preview binding and fenced export publication state. |
| 033 | Immutable admitted run-scope snapshot. |

Applied migration files must not be edited. Drain incompatible writers, preserve
external backup/key material, apply additive changes and explicitly baseline old
active vault values. Startup refuses unenrolled projects. A baseline labels
existing ciphertext; it cannot invent earlier history. Legacy runs lacking an
admitted scope snapshot remain unknown and cannot perform provider operations;
current mutable bindings are not used to invent missing authority.

## Journals, custody and recovery

Value mutations commit current ciphertext, immutable revision, required audit and
outbox together. Rotation retains keys referenced by history/promotion snapshots;
restoration appends with a revision precondition. Lifecycle has its own revision,
responsible member and declared dates; changing metadata does not rewrite a value
revision. Tombstones and retained keys have no automatic purge in this candidate.

Recovery v2 bundles carry encrypted vault records/history/key/snapshot and
lifecycle dependencies with integrity metadata. Readers accept legacy v1, whose
missing lifecycle fields remain unknown. Isolated recovery creates a new local
custodian/project and requires explicit lifecycle-owner mapping. Selected live
restore checks current value/metadata revisions and current permitted owners.
Bundles exclude users, sessions, grants, approvals, operation results and proof
payloads. Ephemeral mail and result ciphertext has short access lifetime and is
not ordinary vault backup content.

Audit HMAC formats stay unchanged. Migration 023 prevents delete cascades from
rewriting hashed identity fields; writers serialize against the persisted head.
Fenced jobs and attempts expose uncertain external effects rather than assuming
a database transaction can commit SMTP/GitHub effects atomically. See
[security](05-security.md) and [recovery](10-operations.md).
