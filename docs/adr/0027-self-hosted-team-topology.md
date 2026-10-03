# ADR-0027: Self-hosted team control and runner topology

- Status: Accepted for local implementation by sponsor instruction, 2026-09-28.
- Authority: the project owner explicitly requested implementation of the complete backend plan in this Codex task.
- Independent Security Engineer and Tech Lead review: **pending**. This is not a reviewer sign-off, merge approval or production authorization.
- Scope: isolated `feat/backend-secure-tool-access-20260928` implementation and synthetic validation.

## Decision

Provide a production-oriented self-hosted container profile separate from development Compose. Keep frontend/API/trusted worker on the control host, PostgreSQL and private durable storage, and an isolated Linux connector runner host. Production reference key custody uses the existing Vault Transit unwrap path. Coordinate migrations, durable jobs, admission limits and audit through PostgreSQL. Prove two-instance operation before advertising horizontal scale.

## Alternatives and tradeoffs

The hosted-first ADR-0016 remains historical context; the owner selected self-hosted teams for this release. Kubernetes is not required for the first team install. Redis, Kafka, multi-region and cloud-KMS expansion are deferred.

## Compatibility and threat boundary

This extends the deployment choices of ADR-0016 rather than editing its history. Master keys remain in the trusted API process under the existing key-provider contract; this is not a non-exportable HSM claim. SQLite is local/test; new platform release guarantees require PostgreSQL.

## Rollback

Disable admission and drain workers before rollback. Preserve persistent records and backups. Version checks reject unsupported schemas. Control-plane recovery must not reactivate old runtime grants.

## Required evidence

The applicable milestone gates in [the execution record](../design/2026-09-28-backend-platform/IMPLEMENTATION.md) must pass. Failures or absent live-provider evidence remain visible. Independent review is required before integration/release.
