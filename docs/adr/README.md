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

## Index

| #    | Title                                  | Status   | Date       |
|------|----------------------------------------|----------|------------|
| 0001 | [Envelope encryption with AES-256-GCM](0001-envelope-encryption.md) | Accepted | 2026-05-12 |
| 0002 | [Authentication model: JWT + API keys](0002-auth-model.md) | Accepted | 2026-05-12 |
| 0003 | [Promotion engine: decrypt-and-rewrap](0003-promotion-engine.md) | Accepted | 2026-05-12 |
| 0004 | [Two-level key hierarchy: master + per-project DEK](0004-key-hierarchy.md) | Accepted | 2026-05-12 |
| 0005 | [`RequireProjectAccess` middleware](0005-require-project-access-middleware.md) | Accepted† | 2026-05-15 |
| 0006 | [Embed widget origin allow-list](0006-embed-widget-origin-allowlist.md) | Accepted† | 2026-05-15 |
| 0007 | [Approver ≠ requester DB invariant](0007-approver-not-requester-db-invariant.md) | Accepted† | 2026-05-15 |
| 0009 | [Mandatory default expiration on `ks_` API keys](0009-default-api-key-expiration.md) | Accepted† | 2026-05-15 |
| 0010 | [MCP gateway command-execution hardening](0010-mcp-gateway-command-execution-hardening.md) | Accepted† | 2026-05-15 |
| 0011 | [Graceful shutdown + DB timeouts](0011-graceful-shutdown-and-db-timeouts.md) | Accepted† | 2026-05-15 |
| 0012 | [KMS auto-unseal (Vault for UAT)](0012-kms-auto-unseal.md) | Accepted† | 2026-05-15 |
| 0013 | [Webhook emission with SSRF guard](0013-webhook-emission-with-ssrf-guard.md) | Accepted† | 2026-05-15 |
| 0016 | [Deployment topology: Vercel + container + Neon](0016-deployment-topology.md) | Accepted† | 2026-05-18 |

(ADRs 0001-0004 are **backfilled** — they document decisions already in the code, not decisions made today. Future ADRs will be written *before* implementation.)

† **Sponsor-authorized**: status flipped to Accepted with implementation landed in PR #54 (Phase 1 sweep). Retroactive Security Engineer + Tech Lead sign-off pending per CLAUDE.md §"Decision classes" (Type-1). ADRs 0008, 0009, 0014, 0015 remain Proposed pending implementation.
