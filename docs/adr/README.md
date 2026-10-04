# Architecture Decision Records (ADRs)

This directory captures **irreversible or high-blast-radius** architectural decisions for KeepSave. It exists because verbal agreement and PR-review nods are not enough: a decision that affects how secrets are encrypted, who can read them, or how they cross environment boundaries must be writable down so the *why* survives the people who made it.

## When to write an ADR

Write one when the decision is **Type-1** as defined in [`docs/ROLES.md`](../ROLES.md) §3.1:

- Changes to encryption scheme, key hierarchy, or how keys are sourced.
- Auth model changes (JWT shape, API key format, session boundary).
- Schema migrations that are not strictly additive.
- Removal or shape-change of audit fields.
- Promotion semantics (what gets re-encrypted, what gets diffed, who approves).
- Multi-tenancy / isolation boundary changes.
- Anything where a wrong decision means *secrets leak* or *audit trail is lost*.

If you have to ask whether it's a Type-1, write the ADR. The cost of an extra ADR is 30 minutes; the cost of an undocumented Type-1 decision is a 6-month archaeology project after the original author leaves.

**Do not** write ADRs for: refactors within a package, dependency upgrades that aren't security-critical, UI tweaks, or anything reversible by a single follow-up PR.

## Lifecycle

1. **Draft** the ADR in this directory using `0000-template.md`. Number it sequentially. Status starts as `Proposed`.
2. **Open a PR** with only the ADR (no implementation yet). Title: `adr: <short title>`.
3. **Reviewers required:** Tech Lead always; Security Engineer for anything touching crypto / auth / promotion (veto power, per ROLES.md §2.2).
4. **Decide:** the PR is merged when the decision is `Accepted` or closed without merge if `Rejected`. Status field is updated to match.
5. **Supersede** instead of editing. If a future ADR overrides this one, mark this one `Superseded by ADR-NNNN` at the top and leave the body untouched.
6. **Review** all `Accepted` ADRs at the monthly ADR review (ROLES.md §6). Older than 90 days that are still valid get reaffirmed; ones that no longer match reality get marked `Superseded` with a new ADR.

## Numbering

Sequential. `0001`, `0002`, `0003`, ... — never reused, never skipped. If a draft is abandoned, mark it `Rejected` and keep the number.

## Current decision index

Reconciled 4 October 2026 from the records below. Status is the original record's
status, not a newly supplied independent signature. Backfilled or sponsor-local
acceptance must not be read as Security Engineer/Tech Lead approval.

