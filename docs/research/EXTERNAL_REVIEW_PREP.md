# External Review Preparation — Pentest Scope-of-Work

- **Audience:** External security firm engaged to pentest KeepSave.
- **Status:** Pre-engagement briefing. All ADRs cited below are `proposed`, not yet accepted.
- **Date of corpus snapshot:** 2026-05-15.
- **Owners:** Research Lead + Technical Writer (joint draft).

This document briefs an external reviewer who has never seen the codebase. It packages what KeepSave is, what we already know from internal audits, what we propose to fix, and where the firm's eyes add the most value. Read the linked artefacts for depth; this is the index.

---

## 1. Initiative scope summary

KeepSave is a secure environment-variable storage and promotion system (Go + Gin backend, PostgreSQL, React/TS dashboard, Web Component embed widget). Architecture and governance: [`CLAUDE.md`](../../CLAUDE.md).

A competitive research initiative ran in parallel to Phase A hardening, scoped to auth/identity, secrets management, and approval workflows. 8 P0 dossiers (Vault, Ory, Keycloak, GitHub PATs+Environments, Infisical, Doppler, SPIFFE/SPIRE, Pomerium) and 5 P1 dossiers (Authentik, Zitadel, Cloudflare Access, 1Password Connect, Teleport). Outputs: 13 competitor dossiers, pattern matrix v1 (15 rows), BEYOND memo, and 11 proposed ADRs (0005–0015) covering every `adopt-now` synthesis output and audit-team-sweep finding. A code-and-docs audit team ran a sweep on the same date (2026-05-15) and produced five reports (Section 2). The research corpus and audit corpus converge: every adopt-now recommendation has either an FU# or a new ADR; every P1/P2 audit finding has an ADR or an FU.

## 2. Audit findings inventory

Five reports, all dated 2026-05-15. See [`/home/user/KeepSave/docs/audits/`](../audits/).

### [`SECURITY_AUDIT_2026-05-15.md`](../audits/SECURITY_AUDIT_2026-05-15.md) — 6 P1 / 5 P2 / 6 P3 / 4 P4 / 5 P5
Top 3:
- **A01-F1 Secret CRUD IDOR (P1)** — `backend/internal/api/handlers_secret.go:33,55,81,109,131` + `backend/internal/service/secret_service.go:41-183`: any authenticated caller who knows a project UUID can read/write/delete its secrets.
- **A01-F2 API-key project scope ignored (P1)** — `backend/internal/api/middleware.go:107-112` sets `api_key_project_id` in context; only `backend/internal/api/handlers_agent.go:42-52` consumes it. All other API-key routes ignore scope.
- **A03-F1 MCP entry-command RCE (P1)** — `backend/internal/api/handlers_mcp_gateway.go:327-339` execs DB-stored `EntryCommand` with decrypted secrets in env.

### [`BACKEND_SPOF.md`](../audits/BACKEND_SPOF.md) — 5 critical
Top 3:
- **No graceful shutdown** — `backend/cmd/server/main.go:217,224`; SIGTERM truncates in-flight requests and leaves the non-transactional `executePromotion` / `RotateProjectKey` half-committed.
- **DB calls with no context** — 182 sites across `backend/internal/repository/**` use `Query`/`QueryRow`/`Exec` (zero `*Context`).
- **No `SetConnMaxLifetime` / `SetConnMaxIdleTime`** — `backend/internal/repository/db.go:32-55`; dead conns accumulate behind NAT / PgBouncer.

### [`BACKEND_CRASH_RISKS.md`](../audits/BACKEND_CRASH_RISKS.md) — 6 critical (all goroutine-no-recover)
Top 3 (same class, ranked by exploitability):
- `backend/internal/api/handlers_mcp.go:47, :163` — `go h.builderService.BuildServer/RebuildServer`; a panic in any unmarshal path kills the API process. Reachable by any authenticated user.
- `backend/internal/service/webhook_service.go:113` — `go ws.deliver(...)`; latent today (no callers), critical the moment ADR-0013 wires it.
- `backend/internal/events/eventbus.go:91, :94` — wildcard + per-key handler dispatch; latent today.

### [`API_RECONCILIATION.md`](../audits/API_RECONCILIATION.md) — 1 critical
- **F-C-001** — `frontend/src/embed/api.ts:101-104` posts to `POST /api/v1/projects/:id/secrets/batch`; backend has no such route. SDK callers 404.

### [`FRONTEND_DEAD_UI.md`](../audits/FRONTEND_DEAD_UI.md) — 0 critical, 4 high
Top 3 high:
- `frontend/src/pages/ProjectDetailPage.tsx:122` — "Rotate all" button with no `onClick`.
- `frontend/src/pages/ProjectDetailPage.tsx:121` — "Export .env" button with no `onClick`.
- `frontend/src/pages/ProjectsPage.tsx:174` — "Import .env" button with no `onClick`.

