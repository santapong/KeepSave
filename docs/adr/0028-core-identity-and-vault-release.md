# ADR-0028: Core identity and reliable vault release

- Status: Accepted for local implementation by sponsor instruction, 2026-10-01.
- Authority: the owner explicitly requested implementation of the complete reviewed backend plan in this task.
- Independent Security Engineer and Tech Lead review: **pending**. This is not integration, release, provider configuration, or deployment approval.
- Scope: preserved feature checkout, synthetic databases/provider fixtures, core first release before M2–M5.

## Decision

Retain Go/Gin and PostgreSQL as a modular monolith. Open registration grants only a user identity; organization ownership is explicit and transactional. Platform administration requires operator-enrolled immutable user-ID grants, never self-claimed email. Canonical email keys cannot automatically merge existing accounts.

Human Bearer sessions retain the login response and a 24-hour maximum. New sessions have a fresh sid/jti and a stored token hash; all human admission checks current database authority. Account linking is bound to the originating session and recent authentication. Legacy human tokens require reauthentication in a coordinated cutover; old binaries cannot remain in traffic or be used for rollback.

All enabled PostgreSQL credential mutations must use the versioned vault with local mutation, immutable journal, required audit, and outbox committed together. Keep AES-256-GCM and existing ciphertext/audit HMAC formats. Backfill is a labeled baseline. Deleted projects are tombstoned and denied everywhere; retained encrypted history and keys are not automatically purged. Audit identity references cannot rewrite hashed fields when resources are deleted.

Encrypted backups contain recoverable vault records and documented key dependencies. Verification and isolated restoration precede selected live restoration. Recovery does not restore human sessions, grants, approvals, or tokens. Unmigrated mutation paths and unfinished optional integrations are deliberately unavailable in the core profile.

The application origin is https://app.keepsave.draveniq.dev; the separate static landing remains https://keepsave.draveniq.dev. No production exposure occurs before provider UAT, independent recovery, operational acceptance, and independent reviews.

## Interfaces and defaults

Preserve /api/v1 and opaque Bearer {user,token} responses. Add current-session logout, owned session metadata/revocation, authorized revision restoration and compatible POST batch secret reads. New platform guarantees are PostgreSQL-only; SQLite/MySQL legacy compatibility is tested separately.

Retain encrypted history/key dependencies indefinitely in this release and audit retention at 365 days. Backup scheduling is opt-in after recovery configuration: daily 02:00 UTC, thirty verified scheduled bundles with a minimum of two verified copies; manual/pre-upgrade artifacts need explicit deletion.

## Review and rollback

Require real-router negative authorization, session and transaction failure tests, key/history continuity, deletion-chain restart proof, SDK contracts, and an isolated PostgreSQL recovery drill. Keep live provider/Codex acceptance separate from fixtures. Drain legacy APIs at session cutover; use only a compatible reviewed binary for rollback. Never recompute old audit hashes or reinterpret a broken chain as valid.

## Remaining milestones

Preserve ADR-0023–0027 and the M2–M5 program: official MCP/OAuth, broker-held read-only GitHub App access, separately isolated runner, immutable private skills/profiles, and measured self-hosted team operation.
