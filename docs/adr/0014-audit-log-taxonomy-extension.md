# ADR-0014: Audit-log taxonomy extension — `role.changed`, `settings.changed`, `actor_type`

- **Status:** Proposed
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (Backend Engineer role)
- **Reviewers required:** Tech Lead; Security Engineer (Type-1 audit-field addition per `CLAUDE.md` §3.1; veto applies)
- **Supersedes:** none
- **Related:** ADR-0012 (KMS / master-key events — parallel, reserves `master_key.*`), ADR-0013 (webhook events — parallel, reserves `webhook.*`), ADR-0015 (per-use API-key audit emission — consumes `actor_type='api_key'`), `docs/AUDIT_LOG_COVERAGE.md`, `docs/THREAT_MODEL.md` §1 row R, `docs/audits/SECURITY_AUDIT_2026-05-15.md` A09, `docs/research/competitors/doppler.md` §6 Cand. 2, `docs/research/competitors/github.md` §6 Cand. 4.

---

## Context

The canonical taxonomy at `docs/AUDIT_LOG_COVERAGE.md:21-43` is missing two state-mutating events, and `AuditEntry` carries no discriminator for *what kind of actor* drove the mutation. The problem has three heads:

1. **Missing taxonomy entries.** The Doppler dossier §6 Candidate 2 (*adopt-now*) names two mutating handlers that emit nothing today: *"KeepSave is missing `role.changed` (`OrgMember.Role` at `models.go:224` is mutable via `router.go:171` with no audit-coverage entry) and `settings.changed` (SSO config updates via `router.go:175`)."* Both endpoints mutate authorization-relevant rows; the per-PR review rule in `CLAUDE.md` ("every state-mutating handler MUST emit an audit event from the canonical taxonomy") has no entry to point at.

2. **Missing `actor_type` discriminator.** The Doppler analyst noted of `audit_repo.go:21`: *"`Create` takes only `userID *uuid.UUID`; for API-key-driven mutations the actor identity is ambiguous."* Verbatim from `backend/internal/models/models.go:63-72`:

   ```go
   type AuditEntry struct {
       ID          uuid.UUID  `json:"id"`
       UserID      *uuid.UUID `json:"user_id,omitempty"`
       ProjectID   *uuid.UUID `json:"project_id,omitempty"`
       Action      string     `json:"action"`
       Environment string     `json:"environment,omitempty"`
       Details     JSONMap    `json:"details"`
       IPAddress   string     `json:"ip_address"`
       CreatedAt   time.Time  `json:"created_at"`
   }
   ```

   The analyst's claim is confirmed: no `actor_type`. When ADR-0015 wires per-use audit emission on the API-key middleware, every such row will carry the *owning user's* UUID — indistinguishable from a row that user wrote via the dashboard.

3. **Coordination with parallel ADRs.** ADR-0012 (KMS) will add `master_key.*` events; ADR-0013 (webhook) will add `webhook.*` events. Rather than each parallel ADR mutating `AUDIT_LOG_COVERAGE.md` independently, this ADR is the single canonical taxonomy update and **reserves both namespaces** so parallel work plugs into named slots without merge conflicts.

The 2026-05-15 audit reinforces head #1. Verbatim from `docs/audits/SECURITY_AUDIT_2026-05-15.md` §A09-F2: *"Secret/Project/APIKey mutations not audited … Severity here is P2 (Repudiation lever exists; combined with A01-* findings, repudiation IS the cover for unauthorized access)."* The Doppler dossier surfaces *additional* holes A09 did not enumerate; the two findings are complementary.

No trust boundary widens; the change strengthens the Repudiation row at `THREAT_MODEL.md` §1 row R by letting the row express API-key and system actors.

## Options considered

### Option A — Single `actor_type` enum column + named taxonomy slots *(recommended)*

- **How it works:** Add `actor_type TEXT NOT NULL CHECK (actor_type IN ('user','api_key','service_account','system'))` to `audit_log`. Two-phase migration: (1) add nullable column, backfill `'user'`; (2) `NOT NULL` + CHECK. Extend `AuditEntry` struct, extend `AuditRepository.Create` to take `actorType`. Add `role.changed` and `settings.changed` rows to `AUDIT_LOG_COVERAGE.md`; reserve `webhook.*` and `master_key.*` as placeholder rows.
- **Pros:** Single column, single CHECK, indexable. Backfill is safe — every existing row predates API-key audit emission, so `'user'` is correct for 100% of historical rows. Adding a fifth class later is one trivial `ALTER`.
- **Cons:** Touches every `audit_repo.Create` site (~30 in `internal/service/`, ~5 in `internal/api/`); a mechanical sweep PR is needed alongside the migration.

