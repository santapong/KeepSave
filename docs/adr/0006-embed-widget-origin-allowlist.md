# ADR-0006: Embed widget origin allow-list (server-side, per-project)

- **Status:** Accepted (sponsor-authorized 2026-05-18; implementation landed in PR #50, this PR adds CORS test coverage; retroactive Security/Tech Lead sign-off pending per CLAUDE.md §Type-1)
- **Date:** 2026-05-15
- **Authors:** ADR Drafter
- **Reviewers required:** Tech Lead; Security Engineer (auth-adjacent — veto applies per `docs/ROLES.md`)
- **Supersedes:** none
- **Related:** ADR-0002; `docs/EMBED_ORIGIN_POLICY.md`; `docs/THREAT_MODEL.md` §4 row S; `docs/research/competitors/pomerium.md`; FU 0b

---

## Context

The embed widget (`<keepsave-widget>`) is the second auth surface in KeepSave. Today it accepts an auth payload via `postMessage` from any window in the same browser: the inbound listener at `frontend/src/embed/auth.ts:21-26` calls `onAuth(...)` without reading `event.origin`, and the outbound auth-request at `frontend/src/embed/auth.ts:33` uses `'*'` as target. There is no server-side notion of "which origins may embed this project".

The 2026-05-15 audit confirms this is exploitable. From `docs/audits/SECURITY_AUDIT_2026-05-15.md` §A04-F2:

> *"**Exploitability confirmed: Yes.** The listener accepts `keepsave-auth` messages from any source (`ev.origin` not read at line 21-26). A malicious host page hosting the widget can `window.frames[0].postMessage({type:'keepsave-auth', token: attackerToken}, '*')` from itself, the widget will accept it, and all subsequent widget API calls run with the attacker's token. The classic confused-deputy: the legitimate user's UI shows the attacker's secrets."*

This matches `docs/THREAT_MODEL.md` §4 row S (line 106):

> *"S | Malicious host page injects fake `keepsave-auth` postMessage | **None today** — listener has no origin check | `frontend/src/embed/auth.ts:21-26` | **High**"*

`docs/EMBED_ORIGIN_POLICY.md:42-60` specifies the target shape; `:104-105` defers per-message MAC to Phase B. FU 0b owns 30-day delivery. The open question — *where the allow-list lives and how the widget retrieves it* — adds a new public route and a `projects` column, is auth-adjacent, and is therefore Type-1.

## Options considered

### Option A — Server-side per-project allow-list, fetched by widget at boot

Add `allowed_origins TEXT[]` and `embed_policy_enabled BOOLEAN DEFAULT FALSE` to `projects`. Expose `GET /api/v1/embed-config/:project_id` (unauthenticated, rate-limited) returning `{ allowed_origins, policy_enabled }`. Widget fetches this *before* installing the `message` listener; inbound is accepted only on strict `event.origin` equality with an allow-list entry; outbound targets the specific origin. No "allow all" sentinel — operators who want no enforcement set `policy_enabled: false` explicitly. Mirrors Pomerium dossier Candidates 1 and 2.

- **Pros:** Source of truth lives in the project row, mutated via the authenticated projects API, inheriting FU 0 audit coverage. Strict-equality closes STRIDE-S outright. Trust boundary narrows; no new entity enters it.
- **Cons:** Adds one unauthenticated route — a project-ID enumeration oracle if unprotected. Mitigation: per-IP rate limit and identical 404 for "unknown project" vs. "policy disabled". One extra round-trip at boot. Operators must populate `allowed_origins`; until they do, postMessage-mode does not authenticate.

### Option B — Client-only allow-list (HTML attribute)

`<keepsave-widget allowed-origins="https://app.example.com">`. No backend change.

- **Pros:** Trivial.
- **Cons:** Defeats the threat model. The attacker controls the host page DOM by hypothesis and would set the attribute to their own origin. Client declarations are not anchored in the relevant trust boundary.

### Option C — postMessage → server ticket handshake

Host page mints a short-lived server-side ticket via its own session; widget receives the ticket via postMessage and exchanges it server-side. An attacker on a malicious origin cannot mint a ticket.

- **Pros:** Strongest; survives an XSS on an allow-listed origin (which Option A does not).
- **Cons:** Substantially larger refactor — new endpoint, new ticket model, integrator-side code changes. Overlaps with the deferred Phase B per-message MAC work. Out of scope for the 30-day FU 0b SLA.

## Decision

**Adopt Option A.** Server-side per-project origin allow-list, retrieved by the widget at boot from an unauthenticated, rate-limited `GET /api/v1/embed-config/:project_id`, enforced via strict `event.origin` equality on inbound and non-wildcard `targetOrigin` outbound.

It wins because it (1) closes the audit's confirmed-exploitable finding and the STRIDE-S row outright with bounded scope; (2) lives where ownership lives, so writes flow through the audited projects API; (3) matches a pattern with public audit precedent (Pomerium per-route allow-list, Cure53 2021).

**Conditional trigger:** Phase A only. The trigger for a successor ADR is `docs/EMBED_ORIGIN_POLICY.md:104-105` — "*Revisit if a customer with a high-threat model asks.*" When fired, Option C-shape work begins; the allow-list remains as defence-in-depth.

## Rejection rationale

- **Option B:** the attacker controls the host page DOM by hypothesis; a client-only declaration is not a security boundary against an attacker who can edit the boundary.
- **Option C** *(Phase A only)*: overlaps with the deferred Phase B per-message MAC work; bundling them risks slipping the 30-day FU 0b SLA.

## Consequences

- **Operational:** One additive column-pair on `projects`; one new unauthenticated route; one project-service method; one new handler file. New support surface: operators populate `allowed_origins` per project. Runbook entry: "Widget not authenticating → check `allowed_origins` and `embed_policy_enabled`."
- **Security:** The new unauthenticated endpoint must be rate-limited and return identical 404 for "unknown project" and "policy disabled / empty list" to avoid enumeration. THREAT_MODEL §4 row S residual moves **High → Low** on completion; add a new §1 row for the embed-config endpoint as enumeration-risk surface mitigated by rate-limit + identical 404.
- **Migration:** `ALTER TABLE projects ADD COLUMN allowed_origins TEXT[] NOT NULL DEFAULT '{}'; ADD COLUMN embed_policy_enabled BOOLEAN NOT NULL DEFAULT FALSE;` — additive, zero-downtime, no backfill.
- **Backwards compatibility:** Direct-credential embeds (`token=` / `api-key=`) are unaffected — that branch returns before `setupPostMessageAuth` at `frontend/src/embed/keepsave-widget.ts:78-87`. PostMessage-mode integrators need their origin added and `embed_policy_enabled=true` before the widget authenticates. Operators who don't populate get a 404 and a clear console error. No silent "allow all" fallback.
- **Reversibility:** Reversible without data migration. See Rollback.

## Implementation plan

1. **Migration** `backend/migrations/009_embed_origin_allowlist.sql` (new): the two ALTERs above.
2. **Model** `backend/internal/models/models.go` `Project`: add `AllowedOrigins []string`, `EmbedPolicyEnabled bool` with json/db tags.
3. **Repository** `backend/internal/repository/project_repo.go`: extend INSERT/UPDATE/SELECT lists; add `GetEmbedConfig(projectID)` returning identical `sql.ErrNoRows` for unknown-project and disabled-policy cases.
4. **Service** `backend/internal/service/project_service.go`: new method `GetEmbedConfig(projectID)` wrapping the repo; generic "not found" for both error paths.
5. **Handler** `backend/internal/api/handlers_embed.go` (new): `EmbedHandler.GetConfig` at `GET /api/v1/embed-config/:project_id`, unauthenticated, rate-limited stricter than the global bucket in `ratelimit.go` (e.g., 10 req/min/IP). Identical 404 body for both error paths. Cf. `backend/internal/api/handlers_oauth.go:216-238` for the existing unauthenticated public-config-endpoint pattern; no embed-config endpoint exists today.
6. **Router** `backend/internal/api/router.go`: mount outside `JWTAuthMiddleware` / `APIKeyAuthMiddleware` with the dedicated rate-limit middleware.
7. **Frontend boot** `frontend/src/embed/keepsave-widget.ts:101-117` (`setupPostMessageAuth`): before `createAuthHandshake`, `fetch` the embed config; on 404 or `policy_enabled === false`, render an error via `WidgetRenderer` and do not install the listener.
8. **Frontend inbound** `frontend/src/embed/auth.ts:21-26`: pass `allowedOrigins: Set<string>` into `createAuthHandshake`; drop messages whose `event.origin` is not in the set. Strict equality only — no `startsWith`, no regex, lint-enforced per `docs/EMBED_ORIGIN_POLICY.md:62`.
9. **Frontend outbound** `frontend/src/embed/auth.ts:33`: replace `'*'` with the allow-listed origin. Never `'*'`.
10. **Tests** `frontend/tests/embed/origin.spec.ts` (new), four required cases: (a) allow-listed origin succeeds; (b) non-allow-listed origin rejected silently; (c) missing/disabled config aborts boot with console error; (d) literal `"*"` in `allowed_origins` is **rejected** at config-load time (footgun guard).
11. **Tests** `backend/internal/api/handlers_embed_test.go` (new): unknown project ID returns identical body to disabled-policy project; rate limit kicks in past threshold.
12. **Threat model** `docs/THREAT_MODEL.md` §4 row S: residual → **Low**; add new §1 row for the embed-config endpoint.

## Rollback plan

Single revert. Reversible without data migration.

1. Revert the widget commits (steps 7-9). The widget returns to today's behaviour (no `ev.origin` check, wildcard outbound); FU 0b re-opens at the severity it has today.
2. Leave `allowed_origins` and `embed_policy_enabled` columns in place — additive migration; `DROP COLUMN` is unsafe under zero-downtime policy and unnecessary. Operator-populated data sits dormant until forward-fix.
3. Leave the route mounted but feature-flag it to return 404, or remove it from the router. Either way, column data is unread.

Forward-fix is preferred: if the policy is over-restrictive for a customer, add their origin via the projects API. True rollback to "as if the ADR never landed" (dropping the columns) is impossible under the zero-downtime policy, but behavioural rollback is total.

## Open questions

1. **Project-ID enumeration mitigation** *(owner: Security Engineer, due: pre-merge).* `docs/EMBED_ORIGIN_POLICY.md:48` flags the concern without numbers. Proposal: 10 req/min/IP with 5-minute lockout on bucket exhaustion; identical 404 body for "unknown project" and "policy disabled / empty list". Confirm threshold; confirm identical-404 is sufficient (vs. e.g. 401 + WWW-Authenticate misdirection).
2. **Sentinel handling for "no enforcement"** *(owner: Security Engineer, due: pre-merge).* Pomerium dossier Security Reviewer (`pomerium.md:194`) flagged `["*"]` meaning "policy disabled" as footgun-shaped. This ADR uses explicit `embed_policy_enabled: bool` instead, and the widget *rejects* literal `"*"` in `allowed_origins` at config-load time. Confirm shape; confirm there's no third state missing (current: "list empty + policy enabled" = deny all, widget renders error).
3. **Phase B per-message MAC interaction with the allow-list** *(owner: Security Engineer + Tech Lead, due: when MAC trigger fires).* When Option C / per-message MAC ships per the `docs/EMBED_ORIGIN_POLICY.md:104-105` trigger, does the allow-list remain as defence-in-depth or is it superseded? Intent: it remains — Pomerium combines per-route policy *and* signed assertions, and an XSS on an allow-listed origin is exactly where the MAC defends and the allow-list does not (Pomerium dossier §7 Candidate 3).

## References

- `frontend/src/embed/auth.ts:21-26` (no origin check), `:33` (wildcard outbound)
- `frontend/src/embed/keepsave-widget.ts:78-87` (direct-credential branch, unaffected), `:101-117` (`setupPostMessageAuth`)
- `backend/internal/api/handlers_oauth.go:216-238` — precedent for unauthenticated public-config endpoint; no embed-config endpoint today
- `backend/internal/models/models.go`, `repository/project_repo.go`, `service/project_service.go`; `backend/migrations/` (next is `009`)
- `docs/EMBED_ORIGIN_POLICY.md:42-60`, `:62`, `:104-105`
- `docs/audits/SECURITY_AUDIT_2026-05-15.md` §A04-F2
- `docs/THREAT_MODEL.md` §4 row S (line 106)
- `docs/research/competitors/pomerium.md` Candidates 1 and 2; Security Reviewer notes `:194-198`
- `docs/FOLLOWUPS.md` 0b
- HTML Living Standard §`window.postMessage`; OWASP HTML5 Security Cheat Sheet
