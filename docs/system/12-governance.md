# Governance

> Part of the **[KeepSave System Documentation](./README.md)**.

This chapter documents how KeepSave is governed as a project: the operating principles, the nine roles and who can veto crypto/auth/promotion changes, the Type-1/2/3 decision classes and their required process, the complete ADR index (0000–0016), the roadmap phases and the current phase, what the project explicitly will **not** build, the tracked-follow-up process, the per-role 30/60/90 plan, and the mandatory PR gates from `CLAUDE.md`. Because KeepSave handles other people's secrets, the governance is deliberately heavyweight: no role ships a PROD change alone.

For the threat model these processes protect see [security](./05-security.md); for the follow-ups that block roadmap items see [operations](./10-operations.md).

---

## 1. Operating principles

From [ROLES](../ROLES.md) §1, these apply to every role:

1. **Threat model first.** Every feature epic begins with a delta to [THREAT_MODEL](../THREAT_MODEL.md). If you can't describe the new attack surface, you're not ready to design.
2. **ADRs for irreversible decisions.** Encryption scheme, key hierarchy, auth model, data retention → write it in `docs/adr/NNNN-*.md` with alternatives and rejection rationale.
3. **Blast radius before convenience.** Always ask: if this is wrong, what is the worst outcome, who is affected, can it be rolled back?
4. **No silent fallbacks.** "Try harder" paths (retry, fallback-to-cache, default-to-allow) must be explicit and reviewed. Security defaults are deny.
5. **Audit-log first, feature second.** If an action is not in the audit log, it didn't happen — and the feature isn't done.
6. **Reversibility budget.** Irreversible actions (migrations, key rotations, schema breaking changes) require written sign-off.

---

## 2. Roles and the veto

Nine roles, each defined by **owned artifacts** (a role that can't point to files/processes it owns isn't real). Summary from [ROLES](../ROLES.md) §2:

| Role | Owns (selected) | Veto / special power |
|------|-----------------|----------------------|
| Tech Lead / Staff Engineer | `docs/adr/`, architecture, RFC template, roadmap sequencing | **Final reviewer / gating sign-off on every ADR** |
| **Security Engineer** | [THREAT_MODEL](../THREAT_MODEL.md), [PENTEST_CHECKLIST](../PENTEST_CHECKLIST.md), `SECURITY_AUDIT.md`, key-rotation runbook | **Veto power on anything touching `internal/crypto`, `internal/auth`, or promotion flows** |
| Backend Engineer (Go) | `backend/internal/**`, `migrations/`, Go tests | — |
| Frontend Engineer | `frontend/src/**`, embed widget contract, postMessage spec | — |
| DevOps / Platform | `docker-compose.yml`, `helm/`, `scripts/`, CI, deploy runbook | — |
| QA / Test Engineer | `tests/`, matrices, regression + fuzz harness | — |
| Product Manager | `Roadmap.md`, phase changelogs, feedback log | — |
| UX / UI Designer | design system, security-sensitive UI specs | sign-off needed for any UI showing plaintext secrets |
| Technical Writer / DevRel | `README.md`, `docs/*_INTEGRATION.md`, security page | — |

The Security Engineer veto is the linchpin: it is "non-negotiable for this product" and applies to crypto correctness, the key hierarchy, the auth model, and the promotion engine. In Phase A the Security Engineer is fractional/part-time **but retains real veto power** — security is the substrate, not a phase.

---

## 3. Decision classes

Every change is classified ([ROLES](../ROLES.md) §3.1, restated in `CLAUDE.md`). **When in doubt, treat it as the next class up — misclassification is itself a bug.**

| Class | Examples | Required process |
|-------|----------|------------------|
| **Type-1** (irreversible / high-blast) | Crypto scheme, key hierarchy, schema **breaking** change, audit-field removal, PROD key rotation, multi-tenancy boundary change, promotion semantics | **ADR + Security Engineer sign-off + Tech Lead sign-off** |
| **Type-2** (reversible / contained) | New endpoint, new UI component, dependency upgrade | RFC if non-trivial; standard PR review otherwise |
| **Type-3** (local / trivial) | Refactor within a package, doc edit | Standard PR review |