## 3. Proposed ADRs

All ADRs are `Proposed`, dated 2026-05-15. Sources: [`/home/user/KeepSave/docs/adr/`](../adr/). Type classification follows `CLAUDE.md` §"Decision classes".

| ADR | Title | Addresses | Type |
|---|---|---|---|
| [0005](../adr/0005-require-project-access-middleware.md) | `RequireProjectAccess` middleware for `/projects/:id/*` | A01-F1..F10 (IDOR family); THREAT_MODEL §2/E new row | Type-1 |
| [0006](../adr/0006-embed-widget-origin-allowlist.md) | Embed widget origin allow-list (server-side, per-project) | A04-F2; FU 0b; THREAT_MODEL §4/S | Type-1 |
| [0007](../adr/0007-approver-not-requester-db-invariant.md) | Approver ≠ requester DB invariant for promotion | A04-F1; FU 0d / FU 5; ADR-0003 open question | Type-1 |
| [0008](../adr/0008-rs256-jwks-rotation.md) | RS256 JWT signing with JWKS + `kid` rotation | Empty JWKS at `handlers_oauth.go:216-218`; Pattern Matrix row 1; Ory dossier Cand. 1 | Type-1 |
| [0009](../adr/0009-default-api-key-expiration.md) | Mandatory default expiration on `ks_` API keys | A07-F1; FU 0k; Pattern Matrix row 6 (convergent: Vault/SPIFFE/GitHub) | Type-1 |
| [0010](../adr/0010-mcp-gateway-command-execution-hardening.md) | Harden MCP gateway exec path (allowlist + sandbox + safego + build budget) | A03-F1; BACKEND_CRASH_RISKS #1, #2; THREAT_MODEL §6 new section | Type-1 |
| [0011](../adr/0011-graceful-shutdown-and-db-timeouts.md) | Graceful shutdown, DB context, HTTP timeouts, pool-lifetime | BACKEND_SPOF #1, #2, #3, #5, #10 (5 critical of the 5) | Type-2 |
| [0012](../adr/0012-kms-auto-unseal.md) | KMS auto-unseal as production default for the master key | FU 1; ADR-0004 follow-through (`main.go:247-248` bails on KMS today) | Type-1 |
| [0013](../adr/0013-webhook-emission-with-ssrf-guard.md) | Webhook emission with SSRF guard, body-buffered retries, signing-secret rotation | A10-F1; BACKEND_SPOF #6, #7; Infisical dossier `adopt-now, blocked-pending-SSRF` | Type-1 |
| [0014](../adr/0014-audit-log-taxonomy-extension.md) | Audit-log taxonomy: `role.changed`, `settings.changed`, `actor_type` | THREAT_MODEL §1/R new row; AUDIT_LOG_COVERAGE gap | Type-1 |
| [0015](../adr/0015-safego-helper-and-audit-emission.md) | `getUserID`/`getActor` helpers + per-use API-key audit emission | FU 0l; BACKEND_CRASH_RISKS #11 (58 sites); A09-F2 | Type-1 |

Count: **11 (0005–0015)**. ADR-0011 is the sole Type-2; the remaining 10 are Type-1 (crypto, auth, promotion, or audit-field surface) and require Security Engineer sign-off.

## 4. Threat-model delta

Authoritative file: [`/home/user/KeepSave/docs/THREAT_MODEL.md`](../THREAT_MODEL.md) v1.2.1 (2026-05-15). Changelog entry for the relevant commit (`4286d0a`-class re-baseline) reads:

