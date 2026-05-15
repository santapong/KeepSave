# ADR-0005: `RequireProjectAccess` middleware for `/projects/:id/*` routes

- **Status:** Proposed
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (on behalf of Backend Engineer)
- **Reviewers required:** Security Engineer (mandatory — touches auth); Tech Lead (Type-1 per `docs/CLAUDE.md` §"Decision classes")
- **Supersedes:** none
- **Related:** ADR-0002 (auth model — this ADR extends it with per-resource authorization); `docs/THREAT_MODEL.md` §2 row E and "Findings new in v1.2.0"; `docs/audits/SECURITY_AUDIT_2026-05-15.md` §3.A01

---

## Context

The 2026-05-15 audit (`docs/audits/SECURITY_AUDIT_2026-05-15.md` §3.A01) surfaced nine P1/P2 IDOR findings sharing one root cause: routes mounted under `/projects/:id/*` are protected by `JWTAuthMiddleware` / `APIKeyAuthMiddleware` (`backend/internal/api/router.go:65-131, 133-139`), both of which only *identify* the caller. Neither asserts that the caller has access to the project in the path. The audit:

> "The `/api/v1/projects/:id/...` route family is mounted with `JWTAuthMiddleware` or `APIKeyAuthMiddleware`, both of which only **identify** the caller — neither enforces that the caller has access to `:id`. ... the check is not reused by sibling routes." (§3.A01 preamble)

Load-bearing findings:

> **A01-F1 — Secret CRUD IDOR (P1).** "Each method calls `s.projectRepo.GetByID(projectID)` to fetch the encrypted DEK but never asserts the project belongs to the authenticated user." (§3.A01-F1) — verified against `backend/internal/service/secret_service.go:41-183`: `Create` (41), `GetByID` (74), `List` (105), `Update` (139), `Delete` (174) all load the DEK with no ownership check.

> **A01-F2 — API key scope ignored (P1).** "`backend/internal/api/middleware.go:107-112` sets `api_key_project_id` ... only `handlers_agent.go:42` consumes it." (§3.A01-F2)

> **A01-F4 — promotion Diff IDOR (P1).** "`backend/internal/api/handlers_promotion.go:60-85` (`Diff`) parses `:id`, calls `promotionService.Diff`, no auth check." (§3.A01-F4)

> **A01-F5 — rotate-keys IDOR (P1).** "`backend/internal/api/handlers_keyrotation.go:22-39` `RotateProjectKey` — no ownership check before rotating the DEK of any project." (§3.A01-F5)

> **A01-F6 — enterprise endpoints IDOR (P1).** "Every handler parses `:id` from path and calls the service directly with no ownership check." (§3.A01-F6)

This invalidates `docs/THREAT_MODEL.md` §2 row E ("API key scope escalation" residual Low) and necessitates a new row "JWT scope absent — single JWT grants access to all of caller's projects" (audit §5.2). The audit's remediation §6 ranks "Central project-access middleware" as item 1, closing A01-F1/F3/F4/F5/F6/F7/F8/F9/F10 — **nine findings in one PR**. The Day-30 GitHub dossier predicted an approver-chain failure; this ADR addresses the broader access-control chain the audit shows is the dominant exposure class on `main` today.

This ADR *narrows* the auth→resource trust boundary. No boundary is widened.

## Options considered

### Option A — `RequireProjectAccess(":id")` middleware mounted per route group

New middleware in `backend/internal/api/middleware.go` that:

1. If `api_key_project_id` is in context (API-key path, `middleware.go:108`), asserts string-equality with `:id`. Environment match deferred to handler since the env-param name varies.
2. Else (JWT path), looks up project membership for `user_id` via one indexed query (`organization_members` → `organization_projects`, with `projects.owner_user_id` fallback).
3. On miss, abort **404** (not 403), emit `auth.access_denied` audit event (`docs/AUDIT_LOG_COVERAGE.md`), stop.

