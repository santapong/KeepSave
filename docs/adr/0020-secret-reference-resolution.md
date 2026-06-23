# ADR-0020: Secret-reference resolution at read time

- **Status:** Accepted
- **Date:** 2026-06-21
- **Authors:** Tech Lead
- **Reviewers required:** Tech Lead; Security Engineer (read path / data exposure)
- **Supersedes:** none
- **Related:** `dependency_service.go` (reference detection), `docs/THREAT_MODEL.md` §2, ADR-0003 (promotion), `docs/research/IMPROVEMENT_PLAN_2026-06.md` §4 (roadmap drift O2)

---

## Context

`Roadmap.md` Phase 12 claims "secret references & interpolation resolved at read
time," but only *detection* exists (`dependency_service.findReferences` records a
dependency graph; nothing substitutes values). Agents commonly compose secrets —
`DATABASE_URL=postgres://${DB_USER}:${DB_PASS}@host/db` — and expect the resolved
value at read time. This closes the roadmap drift by actually resolving.

Constraints:
- Resolution must **terminate**: cyclic (`A=${B}`, `B=${A}`) or very deep chains
  must not loop.
- It must **not widen a trust boundary**: resolution is within a single
  environment only (no cross-env reads), and it must **never** run in the
  promotion/diff path, which hashes/copies the stored (raw) ciphertext — resolving
  there could leak a referenced value across an environment boundary.
- It must be **non-breaking**: existing consumers that store literal `${...}`
  values must keep getting raw values unless they opt in.

## Options considered

### Option A — Opt-in read-time resolution within an environment (chosen)
`?resolve=true` on the secret list path interpolates the four existing reference
forms (`${VAR}`, `$VAR`, `{{VAR}}`, `%VAR%`) against the same environment's keys,
transitively, with a depth cap (16) and cycle detection; unknown/cyclic/too-deep
tokens are left literal (never errors). Raw `List` is unchanged.

- **Pros:** Non-breaking; terminates; same-env scope keeps the trust boundary;
  one server-side implementation serves the CLI/SDKs/agents uniformly; reuses the
  existing `referencePatterns`/`findReferences`.
- **Cons:** Resolution is O(keys × refs) per read (env-sized, small).

### Option B — Resolve at write time (store the resolved value)
- **Cons:** Destroys the reference, so updating the referent doesn't propagate;
  and a rotated `DB_PASS` would leave stale composed values. Rejected.

### Option C — Client-side only (each SDK resolves)
- **Cons:** Every SDK/integration reimplements resolution with subtly different
  cycle/precedence rules; inconsistent and error-prone. Rejected.

## Decision

**Option A.** A pure resolver (`ResolveEnvReferences`) over an environment's
key→value map, exposed via `SecretService.ListResolved` and gated behind
`?resolve=true`. The promotion/diff engine does not call it.

## Consequences

- **Security:** New resolution surface is the read path only; same-env scope
  prevents cross-environment leakage; the promotion/diff path stays on raw
  ciphertext. `docs/THREAT_MODEL.md` §2 updated.
- **Behaviour:** Default reads are unchanged (raw). `?resolve=true` callers get
  interpolated values; unresolved tokens (unknown key, cycle, depth>16) are
  returned literally.
- **Migration:** none (additive, opt-in).
- **Reversibility:** Drop the query param + `ListResolved`; raw `List` is untouched.

## Rollback plan
Remove the `?resolve=true` branch and `ListResolved`. No data change; raw values
were never altered at rest.

## Open questions
- **Per-key resolution on single GET** (`GetByID`) is not wired (it would need to
  load the whole env). Add if a use case appears. *Owner: Backend. Due: on demand.*
- **`keepsave run` / SDK** should pass `?resolve=true` by default for agent
  ergonomics. *Owner: Backend. Due: with SDK work.*

## References
- `backend/internal/service/dependency_service.go` (ResolveEnvReferences, resolveOne, substituteRefs)
- `backend/internal/service/secret_service.go` (ListResolved)
- `backend/internal/api/handlers_secret.go` (List, ?resolve)
