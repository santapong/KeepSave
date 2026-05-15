# ADR-0015: `getUserID`/`getActor` context helpers and per-use API-key audit emission

- **Status:** Proposed
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (Backend Engineer role)
- **Reviewers required:** Tech Lead (gating on Head 1); Security Engineer (mandatory veto on Head 2 — touches `internal/auth` middleware + audit log, per `docs/ROLES.md` §3.1 Type-1).
- **Supersedes:** none
- **Related:** ADR-0010 part C; ADR-0011 part 5 (same `safego` helper); ADR-0014 (atomic — adds `actor_type` to `AuditEntry`); `docs/audits/BACKEND_CRASH_RISKS.md` §Risk 11; `docs/research/competitors/github.md` Candidate 4; `docs/FOLLOWUPS.md` 0l.

---

## Context

Two findings hit the same code path; closing them in one ADR is economical.

**Finding 1 — `c.MustGet("user_id").(uuid.UUID)` × 58 sites.** Verbatim from `docs/audits/BACKEND_CRASH_RISKS.md:53` (Risk 11):

> *"`internal/api/handlers_*.go` × 58 sites (`c.MustGet("user_id").(uuid.UUID)`) — type-assertion + panic-explicit. Route registered under `JWTAuthMiddleware` or `APIKeyAuthMiddleware` — those always set `user_id`. If a future PR ever wires a handler in router that isn't under auth, MustGet panics on missing key and the type assertion panics on wrong type. Suggested fix: introduce `getUserID(c)` helper that does `c.Get` + comma-ok + 401 on miss."*

Every site is reachable only under `JWTAuthMiddleware` or `APIKeyAuthMiddleware`, both setting `user_id` to `claims.UserID`; `gin.Recovery()` at `router.go:29` converts the panic to a per-request 500. The forward-looking risk is the foot-gun the audit names — the moment a future PR mounts a handler outside an auth group, every request panics.

`docs/FOLLOWUPS.md:90-96` (FU 0l) records this verbatim:

> *"### 0l. `c.MustGet("user_id").(uuid.UUID)` panic surface (58 sites) → `safego` + helper refactor — Status: OPEN — discovered during 2026-05-15 audit team sweep (`docs/audits/BACKEND_CRASH_RISKS.md` §Risk 11). … Why it matters: Defense in depth. Also a prerequisite for ADR-0015 (per-use API-key audit emission) which needs the helper as its call point. … Pattern: new `getUserID(c *gin.Context) (uuid.UUID, bool)` helper that does `c.Get` + comma-ok type assertion + 401 on miss. Replace all 58 sites."*

**Finding 2 — `github.md` Candidate 4 (per-use API-key audit emission), accepted.** Verbatim from `docs/research/competitors/github.md:89-93`:

> *"Candidate 4: Per-use API-key audit emission — Pros: Closes §5 row 4 — today zero middleware-layer trail of API-key actions. Sibling to FU 0 (service-layer audit gaps). Cons (operational): Audit-log volume. Agent polling 1/s → 86,400 rows/day per key. Mitigation: per-key/per-day dedupe. Without dedupe, pruner becomes load-bearing and any tuning bug is a DB-fill incident."*

The two couple: emission lives in `APIKeyAuthMiddleware` (`middleware.go:88-120`); the audit row needs the same `(user_id, api_key_project_id, api_key_id)` triple handlers need. One helper keeps typing single-sourced. `docs/THREAT_MODEL.md` v1.2.0 Findings §1 names the Repudiation gap Head 2 closes — trust boundary unchanged, gap shrinks.

## Options considered

### Option A — Custom Gin panic handler converting `MustGet` panic to 401
- **Pros:** Zero handler edits.
- **Cons:** Panic still unwinds, logs a stack trace; type-assertion panics still produce 500; normalises panic-as-control-flow. Rejected.

### Option B — Code-generation tool maintaining the 58 sites
- **Pros:** Pattern proliferation impossible.
- **Cons:** Generator adds its own surface; pattern won't proliferate once the helper exists. Single-PR find-and-replace is simpler. Rejected.

### Option C — `getUserID` + `getActor` helpers + middleware audit emit (chosen)
- **Pros:** Plain Go; no codegen; matches audit's named recommendation; one place to wire `actor_type` (ADR-0014) at emit.
- **Cons:** Touches 58 files in one PR. Diff is mechanical.

## Decision

Adopt Option C with two heads tracked separately so they roll back independently. Bundling justified by call-site coupling: Head 2 needs Head 1's helper to read actor identity.

