# KeepSave <-> Seidr end-to-end harness

This harness exercises the KeepSave HTTP surface that Seidr's
`KeepSaveSecretProvider` (Seidr v0.7.0+) depends on at runtime.

It brings up Postgres + KeepSave via `docker compose` and runs a small Go
tester that walks the same API calls Seidr makes to fetch a secret:

1. `POST /api/v1/auth/register` + `POST /api/v1/auth/login` (bootstrap
   a user and obtain a JWT)
2. `POST /api/v1/projects` (create the target project)
3. `POST /api/v1/projects/:id/secrets` (store a marker secret in `alpha`,
   server-side envelope-encrypted with the in-memory master key)
4. `POST /api/v1/api-keys` (mint a scoped, environment-locked `read` key
   — the same shape Seidr uses for agents)
5. `GET /api/v1/projects/:id/secrets?environment=alpha` with the scoped
   API key and confirm the plaintext matches what we stored

The tester exits `0` on success and non-zero on any failure. Compose is
configured so `--abort-on-container-exit --exit-code-from tester`
surfaces that exit code to CI.

## Run

```bash
# From the repo root
cd tests/e2e/seidr
docker compose up --build --abort-on-container-exit --exit-code-from tester
```

Expected tail of the output:

```
[e2e] keepsave ready after ...
[e2e] project: <uuid>
[e2e] api key created
[e2e] [OK] secret round-trip via scoped API key succeeded

[OK] KeepSave <-> Seidr-style E2E passed.
```

## What this test proves

- Envelope encryption round-trips correctly on a fresh DB: the value
  written via JWT auth comes back identical when fetched via a scoped
  API key. If the `MasterKeyProvider` swap (v1.1) broke the crypto wire,
  this test fails.
- JWT + API-key auth both route through the same secret-retrieval path,
  matching what Seidr's provider relies on.
- Environment scoping (`environment=alpha`) returns only the intended
  secret, so a Seidr agent locked to alpha can't accidentally read prod.
- `/readyz` reports the DB connection and KMS/env master-key resolution
  before the tester runs; boot failures in either fail fast.

## What this test does NOT prove

- An actual Seidr runtime is not started in this compose. The tester
  mimics `KeepSaveSecretProvider.Get`'s HTTP contract but does not
  exercise Seidr's own circuit breaker, TTL cache, or SLO ledger. A
  follow-up harness that boots a minimal Seidr container (pulling the
  real image from a registry this compose can reach) is tracked in
  `SECURITY_AUDIT.md` "Known follow-ups".
- Key rotation across a running Seidr is not tested here; that requires
  the Seidr runtime to observe a new value post-rotation, which only
  makes sense with a real Seidr container.
- Circuit-breaker behavior on forced 503s is also deferred to the
  Seidr-runtime follow-up.

## Local debugging

The KeepSave container publishes on host port `18080`, so you can hit
the API directly while the harness is up:

```bash
curl http://localhost:18080/healthz
```

To iterate on the tester without rebuilding the KeepSave image every
time, run KeepSave once and invoke the tester locally:

```bash
docker compose up -d db keepsave
cd tester
go run . # requires KEEPSAVE_URL=http://localhost:18080
```