Mounted on every existing `/projects/:id/...` group (`router.go:75-85` sec, `:87-131` pm, `:133-139` ls). Per-request memoization via `c.Set("project_access_ok", true)`; no per-process cache.

- **Pros:** single source of truth; one indexed query per request; testable unit; closes nine findings in one PR; new routes inherit the check by mounting in the right group.
- **Cons:** one extra query per request (~0.5-2 ms p50 on a warm `(project_id, user_id)` index — dominant cost is round-trip); per-process caching unsafe for revocation freshness; if a future contributor adds a new group without the middleware, the regression is silent. Mitigated by an integration test that enumerates router routes and asserts middleware presence.

### Option B — Enforce in each handler / service method

Add `projectRepo.UserHasAccess(projectID, userID)` at the top of every handler.

- **Pros:** no new abstraction; per-call-site error semantics.
- **Cons:** this is the *status quo intent*. The audit is empirical proof it fails: nine routes already missed the check. Easy to forget; cannot be statically enforced; every new endpoint is a fresh regression opportunity.

### Option C — Postgres row-level security (RLS)

RLS policies on `secrets`, `secret_versions`, `promotions`, `webhooks`, `backups`, `policies`, etc., tied to a session GUC (`SET LOCAL keepsave.current_user_id = ...`) set per-request.

- **Pros:** defense-in-depth at the strongest layer; survives application regressions.
- **Cons:** Postgres-specific; harder to unit-test (needs a real DB for every test); per-request GUC adds pool complexity; non-trivial migration touching every per-tenant table; doesn't naturally express API-key vs. JWT distinction. Strong long-term goal — not the *first* fix.

## Decision

**Adopt Option A — `RequireProjectAccess` middleware mounted on every `/projects/:id/*` route group.** Option C remains a long-term defense-in-depth goal; Option A is the high-leverage single-PR fix that closes nine P1/P2 findings now.

**Re-decision trigger:** if a future deployment target lacks Postgres-compatible RLS, C is permanently off the table; if all targets support RLS within 12 months, we layer C on top of A — not instead of A. Application-layer authz remains primary and testable.

**Auth source priorities:**

1. **API key:** `api_key_project_id` (already at `middleware.go:108`) compared to `:id`. Mismatch is a hard reject.
2. **JWT:** membership lookup for `user_id` via the org-membership graph, with the `projects.owner_user_id` fallback for legacy single-owner rows.

**Failure mode: 404, not 403.** 403 confirms "this project exists", giving an enumeration oracle to attackers iterating UUIDs (and confirming hit/miss for stolen UUIDs). 404 is indistinguishable from "no such project".

**Audit emission.** Every reject emits `auth.access_denied` with `{actor, requested_project_id, route, auth_mode}` per `docs/AUDIT_LOG_COVERAGE.md`. Acceptances are **not** audited (volume).

## Rejection rationale

- **Option B** — the audit is empirical proof "every handler checks" doesn't survive a growing codebase; nine routes already missed. Centralisation is the structural fix.
- **Option C** as a first step — heavy migration, worse test ergonomics, brittle failure mode ("all queries fail until GUC is set"). Strong defense-in-depth follow-up, not first move.

## Consequences

- **Operational:** one extra indexed DB query per `/projects/:id/*` request. Expected ≤2 ms p50 against a warm `project_members(project_id, user_id)` index; capture `EXPLAIN` baseline pre-merge. New runbook entry: "404 rate spike → check `auth.access_denied` audit for affected actor".
- **Security:** closes A01-F1, F3, F4, F5, F6, F7, F8, F9, F10 (9 of 11 IDOR findings). New `auth.access_denied` event becomes the canonical signal for cross-tenant probing. Threat-model §2 row E residual returns from High (post-audit) to Low after merge. A new testable invariant: "every `/projects/:id/*` route is preceded by `RequireProjectAccess`".
- **Migration:** no schema change if `organization_members` / `organization_projects` cover all live projects (verify against `backend/migrations/`). Legacy `projects.owner_user_id` rows handled by a `UNION` fallback in the membership query. No data migration.
- **Reversibility:** straightforward. Single-PR revert restores prior (insecure) behaviour without data migration. No ciphertext rewritten; no state touched.