The RFC/ADR lifecycle requires: problem statement → constraints (incl. threat-model) → ≥ 2 options with honest tradeoffs → decision **with explicit rejection rationale** → rollback plan → open questions. Post-mortems for any incident touching customer secrets or auth are blameless and mandatory, producing action items tracked in [FOLLOWUPS](../FOLLOWUPS.md).

---

## 4. The ADR process and complete index

ADRs capture **Type-1** decisions so the *why* survives the people who made it. Process ([ADR README](../adr/README.md)): draft from `0000-template.md` with sequential numbering (never reused, never skipped) → open a PR with **only the ADR** (no implementation) → Tech Lead always reviews, Security reviews anything touching crypto/auth/promotion (veto) → merge as `Accepted` or close as `Rejected` → **supersede instead of editing**. All Accepted ADRs are revisited at the monthly review.

> **Status nuance:** ADRs 0001–0004 are **backfilled** — they document decisions already in code. ADRs 0005–0007, 0009–0013, 0016 are **"Accepted (sponsor-authorized)"** with implementation landed under PR #54; their **retroactive Security + Tech Lead sign-off is still pending** per `CLAUDE.md`. ADRs 0008, 0014, 0015 are **Proposed** (not yet implemented). Note that "Accepted" here does not always mean fully implemented — several accepted ADRs have parts still un-wired (see [operations](./10-operations.md) §3–4).

| # | Title | Status | One-line summary |
|---|-------|--------|------------------|
| 0000 | Template | n/a | The ADR skeleton (problem / constraints / options / decision / rollback / open questions). |
| 0001 | [Envelope encryption with AES-256-GCM](../adr/0001-envelope-encryption.md) | Accepted (backfilled) | AEAD-at-rest using only the Go stdlib; assumes the DB can be exfiltrated, so plaintext-at-rest is not an option. |
| 0002 | [Auth model — JWT + API keys](../adr/0002-auth-model.md) | Accepted (backfilled) | JWT for interactive humans (session-bound); scoped `ks_` API keys for agents. |
| 0003 | [Promotion engine — decrypt-and-rewrap](../adr/0003-promotion-engine.md) | Accepted (backfilled) | Alpha→UAT→PROD promotion re-encrypts values; PROD requires an approval gate (one approver ≠ requester). |
| 0004 | [Two-level key hierarchy](../adr/0004-key-hierarchy.md) | Accepted (backfilled) | Master KEK + per-project DEK; fixes blast radius and rotation cost; `EnvProvider` is dev-only. |
| 0005 | [`RequireProjectAccess` middleware](../adr/0005-require-project-access-middleware.md) | Accepted† | Closes nine IDOR findings: `/projects/:id/*` routes must assert the caller has access to `:id`, not just identify them. |
| 0006 | [Embed widget origin allow-list](../adr/0006-embed-widget-origin-allowlist.md) | Accepted† | Server-side, per-project allow-list for which origins may embed the widget; closes the `postMessage` confused-deputy. |
| 0007 | [Approver ≠ requester DB invariant](../adr/0007-approver-not-requester-db-invariant.md) | Accepted† | Enforces the promotion multi-party-control invariant at the **database** layer, not just service code. |
| 0008 | [RS256 JWT signing with JWKS + `kid` rotation](../adr/0008-rs256-jwks-rotation.md) | **Proposed** | Move JWT off shared-secret HS256 to RS256 + JWKS so tokens can be verified-not-forged and rotated with overlap. |
| 0009 | [Mandatory default expiration on `ks_` API keys](../adr/0009-default-api-key-expiration.md) | Accepted† | No immortal credentials: default 90-day expiry, 365-day ceiling, refuse past times; existing NULL rows grandfathered. |
| 0010 | [MCP gateway command-execution hardening](../adr/0010-mcp-gateway-command-execution-hardening.md) | Accepted† | Interpreter allowlist + scrubbed-env sandbox + `safego` goroutine harness + build budget for the MCP RCE surface. |
| 0011 | [Graceful shutdown + DB/HTTP timeouts](../adr/0011-graceful-shutdown-and-db-timeouts.md) | Accepted† | SIGTERM drain via `srv.Shutdown`; per-query DB context, HTTP timeouts, pool-lifetime tuning, shared `safego`. |
| 0012 | [KMS auto-unseal](../adr/0012-kms-auto-unseal.md) | Accepted† | Make non-`EnvProvider` the production default; wire AWS/GCP KMS adapters with retry/backoff (depends on ADR-0011). |
| 0013 | [Webhook emission with SSRF guard](../adr/0013-webhook-emission-with-ssrf-guard.md) | Accepted† | Atomic: wire webhook emission **only** alongside an SSRF allow/deny guard, body-buffered retries, and signing-secret rotation. |
| 0014 | [Audit-log taxonomy extension](../adr/0014-audit-log-taxonomy-extension.md) | **Proposed** | Add `role.changed` / `settings.changed` events and an `actor_type` discriminator to the audit taxonomy. |
| 0015 | [`getUserID`/`getActor` helpers + per-use audit](../adr/0015-safego-helper-and-audit-emission.md) | **Proposed** | Replace 58 `MustGet` panic sites with safe context helpers; emit a per-use audit event for API-key access. |
| 0016 | [Deployment topology — Vercel + container + Neon](../adr/0016-deployment-topology.md) | Accepted† | Frontend on Vercel, long-running container backend, managed Neon Postgres, KMS via Vault (UAT) → cloud (PROD). |