**Head 1 — `auth_context.go` helpers.** New file `backend/internal/api/auth_context.go` exporting:

```go
type ActorType string
const ( ActorUser ActorType = "user"; ActorAPIKey ActorType = "api_key" )

type Actor struct {
    Type      ActorType
    UserID    uuid.UUID
    APIKeyID  *uuid.UUID  // nil for JWT-driven
    ProjectID *uuid.UUID  // nil for JWT-driven
}

func getUserID(c *gin.Context) (uuid.UUID, bool)
func getActor(c *gin.Context) (Actor, bool)
```

`getUserID` does `c.Get("user_id")` + comma-ok; returns `(uuid.Nil, false)` on miss or wrong type. **Does not respond directly** — caller responds 401 (OQ-3). `getActor` additionally consults `api_key_project_id` (set at `middleware.go:107-111`). Both pure, table-test exhaustive.

The 58 sites become `userID, ok := getUserID(c); if !ok { RespondError(c, 401, "unauthenticated"); return }`. Middleware guarantees `ok=true` today; the branch is dead code at write time and live insurance against the forward-looking footgun.

**Head 2 — Per-request audit emission in `APIKeyAuthMiddleware`.** `middleware.go:88-120` emits `auth.call` on every successful API-key authentication and `auth.call_rejected` on failure. Uses `getActor` for `actor_type='api_key'`, `actor_id`, `api_key_id`, `project_id`, `target_resource=<method+route template>`, `outcome`. JWT-driven calls **do not** emit `auth.call` — they rely on service-layer `secret.read`/`secret.created` per `docs/AUDIT_LOG_COVERAGE.md`. Volume is asymmetric: agent polling is the high-cardinality stream we need; dashboards aren't.

**Phase A volume policy: always-emit.** Storage bounded by `startAuditLogPruner` (`cmd/server/main.go:196`). The dossier's per-key/per-day dedupe is deferred to Phase B behind per-org config: dedupe weakens the threat-model claim ("every API-key action leaves a trail"); Phase-A visibility outweighs storage cost. OQ-1 forces re-evaluation at first volume signal.

`actor_type` is added to `AuditEntry` by ADR-0014; Head 2 must not ship before ADR-0014. The two are atomic. Head 1 is independent and can ship first.

Type-1: Head 2 touches authentication + audit log; Head 1 alone is Type-3 but bundled by coupling. Security Engineer veto applies on Head 2.

## Rejection rationale

- **Option A** lost: global recovery still produces stack traces in production logs and does nothing for type-assertion panics.
- **Option B** lost: the helper, once in place, eliminates the pattern's reason to re-appear; a generator is tooling for a one-time refactor.

## Consequences

- **Operational.** One audit row per API-key call. At 100 req/s = ~8.6M rows/day (OQ-1). New taxonomy (`auth.call`, `auth.call_rejected`) updates `docs/AUDIT_LOG_COVERAGE.md`. RUNBOOK: alert at 50% audit-table disk; pruner SLA tightened.
- **Security.** Trust boundary unchanged; Repudiation gap shrinks. New surface: `getActor` is canonical actor-identity source — regression is privilege-confusion class. Mitigated by unit tests and 58+ uses (regression breaks the build).
- **Migration.** No schema change here (ADR-0014 lands `actor_type`). 58-site refactor mechanical; existing tests compile unchanged.
- **Reversibility.** Both heads independently revertible.

## Implementation plan