### Option B — Two FK columns (`actor_user_id`, `actor_apikey_id`)

- **How it works:** Add `actor_apikey_id *uuid.UUID` alongside `user_id`. Exactly one is non-NULL per row; both NULL = system.
- **Pros:** Joins are natural; no string enum.
- **Cons:** Doesn't scale — each new actor class needs another FK column (`service_account`, `scheduled_job`, future SPIFFE SVID). Joins become a NULL-coalescing ladder. The Doppler and GitHub dossiers both name the multi-actor future; two columns will not last a year.

### Option C — JSON `actor` blob `{type, id, scopes[]}`

- **How it works:** Replace `user_id` with `actor JSONB`.
- **Pros:** Open-ended.
- **Cons:** Query-by-actor becomes JSONB extraction on the hottest table. `AUDIT_LOG_COVERAGE.md` already flags read-volume as a deferred ADR; JSONB extraction in the hot path is the wrong direction.

## Decision

We pick **Option A**: extend `audit_log` with `actor_type` (enum-constrained), update `AuditEntry` and `AuditRepository.Create`, add `role.changed` / `settings.changed` to `AUDIT_LOG_COVERAGE.md`, and reserve `webhook.*` / `master_key.*` for ADRs 0013 / 0012.

Option A wins because its cost (one column, one CHECK, one signature change applied via mechanical sweep) is strictly less than Option B's cost on every new actor class (a new column, a new join branch) or Option C's cost on every query (JSONB extraction). The enum grows by one-line CHECK migrations — no ADR rewrite.

## Rejection rationale

- **Option B:** the multi-actor future is named in Doppler §6 Cand. 1 (service accounts) and GitHub §6 Cand. 4 (per-use emission); a two-column shape is already wrong on arrival of ADR-0015.
- **Option C:** the `audit_log` table is the hottest table in the system; JSONB extraction on every audit query is a future incident waiting to happen.

## Consequences

- **Operational:** A migration runs across Alpha → UAT → PROD. Phase 1 backfill is O(N) over `audit_log`; phase 2 is metadata-only on PG16. ~35 call sites updated mechanically.
- **Security:** Narrows Repudiation row at `THREAT_MODEL.md` §1 row R. No new trust boundary; the same boundary carries more truth on each row. ADR-0015 is the consumer of `actor_type='api_key'`; this ADR produces the column.
- **Migration:** New column `actor_type`. Existing rows backfilled `'user'` — safe; every historical row predates API-key audit emission (verified: no `audit_repo.Create` in `internal/api/middleware.go` as of 2026-05-15).
- **Reversibility:** **Reversible without data loss** — see §Rollback plan.

## Implementation plan

1. **Migration `009_audit_log_actor_type.sql`** (`backend/migrations/`):
   - `ALTER TABLE audit_log ADD COLUMN actor_type TEXT;`
   - `UPDATE audit_log SET actor_type = 'user' WHERE actor_type IS NULL;`
   - `ALTER TABLE audit_log ALTER COLUMN actor_type SET NOT NULL;`
   - `ALTER TABLE audit_log ADD CONSTRAINT audit_log_actor_type_chk CHECK (actor_type IN ('user','api_key','service_account','system'));`
   - SQLite path (per `repository/dialect.go`): `CREATE TABLE audit_log_new ... INSERT ... DROP ... RENAME` (SQLite cannot `ALTER ... SET NOT NULL` on a populated column).

2. **`backend/internal/models/models.go:63-72`** — add `ActorType string `json:"actor_type"`` to `AuditEntry`.

3. **`backend/internal/repository/audit_repo.go:21`** — `Create` signature gains `actorType string`:
   ```go
   func (r *AuditRepository) Create(userID *uuid.UUID, actorType string, projectID *uuid.UUID, action, environment string, details models.JSONMap, ipAddress string) error
   ```
   The `INSERT` and the `ListByProjectID` SELECT (line 42) gain the new column; `Scan` (line 54) gains `&e.ActorType`.

