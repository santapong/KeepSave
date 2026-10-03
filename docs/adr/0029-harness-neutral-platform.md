# ADR-0029: Harness-neutral access platform

- Status: accepted for isolated local implementation by the owner's explicit implementation instruction, 2026-10-02.
- Independent Security Engineer and Tech Lead review: pending. No integration, tag or production authorization is established here.
- Baseline: develop candidate `3878e69`; preserve existing frontend and planning changes.

## Decision

Keep the Go/Gin/PostgreSQL modular monolith and trusted worker. Shared identity,
policy, runs, grants, custody and audit do not depend on a harness. Codex and
Hermes qualify through separate public OAuth clients and client-owned runs.
Portable immutable profiles and instruction-only skill sources have separately
versioned native packages. Client names, reported packages and skills grant no
authority. Local controls remain additional evidence, not device attestation.

Use a new resource-bound OAuth path with atomic code redemption and refresh
families. Keep legacy issuance and API-host execution disabled. Broker-held
GitHub App tokens authorize explicit read-only repository/commit operations.
The separately enrolled rootless Linux supervisor runs approved digest-pinned
connectors with no IP network; per-operation relay and mTLS connect to Broker.
Durable attempts/results/receipts separate dispatch outcomes from publication
authority; cancellation and revocation cannot recall returned data.

Extend ordered authority locking, safe audit projections, lifecycle metadata,
in-app reminders, verified contacts/invitations/scoped offboarding and recovery.
Operator SMTP uses ephemeral encrypted proofs and ID-only outbox messages.
Vault recovery restores no identity or grant authority. Retain AES-256-GCM and
existing encryption formats; version recovery schemas for new metadata.

## Constraints and delivery

New platform guarantees are PostgreSQL-only and capability flags default off.
Only Vault/Broker may obtain decrypted provider credential material. Narrow
custody ports also permit the trusted mail adapter to open ephemeral delivery
proofs and authorized result services to open ephemeral operation results;
neither exception exposes provider credentials or arbitrary vault values.
Network calls occur outside
database locks after durable admission. Uncertain external effects are not
blindly replayed. Additive migrations preserve applied SQL. Independent reviews,
real provider/harness UAT, runner isolation and operational drills remain gates.
External secret delivery, executable skills, arbitrary builds and additional
provider/harness qualification are deferred. Source and synthetic tests do not
establish production acceptance.

## Required evidence

Actual router and PostgreSQL failure/race tests, tenant and lineage denials,
atomic audit/outbox assertions, expiry/replay/revocation tests, two API processes,
native Codex/Hermes qualification and provider-token canaries. Production release
requires real consent/SMTP/GitHub and isolated-host runner/recovery acceptance.

## Rollback

Disable new admission and dispatch while retaining revocation/status/audit and
recovery. Drain incompatible old writers before journal/session cutover. Never
roll back onto unsupported encryption or schema readers. Keep existing landing
deployment and frontend identity preserved.
