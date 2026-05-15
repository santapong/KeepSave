# ADR-0009: Mandatory default expiration on `ks_` API keys

- **Status:** Proposed
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (with Backend Engineer, Security Engineer input)
- **Reviewers required:** Tech Lead; Security Engineer (Type-1 — touches `internal/auth`)
- **Supersedes:** none
- **Related:** ADR-0002 (auth model — this ADR extends the API-key half), `docs/THREAT_MODEL.md` §2 row T (line 84), `docs/FOLLOWUPS.md` 0k, `docs/research/competitors/github.md` Cand. 3, `docs/research/competitors/spiffe.md` §5, `docs/research/competitors/vault.md` Cand. 2, `docs/audits/SECURITY_AUDIT_2026-05-15.md` A07-F1

---

## Context

Every `ks_` API key minted today is valid indefinitely. The `APIKey.ExpiresAt *time.Time` column exists (`backend/internal/models/models.go:59`) and the middleware enforces it correctly when non-NULL (`backend/internal/api/middleware.go:101-105`). The issuance path is the gap:

- `backend/internal/api/validation.go:33-38` — `CreateAPIKeyRequest` exposes no `ExpiresAt`.
- `backend/internal/service/apikey_service.go:33-60` — `Create(...)` takes no expiry argument; line 51 calls the repo without one.
- `backend/internal/repository/apikey_repo.go:33-36` — INSERT omits `expires_at` → row stored NULL → middleware treats as no expiry.

A leaked key works forever unless an operator manually deletes it. Below segment baseline: GitHub PAT mandates ≤366d; SPIFFE JWT-SVID recommends ≤15min for workload identity; CircleCI January 2023 is the canonical real-world cost.

`docs/FOLLOWUPS.md` 0k captures the gap:

> *"### 0k. Default expiration on `ks_` API keys (no immortal credentials) — Status: OPEN — discovered during competitor synthesis (Vault / SPIFFE / GitHub dossiers converge) ... every `ks_` key minted is valid indefinitely until manual deletion ... Due: 30 days (Phase A). Pattern: GitHub fine-grained PAT mandatory expiration; default 90d, ceiling 365d ..."*

`docs/audits/SECURITY_AUDIT_2026-05-15.md` A07-F1 independently re-derived and escalated the finding:

> *"Security-class severity: P3 Medium standalone (a leaked agent key works forever). When combined with A01-F2 (API key scope not enforced), the chain becomes P2 — a leaked key for any project P1 reads every project, forever. Recommendation: rate this gap as P2 in combination, not as a separate operational concern."*

This ADR is the **Phase-A bridge** to the SPIFFE-shaped short-lived agent token path (`docs/research/competitors/spiffe.md` §10, Phase B). Phase A bounds leak windows from forever to ≤90 days by default, ≤365 days max. Phase B compresses to ≤15 minutes when triggers fire. The two designs compose at the already-wired `middleware.go:101-105` `ExpiresAt` check.

## Options considered

### Option A — Mandatory default expiration with 90d default, 365d ceiling *(recommended)*

`CreateAPIKeyRequest` accepts an optional `expires_at`. Service defaults to `now + 90d` when omitted. Maximum 365d (matches GitHub PAT ceiling). UI presents a 30 / 90 / 180 / 365-day picker. Existing NULL `expires_at` rows are grandfathered (middleware short-circuits NULL to "not expired") and flagged in the UI as "legacy — please rotate."

- **Pros:** Caps leak window at a meaningful boundary without surprising existing customers. Lights up an already-tested middleware path. Single-PR scope. Composes with the future SPIFFE Phase-B ADR.
- **Cons:** Customer CI integrations that mint-once must adopt the renewal endpoint or accept periodic rotation. 365d ceiling is high vs NIST SP 800-63B short-credential guidance but matches segment baseline.

### Option B — Mandatory hard cap of 90 days, no opt-out