> **1.2.1 (2026-05-15):** Audit team sweep delta. **5 new STRIDE rows** (§2/E JWT-no-project-bind, §3/I Diff plaintext, §1/I webhook SSRF, §1/R audit-log tamper + taxonomy gaps, §6/T MCP entry-command RCE, §6/D MCP build DoS, §7/I secret-version retention) and **4 residual-risk updates** (§2/E API-key scope → High; §3/R approval-merge cross-refs ADR-0007; §1/T KMS throttle noted moot pending FU#1; §1/R audit-log row added). New top-level sections §6 (MCP tool execution) and §7 (Secret-version retention).

Net change: two new STRIDE sections, six previously-Low/Medium rows reclassified to **High** pending the ADR implementations above. See the file for per-row residual ratings and `file:line` anchors.

## 5. Open follow-ups not yet addressed by ADRs

From [`FOLLOWUPS.md`](../FOLLOWUPS.md). Items below have no proposed ADR:

- **FU 0a** — `err.Error()` leak fix; tracked against `ERROR_HANDLING_STANDARD.md`.
- **FU 0c** — no handler-level negative-auth tests; covered by `tests/NEGATIVE_AUTH_PLAN.md`.
- **FU 0e / FU 4** — promotion kill switch; tracked as feature-flag, not ADR-shaped.
- **FU 0g** — CODEOWNERS for security-critical paths.
- **FU 0h** — dashboard JWT localStorage key-name inconsistency in `HelpPage.tsx`.
- **FU 0i** — auto-hide timer on revealed secrets (widget + dashboard).
- **FU 0j** — destructive actions use `window.confirm` instead of typed confirmation.
- **FU 2** — backup tamper-detection test (formalises ADR-0001 AEAD-auth guarantee).
- **FU 3** — DEK rotation API; ADR-0004 §Open Questions, not yet ADR-drafted.
- **FU 6** — Phase 15 service unit tests; blocked on SQLite harness.
- **FU 7** — nonce-collision counter per DEK (2^28 alert threshold).
- **FU 8** — Seidr-runtime real container in E2E harness.
- **FU 9** — audit-log assertion coverage gaps (ADR-0014 covers schema only).
- **FU 10** — FIPS-mode evaluation (decision item).
- **Phase B deferred** (trigger-bound; see [`ROADMAP_NOT.md`](../ROADMAP_NOT.md)): per-environment DEK, JWT denylist, fine-grained API key scopes, three-of-N approval, HKDF subkey derivation, SPIFFE-shaped SVIDs, lease-renewal grammar. Out of scope for the current pentest.

## 6. Suggested pentest scope

Priority order, post-implementation of the named ADRs:

1. **Embed widget origin allow-list (ADR-0006).** Audit confirmed the wildcard `postMessage` exploitable today (`frontend/src/embed/auth.ts:21-26, :33`); the ADR proposes a per-project allow-list served from `/api/v1/projects/:id/embed-config` plus strict `ev.origin` equality. Bug shapes: origin-normalisation bypasses (trailing dot, IDN, case), cache-poisoning of the embed-config response, races between widget bootstrap and config fetch, and confused-deputy variants where a legitimate top-level page co-frames a malicious sibling.

2. **MCP gateway command execution (ADR-0010).** P1 authenticated RCE primitive today (`backend/internal/api/handlers_mcp_gateway.go:327-339`). The ADR is a four-part design (binary allow-list, container sandbox, `safego` wrapper, per-user build-time budget) — the most complex of the eleven. Bug shapes: allow-list bypass via symlinks / PATH lookup, env-var leakage past the sandbox boundary, build-budget exhaustion as a DoS oracle, subprocess-timing side channels that leak secret contents.

3. **Promotion engine multi-party-control (ADR-0007).** DB-`CHECK` constraint approach is canonical. Open question 3 is the system-actor edge case (does a scheduled / service-account promotion satisfy the invariant?). External sanity-check should target: confused-actor attacks against impersonated service accounts, replay of a `promote` request after the requester is deactivated, approver-self-approval via an alternate identity.

4. **KMS unseal under transient failure (ADR-0011 + ADR-0012).** KMS adapter path lands together with graceful shutdown and pool-lifetime fixes. Bug shapes: cold-start under KMS throttling (does the pod publish secrets via crashloop logs?), behaviour during KMS key rotation, master-key-version skew across replicas, key-cache poisoning across a rolling deploy.

5. **Webhook SSRF guard (ADR-0013).** Audit confirmed IMDS reachability (`webhook_service.go:136, :158`). The ADR pairs an allow/deny URL validator with DNS-rebinding-safe re-resolve at request time, body-buffered retries, and per-org signing-secret rotation. Classic pentest-bait: time-of-check vs time-of-use, IPv6-mapped addresses, redirect chains that escape the allow-list, DNS responses that flip between resolutions.

6. **Audit-log integrity (ADR-0014).** Extends the taxonomy (`role.changed`, `settings.changed`, `actor_type`); THREAT_MODEL adds a §1/R row for tamper-evidence. Chain-of-evidence review should target: privileged-actor backfill or rewrite paths, timestamp monotonicity and trust, and whether the audit record survives replication / backup paths intact.

## 7. Where to find the corpus

- [`/home/user/KeepSave/docs/research/competitors/`](competitors/) — 13 dossiers.
- [`/home/user/KeepSave/docs/research/PATTERN_MATRIX.md`](PATTERN_MATRIX.md) — matrix v1.
- [`/home/user/KeepSave/docs/research/BEYOND.md`](BEYOND.md) — future ideas.
- [`/home/user/KeepSave/docs/audits/`](../audits/) — 5 audit reports.
- [`/home/user/KeepSave/docs/adr/`](../adr/) — ADRs 0001-0004 historical, 0005-0015 proposed.
- [`/home/user/KeepSave/docs/THREAT_MODEL.md`](../THREAT_MODEL.md) — current threat model (v1.2.1).
- [`/home/user/KeepSave/docs/FOLLOWUPS.md`](../FOLLOWUPS.md) — open work.
