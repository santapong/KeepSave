# Test Pyramid Audit (QA 30-day)

Census of current test coverage by package, organized as a pyramid. Output is honest about what's strong and what's missing — and the bottom of the pyramid (repository, handler-integration) is dangerously thin.

This is QA / Test Engineer 30-day work item §1 from `docs/ROLES_30_60_90.md`.

---

## Per-package census (backend/internal/)

Format: package | code files | test files | `Test*` funcs | benchmarks | fuzz | note

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

## Shape: inverted pyramid

The conventional shape is **many unit tests, fewer integration, fewest E2E**:

```
      E2E       ▲
   integration  ▲▲▲
     unit       ▲▲▲▲▲▲▲▲
```

Current state is closer to:

```
      E2E       ▲                  (Seidr E2E harness exists; minimal coverage)
   integration  ▲                  (essentially none for state-mutating endpoints)
     unit       ▲▲▲▲▲▲▲▲▲▲▲▲▲     (heavy in crypto/auth/promotion-validation)
```

This means: a refactor that breaks the *plumbing* between layers (handler ↔ service ↔ repo) can land green and break production. The thinnest layer is doing the most blast-radius work (the API).

## Hot zones — where to add tests first

In order of leverage:

1. **Handler-level integration tests for state-mutating endpoints** (Secret / Project / APIKey / Promotion CRUD). Each handler gets:
   - Happy path (201/200, expected response shape).
   - Authorization (wrong project, wrong env, missing header, expired JWT, revoked API key).
   - Audit-log assertion (per `docs/AUDIT_LOG_COVERAGE.md`).
   - Error response sanitization (per `docs/ERROR_HANDLING_STANDARD.md`).

2. **Repository layer.** 20 files, 1 test file. Each repo gets a test against a Postgres test container (or SQLite in-process for fast feedback). Read/write/delete round-trip; constraint enforcement; concurrent-write behavior.

3. **Fuzz tests for crypto.** GCM nonce-length, key-length, ciphertext-truncation, AAD mismatch. `go test -fuzz` runs in CI for 30 seconds per fuzz target — finds the edge cases unit tests miss.

4. **Service layer for state-mutating services.** secret / project / apikey services need their own unit tests once `auditRepo` is plumbed (see audit-log spec).

## Coverage targets

Not vanity numbers — gates that mean something:

- **Critical packages** (`crypto`, `auth`, `service/promotion_service.go`): line coverage ≥ 90%, plus branch coverage ≥ 85% on the file level. Enforced in CI as a hard gate.
- **API handlers:** every state-mutating handler must have at least: happy-path test + 1 auth-failure test + 1 audit-assertion test. Enforced as a per-file presence check (no line-coverage gate — too easy to game with trivial tests).
- **Repository:** every method must have a round-trip test. Presence-checked.
- **Other packages:** no gate. Test or don't, but the package is on you.

## Anti-vanity rules

- **Don't test what the language guarantees.** No tests for getters, no tests for "constructor returns non-nil".
- **Don't test mocks.** A test whose only assertion is that the mock was called with X is a test of the test setup, not the code.
- **Don't add a test "to fix coverage."** If a line isn't worth a test, the line probably isn't worth keeping.

## Definition of done for a Phase-A test

- It can fail. (Tests that always pass don't catch regressions.)
- It fails for the right reason. (Test name and assertion message tell you *what* broke.)
- It runs in CI. (Locally-only tests rot.)
- It runs in under 100 ms (unit) or 5 s (integration). Slow tests get skipped.
- Negative paths are the explicit goal at this stage — we have happy paths. We need attackers.

## Process notes

- **Flaky tests are bugs.** See `tests/FLAKY.md` (separate doc) for the tracking process.
- **Coverage reports** uploaded as CI artifact already (`backend-coverage`). Add a dashboard reading from those — QA 60-day item.
- **Mutation testing** (e.g., `go-mutesting`) — Phase B. Not needed yet; first land the basic tests.

## References

- `docs/ROLES_30_60_90.md` §6 (QA action plan)
- `docs/AUDIT_LOG_COVERAGE.md` (test obligations for audit assertions)
- `docs/ERROR_HANDLING_STANDARD.md` (test obligations for error sanitization)
- `docs/THREAT_MODEL.md` v1.2.0 "Findings new" §3-§5 (the gaps these tests close)
