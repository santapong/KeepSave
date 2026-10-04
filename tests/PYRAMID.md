# KeepSave test pyramid and evidence

Source audit: October 4, 2026. Current implementation is the unreleased
harness-neutral source candidate; see the [documentation hub](../docs/README.md),
[architecture](../docs/ARCHITECTURE.md), [testing chapter](../docs/system/11-testing.md)
and [acceptance ledger](../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).
Test source is not proof that a particular revision executed it successfully.

## Current layers

| Layer | Purpose and representative source |
|---|---|
| Pure/unit | Crypto/auth/config, policy/schema validation, native package parser/renderer and safe DOM behavior. |
| Real database/router | `backend/internal/api/platform_*_test.go`, identity/session/social, vault/history/recovery, OAuth, runs and audited transaction/concurrency failures on PostgreSQL. |
| Process/HTTP | `scripts/test-platform-two-api.py`: bounded two-API revocation and vault journey. |
| Compatibility clients | Go/Node/Python SDK session fixtures, widget tests/build and existing CLI contracts. |
| Retained E2E | Robot and Seidr-style source fixtures; stale current healthchecks/project response helpers must be repaired and rerun. They are not current complete platform gates. |
| External qualification | Real Google/GitHub/SMTP, exact native clients, GitHub App and separate-host isolation; separately authorized and still pending. |

The old assertion that no handler integration, repository or fuzz tests exist is
obsolete. Current source includes actual-router audit/auth contracts, real
PostgreSQL fixtures and bounded fuzz targets. New transaction/concurrency/epoch/
recovery guarantees remain PostgreSQL-only; SQLite and MySQL have separate bounded
legacy evidence rather than equivalent platform support.

## Commands and implemented gates

From the canonical repository root (`/mnt/data/company/apps/KeepSave`):

```bash
bash scripts/test-platform-postgres.sh
bash scripts/test-legacy-mysql.sh
node scripts/generate-core-api-types.mjs --check
```

The database scripts create only unique disposable resources, ignore operator
DATABASE_URL and do not publish a database port. Read their prerequisites before
running; do not rerun broad suites merely for this documentation edit. Backend CI
retains race/shuffle/vet and bounded fuzz; frontend runs tests, configured lint and
application/widget builds. The current workflow coverage ratchet is **crypto 85%
/auth 90%**, not a universal 90%/85%-branch gate. No complete endpoint-presence or
per-repository-method gate is inferred from policy prose. Exact-revision remote
CI must be recorded separately; October 3 main/develop runs were checked failed
on October 4.

Acceptance prioritizes meaningful success/denial/rollback/race tests over inflated
counts. Required audit assertions live in the owning Go tests; HTTP/E2E supplements
them. Tests that only mirror implementation or assert a mocked call do not replace
stored-authority tests. Real external effects stay out of synthetic CI fixtures.
See [negative-authority coverage](NEGATIVE_AUTH_PLAN.md) and [flaky policy](FLAKY.md).

## Dated execution and historical census

The October 2 ledger records the final ten-package PostgreSQL race gate, affected
recovery follow-up, frontend 34 files/161 tests plus later nine affected tests and
14 top-level/35 native packaging cases. These are dated receipts, not a fresh suite
run or real harness interoperability. Do not claim a single final 162-test run.

The following original Phase-A static census is retained for provenance. Its
collection date was not recorded in this file; it predates the current candidate
and its “no tests”/percentage notes are superseded. Static file/function counts
are not executed coverage measurements.

| Package         | Code | Tests | Funcs | Bench | Fuzz | Note                                                                       |
|-----------------|-----:|------:|------:|------:|-----:|----------------------------------------------------------------------------|
| **api**         |   30 |     3 |    12 |     3 |    0 | health, rate-limit, highload only — **NO handler-level integration tests** |
| **auth**        |    4 |     2 |     8 |     0 |    0 | password / JWT unit-tested; no middleware integration tests                 |
| **config**      |    1 |     1 |     3 |     0 |    0 | config parsing covered                                                      |
| **crypto**      |    6 |     3 |     9 |     7 |    0 | strongest coverage; benchmarks present; **no fuzz**                         |
| **events**      |    1 |     1 |     5 |     0 |    0 | EventBus covered                                                            |
| **logging**     |    2 |     1 |     5 |     0 |    0 | covered                                                                     |
| **metrics**     |    2 |     1 |     8 |     0 |    0 | renderer covered (corrected during 30-day plan)                              |
| **models**      |    7 |     0 |     0 |     0 |    0 | no behavior to test (data types only)                                       |
| **plugins**     |    1 |     1 |     5 |     0 |    0 | loader covered                                                              |
| **repository**  |   20 |     1 |     4 |     0 |    0 | **95% UNTESTED** — only `audit_repo_test.go` exists                          |
| **service**     |   23 |     6 |    23 |     0 |    0 | partial — promotion validation covered; secret/project/apikey services largely untested |
| **tracing**     |    1 |     1 |     6 |     0 |    0 | covered                                                                     |
