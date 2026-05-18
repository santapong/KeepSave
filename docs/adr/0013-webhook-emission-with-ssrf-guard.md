# ADR-0013: Webhook emission with atomic SSRF guard, body-buffered retries, and per-org signing-secret rotation

- **Status:** Accepted (sponsor-authorized 2026-05-18; retroactive Security/Tech Lead sign-off pending per CLAUDE.md §Type-1)
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (Backend Engineer role)
- **Reviewers required:** Tech Lead; Security Engineer (mandatory veto — Type-1: outbound HTTP, SSRF surface, HMAC integrity boundary, per `docs/ROLES.md` §2.2/§3.1)
- **Supersedes:** none
- **Related:** ADR-0001, ADR-0004 (key hierarchy — reused for signing-secret wrap), ADR-0005 (`RequireProjectAccess`), ADR-0008 (rotation cadence shape), `docs/THREAT_MODEL.md` §1 row I (commit `4286d0a`), audit A10-F1 + A01-F10, SPOF #6, infisical.md §10 Cand. 1

---

## Context

The Infisical dossier (`docs/research/competitors/infisical.md` §10) ranks webhook emission `adopt-now, blocked-pending-SSRF-guard`. Plumbing exists (`backend/internal/service/webhook_service.go:91-127` `Notify` + `shouldDeliver`; HMAC at `:146-149`) but `Notify` has zero production callers — every state-mutating handler (Secret, Project, Promotion, APIKey) is missing the emission. Three converging defects make wiring it unsafe today:

**Finding 1 — A10-F1, P2 High** (verbatim from `docs/audits/SECURITY_AUDIT_2026-05-15.md:318-331`):

> *"A10-F1 — Webhook URL allows internal targets (P2 High). Evidence: `webhook_service.go:136, 158`. `http.NewRequest(\"POST\", config.URL, ...)` with no URL allowlist. Repro: 1. Attacker authenticates (or compromises a key for) project P. Registers a webhook with `url: \"http://169.254.169.254/latest/meta-data/iam/security-credentials/<role>\"` (AWS IMDS). 2. Triggers any event (e.g., `promotion_completed` for P). 3. The KeepSave API container makes the outbound request. Response status code is recorded (`webhook_service.go:185`); response body is **not** persisted, but the **request itself** can be a write or trigger (e.g., target an internal admin endpoint). Severity: P2 High because: the webhook target is invoked from the KeepSave container's network position. Response body is not echoed back, which limits the exfiltration variant, but cloud metadata APIs are query-only (no body needed; the *fact* that the call succeeded with 200 is an oracle). Fix: Resolve URL hostname, reject any RFC1918 / loopback / link-local IP, including AWS IMDS (`169.254.169.254`), GCP metadata (`metadata.google.internal`), and Kubernetes service IPs. Reject `http://` (HTTPS only). Add the webhook into a denylist when delivery returns 200 from a denied target (defense-in-depth)."*

**Finding 2 — `BACKEND_SPOF.md` row #6, High** (verbatim from `docs/audits/BACKEND_SPOF.md:82`):

> *"Webhook retry resends empty body (`webhook_service.go:136-170`). After 1st failure, retry 2 & 3 POST an empty body; remote integration sees a malformed event and treats it as success (200) or rejects (4xx). Plus the signature header is still set for the full payload → downstream verification fails on retries. Combined with the fire-and-forget goroutine spawn (`webhook_service.go:113`), failed deliveries are silently mis-retried. Retry count = 3, exponential backoff. Severity: high. Hardening: Re-build `req.Body` each iteration (e.g., `req.Body = io.NopCloser(bytes.NewReader(payload))` per attempt) or use `req.GetBody`."*

**Finding 3 — Threat model widens** (verbatim from `docs/THREAT_MODEL.md` §1 row I, added in commit `4286d0a`):

> *"| I      | Webhook SSRF reaches IMDS / RFC1918 hosts    | None today; webhook URL is user-controlled with no allow/deny list. ADR for webhook-SSRF guard pending (Infisical Cand. 1 blocked-pending-SSRF-guard). | `backend/internal/service/webhook_service.go:136,158`            | **High** |"*