| ADR | Decision | Recorded state |
|---|---|---|
| 0001 | [Envelope encryption with AES-256-GCM](0001-envelope-encryption.md) | Accepted (backfilled) |
| 0002 | [Authentication model — JWT for humans, API keys for agents](0002-auth-model.md) | Accepted (backfilled) |
| 0003 | [Promotion engine — decrypt-and-rewrap, with PROD approval gate](0003-promotion-engine.md) | Accepted (backfilled) |
| 0004 | [Two-level key hierarchy — master KEK + per-project DEK](0004-key-hierarchy.md) | Accepted (backfilled) |
| 0005 | [`RequireProjectAccess` middleware for `/projects/:id/*` routes](0005-require-project-access-middleware.md) | Accepted (sponsor-authorized); retroactive reviews pending |
| 0006 | [Embed widget origin allow-list (server-side, per-project)](0006-embed-widget-origin-allowlist.md) | Accepted (sponsor-authorized); retroactive reviews pending |
| 0007 | [Enforce approver ≠ requester at the database layer for promotion approvals](0007-approver-not-requester-db-invariant.md) | Accepted (sponsor-authorized); retroactive reviews pending |
| 0008 | [RS256 JWT signing with JWKS + `kid` rotation](0008-rs256-jwks-rotation.md) | Accepted (see record for scope) |
| 0009 | [Mandatory default expiration on `ks_` API keys](0009-default-api-key-expiration.md) | Accepted (sponsor-authorized); retroactive reviews pending |
| 0010 | [Harden the MCP gateway command-execution path (allowlist + sandbox + safe-goroutine + build budget)](0010-mcp-gateway-command-execution-hardening.md) | Accepted (sponsor-authorized); retroactive reviews pending |
| 0011 | [Graceful shutdown, DB context timeouts, HTTP server timeouts, pool-lifetime tuning, and a shared `safego` helper](0011-graceful-shutdown-and-db-timeouts.md) | Accepted (sponsor-authorized); retroactive reviews pending |
| 0012 | [KMS auto-unseal as production default for the master key](0012-kms-auto-unseal.md) | Accepted (sponsor-authorized); retroactive reviews pending |
| 0013 | [Webhook emission with atomic SSRF guard, body-buffered retries, and per-org signing-secret rotation](0013-webhook-emission-with-ssrf-guard.md) | Accepted (sponsor-authorized); retroactive reviews pending |
| 0014 | [Audit-log taxonomy extension — `role.changed`, `settings.changed`, `actor_type`](0014-audit-log-taxonomy-extension.md) | Proposed |
| 0015 | [`getUserID`/`getActor` context helpers and per-use API-key audit emission](0015-safego-helper-and-audit-emission.md) | Proposed |
| 0016 | [Deployment topology — Vercel frontend, container backend, Neon Postgres](0016-deployment-topology.md) | Accepted (sponsor-authorized); retroactive reviews pending |
| 0017 | [Promotion engine integrity — atomic execution, idempotent claim, complete rollback](0017-promotion-engine-integrity.md) | Accepted |
| 0018 | [Crypto integrity — atomic DEK rotation, service-secret sub-keys, key-material zeroization](0018-crypto-integrity-rotation-zeroization.md) | Accepted |
| 0019 | [Tamper-evident audit log via a keyed hash chain](0019-tamper-evident-audit-chain.md) | Accepted (design); implementation tracked as the next Wave-3 task |
| 0020 | [Secret-reference resolution at read time](0020-secret-reference-resolution.md) | Accepted |
| 0021 | [Short-lived agent tokens with a JWT denylist](0021-short-lived-agent-tokens-denylist.md) | Accepted |
| 0022 | [Per-secret / per-action API-key scope grammar](0022-per-secret-scope-grammar.md) | Accepted |
| 0023 | [Authorized use cases and bounded grants](0023-authorized-use-cases.md) | Sponsor-authorized implementation; independent reviews pending |
| 0024 | [Transactional audit, key versions and recoverable history](0024-transactional-audit-recovery.md) | Sponsor-authorized implementation; independent reviews pending |
| 0025 | [Brokered credentials and isolated connector execution](0025-brokered-isolated-execution.md) | Sponsor-authorized implementation; independent reviews pending |
| 0026 | [MCP, approved skills and managed Codex compatibility](0026-mcp-skills-codex.md) | Sponsor-authorized implementation; independent reviews pending |
| 0027 | [Self-hosted team control and runner topology](0027-self-hosted-team-topology.md) | Sponsor-authorized implementation; independent reviews pending |
| 0028 | [Core identity and reliable vault release](0028-core-identity-and-vault-release.md) | Sponsor-authorized implementation; independent reviews pending |
| 0029 | [Harness-neutral access platform](0029-harness-neutral-platform.md) | Sponsor-authorized implementation; independent reviews pending |

## Scope changes and publication

[ADR0029](0029-harness-neutral-platform.md) is the current harness-neutral
direction; [ADR0027](0027-self-hosted-team-topology.md) selects separate self-hosted
control/runner hosts. Earlier Codex-only and hosted deployment decisions remain
historical records. This index does not rewrite their original status or body.
[ADR0028](0028-core-identity-and-vault-release.md) records the core candidate and
its bounded develop integration. October 3 source publication is recorded
[separately](../releases/2026-10-03-source-publication.md); it supplies no tag,
production acceptance or standing review waiver.

Use [current architecture](../ARCHITECTURE.md), [status](../STATUS.md) and
[the documentation hub](../README.md) to reconcile source with dated evidence.
New irreversible changes still follow the lifecycle above.