Every new key expires at exactly `now + 90d`. No customer escape hatch.

- **Pros:** Tightest leak window without an explicit service-token feature.
- **Cons:** Operational shock for existing customers on long-lived keys. No safety valve for the rare audited "indefinite key" use case (addressed out-of-band via a separate service-token ADR). Rejected.

### Option C — Optional `expires_at` with UI default of 90 days

Field is optional; UI defaults to 90d; direct API callers can still omit it (NULL stored).

- **Pros:** Smallest behavioural change.
- **Cons:** Audit shows ~100% of existing keys are NULL. Voluntary defaults have zero adoption — operators paste from prior runbooks and omit the field. Status-quo dressed up. Rejected.

## Decision

**Option A — mandatory default expiration with 90-day default and 365-day ceiling.** The service enforces a non-NULL `ExpiresAt` on every new key: clients may submit a value (validated `> now` and `≤ now + 365d`) or omit it (defaulted to `now + 90d`). NULL `expires_at` becomes unreachable on the issuance path; existing NULL rows are grandfathered and flagged in the UI. Middleware is unchanged — it already enforces non-NULL expiry.

Option A won over B because operational shock without an audited escape hatch forces customers into worse workarounds (daily-rotation scripts multiply leak surface). It won over C because audit evidence shows voluntary defaults do not adopt.

**Out-of-scope:** an unrotatable "service token" path. Operators needing an audited indefinite key get that via a separate service-token ADR. This ADR does **not** accept `9999-12-31` or NULL on new rows.

## Rejection rationale

- **Option B** lost because removing the safety valve forces customers into rapid-rotation scripts that increase leak surface.
- **Option C** lost because audit evidence is unambiguous: voluntary defaults are not adopted. The point of a default is the floor it creates; making it optional removes the floor.

## Consequences

- **Operational:** Customers integrating post-ADR implement either rotation or the renewal call. Release notes call out the 90d default + renewal endpoint. Runbook gains an "extend an API key" entry.
- **Security:** Trust boundary narrows along the time axis. Leaked-key validity falls from forever to ≤365d (90d by default). Closes the API-key half of `docs/THREAT_MODEL.md` §2 row T. Combined with A01-F2 (separate fix on a faster clock), leak blast radius collapses.
- **Migration:** None mandatory. Column already exists. Existing NULL rows untouched (see Open Question (a)).
- **Reversibility:** Single-PR revert. Keys minted with `expires_at` continue to expire (operators wanted that). Reversible **without** data migration.

## Rollback plan

Revert the validation enforcement and the service-layer default. The `expires_at` column stays harmlessly populated on keys minted during the ADR-active window. Keys with `expires_at` set continue to expire as scheduled. Single-PR revert; **no data migration required**.

If a customer-blocking issue surfaces post-rollback (e.g., the 90-day default broke a known integration), new keys revert to indefinite, in-flight keys honour their committed expiry. Forward fix: re-land with a longer default after re-scoping the customer contract.

## Implementation

Touchpoints, in dependency order:

1. **`backend/internal/api/validation.go:33-38`** — add `ExpiresAt *time.Time` to `CreateAPIKeyRequest` with a custom validator that rejects `≤ time.Now()` or `> time.Now().Add(365 * 24 * time.Hour)`.
2. **`backend/internal/service/apikey_service.go:33-60`** — extend `Create` to accept `expiresAt *time.Time`. When nil, default to `time.Now().Add(90 * 24 * time.Hour)`. When non-nil, re-check the ceiling (defence in depth).
3. **`backend/internal/repository/apikey_repo.go:33-36`** — add `expires_at` to the INSERT column list + parameter binding (both PG and SQLite branches). SELECT already reads it.
4. **Migration** — none required. Column exists and is nullable; we tighten the app layer only. Verified by reading `backend/migrations/001_initial_schema.sql` and dialect copies (`migrations/postgres`, `migrations/sqlite`). A future ADR can add a DB-level `NOT NULL` after legacy NULLs are rotated.
5. **New handler `backend/internal/api/handlers_apikey_extend.go`** — `POST /api/v1/api-keys/:id/extend` accepts `{ "additional_seconds": int }`. Rule: `new_expires_at = min(current_expires_at + delta, original_created_at + 365d)`. Owner-only.
6. **Audit taxonomy** — extend `docs/AUDIT_LOG_COVERAGE.md` with two events:
   - `apikey.expired` — emitted by a background sweeper when `expires_at` passes; metadata `key_id`, `project_id`, `owner_id`.
   - `apikey.extended` — emitted from the extend handler; metadata `key_id`, `project_id`, `actor_id`, `previous_expires_at`, `new_expires_at`.
   - `apikey.created` continues, now always with non-NULL `expires_at`.