| # | File | Detail |
|---|---|---|
| 1 | `backend/internal/runtime/safego.go` (new) | **Co-owned with ADR-0010 part C and ADR-0011 part 5; this ADR does NOT duplicate the file.** Whichever lands first creates it. Canonical signature `Launch(ctx, name, fn func(context.Context))` owned by ADR-0011 (`docs/adr/0011-graceful-shutdown-and-db-timeouts.md:90`); this ADR consumes. One file, three consumers. |
| 2 | `backend/internal/api/auth_context.go` (new) | `ActorType`, `Actor`, `getUserID`, `getActor`. Pure, package-private. |
| 3 | `backend/internal/api/middleware.go:88-120` | After `apikeyRepo.GetByHashedKey` + expiry check (line 105), `auditRepo.Create(...)` with `auth.call`. On 401 returns at lines 96-98 and 101-104, emit `auth.call_rejected` (`reason='invalid_key'`/`'expired'`). Wrap emit in `safego.Launch`. JWT branch at line 118 not modified. |
| 4 | `backend/internal/api/handlers_*.go` × 58 sites | Find-and-replace: `userID := c.MustGet("user_id").(uuid.UUID)` → `userID, ok := getUserID(c); if !ok { RespondError(c, http.StatusUnauthorized, "unauthenticated"); return }`. Omit-the-check optimisation rejected — defensive 401 is the point. Sites: `grep -rn 'c\.MustGet("user_id")' backend/internal/api/handlers_*.go`; audit names `handlers_mcp_gateway.go:51,165,199`; `handlers_application.go:44,56,130,148,165`; `handlers_intelligence.go:38,211,403,441,458` + 40 further. |
| 5 | `docs/AUDIT_LOG_COVERAGE.md` | Add `auth.call` (`actor_id`, `actor_type`, `api_key_id`, `project_id`, `route`, `method`) and `auth.call_rejected` (`actor_type='api_key'`, `reason ∈ {invalid_key, expired, missing_header}`, `ip`). |
| 6 | `auth_context_test.go` (new) | Table-driven `getUserID`: missing/wrong-type → `(uuid.Nil,false)`; valid → `(uuid,true)`. `getActor`: JWT-only → `Type:user`; API-key → `Type:api_key`; neither → false. |
| 7 | `middleware_audit_test.go` (new) | Integration: API-key request → one `auth.call`; bad key → one `auth.call_rejected`; JWT → no `auth.call`. |
| 8 | `docs/FOLLOWUPS.md` 0l | Mark **CLOSED BY ADR-0015** once Head 1 ships. |

## Rollback plan

Both heads roll back independently. **Confirmed:** neither revert requires the other.

- **Head 1:** revert the find-and-replace; re-introduce `c.MustGet("user_id").(uuid.UUID)`. Single-PR `git revert`. Codebase returns to forward-looking-risk state — functional. Helper may stay or be deleted.
- **Head 2:** remove `auditRepo.Create` from `APIKeyAuthMiddleware`. Existing rows stay (harmless); pruner clears on schedule. Single-PR revert. No schema to undo.
- **Shared `safego.go`:** controlled by the last consumer among ADR-0010 / 0011 / 0015; reverting this ADR's references does not delete the file.

If OQ-1 fires early, Head 2 can be partially rolled back by adding per-key/per-day dedupe without removing the emit.

## Open questions

1. **(Volume — Backend + DevOps. Due: 30d after Head 2 ships.)** `auth.call` produces one row per authenticated API-key request. At 100 req/s = 8.6M rows/day. Is always-emit acceptable for Phase A, or must per-key/per-day dedupe (`docs/research/competitors/github.md:92`) ship in the same PR? Criteria: real sustained QPS in staging, AND cost of dropping per-action granularity. Default: always-emit, revisit at first volume alert.
2. **(Helper signature — Backend + Tech Lead. Due: before merge.)** Narrow `getUserID` (matches the 58 sites) or rich `getActor` (audit emit + future scope checks)? Recommendation: ship both — narrow for handlers, rich for middleware.
3. **(Behavior on miss — Security Engineer. Due: before merge.)** Should `getUserID` return `(uuid.Nil, false)` and let the caller respond 401 (chosen), or terminate via `c.AbortWithStatus(401)`? Chosen keeps helper pure and matches Go idiom; alternative couples helper to response writer. Tie-breaker: Security Engineer prefers the auditable explicit-401 branch.

## References

- `backend/internal/api/middleware.go:88-120` — `APIKeyAuthMiddleware` (Head 2 emit)
- `backend/internal/api/middleware.go:107-111` — context keys set by API-key auth
- `backend/internal/api/handlers_*.go` × 58 sites — `c.MustGet("user_id").(uuid.UUID)` (Head 1)
- `backend/internal/api/router.go:29` — `gin.Recovery()`
- `backend/internal/models/models.go:63-72` — `AuditEntry` (ADR-0014 adds `actor_type`)
- New: `backend/internal/api/auth_context.go`, `auth_context_test.go`, `middleware_audit_test.go`
- New: `backend/internal/runtime/safego.go` — co-owned with ADR-0010 part C, ADR-0011 part 5; signature owned by ADR-0011
- `docs/audits/BACKEND_CRASH_RISKS.md:53` — Risk 11 (verbatim in Context)
- `docs/research/competitors/github.md:89-93` — Candidate 4 (verbatim in Context)
- `docs/FOLLOWUPS.md:90-96` — FU 0l (verbatim in Context)
- `docs/AUDIT_LOG_COVERAGE.md`; `docs/THREAT_MODEL.md` v1.2.0 Findings §1 lines 38-41
- ADR-0010 part C; ADR-0011 part 5; ADR-0014 (atomic with this ADR)