† Sponsor-authorized; retroactive Security + Tech Lead sign-off pending per `CLAUDE.md`.

---

## 5. Roadmap phases and current phase

Two views coexist in the repo, and it is important to keep them straight:

- **Build phases (1–15)** in `Roadmap.md` are the historical *yes* list of capabilities (Foundation, Promotion Engine, Frontend Dashboard, Embeddable Widget, Hardening, Advanced Features, Observability, SDKs, Enterprise, Security Hardening, AI Agent Experience, Platform Ecosystem, OAuth+MCP Hub, Application Dashboard, AI Intelligence). Their checklists are largely complete.
- **Governance phases (A / B / C)** in [ROLES](../ROLES.md) §4 describe *team and discipline maturity*. **The current phase is Phase A — MVP hardening** (`Roadmap.md` §"Current phase").

Phase A closes the gaps surfaced by the v1.2.0 threat-model re-baseline: audit-log coverage on state-mutating endpoints, error-response sanitization, embed-widget origin policy, negative-auth tests, and the requester-cannot-self-approve invariant. **Phase B — multi-tenant + approval workflows** is deferred; Phase-A schema work is **additive only** (no breaking changes). Phases are "done by criteria, not by date."

---

## 6. What we are explicitly NOT building

[ROADMAP_NOT](../ROADMAP_NOT.md) is the equally-binding *no* list. Moving anything off it requires (a) a written reason and (b) something else coming off the roadmap to make room.

| Hard "no" (Phase A) | Reason |
|---------------------|--------|
| Multi-tenant runtime isolation | Phase B scope (additive schema prep only in A) |
| SSO / SAML / OIDC for end-user login | no customer demand signal yet |
| Mobile / native SDKs | the web embed widget covers the surface; native is maintenance-heavy |
| GUI policy editor | policies are code-driven (ADR-0003); current customers are technical |
| Internationalization | all current customers use English; cheaply deferrable |
| Public marketplace for community integrations/MCP | no signature/trust scheme yet; Phase B+ |
| Built-in secrets generator / password-manager features | scope creep into 1Password/Bitwarden territory |

**Explicit *never* non-goals:** a general-purpose database, a code-deployment tool, a vendor-independent KMS abstraction layer (the `MasterKeyProvider` interface is intentionally thin), and a logging/observability platform. Soft "no"s (SIEM streaming, SOC 2/ISO certs, self-service signup, multi-region) are revisited at Phase B planning.

---

## 7. Tracked follow-ups / tech-debt process