7. **UI** — add a 30/90/180/365-day picker. **Caveat:** `docs/audits/FRONTEND_DEAD_UI.md` item #15 flags `frontend/src/pages/APIKeysPage.tsx` as orphaned (declared, never imported; no `/api-keys` route). The live surface is `ProjectAPIKeysPanel`. Scope adjustment: picker lands in `ProjectAPIKeysPanel`; the orphaned page is a separate cleanup.
8. **Tests:** validator table-driven tests (`≤ now`, `> now+365d`, omitted, exact boundary); service-layer test for default-on-nil and ceiling enforcement; handler test asserting `apikey.created` carries the expected `expires_at`; handler test for `apikey.extended`; middleware regression that an expired key returns 401.

## Open questions

1. **Grandfathering existing NULL-expiry rows.** Leave them untouched, or run a one-time migration to set `now + 90d`? Untouched is safer (no silent breakage of existing CI integrations) but leaves a tail of immortal keys until operators rotate. Migration risk: a forgotten CI key fails at migration-timestamp + 90d as a surprise outage. **Recommendation:** untouched, plus a UI "legacy — please rotate" badge and a Phase-A follow-up to nudge rotation. *Owner: Security Engineer. Due: ADR sign-off.*
2. **Pre-expiry notification subsystem prerequisite.** 14-day and 1-day pre-expiry emails depend on a notification subsystem that may not exist. If absent, this ADR creates a dependency on a new notification ADR (Type-2). **Recommendation:** ship expiration semantics now; gate notifications on the notification subsystem's arrival. *Owner: Backend Engineer + PM. Due: prior to merge.*
3. **Service-token escape hatch placeholder.** Audited indefinite-key operators are out-of-scope here. Add a placeholder FU so the need is not lost when a customer surfaces it? **Recommendation:** add `docs/FOLLOWUPS.md` Phase B entry "Audited service-token path (indefinite, restricted)" — trigger: "first customer with a documented need for an unrotatable key." *Owner: Tech Lead. Due: same PR as this ADR.*

## References

- ADR-0002 — this ADR extends the API-key half of the auth model.
- `backend/internal/api/validation.go:33-38`, `service/apikey_service.go:33-60`, `repository/apikey_repo.go:33-36` — issuance-path touchpoints.
- `backend/internal/api/middleware.go:101-105`, `models/models.go:59` — already-correct expiry check + field.
- `docs/THREAT_MODEL.md` §2 row T (line 84); `docs/FOLLOWUPS.md` 0k (lines 83-88); `docs/AUDIT_LOG_COVERAGE.md`.
- `docs/audits/SECURITY_AUDIT_2026-05-15.md` A07-F1; `docs/audits/FRONTEND_DEAD_UI.md` item #15.
- `docs/research/competitors/github.md` Cand. 3; `docs/research/competitors/vault.md` Cand. 2; `docs/research/competitors/spiffe.md` §5, §10.
- NIST SP 800-63B §4.2-4.3; CircleCI January 2023 incident report; GitHub fine-grained PAT GA (2022-10-18).
