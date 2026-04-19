# KeepSave <-> Seidr Integration (Planned)

**Status:** design only. No code ships in v1.1.0. This document defines the
contract so that when Seidr is ready, both projects already agree on shape.

## Background

Seidr is a sibling project that needs to consume secrets from KeepSave. To
keep both codebases honest, Seidr will implement a `KeepSaveSecretProvider`
that calls KeepSave's existing `GET /api/v1/projects/:id/secrets` endpoint.

## Contract

### Environment variables (Seidr side)

| Var | Required | Purpose |
|---|---|---|
| `SEIDR_SECRET_PROVIDER` | Yes | Must be `keepsave` to activate this path |
| `KEEPSAVE_URL` | Yes | Base URL of the KeepSave API |
| `KEEPSAVE_API_KEY` | Yes | Scoped API key with `read` on the target project |
| `KEEPSAVE_PROJECT_ID` | Yes | UUID of the project to read from |
| `KEEPSAVE_ENV` | Yes | `alpha`, `uat`, or `prod` |
| `KEEPSAVE_CACHE_TTL_SEC` | No (default 60) | Local cache TTL per key |
| `KEEPSAVE_CB_THRESHOLD` | No (default 5) | Consecutive 503s before opening breaker |

### Behavior required of `KeepSaveSecretProvider`

1. **Fetch**: `get(name)` calls the KeepSave API and returns decrypted value.
2. **Cache**: successful reads cached for `KEEPSAVE_CACHE_TTL_SEC` seconds.
   Cache is in-memory only; no disk writes.
3. **Refetch on rotation**: when KeepSave emits a webhook
   `secret.rotated` (event bus), Seidr invalidates the cache entry and
   refetches on next access. Webhook listener is optional; TTL alone also
   works.
4. **Circuit breaker**: after `KEEPSAVE_CB_THRESHOLD` consecutive 503/5xx
   responses, open the breaker for 30 seconds; calls fail fast with
   `ErrKeepSaveUnavailable` during that window.
5. **Retry**: on network error or 429, retry with exponential backoff
   (200ms, 400ms, 800ms) up to 3 attempts.

### Observability

Seidr must emit these metrics:

- `seidr_keepsave_fetch_total{result}` counter, `result` in `hit`, `miss`, `error`
- `seidr_keepsave_fetch_latency_seconds` histogram
- `seidr_keepsave_breaker_state{state}` gauge

## Verification plan (future)

When Seidr lands its implementation, add `tests/e2e/seidr/` to this repo
with a docker-compose stack that:

1. Starts KeepSave + Postgres.
2. Creates a project, API key, and two secrets.
3. Starts a minimal Seidr runtime configured to use the KeepSave provider.
4. Asserts the provider returns the expected values.
5. Rotates the master key; asserts Seidr refetches within TTL.
6. Forces a 503 from KeepSave; asserts the breaker opens.

## Why docs-only right now

The Seidr project does not yet exist in this organization, so the harness
would test a stub against a stub. Shipping the contract lets both teams
move in parallel without picking up accidental coupling.
