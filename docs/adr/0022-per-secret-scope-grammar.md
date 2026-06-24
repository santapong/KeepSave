# ADR-0022: Per-secret / per-action API-key scope grammar

- **Status:** Accepted
- **Date:** 2026-06-23
- **Authors:** Tech Lead
- **Reviewers required:** Security Engineer (auth / least-privilege — veto); Tech Lead
- **Supersedes:** none
- **Related:** ADR-0002 (auth model), ADR-0005 (RequireProjectAccess), ADR-0021 (agent tokens), `api/middleware.go` (EnforceAPIKeyScope), `docs/research/IMPROVEMENT_PLAN_2026-06.md` (O3)

---

## Context

API-key scopes today are a flat action vocabulary: `read`, `write`, `delete`,
`promote` (`api/middleware.go` `scopeForMethod`/`apiKeyHasScope`). A key scoped
`["read"]` can read **every** secret in the project; there is no way to issue a
key that may read only `DATABASE_*` or write only `APP_CONFIG`. Agents and CI
jobs routinely need exactly one or two secrets, so the coarse model forces
over-provisioning — the opposite of least privilege, and the larger the blast
radius when such a key leaks.

We want a scope grammar that expresses **action × key-set** while staying
**100% backward compatible** with the existing `["read","write",...]` keys and
requiring **no schema migration** (scopes are already stored as a JSON string
array in `api_keys.scopes`).

Constraints:
- A bare legacy scope (`read`) must keep meaning "this action on all keys."
- JWT/user callers (no `api_key_scopes` in context) must remain unaffected.
- Enforcement must be correct where the key is actually known: secrets are
  addressed by `:secretId` (a UUID), not by key, so per-key checks land in the
  handler (which loads the row's `Key`) for read/update/delete, in the handler
  for create (the body carries `key`), and as a **filter** for list.

## Options considered

### Option A — `action[:keyGlob]` grammar, enforced action-coarse in middleware + per-key in handlers (chosen)
A scope is `<action>` or `<action>:<keyGlob>`, where `keyGlob` is a simple glob
(`*` = any run of characters), e.g. `read:DB_*`, `write:APP_CONFIG`, `read:*`.
A bare `read` parses to action=`read`, glob=`` ⇒ all keys (legacy semantics).
- The middleware **action gate** is unchanged in spirit: it passes if *any*
  granted scope grants the request's action (parsing the action prefix). This
  fails fast and keeps env-locking.
- The **per-key gate** is applied where the key is known: `Create` (body `key`),
  `Get/Update/Delete` (the loaded row's `Key`), and `List` (results filtered to
  permitted keys). The `write`⇒`delete` implication carries the key glob.
- **Pros:** least-privilege per secret; zero migration; fully back-compatible;
  one matcher reused at both layers; list returns only visible keys (no 403 storm).
- **Cons:** per-key enforcement is spread across handlers (not one middleware
  line) because the key is not in the path for `:secretId` routes.

### Option B — Resource-prefixed grammar `secret:read:KEY`, RBAC-style
- **Cons:** more verbose, introduces a resource dimension we don't need yet
  (only secrets are key-addressable), and still can't be enforced in middleware
  for `:secretId` routes. Rejected as over-built for O3.

### Option C — New `scope_rules` table (structured rows)
- **Cons:** schema migration + join on the hot auth path for a value that fits
  in the existing JSON array. Rejected.

## Decision

**Option A.** Grammar `action[:keyGlob]` in the existing `scopes` array.

- `parseScope("read:DB_*") → {action:"read", keyGlob:"DB_*"}`; bare `read` →
  glob `""` (= match all).
- `apiKeyHasScope(scopes, action)` (middleware action gate) now compares the
  **action prefix** of each scope, preserving `write`⇒`delete`.
- `apiKeyScopeAllowsKey(scopes, action, key)` returns true iff some scope grants
  the action (with the implication) **and** its glob matches `key`; an empty
  glob matches every key (legacy). Exposed to handlers as
  `APIKeyScopeAllowsKey(c, action, key)`, which is a **no-op (allow)** when the
  caller is not API-key auth — JWT users are unaffected.
- Handlers enforce per-key: `Create` rejects a `key` outside scope (403);
  `Get` returns 404 for an out-of-scope key (anti-enumeration, matching the
  existing not-found behaviour); `Update`/`Delete` pre-resolve the row's `Key`
  and reject out-of-scope (404); `List` filters to permitted keys.

## Consequences

- **Security (positive):** keys can be minted with secret-level least privilege;
  a leaked `read:DB_*` key cannot read `STRIPE_KEY`. `docs/THREAT_MODEL.md` §2
  (API-key scope escalation) updated.
- **Backward compatibility:** bare scopes behave exactly as before; no migration;
  existing keys and tests unchanged.
- **Behaviour:** `List` now returns a *subset* for key-scoped callers — a visible,
  intentional consequence of least privilege (documented in the API surface).
- **Performance:** matching is O(scopes) string work on already-loaded data; no
  new query except the `Update`/`Delete` pre-resolve (one indexed GetByID).
- **Reversibility:** remove the per-key handler checks and the glob branch of the
  matcher; bare-scope behaviour is the matcher's identity case.

## Rollback plan
Stop issuing grammar scopes and remove the `APIKeyScopeAllowsKey` handler calls;
the matcher's empty-glob path is exactly the legacy behaviour, so coarse scopes
keep working with no data change.

## Open questions
- **Versions sub-resource** (`/:secretId/versions`) currently inherits the read
  action gate but is not per-key filtered (it is read-only and already
  project-scoped). Wire `APIKeyScopeAllowsKey` there if a use case appears.
  *Owner: Backend. Due: on demand.*
- **Glob richness** (character classes, alternation) — intentionally omitted;
  `*` runs cover the known cases. *Owner: Backend. Due: on demand.*

## References
- `backend/internal/api/middleware.go` (parseScope, apiKeyHasScope, apiKeyScopeAllowsKey, APIKeyScopeAllowsKey, EnforceAPIKeyScope)
- `backend/internal/api/handlers_secret.go` (Create/List/Get/Update/Delete per-key enforcement)
- `backend/internal/api/scope_grammar_test.go`
