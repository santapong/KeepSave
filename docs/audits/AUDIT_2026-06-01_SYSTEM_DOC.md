# Audit — System Documentation Set (`docs/system/`) — Completeness & Accuracy

**Date:** 2026-06-01
**Subject:** The consolidated "entire system" documentation set under [`docs/system/`](../system/README.md) — the index plus chapters 01–13.
**Method:** The set was drafted from a pass over the codebase, then put through a multi-round completeness/accuracy audit. Each audit pass **re-derived ground truth from the source** (backend, frontend, migrations, Helm, CI, tests, ADRs, SDKs) rather than trusting the prose, and graded findings BLOCKER / MAJOR / MINOR. The loop ran **fix → re-audit** until no BLOCKER or MAJOR gaps remained.

## Final verdict

> **Round 3: PASS — no BLOCKER or MAJOR gaps remain.** All MINOR nits raised across the three rounds were also addressed.

The set is verified, at the **self-contained-master-doc bar** (comprehensive inline, linking to canonical docs for last-mile specifics), to completely and accurately describe the system as of 2026-06-01.

## Loop summary

| Round | What ran | Result |
|-------|----------|--------|
| 0 | Drafting — 4 parallel authors produced 14 files (README + 01–13); index + overview assembled and Mermaid-validated. | 14 files, ~3,300 lines, 25 diagrams, 0 broken links. |
| 1 | Initial audit — **3 parallel auditors** (backend/API/data · security/promotion · frontend/infra/ops/test/gov/sdk). | **1 BLOCKER, 1 MAJOR, 11 MINOR.** All fixed. |
| 2 | Re-audit — round-1 fixes verified; fresh sweep. | Round-1 fixes RESOLVED; **1 fresh MAJOR + 3 MINOR** found. All fixed. |
| 3 | Final re-audit — round-2 fixes verified; final cross-chapter consistency sweep. | Round-2 fixes RESOLVED; **PASS**; 3 MINOR nits — also fixed. |

## Round 1 — gaps & resolutions