## Rollback plan

If `RequireProjectAccess` causes an unexpected outage:

1. Revert the PR. Middleware removal restores pre-merge behaviour — *insecure* (the IDOR cluster returns) but *unbreaking*: no in-flight requests fail, no rows need rewriting.
2. Re-issue a narrower follow-up within the same change window, e.g. mount the middleware on one group at a time.
3. If only a subset of groups misbehaves, gate the mount per-group. A feature flag is not warranted — the middleware is either on or off per group, and rollback granularity at the `r.Use(...)` line suffices.

Rollback **does not** require data migration. The new `auth.access_denied` audit rows are additive and remain valid even if the middleware is removed.

## Open questions

For Security Engineer review before status moves to Accepted:

1. **Pre-auth vs. post-auth 404 semantics:** should `RequireProjectAccess` 404 **before** auth (wrong UUID before login is checked) or **after** auth? Pre-auth is tighter; post-auth distinguishes "no session" (401) from "no such project for you" (404). Proposed: post-auth — auth middlewares run first. *Owner: Security Engineer. Due: before merge.*
2. **`api_key_project_id` mismatch — soft-deny + audit, or hard kill?** A key presented against a project it isn't scoped to is either a misconfigured agent (soft: 404+audit, key remains usable on its real project) or an exfiltrated key being abused (hard: revoke immediately). Proposed: soft-deny + audit, with auto-revocation after N mismatches in a rolling window (N TBD). *Owner: Security Engineer. Due: before merge.*
3. **Membership-cache coherence on revocation:** if we add per-process caching, how do we invalidate on `OrganizationService.RemoveMember`? Proposed: **no caching beyond per-request** (`c.Set`); every request re-queries. Per-process caching rejected on freshness grounds — a revoked member must lose access within one request. Revisit only if benchmarks show the query is a hot spot. *Owner: Security Engineer + Backend Engineer. Due: before merge.*

## References

- **Audit:** `docs/audits/SECURITY_AUDIT_2026-05-15.md` §3.A01 (F1-F10), §5.1 (threat-model invalidations), §6 item 1 (remediation priority).
- **Threat model:** `docs/THREAT_MODEL.md` §2 row E (residual changes after merge); new row "JWT scope absent" to land in the same PR.
- **Current state file:line:**
  - Route groups: `backend/internal/api/router.go:75-85` (secrets), `:87-131` (project handlers), `:133-139` (leases).
  - Auth middleware: `backend/internal/api/middleware.go:59-86` (JWT), `:88-120` (API key), `:107-112` (`api_key_project_id` set).
  - Service missing-check: `backend/internal/service/secret_service.go:41, 74, 105, 139, 174`.
  - Sole consumer of `api_key_project_id` today: `backend/internal/api/handlers_agent.go:42-52`.
- **Implementation touchpoints:**
  - New `RequireProjectAccess(paramName string) gin.HandlerFunc` in `backend/internal/api/middleware.go` (appended after `APIKeyAuthMiddleware`).
  - Mount sites: `router.go:76`, `:88`, `:134`.
  - New `MembershipService.UserHasProjectAccess(userID, projectID) (bool, error)` in new file `backend/internal/service/membership_service.go`.
  - Tests: new `tests/integration/project_access_idor_test.go` enumerating every `/projects/:id/*` route from `router.go` and asserting 404 for cross-tenant access (per `tests/NEGATIVE_AUTH_PLAN.md`).
- **Related ADRs:** ADR-0002 (auth identifies caller; this ADR adds per-project authorization), ADR-0003 (promotion Diff is among the protected routes).