[FOLLOWUPS](../FOLLOWUPS.md) is the **single source of truth** for tracked tech debt and deferred work. Every entry has an **owner** (a role) and a **due date** — no-owner items are not tracked (they're assigned or deleted). The file is reviewed at the monthly ADR review; items past due without explanation get pulled into the next 30-day plan.

Process: **adding** an item = a PR adding a numbered entry under Phase A or B with owner + due date, reviewed by the named owner. **Closing** = move it under "Closed" with the artifact that closes it (SHA / file / PR) — never delete (historical follow-ups are useful in post-mortems). **Slipping** a due date is allowed *once* with a written reason; a second slip triggers a re-plan of that role's 30/60/90.

Notable open Phase-A debt (cross-referenced in [operations](./10-operations.md)): KMS adapter wiring (#1), DEK rotation API (#3), nonce-collision monitoring (#7), Seidr-runtime E2E boot (#8), and the deferred ~90-cell negative-auth matrix (S-M3). The `safego`/goroutine-recover hardening rides on ADRs 0010/0011/0015.

---

## 8. Per-role 30/60/90 plan

[ROLES_30_60_90](../ROLES_30_60_90.md) is the Phase-A action plan, one row each for 30/60/90 days per role. Its reality check is honest about staffing: the Phase-A team is **1 Tech Lead, 1 Backend, 1 Frontend, part-time Security, part-time DevOps, shared QA** — PM/UX/Technical-Writer slots are **not yet hired**, and an **Interim owner** column says who carries each un-staffed responsibility (usually the Tech Lead). Examples of the leverage chain:

- **Security 30-day:** re-baseline [THREAT_MODEL](../THREAT_MODEL.md), full STRIDE pass on the promotion engine (highest-value surface).
- **Backend 30/60/90:** audit-log + negative-auth wiring → concurrency tests → additive multi-tenant schema prep (Type-1, needs ADR + sign-off).
- **DevOps 30-day:** the secret-source map ([SECRET_SOURCES](../SECRET_SOURCES.md)) and CI least-privilege ([CI_PERMISSIONS](../CI_PERMISSIONS.md)) — both already landed.
- **QA 30-day:** the test pyramid census, negative-auth gap list, and flaky-test tracker.

The plan is a budget, not a wishlist: "if something here gets pulled into Phase A, something else has to be pushed out."

---

## 9. Mandatory PR gates (`CLAUDE.md`)

Two project-wide gates are lint/review-enforced and reject PRs that fail them:

1. **No raw error leakage.** Handlers must never return `err.Error()` directly to clients; they use the `httperror` package per [ERROR_HANDLING_STANDARD](../ERROR_HANDLING_STANDARD.md). The AST-walking regression gate is `internal/api/error_leak_test.go` (125 leak sites were migrated in the Phase 1 sweep).
2. **Audit event + test assertion.** Every state-mutating handler (secret / project / api-key / promotion) MUST emit an audit event from the canonical taxonomy in [AUDIT_LOG_COVERAGE](../AUDIT_LOG_COVERAGE.md), **and the test MUST assert the audit row was written.** PRs failing either are rejected (see [testing](./11-testing.md) §8).

Additional process rules from `CLAUDE.md`: conventional commits (`feat:`/`fix:`/`docs:`/`chore:`), feature branches off `main` with required PRs, and "read before non-trivial changes" pointers to [ROLES](../ROLES.md), [ADR](../adr/), [THREAT_MODEL](../THREAT_MODEL.md), and [FOLLOWUPS](../FOLLOWUPS.md).

---

## See also

- [ROLES](../ROLES.md) — roles, veto, decision classes, rituals
- [ROLES_30_60_90](../ROLES_30_60_90.md) — Phase-A per-role action plan
- [FOLLOWUPS](../FOLLOWUPS.md) — tracked tech debt
- [ROADMAP_NOT](../ROADMAP_NOT.md) — the explicit *no* list
- [Roadmap](../../Roadmap.md) — the *yes* list and current phase
- [ADR index](../adr/README.md) — full ADR directory
- [Security](./05-security.md) — the threat model these processes protect
