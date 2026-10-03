# ADR-0024: Transactional audit, key versions and recoverable history

- Status: Accepted for local implementation by sponsor instruction, 2026-09-28.
- Authority: the project owner explicitly requested implementation of the complete backend plan in this Codex task.
- Independent Security Engineer and Tech Lead review: **pending**. This is not a reviewer sign-off, merge approval or production authorization.
- Scope: isolated `feat/backend-secure-tool-access-20260928` implementation and synthetic validation.

## Decision

Mutations, immutable versions, audit append and outbox records share one SQL transaction. Serialize the existing global audit chain through a persisted database head. Introduce additive project-key version references for every ciphertext including promotion snapshots. Retain AES-256-GCM and existing readable formats. Restore appends a new revision with expected-revision checks. Backups include encrypted records, wrapped key dependencies and authenticated format metadata; verify separately before selected live restoration.

## Alternatives and tradeoffs

Best-effort audit cannot support credential admission evidence. Rotating every retained historical value at once increases lock time and recovery risk. Metadata-only backups are not recoverable.

## Compatibility and threat boundary

No existing audit fields are removed. Backfill captures the current baseline only. Deletion remains explicit; backup recovery must not silently resurrect subsequently deleted records. Concurrent writes, rotation, and corruption must be tested before release.

## Rollback

Use expand/backfill/verify/cutover migrations; retain old keys and ciphertext readers. Disable new restore/rotation operations on failure. Never run an older binary against an unknown encryption format. Keep independently recoverable backups before cutover.

## Required evidence

The applicable milestone gates in [the execution record](../design/2026-09-28-backend-platform/IMPLEMENTATION.md) must pass. Failures or absent live-provider evidence remain visible. Independent review is required before integration/release.