| # | Sev | Chapter | Gap | Resolution |
|---|-----|---------|-----|------------|
| 1 | **BLOCKER** | 05-security | Overstated API-key scope enforcement ("per-handler"). In code, `api_key_scopes` is set in context (`middleware.go:184`) but **never read** — a `read` key can write/delete. | Reworded §5.5 to "advisory only, not enforced at any handler"; added a residual-risk caveat to the invariants checklist and broadened the doc/code-delta row. |
| 2 | **MAJOR** | 03-api-reference | Stale route count (126 / 5+121); inherited from a stale reconciliation doc. | Corrected to **142 (5 root + 137 `/api/v1`)** in the intro and summary; per-route tables already enumerated all 142. |
| 3 | MINOR | 05-security | Headers table omitted `X-XSS-Protection`. | Row added (flagged legacy/deprecated). |
| 4 | MINOR | 06-promotion | Cited handler `ListAuditLog` (it's the handler `AuditLog` calling `service.ListAuditLog`). | Corrected. |
| 5 | MINOR | 02-backend | `SecurityEventRepository` listed as live but never constructed. | Annotated "defined but not wired; no runtime writer". |
| 6 | MINOR | 02-backend | `IPAllowlistService` listed as live but never constructed. | Annotated "not constructed; `CheckIPAllowed` never called"; removed from the raw-SQL services note. |
| 7 | MINOR | 02-backend | Counts "≈21 repositories / 30 services". | Tightened to "19 repositories and 27 services (plus 4 AI-provider adapters)". |
| 8 | MINOR | 03-api-reference | Stale "frontend calls 87" stat. | Removed. |
| 9 | MINOR | 11-testing | Negative-auth matrix denominators inconsistent. | Restated as 12×11 = 132 cells; ~42 covered / ~90 deferred. |
| 10 | MINOR | 10-operations / 13-sdks | Understated landed MCP exec hardening. | Corrected: binary allowlist (`node`/`python`/`python3`) + path/metachar rejection + scrubbed env + `CommandContext` timeout ARE enforced; only `go-binary`, argv-arrays, `safego`, and the build budget pending. |
| 11 | MINOR | 08-embed-widget | `index.ts` snippet omitted the type re-exports. | Snippet updated to match source. |
| 12 | MINOR | 07-frontend | Hierarchy diagram omitted the AI page's own `OverviewTab`. | Added as a distinct Mermaid node (re-validated). |
| 13 | MINOR | 07-frontend | `getPrometheusMetrics` same-origin `/metrics` path not noted. | Exception noted. |

## Round 2 — gaps & resolutions

| # | Sev | Chapter | Gap | Resolution |
|---|-----|---------|-----|------------|
| 1 | **MAJOR** | 03-api-reference §15 | Falsely claimed OAuth tokens are RS256 JWTs with a JWKS endpoint — contradicting ch.05 and ch.13. In code, OAuth tokens are **opaque random strings** (stored hashed, DB-validated); session JWTs are **HS256**; JWKS returns an **empty set**; RS256/JWKS is ADR-0008 (Proposed). | Rewrote the sentence to the accurate, cross-consistent statement. |
| 2 | MINOR | README | ToC said "migrations 001–008". | Corrected to 001–009. |
| 3 | MINOR | 13-sdks | Batch-fetch listed without the "no backend route / 404 / F-C-001" caveat. | Caveat added. |
| 4 | MINOR | 11-testing | Off-by-one (31 N/A / 101 applicable). | Corrected to 32 N/A / 100 applicable. |

## Round 3 — final sweep (PASS) & MINOR nits resolved

No BLOCKER or MAJOR gaps. Three MINOR nits, all fixed:

| # | Chapter | Nit | Resolution |
|---|---------|-----|------------|
| 1 | 09-infrastructure | Real Go-toolchain skew printed but unflagged (Dockerfile `golang:1.24` vs go.mod `go 1.25.0` vs CI 1.25.x). | Added a "toolchain skew (repo bug)" note recommending the Dockerfile base be bumped. |
| 2 | 05-security | `getUserID` migrated-site count given three ways (~58 / 61-of-62 / ADR's 58). | Dropped the imprecise number in 05; the precise live figure lives in [operations](../system/10-operations.md). |
| 3 | 08-embed-widget | Batch row omitted the F-C-001/404 caveat carried elsewhere. | Caveat added. |

## Coverage confirmed by the audits (highlights)

Re-derived from source and confirmed accurate: the envelope-encryption key hierarchy and decrypt path; JWT (HS256/24h) and API-key format/hashing/expiry; the four-eyes promotion invariant (service guard + DB CHECK) and the forward-only/no-skip pipeline; the plaintext-free HMAC diff; the audit taxonomy and error-sanitization model; the full **142-route** surface; the table-by-table schema for migrations `postgres/001–009`; the ADR index 0000–0016 with correct statuses; the CI/Helm/Dockerfile facts; the SDKs (Go/Node/Python) and integrations; and the careful **implemented-vs-planned** separation throughout (KMS adapters, RS256/JWKS, several audit events/metrics, `safego`, per-query timeouts — all correctly marked deferred).

## Out-of-scope code observations surfaced by the documentation effort

These are **real code/config issues** the audit uncovered while verifying the docs. They are recorded here (and, where relevant, noted in the chapters) but are **code fixes, not doc changes** — they warrant separate backend tickets and the usual review per [`docs/ROLES.md`](../ROLES.md):

1. **Go-toolchain skew** — `backend/Dockerfile` builds with `golang:1.24-alpine` while `backend/go.mod` requires `go 1.25.0`; bump the Dockerfile base. (Noted in `09-infrastructure.md §1.1`.)
2. **Phase-14/15 migration gap** — the `applications`/`application_favorites` and the nine Phase-15 AI/quota tables have `CREATE TABLE` only in the flat top-level migrations, not in the per-dialect `postgres/` directory that `migrate.go` actually runs; a per-dialect Postgres deploy is therefore missing them. (Documented prominently in `04-data-model.md`.)
3. **OIDC discovery over-advertises PKCE `plain`** — the discovery document lists `code_challenge_methods_supported: ["S256","plain"]`, but `verifyPKCE` enforces `S256` only. (Docs correctly describe S256-only; fix the discovery doc.)
4. **MCP gateway timeout comment** — a code comment claims the process *group* is killed via `SysProcAttr`, but none is set, so `CommandContext` kills only the direct child.

## Sign-off

The `docs/system/` set is **complete and accurate** at the agreed bar as of **2026-06-01**, with a clean three-round audit trail. Maintain it like the other living docs: when the system changes, update the affected chapter (and any canonical doc it summarizes) in the same PR.