Today SSRF requires an attacker-registered webhook plus a manual trigger. **Wiring emission turns every state-mutating handler into the trigger.** `A01-F10` (webhook-reg IDOR) compounds; ADR-0005's `RequireProjectAccess` closes that vector and is a hard prerequisite. This is therefore not a "ship webhooks" decision but an **atomic** one: emission, SSRF guard, body buffer, signed-secret rotation, and bounded delivery budget land together or none ship. Any subset is worse than the status quo.

## Options considered

### Option A — Do nothing; keep emission stub off

- **How:** leave `Notify` uncalled; document webhook plumbing as "not production."
- **Pros:** zero new code; A10-F1 stays at "needs attacker step."
- **Cons:** product gap persists; A01-F10 + retry-body bug stay in the code surface; threat-model row I stays open at High.

### Option B — Ship emission first, fix SSRF in a follow-up

- **How:** wire `Notify`; track SSRF + retry-body as P2 follow-ups.
- **Pros:** fastest customer-visible win.
- **Cons:** **rejected outright** by Security Engineer veto path — amplifies an existing High-residual SSRF into a one-trigger-per-mutation primitive. Dossier §10 + SR pass make atomicity load-bearing.

### Option C — Atomic 7-part landing (this ADR)

- **How:** emission + SSRF guard + body buffer + HMAC + per-org signing-secret + budget + audit, all on by default; each part feature-flagged independently for rollback.
- **Pros:** closes A10-F1, SPOF #6, A01-F10 chain, and the Infisical parity gap in a single PR sequence.
- **Cons:** larger PR; reviewer covers crypto, networking, and a DB migration in one pass.

### Option D — Out-of-process delivery queue (Redis / SQS)

- **How:** API enqueues; separate worker delivers.
- **Pros:** survives API restart; goroutine pressure leaves the request path.
- **Cons:** new infra dep not in CLAUDE.md stack; SPOF row #11 (multi-replica state) is the stronger driver. Tracked as Phase B in `docs/FOLLOWUPS.md`.

## Decision

**Adopt Option C** as a single PR sequence with seven independently-revertible parts, all on by default. Option D (out-of-process queue) is a future ADR triggered by horizontal scale-out, not by this work.

**Part A — Wire emission from state-mutating handlers.** `secret_service.{Create,Update,Delete}`, `project_service.{Create,Update,Delete}`, `promotion_service.{Request,Approve,Execute}`, `apikey_service.{Create,Revoke}` each call `WebhookService.Notify(projectID, "<eventType>", data)` on success **after** the audit write. Payloads carry `{key, action, env, user, project_id}` only — never `value` / `EncryptedValue` (Invariant 3, dossier §12).