4. **Call-site sweep.** All callers (~30 in `internal/service/`, e.g. `promotion_service.go:189,257,357,399`) pass `'user'` if JWT-authenticated. `'api_key'` is introduced only at sites ADR-0015 wires up; `'system'` is reserved for cron-driven emission (e.g. ADR-0012's master-key rotation). A helper `auditActorTypeFromContext(c *gin.Context) string` in `internal/api/middleware_helpers.go` reads `api_key_id` / `user_id` from gin context and returns the discriminator.

5. **`docs/AUDIT_LOG_COVERAGE.md`** — add two new rows after `apikey.deleted`:
   - `role.changed` — `org_id`, `target_user_id`, `actor_id`, `old_role`, `new_role`
   - `settings.changed` — `org_id`, `actor_id`, `setting_name`, `changed_fields[]` (never the value of `ClientSecretEncrypted`, per Doppler-reviewer note)
   - Plus reservation lines for `webhook.*` (ADR-0013) and `master_key.*` (ADR-0012).

6. **Tests.** Table-driven:
   - `audit_repo_test.go` — extend each test row to round-trip `ActorType` across all four enum values; add a negative test that `actor_type='invalid'` is rejected by the CHECK constraint.
   - `migrations_test.go` — backfill verification: fixture of 100 pre-existing rows, assert every row ends with `actor_type='user'`, assert subsequent INSERT without `actor_type` fails.
   - Handler tests for `role.changed` and `settings.changed` per `AUDIT_LOG_COVERAGE.md` §"Test obligations".

## Rollback plan

Mechanical and lossless:

1. Revert the call-site sweep commit.
2. Revert `AuditRepository.Create` signature.
3. Revert the `AuditEntry` struct change.
4. Down-migration: `ALTER TABLE audit_log DROP CONSTRAINT audit_log_actor_type_chk; ALTER TABLE audit_log DROP COLUMN actor_type;`
5. Revert the taxonomy and reservation rows in `AUDIT_LOG_COVERAGE.md`.

**No data destroyed.** Every historical row retains `user_id`, `action`, `details`, `ip_address`, `created_at`. The only loss is the actor-class discriminator on rows written between this ADR landing and the rollback; the JWT/api-key distinction returns to pre-ADR state (absent from the row, inferrable from `details` if the call site populated it). True rollback, not forward-fix.

## Open questions

1. **Service-account vs system actor distinction.** Phase B SPIFFE-SVID work (per `BEYOND.md` §2.5) will introduce identities that are neither human users nor `ks_` API keys. Does it fit `service_account` or warrant a fifth value (`spiffe`)? *Owner:* Security Engineer + Tech Lead. *Due:* before ADR-Phase-B opens (90 days).

2. **Index on `actor_type`.** Queries like "every action by API-key actors in 24h" become natural under this ADR. Do we add `CREATE INDEX idx_audit_log_actor_type ON audit_log(actor_type, created_at DESC)` now (cheap; marginal write cost) or defer until a query exists (YAGNI)? *Owner:* Backend Engineer. *Due:* 30 days after ADR-0015 lands; revisit if `actor_type` filters appear in SIEM-export work.

3. **Coordination with ADR-0015.** Does ADR-0015 need to ship in the *same release* as this ADR, or can the taxonomy land first with API-key sites opportunistically passing `actor_type='api_key'` as emission spreads? *Owner:* Tech Lead. *Due:* before ADR-0015 review opens. Default: this ADR lands first; 0015 builds on it.

## References

- `backend/internal/models/models.go:63-72` — `AuditEntry` (quoted in §Context).
- `backend/internal/repository/audit_repo.go:21` — `Create` signature.
- `backend/internal/repository/audit_repo.go:37-60` — `ListByProjectID` (also updated).
- `backend/internal/api/middleware.go:107-112` — gin context keys read by the new helper.
- `backend/internal/api/router.go:171, 175, 177` — `role.changed` / `settings.changed` emission sites.
- `docs/AUDIT_LOG_COVERAGE.md:21-43` — canonical taxonomy (extended).
- `docs/research/competitors/doppler.md` §6 Cand. 2, §10 (*adopt-now*).
- `docs/research/competitors/github.md` §6 Cand. 4, §10.
- `docs/audits/SECURITY_AUDIT_2026-05-15.md` §A09-F2.
- `docs/THREAT_MODEL.md` §1 row R.
- ADR-0012 (parallel, reserves `master_key.*`), ADR-0013 (parallel, reserves `webhook.*`), ADR-0015 (consumes `actor_type='api_key'`).