**Part B — URL allow-list + RFC-block-list at registration AND at delivery time (defense in depth vs DNS rebinding).** New `backend/internal/service/url_validator.go`. `Validate(url)` rejects: non-`https` scheme; hostname resolving to `127.0.0.0/8`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16` (link-local incl. AWS IMDS), `fc00::/7`, `fe80::/10`, `::1/128`, any `*.svc.cluster.local`, `metadata.google.internal`. At delivery time the URL is re-resolved and re-checked against the same denylist immediately before `client.Do` — defeats DNS rebinding (attacker authoritative DNS, TTL=0, swaps to `169.254.169.254` between registration and delivery). Redirects disabled (`CheckRedirect` returns `http.ErrUseLastResponse`). Per-org operator override: org-admin may allow specific private CIDRs — every override write emits `webhook.allowlist.modified`.

**Part C — Body buffering for retries.** Marshaled payload held in a `[]byte` for the delivery's lifetime; each retry builds a fresh `*http.Request` with `bytes.NewReader(payload)`. HMAC computed once over `payload`. Closes SPOF #6.

**Part D — HMAC-SHA256 over the buffered body.** Header `X-KeepSave-Signature: sha256=<hex>` (already at `webhook_service.go:146-149`). Receiver-side verification snippet (Go, Node, Python) added to `docs/research/EXTERNAL_REVIEW_PREP.md`; constant-time compare documented as a receiver requirement.

**Part E — Per-org signing-secret + rotation endpoint.** New table `webhook_secrets (id, org_id, ciphertext, kek_id, created_at, retired_at NULL)` — ciphertext wrapped under the org KEK per ADR-0001/0004. `webhooks.signing_secret_ref` FK. `POST /api/v1/orgs/:id/webhook-secret/rotate` mints a new secret, sets `retired_at` on previous, keeps both verifiable for a 24h overlap. Cadence default 90d (mirrors ADR-0008 §65). Plaintext `WebhookConfig.Secret` is removed.

**Part F — Per-webhook delivery budget.** Replace `go ws.deliver(...)` at `:113` with a bounded worker pool (default 8, `WEBHOOK_DELIVERY_CONCURRENCY`); per-org ceiling 5 in-flight (`WEBHOOK_PER_ORG_CONCURRENCY`); retries 3 with exponential backoff `1s/4s/16s` (was `1s/2s/4s` — extends tail to absorb downstream restarts). Overflow/exceeded → drop event + emit `webhook.budget_exceeded`. Combined with SPOF row #7, closes the unbounded-goroutine vector.

**Part G — Audit events.** New rows in `docs/AUDIT_LOG_COVERAGE.md`: `webhook.delivered`, `webhook.failed`, `webhook.budget_exceeded`, `webhook.url_rejected`, `webhook.allowlist.modified`, `webhook.secret.rotated`. Each carries `org_id`, `project_id`, `webhook_id`, `event_id`, `attempt`, `reason`.

## Rejection rationale

- **Option A (do nothing):** product parity gap persists; SR-pass §10 made this option moot — the threat-model row I + the dossier's `blocked-pending-SSRF-guard` verdict committed the project to landing the guard.
- **Option B (emission first, SSRF later):** explicit Security Engineer veto path — amplifies A10-F1 from "manual trigger" to "any audit event." Dossier §10 names Option B's failure mode verbatim.
- **Option D (out-of-process queue):** correct long-term shape, but requires Redis/SQS infra not in CLAUDE.md stack; not driven by this ADR's threat surface. Phase B follow-up.

## Consequences

- **Operational:** new env vars (`WEBHOOK_DELIVERY_CONCURRENCY`, `WEBHOOK_PER_ORG_CONCURRENCY`, `WEBHOOK_RETRY_BACKOFF_BASE`, `WEBHOOK_PRIVATE_CIDR_ALLOWLIST`); new RUNBOOK entry "webhook delivery degraded / budget exceeded"; metric `webhook_delivery_total{outcome=...}`.
- **Security:** trust boundary **narrows** for SSRF (residual High → Low; row I in `docs/THREAT_MODEL.md` §1 gets updated mitigation ref). Boundary **widens** by one entity (customer-controlled receiver) — already noted in dossier §9, bounded by Parts B/D/E.
- **Migration:** `webhook_secrets` additive; `webhooks.signing_secret_ref` backfilled by a one-shot job from existing `WebhookConfig.Secret`; plaintext `Secret` column dropped after a 30-day receiver-side cutover window.
- **Reversibility:** 7 parts revert independently (see Rollback). Migrations stay forward-only — no data loss on revert.

## Implementation plan

1. **`backend/internal/service/url_validator.go`** (new): `Validate(rawURL) error`, `ResolveAndCheck(host, orgAllowlist) error`. Table-driven test covers the 9 denial classes.
2. **`webhook_service.go:91-127`** (`Notify`): interface unchanged; internally submits to the Part F worker pool.
3. **`webhook_service.go:113`**: replace `go ws.deliver(...)` with `ws.workerPool.Submit(...)`; per-org semaphore acquired before submit; on overflow emit `webhook.budget_exceeded`.
4. **`webhook_service.go:129-149`**: marshal `payload` once; compute HMAC once over `payload`; set signature header once.
5. **`webhook_service.go:136`**: pre-flight `urlValidator.Validate(config.URL)`; on reject emit `webhook.url_rejected`.
6. **`webhook_service.go:153-170`** (retry loop): for each attempt build a fresh `req, _ := http.NewRequestWithContext(ctx, "POST", config.URL, bytes.NewReader(payload))`; re-set Content-Type/Event/Delivery/Signature; backoff `1s/4s/16s`; `http.Client.CheckRedirect` returns `ErrUseLastResponse`.
7. **`webhook_service.go:158`**: SSRF re-resolve immediately before `client.Do` — closes DNS rebinding.
8. **`WebhookSecretService` + endpoint:** `POST /api/v1/orgs/:id/webhook-secret/rotate` (`handlers_webhook.go`); reuses `crypto.Service` envelope encryption (ADR-0001/0004).
9. **Migration `backend/migrations/00NN_webhook_secrets.sql`**: create `webhook_secrets`; add `webhooks.signing_secret_ref` FK; backfill; mark `webhooks.secret` deprecated.
10. **State-mutating handler wiring** — emission happens **after** the audit-row write:
    - `secret_service.go:41,74,105,139,174` → `secret.<created|updated|deleted>`
    - `project_service.go:53-87` → `project.<created|updated|deleted>`
    - `promotion_service.go:181,215,272` → `promotion.<requested|approved|completed>`
    - `apikey_service.go:33-60` → `apikey.<created|revoked>`
11. **Tests:**
    - `tests/integration/webhook_ssrf_test.go`: IMDS rejected at registration; RFC1918 rejected at registration; DNS rebinding rejected at delivery; redirect to denied host rejected.
    - `tests/integration/webhook_retry_test.go`: receiver returns 503 twice then 200; assert all 3 attempts carry identical body bytes and the same HMAC.
    - Unit: payload struct contains no `value` / `secret` / `password` field (Invariant 3).
    - Unit: budget-exceeded emits `webhook.budget_exceeded` and drops the event.

## Rollback plan

Each part reverts independently; migrations stay forward-only, no data loss.

- **Part A (emission):** flag `WEBHOOK_EMISSION_ENABLED=false` — receivers stop seeing events; acceptable degradation.
- **Part B (URL validator):** flag `WEBHOOK_URL_VALIDATION_ENABLED=false` — **only safe if A is also off**, otherwise SSRF reopens; boot-time validator refuses `A=true && B=false`.
- **Part C (body buffering):** flag `WEBHOOK_RETRY_BODY_BUFFER=false` reverts to single-shot (no retry); strictly safer than the SPOF #6 status quo.
- **Part D (HMAC):** load-bearing; cannot be disabled. Algorithm change is a separate ADR.
- **Part E (signing-secret rotation):** rotation endpoint feature-flagged off; existing per-org secrets keep working.
- **Part F (delivery budget):** raise env defaults to effectively unlimited; no code change.
- **Part G (audit events):** taxonomy additions are append-only; consumers MUST ignore unknown types per `AUDIT_LOG_COVERAGE.md` policy.

## Open questions

1. **Per-org private-CIDR allowlist — operator UX.** Org-admin UI form with explicit acknowledgment, or JSON config under a feature flag? *Owner: PM + Security Engineer. Due: before GA.*
2. **Signing-secret rotation cadence — match JWT keys (90d per ADR-0008 §65)?** Receivers may not handle rotation gracefully; 180d could reduce integrator pain. *Owner: DevRel + Security Engineer. Due: 30 days.*
3. **Drop-on-budget audit destination.** Org per-event timeline (org-admin visibility), security audit log (operator visibility), or both? Both is conservative but doubles volume. *Owner: Tech Lead + Security Engineer. Due: 30 days.*

## References

- Code: `backend/internal/service/webhook_service.go:91-127` (`Notify`); `:113` (goroutine); `:136,:158` (SSRF surface); `:146-149` (HMAC); `:153-170` (retry body-reuse bug).
- New files: `internal/service/url_validator.go`; `internal/service/webhook_secret_service.go`; `migrations/00NN_webhook_secrets.sql`.
- `docs/THREAT_MODEL.md` §1 row I (commit `4286d0a`).
- `docs/audits/SECURITY_AUDIT_2026-05-15.md` A10-F1, A01-F10.
- `docs/audits/BACKEND_SPOF.md` rows #6, #7, #11.
- `docs/research/competitors/infisical.md` §10 Candidate 1; §12 Invariants 3, 7.
- Prior ADRs: 0001, 0004, 0005, 0008.
- `docs/AUDIT_LOG_COVERAGE.md` (new rows in this PR).
- RFC 6890 (private address space), RFC 4193 (ULA), RFC 2104 (HMAC).
